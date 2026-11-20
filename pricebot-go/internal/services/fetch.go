package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
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
	gate  func(name string) bool // site genel olarak acik mi (nil = hepsi acik)
}

// SetSiteGate admin'in kapattigi siteleri (bakimda) atlamak icin kanca.
func (f *Fetcher) SetSiteGate(g func(name string) bool) { f.gate = g }

var ErrSiteDisabled = errors.New("Site gecici olarak kapali (bakimda)")

func (f *Fetcher) siteOpen(name string) bool { return f.gate == nil || f.gate(name) }

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
	if !f.siteOpen(name) {
		return nil, ErrSiteDisabled
	}
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
		r, err := safePrice(uctx, p, m, q)
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
	if !f.siteOpen(name) {
		return nil, ErrSiteDisabled
	}
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
		r, err := safePackage(uctx, pp, m, q)
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

// safePrice / safePackage provider paniklerini hataya cevirir.
// isTransient gecici ag hatasi: zaman asimi, baglanti kopmasi, 5xx/429. Bir kez yeniden denemeye deger.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"deadline exceeded", "timeout", "i/o timeout", "connection reset", "eof", "status code 5", "status code 429", "tls handshake"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// retryTransient fn'i calistirir; gecici hatada ve butce (ctx) yeterliyse kisa bekleyip BIR kez daha dener.
func retryTransient[T any](ctx context.Context, name string, fn func() (T, error)) (T, error) {
	r, err := fn()
	if err == nil || !isTransient(err) || ctx.Err() != nil {
		return r, err
	}
	if dl, ok := ctx.Deadline(); ok && time.Until(dl) < 25*time.Second {
		return r, err
	}
	slog.Warn("provider gecici hata, yeniden deneniyor", "provider", name, "err", err.Error())
	select {
	case <-time.After(1500 * time.Millisecond):
	case <-ctx.Done():
		return r, err
	}
	r2, err2 := fn()
	if err2 != nil {
		return r2, fmt.Errorf("%w (yeniden deneme de basarisiz)", err2)
	}
	return r2, nil
}

func safePrice(ctx context.Context, p provider.Provider, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	return retryTransient(ctx, p.Name(), func() (r *provider.PriceResult, err error) {
		defer func() {
			if rec := recover(); rec != nil {
				r, err = nil, fmt.Errorf("provider panic: %v", rec)
			}
		}()
		return p.GetPrice(ctx, m, q)
	})
}

func safePackage(ctx context.Context, pp provider.PackageProvider, m *provider.Mapping, q types.PackageQuery) (*provider.PackageResult, error) {
	name := ""
	if p, ok := pp.(provider.Provider); ok {
		name = p.Name()
	}
	return retryTransient(ctx, name, func() (r *provider.PackageResult, err error) {
		defer func() {
			if rec := recover(); rec != nil {
				r, err = nil, fmt.Errorf("provider panic: %v", rec)
			}
		}()
		return pp.GetPackagePrices(ctx, m, q)
	})
}
