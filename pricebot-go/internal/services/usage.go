package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"pricebot/internal/auth"
	"pricebot/internal/cache"
	"pricebot/internal/db"
)

// Usage kredi/kota muhasebesi: anlik sayac Redis'te, kalici toplamlar Postgres'te.
type Usage struct {
	db  *db.DB
	rdb *redis.Client
}

func NewUsage(d *db.DB, c *cache.Cache) *Usage {
	u := &Usage{db: d}
	if c.Enabled() {
		u.rdb = c.Redis()
	}
	return u
}

func dayKey(t time.Time) string { return t.UTC().Format("20060102") }

func quotaKey(clientID int, t time.Time) string {
	return fmt.Sprintf("quota:%d:%s", clientID, dayKey(t))
}

// Hook Fetcher'a baglanan kredi dusumu: her gercek site cagrisi = credits.
func (u *Usage) Hook(ctx context.Context, providerName string, credits int) {
	id := auth.From(ctx)
	if id == nil || id.IsAdmin || id.ClientID == 0 {
		return
	}
	now := time.Now()
	if u.rdb != nil {
		k := quotaKey(id.ClientID, now)
		pipe := u.rdb.Pipeline()
		pipe.IncrBy(ctx, k, int64(credits))
		pipe.Expire(ctx, k, 48*time.Hour)
		_, _ = pipe.Exec(context.WithoutCancel(ctx))
	}
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := u.db.RecordUsage(c, now, id.ClientID, id.APIKeyID, providerName, credits, 0, 0); err != nil {
			slog.Warn("kullanim kaydi yazilamadi", "err", err)
		}
	}()
}

// RecordRequest istek sayacini artirir (provider='*').
func (u *Usage) RecordRequest(id *auth.Identity) {
	if id == nil || id.IsAdmin || id.ClientID == 0 {
		return
	}
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = u.db.RecordUsage(c, time.Now(), id.ClientID, id.APIKeyID, "*", 0, 1, 0)
	}()
}

// UsedToday musterinin bugunku kredisi (Redis; yoksa DB).
func (u *Usage) UsedToday(ctx context.Context, clientID int) int {
	if u.rdb != nil {
		if n, err := u.rdb.Get(ctx, quotaKey(clientID, time.Now())).Int(); err == nil {
			return n
		}
	}
	n, _ := u.db.CreditsUsedToday(ctx, clientID)
	return n
}

// RateAllow dakikalik istek sinirini uygular (sabit pencere). limit<=0 ise sinirsiz.
func (u *Usage) RateAllow(ctx context.Context, keyID, limit int) bool {
	if limit <= 0 || u.rdb == nil {
		return true
	}
	k := fmt.Sprintf("rl:%d:%s", keyID, time.Now().UTC().Format("200601021504"))
	n, err := u.rdb.Incr(ctx, k).Result()
	if err != nil {
		return true
	}
	if n == 1 {
		u.rdb.Expire(ctx, k, 2*time.Minute)
	}
	return n <= int64(limit)
}
