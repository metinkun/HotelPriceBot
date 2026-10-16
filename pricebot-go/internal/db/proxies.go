package db

import (
	"context"
	"time"

	"pricebot/internal/model"
)

// ProxyUpsert Webshare'den gelen tek proxy.
type ProxyUpsert struct {
	WebshareID       string
	ProxyAddress     string
	Port             int
	Username         string
	Password         string
	CountryCode      *string
	Valid            bool
	LastVerification *time.Time
}

// UpsertProxies tum listeyi tek transaction'da upsert eder ve listede olmayanlari siler.
func (d *DB) UpsertProxies(ctx context.Context, list []ProxyUpsert) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	ids := make([]string, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.WebshareID)
		_, err := tx.Exec(ctx, `
INSERT INTO proxies (webshare_id, proxy_address, port, username, password, country_code, valid, last_verification, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)
ON CONFLICT (webshare_id) DO UPDATE SET
  proxy_address=EXCLUDED.proxy_address, port=EXCLUDED.port, username=EXCLUDED.username, password=EXCLUDED.password,
  country_code=EXCLUDED.country_code, valid=EXCLUDED.valid, last_verification=EXCLUDED.last_verification, updated_at=EXCLUDED.updated_at`,
			p.WebshareID, p.ProxyAddress, p.Port, p.Username, p.Password, p.CountryCode, p.Valid, p.LastVerification, now)
		if err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM proxies WHERE NOT (webshare_id = ANY($1))`, ids); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ValidProxies gecerli proxy'leri id sirasiyla doner.
func (d *DB) ValidProxies(ctx context.Context) ([]model.Proxy, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id, webshare_id, proxy_address, port, username, password, country_code, valid, last_verification, created_at, updated_at FROM proxies WHERE valid=true ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Proxy{}
	for rows.Next() {
		var p model.Proxy
		if err := rows.Scan(&p.ID, &p.WebshareID, &p.ProxyAddress, &p.Port, &p.Username, &p.Password, &p.CountryCode, &p.Valid, &p.LastVerification, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
