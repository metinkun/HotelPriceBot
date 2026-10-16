// Package canary belirli bir "kanarya" oteli periyodik olarak tum provider'lardan sorgular;
// bir sitenin yapisi degisince (found=false / hata) erken uyari verir.
package canary

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"pricebot/internal/cache"
	"pricebot/internal/config"
	"pricebot/internal/services"
	"pricebot/internal/types"
)

type ProviderCheck struct {
	Provider   string   `json:"provider"`
	OK         bool     `json:"ok"`
	Found      bool     `json:"found"`
	Price      *float64 `json:"price,omitempty"`
	DurationMs *int64   `json:"durationMs,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type Report struct {
	HotelID  string          `json:"hotelId"`
	CheckIn  string          `json:"checkIn"`
	CheckOut string          `json:"checkOut"`
	RanAt    time.Time       `json:"ranAt"`
	Healthy  int             `json:"healthy"`
	Total    int             `json:"total"`
	Checks   []ProviderCheck `json:"checks"`
}

type Canary struct {
	cfg    *config.Config
	prices *services.PriceService
	cache  *cache.Cache

	mu   sync.RWMutex
	last *Report
	stop chan struct{}
	wg   sync.WaitGroup
}

const redisKey = "canary:last"

func New(cfg *config.Config, prices *services.PriceService, c *cache.Cache) *Canary {
	k := &Canary{cfg: cfg, prices: prices, cache: c, stop: make(chan struct{})}
	// Onceki calismanin raporunu Redis'ten yukle
	var r Report
	if ok, _ := c.GetJSON(context.Background(), redisKey, &r); ok {
		k.last = &r
	}
	return k
}

// Run bir kanarya turu calistirir (fresh=true: cache'i atlar).
func (k *Canary) Run(ctx context.Context) *Report {
	hotel := k.cfg.CanaryHotelID
	if hotel == "" {
		return nil
	}
	ci := time.Now().AddDate(0, 0, 30)
	q := types.PriceQuery{HotelID: hotel, CheckIn: ci.Format("2006-01-02"), CheckOut: ci.AddDate(0, 0, 3).Format("2006-01-02"), Adults: 2, ChildAges: []int{}}

	rep := &Report{HotelID: hotel, CheckIn: q.CheckIn, CheckOut: q.CheckOut, RanAt: time.Now(), Checks: []ProviderCheck{}}
	res, err := k.prices.GetMultiProviderPrices(ctx, q, nil, true)
	if err != nil {
		rep.Checks = append(rep.Checks, ProviderCheck{Provider: "*", Error: err.Error()})
	} else {
		for _, r := range res.Results {
			c := ProviderCheck{Provider: r.Provider, OK: true, Found: r.Found, DurationMs: r.DurationMs}
			if r.Price != nil {
				c.Price = r.Price.Price
			}
			rep.Checks = append(rep.Checks, c)
		}
		for _, e := range res.Errors {
			rep.Checks = append(rep.Checks, ProviderCheck{Provider: e.Provider, Error: e.Error})
		}
	}
	rep.Total = len(rep.Checks)
	for _, c := range rep.Checks {
		if c.OK && c.Found {
			rep.Healthy++
		} else {
			slog.Warn("kanarya: provider sorunlu", "provider", c.Provider, "found", c.Found, "err", c.Error)
		}
	}
	slog.Info("kanarya tamamlandi", "healthy", rep.Healthy, "total", rep.Total)

	k.mu.Lock()
	k.last = rep
	k.mu.Unlock()
	_ = k.cache.SetJSON(context.Background(), redisKey, rep, 30*24*time.Hour)
	return rep
}

// Last son raporu doner (yoksa nil).
func (k *Canary) Last() *Report {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.last
}

// Start ilk turu 1 dk sonra, sonrakileri CanaryInterval ile calistirir.
func (k *Canary) Start() {
	if k.cfg.CanaryHotelID == "" {
		slog.Info("kanarya devre disi (CANARY_HOTEL_ID bos)")
		return
	}
	k.wg.Add(1)
	go func() {
		defer k.wg.Done()
		first := time.NewTimer(time.Minute)
		defer first.Stop()
		select {
		case <-k.stop:
			return
		case <-first.C:
			k.Run(context.Background())
		}
		t := time.NewTicker(k.cfg.CanaryInterval)
		defer t.Stop()
		for {
			select {
			case <-k.stop:
				return
			case <-t.C:
				k.Run(context.Background())
			}
		}
	}()
}

func (k *Canary) Stop() {
	select {
	case <-k.stop:
	default:
		close(k.stop)
	}
	k.wg.Wait()
}
