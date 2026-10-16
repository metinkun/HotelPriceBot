package services

import (
	"context"
	"time"

	"pricebot/internal/cache"
	"pricebot/internal/limiter"
	"pricebot/internal/provider"
	"pricebot/internal/types"
)

// UsageHook bir siteye gercek istek atildiginda cagrilir (kredi dusumu). provider adi + kredi.
type UsageHook func(ctx context.Context, providerName string, credits int)

// Fetcher provider cagrilarini cache, tek ucus, eszamanlilik siniri ve devre kesiciden gecirir.
type Fetcher struct {
	cache *cache.Cache
	lim   *limiter.Limiter
	usage UsageHook
}

func NewFetcher(c *cache.Cache, l *limiter.Limiter) *Fetcher {
	return &Fetcher{cache: c, lim: l}
}

// SetUsageHook kredi dusumu icin kanca (coklu musteri fazinda baglanir).
func (f *Fetcher) SetUsageHook(h UsageHook) { f.usage = h }

// Upstream cagri icin ilk istemcinin iptalinden bagimsiz, ama sinirli bir context.
func upstreamCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 4*time.Minute)
}

// Price tek provider icin fiyat: cache -> tek ucus -> limiter/devre kesici -> provider.
func (f *Fetcher) Price(ctx context.Context, name string, p provider.Provider, m *provider.Mapping, q types.PriceQuery, fresh bool) (*provider.PriceResult, error) {
	key := cache.PriceKey(name, m.ProviderHotelID, q)
	if !fresh {
		var cached provider.PriceResult
		if ok, _ := f.cache.GetJSON(ctx, key, &cached); ok {
			cached.Cached = true
			cached.InternalHotelID = m.InternalHotelID
			if m.HotelName != nil {
				cached.HotelName = m.HotelName
			}
			return &cached, nil
		}
	}
	v, err, _ := f.cache.Do(key, func() (any, error) {
		uctx, cancel := upstreamCtx(ctx)
		defer cancel()
		release, err := f.lim.Acquire(uctx, name)
		if err != nil {
			return nil, err
		}
		r, err := p.GetPrice(uctx, m, q)
		release(err, err == nil && r != nil && r.Found)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		r.FetchedAt = &now
		r.Cached = false
		_ = f.cache.SetJSON(uctx, key, r, 0)
		if f.usage != nil {
			f.usage(ctx, name, 1)
		}
		return r, nil
	})
	if err != nil {
		return nil, err
	}
	out := *v.(*provider.PriceResult) // kopya: paylasilan sonuc degistirilmesin
	out.InternalHotelID = m.InternalHotelID
	return &out, nil
}

// Package tek provider icin paket fiyatlari (ayni akis).
func (f *Fetcher) Package(ctx context.Context, name string, pp provider.PackageProvider, m *provider.Mapping, q types.PackageQuery, fresh bool) (*provider.PackageResult, error) {
	key := cache.PackageKey(name, m.ProviderHotelID, q)
	if !fresh {
		var cached provider.PackageResult
		if ok, _ := f.cache.GetJSON(ctx, key, &cached); ok {
			cached.Cached = true
			cached.InternalHotelID = m.InternalHotelID
			return &cached, nil
		}
	}
	v, err, _ := f.cache.Do(key, func() (any, error) {
		uctx, cancel := upstreamCtx(ctx)
		defer cancel()
		release, err := f.lim.Acquire(uctx, name)
		if err != nil {
			return nil, err
		}
		r, err := pp.GetPackagePrices(uctx, m, q)
		release(err, err == nil && r != nil && r.Available)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		r.FetchedAt = &now
		r.Cached = false
		_ = f.cache.SetJSON(uctx, key, r, 0)
		if f.usage != nil {
			f.usage(ctx, name, 1)
		}
		return r, nil
	})
	if err != nil {
		return nil, err
	}
	out := *v.(*provider.PackageResult)
	out.InternalHotelID = m.InternalHotelID
	return &out, nil
}

// Bulk native toplu sorgu: sadece limiter/devre kesici (cache yok).
func (f *Fetcher) Bulk(ctx context.Context, name string, p provider.Provider, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	release, err := f.lim.Acquire(ctx, name)
	if err != nil {
		return nil, err
	}
	results, err := p.GetBulkPrices(ctx, ms, q)
	found := false
	for _, r := range results {
		if r.Found {
			found = true
			break
		}
	}
	release(err, found)
	if err == nil && f.usage != nil {
		f.usage(ctx, name, len(ms))
	}
	return results, err
}
