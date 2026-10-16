// Package db PostgreSQL erisimi (pgx) ve sema olusturma.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

// Connect baglanti havuzu acar ve semayi (idempotent) olusturur.
func Connect(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL parse edilemedi: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Baglantiyi bekle (container ayaga kalkiyor olabilir)
	var lastErr error
	for i := 0; i < 30; i++ {
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = pool.Ping(pctx)
		cancel()
		if lastErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if lastErr != nil {
		pool.Close()
		return nil, fmt.Errorf("veritabanina baglanilamadi: %w", lastErr)
	}

	d := &DB{Pool: pool}
	if err := d.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() { d.Pool.Close() }

// Sema Prisma migration'lariyla birebir ayni tablo/indeks adlarini kullanir;
// mevcut bir pricebot veritabani uzerinde de sorunsuz calisir.
const schema = `
CREATE TABLE IF NOT EXISTS "hotel_mappings" (
    "id" SERIAL NOT NULL,
    "provider" TEXT NOT NULL,
    "internal_hotel_id" TEXT NOT NULL,
    "provider_hotel_id" TEXT NOT NULL,
    "hotel_name" TEXT,
    "metadata" JSONB,
    "is_active" BOOLEAN NOT NULL DEFAULT true,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "hotel_mappings_pkey" PRIMARY KEY ("id")
);
CREATE INDEX IF NOT EXISTS "hotel_mappings_provider_provider_hotel_id_idx" ON "hotel_mappings"("provider", "provider_hotel_id");
CREATE INDEX IF NOT EXISTS "hotel_mappings_provider_is_active_idx" ON "hotel_mappings"("provider", "is_active");
CREATE UNIQUE INDEX IF NOT EXISTS "hotel_mappings_provider_internal_hotel_id_key" ON "hotel_mappings"("provider", "internal_hotel_id");

CREATE TABLE IF NOT EXISTS "proxies" (
    "id" SERIAL NOT NULL,
    "webshare_id" TEXT NOT NULL,
    "proxy_address" TEXT NOT NULL,
    "port" INTEGER NOT NULL,
    "username" TEXT NOT NULL,
    "password" TEXT NOT NULL,
    "country_code" TEXT,
    "valid" BOOLEAN NOT NULL DEFAULT true,
    "last_verification" TIMESTAMP(3),
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "proxies_pkey" PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX IF NOT EXISTS "proxies_webshare_id_key" ON "proxies"("webshare_id");
CREATE INDEX IF NOT EXISTS "proxies_valid_idx" ON "proxies"("valid");
`

func (d *DB) migrate(ctx context.Context) error {
	// 0001: temel sema (idempotent; Prisma ile ayni tablo adlari)
	if _, err := d.Pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("sema olusturulamadi: %w", err)
	}
	// 0002+: sirali, bir kez uygulanan migration dosyalari
	return d.runMigrations(ctx)
}
