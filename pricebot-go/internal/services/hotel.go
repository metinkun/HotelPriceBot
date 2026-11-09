// Package services is mantigi: otel CRUD, fiyat ve paket sorgulari.
package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"sync"

	"pricebot/internal/apperr"
	"pricebot/internal/db"
	"pricebot/internal/model"
	"pricebot/internal/providers"
)

type HotelService struct {
	db  *db.DB
	reg *providers.Registry
}

func NewHotelService(d *db.DB, reg *providers.Registry) *HotelService {
	return &HotelService{db: d, reg: reg}
}

// generateHotelID otomatik internalHotelId: htl_<8 hex>
func generateHotelID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "htl_" + hex.EncodeToString(b)
}

type AddHotelInput struct {
	URLs            map[string]string `json:"urls"`
	InternalHotelID string            `json:"internalHotelId"`
	HotelName       string            `json:"hotelName"`
}

type ResolveError struct {
	Provider string `json:"provider"`
	Error    string `json:"error"`
}

type resolveResult struct {
	provider        string
	ok              bool
	providerHotelID string
	hotelName       *string
	metadata        map[string]any
	err             string
}

type HotelProviderView struct {
	Provider        string         `json:"provider"`
	ProviderHotelID string         `json:"providerHotelId"`
	HotelName       *string        `json:"hotelName,omitempty"`
	SourceURL       *string        `json:"sourceUrl"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	IsActive        bool           `json:"isActive"`
}

type HotelView struct {
	InternalHotelID string              `json:"internalHotelId"`
	HotelName       *string             `json:"hotelName"`
	Providers       []HotelProviderView `json:"providers"`
}

type AddHotelResult struct {
	HotelView
	Errors []ResolveError `json:"errors"`
}

func sourceURLOf(m *model.HotelMapping) *string {
	if s := m.MetaString("sourceUrl"); s != "" {
		return &s
	}
	return nil
}

func firstName(ms []model.HotelMapping) *string {
	for _, m := range ms {
		if m.HotelName != nil && *m.HotelName != "" {
			return m.HotelName
		}
	}
	return nil
}

// AddFromURLs verilen URL'leri paralel cozer ve tek internalHotelId altinda mapping olusturur/gunceller.
func (s *HotelService) AddFromURLs(ctx context.Context, in AddHotelInput) (*AddHotelResult, error) {
	names := []string{}
	for k, v := range in.URLs {
		if strings.TrimSpace(v) != "" {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, apperr.BadRequestf("En az bir site URL'i gerekli. Desteklenen provider'lar: %s", strings.Join(s.reg.Names(), ", "))
	}
	for _, n := range names {
		p, err := s.reg.Get(n)
		if err != nil {
			return nil, err
		}
		if _, ok := providers.ResolverOf(p); !ok {
			return nil, apperr.BadRequestf("Provider '%s' URL cozumlemeyi desteklemiyor", n)
		}
	}

	internalID := strings.TrimSpace(in.InternalHotelID)
	if internalID == "" {
		internalID = generateHotelID()
	}

	results := make([]resolveResult, len(names))
	var wg sync.WaitGroup
	for i, n := range names {
		wg.Add(1)
		go func(i int, n string) {
			defer wg.Done()
			p, _ := s.reg.Get(n)
			rr, _ := providers.ResolverOf(p)
			r, err := rr.ResolveFromURL(ctx, strings.TrimSpace(in.URLs[n]))
			if err != nil {
				results[i] = resolveResult{provider: n, err: err.Error()}
				return
			}
			results[i] = resolveResult{provider: n, ok: true, providerHotelID: r.ProviderHotelID, hotelName: r.HotelName, metadata: r.Metadata}
		}(i, n)
	}
	wg.Wait()

	errs := []ResolveError{}
	okCount := 0
	for _, r := range results {
		if r.ok {
			okCount++
		} else {
			errs = append(errs, ResolveError{Provider: r.provider, Error: r.err})
		}
	}
	if okCount == 0 {
		details := make([]map[string]any, 0, len(results))
		for _, r := range results {
			details = append(details, map[string]any{"provider": r.provider, "ok": false, "error": r.err})
		}
		return nil, apperr.BadRequestf("Hicbir URL cozumlenemedi").WithDetails(details)
	}

	for _, r := range results {
		if !r.ok {
			continue
		}
		var name *string
		if strings.TrimSpace(in.HotelName) != "" {
			n := in.HotelName
			name = &n
		} else if r.hotelName != nil && *r.hotelName != "" {
			name = r.hotelName
		}
		if _, err := s.db.UpsertMapping(ctx, db.MappingUpsert{
			Provider: r.provider, InternalHotelID: internalID, ProviderHotelID: r.providerHotelID,
			HotelName: name, Metadata: r.metadata, IsActive: true,
		}); err != nil {
			return nil, err
		}
	}

	hotel, err := s.Get(ctx, internalID)
	if err != nil {
		return nil, err
	}
	s.db.EnsureHotelMeta(ctx, internalID, hotel.HotelName)
	return &AddHotelResult{HotelView: *hotel, Errors: errs}, nil
}

// Get bir internalHotelId'nin tum provider eslestirmelerini gruplu doner.
func (s *HotelService) Get(ctx context.Context, internalID string) (*HotelView, error) {
	ms, err := s.db.ListMappings(ctx, db.MappingFilter{InternalHotelID: internalID, OrderBy: "provider ASC"})
	if err != nil {
		return nil, err
	}
	if len(ms) == 0 {
		return nil, apperr.NotFoundf("Otel bulunamadi: %s", internalID)
	}
	view := &HotelView{InternalHotelID: internalID, HotelName: firstName(ms), Providers: []HotelProviderView{}}
	for i := range ms {
		m := &ms[i]
		view.Providers = append(view.Providers, HotelProviderView{
			Provider: m.Provider, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
			SourceURL: sourceURLOf(m), Metadata: m.Metadata, IsActive: m.IsActive,
		})
	}
	return view, nil
}

// List sistemdeki tum otelleri (internalHotelId bazinda gruplu) listeler.
func (s *HotelService) List(ctx context.Context, search string, active *bool) ([]HotelView, error) {
	ms, err := s.db.ListMappings(ctx, db.MappingFilter{Search: search, Active: active})
	if err != nil {
		return nil, err
	}
	order := []string{}
	groups := map[string][]model.HotelMapping{}
	for _, m := range ms {
		if _, ok := groups[m.InternalHotelID]; !ok {
			order = append(order, m.InternalHotelID)
		}
		groups[m.InternalHotelID] = append(groups[m.InternalHotelID], m)
	}
	out := make([]HotelView, 0, len(order))
	for _, id := range order {
		g := groups[id]
		v := HotelView{InternalHotelID: id, HotelName: firstName(g), Providers: []HotelProviderView{}}
		for i := range g {
			m := &g[i]
			v.Providers = append(v.Providers, HotelProviderView{
				Provider: m.Provider, ProviderHotelID: m.ProviderHotelID, SourceURL: sourceURLOf(m), IsActive: m.IsActive,
			})
		}
		out = append(out, v)
	}
	return out, nil
}

// Update otelin URL(ler)ini yeniden cozer ve/veya adini gunceller.
func (s *HotelService) Update(ctx context.Context, internalID string, urls map[string]string, hotelName *string) (*HotelView, error) {
	existing, err := s.db.ListMappings(ctx, db.MappingFilter{InternalHotelID: internalID})
	if err != nil {
		return nil, err
	}
	if len(existing) == 0 {
		return nil, apperr.NotFoundf("Otel bulunamadi: %s", internalID)
	}
	if len(urls) > 0 {
		name := ""
		if hotelName != nil {
			name = *hotelName
		}
		if _, err := s.AddFromURLs(ctx, AddHotelInput{URLs: urls, InternalHotelID: internalID, HotelName: name}); err != nil {
			return nil, err
		}
	} else if hotelName != nil {
		if err := s.db.UpdateHotelName(ctx, internalID, hotelName); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, internalID)
}

type DeleteResult struct {
	InternalHotelID string `json:"internalHotelId"`
	DeletedMappings int64  `json:"deletedMappings"`
}

// Delete otelin tum provider eslestirmelerini siler.
func (s *HotelService) Delete(ctx context.Context, internalID string) (*DeleteResult, error) {
	n, err := s.db.DeleteMappingsForHotel(ctx, internalID)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, apperr.NotFoundf("Otel bulunamadi: %s", internalID)
	}
	return &DeleteResult{InternalHotelID: internalID, DeletedMappings: n}, nil
}
