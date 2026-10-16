// Package cache Redis destekli sonuc cache'i (provider basina) + ayni anda gelen esdeger sorgular icin tek ucus.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"pricebot/internal/types"
)

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
	sf  singleflight.Group
}

// New Redis'e baglanir. Baglanti kurulamazsa cache devre disi (nil rdb) ama uygulama calisir.
func New(ctx context.Context, redisURL string, ttl time.Duration) *Cache {
	c := &Cache{ttl: ttl}
	if redisURL == "" {
		slog.Warn("REDIS_URL bos, cache devre disi")
		return c
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Warn("REDIS_URL parse edilemedi, cache devre disi", "err", err)
		return c
	}
	opt.DialTimeout = 3 * time.Second
	opt.ReadTimeout = 2 * time.Second
	opt.WriteTimeout = 2 * time.Second
	rdb := redis.NewClient(opt)
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pctx).Err(); err != nil {
		slog.Warn("Redis'e baglanilamadi, cache devre disi (istekler dogrudan gider)", "err", err)
	}
	c.rdb = rdb
	return c
}

// Redis ham istemci (jobs/usage gibi diger moduller icin). Yoksa nil.
func (c *Cache) Redis() *redis.Client { return c.rdb }

func (c *Cache) Enabled() bool { return c != nil && c.rdb != nil }

// Ping /ready icin.
func (c *Cache) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return errors.New("redis yapilandirilmamis")
	}
	return c.rdb.Ping(ctx).Err()
}

func (c *Cache) TTL() time.Duration { return c.ttl }

func agesKey(ages []int) string {
	if len(ages) == 0 {
		return "-"
	}
	a := append([]int{}, ages...)
	sort.Ints(a)
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = fmt.Sprint(v)
	}
	return strings.Join(parts, ",")
}

// PriceKey provider + otel + tarih + kisi bilgisinden anahtar uretir.
func PriceKey(provider, providerHotelID string, q types.PriceQuery) string {
	return fmt.Sprintf("price:%s:%s:%s:%s:%d:%d:%s", provider, providerHotelID, q.CheckIn, q.CheckOut, q.Adults, q.Children, agesKey(q.ChildAges))
}

// PackageKey paket sorgusu anahtari (kalkis havalimani dahil).
func PackageKey(provider, providerHotelID string, q types.PackageQuery) string {
	return fmt.Sprintf("pkg:%s:%s:%s:%s:%s:%d:%d:%s", provider, providerHotelID, strings.ToUpper(q.AirportCode), q.CheckIn, q.CheckOut, q.Adults, q.Children, agesKey(q.ChildAges))
}

// GetJSON anahtari okur ve v'ye acar. Bulunamazsa (false, nil).
func (c *Cache) GetJSON(ctx context.Context, key string, v any) (bool, error) {
	if !c.Enabled() {
		return false, nil
	}
	b, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return false, err
	}
	return true, nil
}

// SetJSON degeri TTL ile yazar. ttl<=0 ise varsayilan TTL.
func (c *Cache) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	if ttl <= 0 {
		ttl = c.ttl
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, ttl).Err()
}

// Delete anahtarlari siler.
func (c *Cache) Delete(ctx context.Context, keys ...string) {
	if c.Enabled() && len(keys) > 0 {
		_ = c.rdb.Del(ctx, keys...).Err()
	}
}

// Do ayni anahtarla es zamanli cagrilari tek calismaya indirger (singleflight).
func (c *Cache) Do(key string, fn func() (any, error)) (any, error, bool) {
	return c.sf.Do(key, fn)
}
