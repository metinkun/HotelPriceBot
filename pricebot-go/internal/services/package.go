package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/db"
	"pricebot/internal/provider"
	"pricebot/internal/providers"
	"pricebot/internal/types"
)

type PackageService struct {
	db    *db.DB
	reg   *providers.Registry
	fetch *Fetcher
}

func NewPackageService(d *db.DB, reg *providers.Registry, f *Fetcher) *PackageService {
	return &PackageService{db: d, reg: reg, fetch: f}
}

// GetPackagePrices tek provider'dan paket fiyatlari.
func (s *PackageService) GetPackagePrices(ctx context.Context, providerName string, q types.PackageQuery, fresh bool) (*provider.PackageResult, error) {
	p, err := s.reg.Get(providerName)
	if err != nil {
		return nil, err
	}
	pp, ok := providers.PackageProviderOf(p)
	if !ok {
		return nil, apperr.BadRequestf("Provider '%s' paket sorgusunu desteklemiyor", providerName)
	}
	m, err := s.db.GetMapping(ctx, providerName, q.HotelID)
	if errors.Is(err, db.ErrNotFound) {
		return nil, apperr.NotFoundf("Otel eslestirmesi bulunamadi: %s (%s)", q.HotelID, providerName)
	}
	if err != nil {
		return nil, err
	}
	if !m.IsActive {
		return nil, apperr.New(apperr.Internal, "Otel eslestirmesi pasif durumda: %s", q.HotelID)
	}
	start := time.Now()
	r, err := s.fetch.Package(ctx, providerName, pp, m, q, fresh)
	if err != nil {
		return nil, err
	}
	r.DurationMs = msSince(start)
	return r, nil
}

type ProviderPackageResult struct {
	Provider string `json:"provider"`
	provider.PackageResult
}

type MultiPackageResult struct {
	HotelID      string                  `json:"hotelId"`
	Results      []ProviderPackageResult `json:"results"`
	Errors       []ProviderError         `json:"errors,omitempty"`
	NotMapped    []string                `json:"notMapped,omitempty"`
	NotSupported []string                `json:"notSupported,omitempty"`
}

// PackageTargets istenen provider'lari paket destekleyen / desteklemeyen olarak ayirir.
func (s *PackageService) PackageTargets(providerNames []string) (pkg []provider.Provider, notSupported []string, err error) {
	var targets []provider.Provider
	if len(providerNames) > 0 {
		for _, n := range providerNames {
			p, err := s.reg.Get(n)
			if err != nil {
				return nil, nil, err
			}
			targets = append(targets, p)
		}
	} else {
		targets = s.reg.All()
	}
	for _, p := range targets {
		if _, ok := providers.PackageProviderOf(p); ok {
			pkg = append(pkg, p)
		} else {
			notSupported = append(notSupported, p.Name())
		}
	}
	return pkg, notSupported, nil
}

// PackageForMapping tek eslestirme icin paket (job calistirici da kullanir).
func (s *PackageService) PackageForMapping(ctx context.Context, m *provider.Mapping, q types.PackageQuery, fresh bool) (*ProviderPackageResult, error) {
	p, err := s.reg.Get(m.Provider)
	if err != nil {
		return nil, err
	}
	pp, ok := providers.PackageProviderOf(p)
	if !ok {
		return nil, apperr.BadRequestf("Provider '%s' paket sorgusunu desteklemiyor", m.Provider)
	}
	start := time.Now()
	r, err := s.fetch.Package(ctx, m.Provider, pp, m, q, fresh)
	if err != nil {
		return nil, err
	}
	r.DurationMs = msSince(start)
	return &ProviderPackageResult{Provider: m.Provider, PackageResult: *r}, nil
}

// GetMultiProviderPackagePrices birden fazla (veya tum) provider'dan paket fiyatlarini paralel sorgular.
func (s *PackageService) GetMultiProviderPackagePrices(ctx context.Context, providerNames []string, q types.PackageQuery, fresh bool) (*MultiPackageResult, error) {
	pkgProviders, notSupported, err := s.PackageTargets(providerNames)
	if err != nil {
		return nil, err
	}
	out := &MultiPackageResult{HotelID: q.HotelID, Results: []ProviderPackageResult{}, NotSupported: notSupported}
	if len(pkgProviders) == 0 {
		return out, nil
	}
	names := make([]string, 0, len(pkgProviders))
	for _, p := range pkgProviders {
		names = append(names, p.Name())
	}
	ms, err := s.db.ActiveMappingsForHotel(ctx, q.HotelID, names)
	if err != nil {
		return nil, err
	}

	type slot struct {
		res *ProviderPackageResult
		err *ProviderError
	}
	slots := make([]slot, len(ms))
	var wg sync.WaitGroup
	for i := range ms {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := s.PackageForMapping(ctx, &ms[i], q, fresh)
			if err != nil {
				slots[i].err = &ProviderError{Provider: ms[i].Provider, Error: err.Error()}
				return
			}
			slots[i].res = r
		}(i)
	}
	wg.Wait()

	queried := map[string]bool{}
	for _, m := range ms {
		queried[m.Provider] = true
	}
	for _, sl := range slots {
		if sl.res != nil {
			out.Results = append(out.Results, *sl.res)
		} else if sl.err != nil {
			out.Errors = append(out.Errors, *sl.err)
		}
	}
	for _, p := range pkgProviders {
		if !queried[p.Name()] {
			out.NotMapped = append(out.NotMapped, p.Name())
		}
	}
	return out, nil
}

// MappingsForPackages paket destekleyen provider'lar icin otelin aktif eslestirmelerini doner;
// desteklemeyen ve eslestirmesi olmayan provider adlarini ayrica verir.
func (s *PackageService) MappingsForPackages(ctx context.Context, hotelID string, providerNames []string) (ms []provider.Mapping, notSupported, notMapped []string, err error) {
	pkgProviders, notSupported, err := s.PackageTargets(providerNames)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(pkgProviders) == 0 {
		return []provider.Mapping{}, notSupported, nil, nil
	}
	names := make([]string, 0, len(pkgProviders))
	for _, p := range pkgProviders {
		names = append(names, p.Name())
	}
	ms, err = s.db.ActiveMappingsForHotel(ctx, hotelID, names)
	if err != nil {
		return nil, nil, nil, err
	}
	have := map[string]bool{}
	for _, m := range ms {
		have[m.Provider] = true
	}
	for _, n := range names {
		if !have[n] {
			notMapped = append(notMapped, n)
		}
	}
	return ms, notSupported, notMapped, nil
}
