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

type PriceService struct {
	db    *db.DB
	reg   *providers.Registry
	fetch *Fetcher
}

func NewPriceService(d *db.DB, reg *providers.Registry, f *Fetcher) *PriceService {
	return &PriceService{db: d, reg: reg, fetch: f}
}

func msSince(t time.Time) *int64 {
	d := time.Since(t).Milliseconds()
	return &d
}

// GetPrice tek provider'dan fiyat.
func (s *PriceService) GetPrice(ctx context.Context, providerName string, q types.PriceQuery, fresh bool) (*provider.PriceResult, error) {
	p, err := s.reg.Get(providerName)
	if err != nil {
		return nil, err
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
	r, err := s.fetch.Price(ctx, providerName, p, m, q, fresh)
	if err != nil {
		return nil, err
	}
	r.DurationMs = msSince(start)
	return r, nil
}

type BulkResult struct {
	Provider   string                 `json:"provider"`
	DurationMs int64                  `json:"durationMs"`
	Results    []provider.PriceResult `json:"results"`
	NotFound   []string               `json:"notFound"`
}

// GetBulkPrices tek provider'dan toplu fiyat (native bulk; cache'siz).
func (s *PriceService) GetBulkPrices(ctx context.Context, providerName string, q types.BulkPriceQuery) (*BulkResult, error) {
	p, err := s.reg.Get(providerName)
	if err != nil {
		return nil, err
	}
	ms, err := s.db.ActiveMappingsForHotels(ctx, providerName, q.HotelIDs)
	if err != nil {
		return nil, err
	}
	if len(ms) == 0 {
		return nil, apperr.New(apperr.Internal, "Hicbir otel eslestirmesi bulunamadi")
	}
	start := time.Now()
	results, err := s.fetch.Bulk(ctx, providerName, p, ms, q)
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, m := range ms {
		have[m.InternalHotelID] = true
	}
	notFound := []string{}
	for _, id := range q.HotelIDs {
		if !have[id] {
			notFound = append(notFound, id)
		}
	}
	return &BulkResult{Provider: providerName, DurationMs: time.Since(start).Milliseconds(), Results: results, NotFound: notFound}, nil
}

type ProviderPriceResult struct {
	Provider string `json:"provider"`
	provider.PriceResult
}

type ProviderError struct {
	Provider string `json:"provider"`
	Error    string `json:"error"`
}

type MultiPriceResult struct {
	HotelID  string                `json:"hotelId"`
	CheckIn  string                `json:"checkIn"`
	CheckOut string                `json:"checkOut"`
	Results  []ProviderPriceResult `json:"results"`
	Errors   []ProviderError       `json:"errors,omitempty"`
}

// MappingsFor bir otelin aktif eslestirmelerini (istenirse provider filtresiyle) doner.
func (s *PriceService) MappingsFor(ctx context.Context, hotelID string, providerNames []string) ([]provider.Mapping, error) {
	ms, err := s.db.ActiveMappingsForHotel(ctx, hotelID, providerNames)
	if err != nil {
		return nil, err
	}
	if len(ms) == 0 {
		return nil, apperr.NotFoundf("Otel eslestirmesi bulunamadi: %s", hotelID)
	}
	return ms, nil
}

// PriceForMapping tek bir eslestirme icin fiyat (job calistirici da kullanir).
func (s *PriceService) PriceForMapping(ctx context.Context, m *provider.Mapping, q types.PriceQuery, fresh bool) (*ProviderPriceResult, error) {
	p, err := s.reg.Get(m.Provider)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	r, err := s.fetch.Price(ctx, m.Provider, p, m, q, fresh)
	if err != nil {
		return nil, err
	}
	r.DurationMs = msSince(start)
	return &ProviderPriceResult{Provider: m.Provider, PriceResult: *r}, nil
}

// GetMultiProviderPrices bir otelin tum (veya secili) provider'larindan paralel fiyat ceker.
func (s *PriceService) GetMultiProviderPrices(ctx context.Context, q types.PriceQuery, providerNames []string, fresh bool) (*MultiPriceResult, error) {
	ms, err := s.MappingsFor(ctx, q.HotelID, providerNames)
	if err != nil {
		return nil, err
	}
	type slot struct {
		res *ProviderPriceResult
		err *ProviderError
	}
	slots := make([]slot, len(ms))
	var wg sync.WaitGroup
	for i := range ms {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := s.PriceForMapping(ctx, &ms[i], q, fresh)
			if err != nil {
				slots[i].err = &ProviderError{Provider: ms[i].Provider, Error: err.Error()}
				return
			}
			slots[i].res = r
		}(i)
	}
	wg.Wait()

	out := &MultiPriceResult{HotelID: q.HotelID, CheckIn: q.CheckIn, CheckOut: q.CheckOut, Results: []ProviderPriceResult{}}
	for _, sl := range slots {
		if sl.res != nil {
			out.Results = append(out.Results, *sl.res)
		} else if sl.err != nil {
			out.Errors = append(out.Errors, *sl.err)
		}
	}
	return out, nil
}
