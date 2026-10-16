package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- Modeller ----

type Plan struct {
	ID               int       `json:"id"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	DailyQuota       int       `json:"dailyQuota"`
	AllowedProviders []string  `json:"allowedProviders"`
	AllowPackages    bool      `json:"allowPackages"`
	AllowFresh       bool      `json:"allowFresh"`
	MaxConcurrency   int       `json:"maxConcurrency"`
	RatePerMinute    int       `json:"ratePerMinute"`
	IsActive         bool      `json:"isActive"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type PlanInput struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	DailyQuota       int      `json:"dailyQuota"`
	AllowedProviders []string `json:"allowedProviders"`
	AllowPackages    bool     `json:"allowPackages"`
	AllowFresh       bool     `json:"allowFresh"`
	MaxConcurrency   int      `json:"maxConcurrency"`
	RatePerMinute    int      `json:"ratePerMinute"`
	IsActive         bool     `json:"isActive"`
}

type Client struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     *string   `json:"email"`
	Notes     *string   `json:"notes"`
	PlanID    *int      `json:"planId"`
	PlanCode  *string   `json:"planCode"`
	IsActive  bool      `json:"isActive"`
	KeyCount  int       `json:"keyCount"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ClientInput struct {
	Name     string  `json:"name"`
	Email    *string `json:"email"`
	Notes    *string `json:"notes"`
	PlanID   *int    `json:"planId"`
	IsActive *bool   `json:"isActive"`
}

type APIKey struct {
	ID         int        `json:"id"`
	ClientID   int        `json:"clientId"`
	KeyPrefix  string     `json:"keyPrefix"`
	Label      *string    `json:"label"`
	IsActive   bool       `json:"isActive"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type Admin struct {
	ID           int        `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Name         *string    `json:"name"`
	IsActive     bool       `json:"isActive"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
}

// KeyIdentity cozulmus API anahtari: anahtar + musteri + plan.
type KeyIdentity struct {
	Key    APIKey
	Client Client
	Plan   *Plan
}

var (
	ErrKeyInvalid  = errors.New("gecersiz veya iptal edilmis API anahtari")
	ErrKeyExpired  = errors.New("API anahtarinin suresi dolmus")
	ErrClientOff   = errors.New("musteri hesabi pasif")
)

// ---- Planlar ----

const planCols = `id, code, name, daily_quota, allowed_providers, allow_packages, allow_fresh, max_concurrency, rate_per_minute, is_active, created_at, updated_at`

func scanPlan(row pgx.Row) (*Plan, error) {
	var p Plan
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.DailyQuota, &p.AllowedProviders, &p.AllowPackages, &p.AllowFresh, &p.MaxConcurrency, &p.RatePerMinute, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	if p.AllowedProviders == nil {
		p.AllowedProviders = []string{}
	}
	return &p, nil
}

func (d *DB) ListPlans(ctx context.Context) ([]Plan, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+planCols+` FROM plans ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Plan{}
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (d *DB) GetPlan(ctx context.Context, id int) (*Plan, error) {
	p, err := scanPlan(d.Pool.QueryRow(ctx, `SELECT `+planCols+` FROM plans WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func normPlan(in *PlanInput) {
	if in.AllowedProviders == nil {
		in.AllowedProviders = []string{}
	}
	if in.MaxConcurrency <= 0 {
		in.MaxConcurrency = 5
	}
	if in.RatePerMinute <= 0 {
		in.RatePerMinute = 60
	}
}

func (d *DB) CreatePlan(ctx context.Context, in PlanInput) (*Plan, error) {
	normPlan(&in)
	return scanPlan(d.Pool.QueryRow(ctx, `INSERT INTO plans (code, name, daily_quota, allowed_providers, allow_packages, allow_fresh, max_concurrency, rate_per_minute, is_active)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+planCols,
		strings.ToLower(strings.TrimSpace(in.Code)), in.Name, in.DailyQuota, in.AllowedProviders, in.AllowPackages, in.AllowFresh, in.MaxConcurrency, in.RatePerMinute, in.IsActive))
}

func (d *DB) UpdatePlan(ctx context.Context, id int, in PlanInput) (*Plan, error) {
	normPlan(&in)
	p, err := scanPlan(d.Pool.QueryRow(ctx, `UPDATE plans SET code=$1, name=$2, daily_quota=$3, allowed_providers=$4, allow_packages=$5, allow_fresh=$6, max_concurrency=$7, rate_per_minute=$8, is_active=$9, updated_at=now()
WHERE id=$10 RETURNING `+planCols,
		strings.ToLower(strings.TrimSpace(in.Code)), in.Name, in.DailyQuota, in.AllowedProviders, in.AllowPackages, in.AllowFresh, in.MaxConcurrency, in.RatePerMinute, in.IsActive, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// ---- Musteriler ----

const clientCols = `c.id, c.name, c.email, c.notes, c.plan_id, p.code, c.is_active,
 (SELECT count(*) FROM api_keys k WHERE k.client_id=c.id AND k.is_active AND k.revoked_at IS NULL), c.created_at, c.updated_at`
const clientFrom = ` FROM clients c LEFT JOIN plans p ON p.id=c.plan_id `

func scanClient(row pgx.Row) (*Client, error) {
	var c Client
	if err := row.Scan(&c.ID, &c.Name, &c.Email, &c.Notes, &c.PlanID, &c.PlanCode, &c.IsActive, &c.KeyCount, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *DB) ListClients(ctx context.Context, search string) ([]Client, error) {
	q := `SELECT ` + clientCols + clientFrom
	args := []any{}
	if search != "" {
		q += ` WHERE c.name ILIKE $1 OR c.email ILIKE $1`
		args = append(args, "%"+search+"%")
	}
	q += ` ORDER BY c.created_at DESC`
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Client{}
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (d *DB) GetClient(ctx context.Context, id int) (*Client, error) {
	c, err := scanClient(d.Pool.QueryRow(ctx, `SELECT `+clientCols+clientFrom+` WHERE c.id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (d *DB) CreateClient(ctx context.Context, in ClientInput) (*Client, error) {
	active := in.IsActive == nil || *in.IsActive
	var id int
	if err := d.Pool.QueryRow(ctx, `INSERT INTO clients (name, email, notes, plan_id, is_active) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		strings.TrimSpace(in.Name), in.Email, in.Notes, in.PlanID, active).Scan(&id); err != nil {
		return nil, err
	}
	return d.GetClient(ctx, id)
}

func (d *DB) UpdateClient(ctx context.Context, id int, in ClientInput) (*Client, error) {
	sets := []string{"updated_at=now()"}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if strings.TrimSpace(in.Name) != "" {
		add("name", strings.TrimSpace(in.Name))
	}
	if in.Email != nil {
		add("email", in.Email)
	}
	if in.Notes != nil {
		add("notes", in.Notes)
	}
	if in.PlanID != nil {
		add("plan_id", *in.PlanID)
	}
	if in.IsActive != nil {
		add("is_active", *in.IsActive)
	}
	args = append(args, id)
	tag, err := d.Pool.Exec(ctx, fmt.Sprintf(`UPDATE clients SET %s WHERE id=$%d`, strings.Join(sets, ", "), len(args)), args...)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return d.GetClient(ctx, id)
}

func (d *DB) DeleteClient(ctx context.Context, id int) error {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM clients WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- API anahtarlari ----

const keyCols = `id, client_id, key_prefix, label, is_active, last_used_at, expires_at, revoked_at, created_at`

func scanKey(row pgx.Row) (*APIKey, error) {
	var k APIKey
	if err := row.Scan(&k.ID, &k.ClientID, &k.KeyPrefix, &k.Label, &k.IsActive, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt); err != nil {
		return nil, err
	}
	return &k, nil
}

func (d *DB) CreateAPIKey(ctx context.Context, clientID int, label *string, hash, prefix string, expires *time.Time) (*APIKey, error) {
	return scanKey(d.Pool.QueryRow(ctx, `INSERT INTO api_keys (client_id, key_hash, key_prefix, label, expires_at) VALUES ($1,$2,$3,$4,$5) RETURNING `+keyCols,
		clientID, hash, prefix, label, expires))
}

func (d *DB) ListAPIKeys(ctx context.Context, clientID int) ([]APIKey, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+keyCols+` FROM api_keys WHERE client_id=$1 ORDER BY created_at DESC`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []APIKey{}
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (d *DB) RevokeAPIKey(ctx context.Context, id int) error {
	tag, err := d.Pool.Exec(ctx, `UPDATE api_keys SET is_active=false, revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) TouchAPIKey(ctx context.Context, id int) {
	_, _ = d.Pool.Exec(ctx, `UPDATE api_keys SET last_used_at=now() WHERE id=$1`, id)
}

// ResolveAPIKey hash ile anahtari, musteriyi ve plani cozer; gecersizse ErrKeyInvalid/ErrKeyExpired/ErrClientOff.
func (d *DB) ResolveAPIKey(ctx context.Context, hash string) (*KeyIdentity, error) {
	k, err := scanKey(d.Pool.QueryRow(ctx, `SELECT `+keyCols+` FROM api_keys WHERE key_hash=$1`, hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrKeyInvalid
	}
	if err != nil {
		return nil, err
	}
	if !k.IsActive || k.RevokedAt != nil {
		return nil, ErrKeyInvalid
	}
	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return nil, ErrKeyExpired
	}
	c, err := d.GetClient(ctx, k.ClientID)
	if err != nil {
		return nil, ErrKeyInvalid
	}
	if !c.IsActive {
		return nil, ErrClientOff
	}
	id := &KeyIdentity{Key: *k, Client: *c}
	if c.PlanID != nil {
		if p, err := d.GetPlan(ctx, *c.PlanID); err == nil && p.IsActive {
			id.Plan = p
		}
	}
	return id, nil
}

// ---- Adminler ----

const adminCols = `id, email, password_hash, name, is_active, created_at, last_login_at`

func scanAdmin(row pgx.Row) (*Admin, error) {
	var a Admin
	if err := row.Scan(&a.ID, &a.Email, &a.PasswordHash, &a.Name, &a.IsActive, &a.CreatedAt, &a.LastLoginAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *DB) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM admins`).Scan(&n)
	return n, err
}

func (d *DB) GetAdminByEmail(ctx context.Context, email string) (*Admin, error) {
	a, err := scanAdmin(d.Pool.QueryRow(ctx, `SELECT `+adminCols+` FROM admins WHERE lower(email)=lower($1)`, strings.TrimSpace(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (d *DB) CreateAdmin(ctx context.Context, email, passwordHash string, name *string) (*Admin, error) {
	return scanAdmin(d.Pool.QueryRow(ctx, `INSERT INTO admins (email, password_hash, name) VALUES ($1,$2,$3) RETURNING `+adminCols, strings.ToLower(strings.TrimSpace(email)), passwordHash, name))
}

func (d *DB) ListAdmins(ctx context.Context) ([]Admin, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+adminCols+` FROM admins ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Admin{}
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (d *DB) TouchAdminLogin(ctx context.Context, id int) {
	_, _ = d.Pool.Exec(ctx, `UPDATE admins SET last_login_at=now() WHERE id=$1`, id)
}

// ---- Kullanim ----

// RecordUsage gunluk sayaclari artirir (upsert).
func (d *DB) RecordUsage(ctx context.Context, day time.Time, clientID, keyID int, provider string, credits, requests, cacheHits int) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO usage_daily (day, client_id, api_key_id, provider, credits, requests, cache_hits)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (day, client_id, api_key_id, provider) DO UPDATE SET
  credits = usage_daily.credits + EXCLUDED.credits,
  requests = usage_daily.requests + EXCLUDED.requests,
  cache_hits = usage_daily.cache_hits + EXCLUDED.cache_hits`,
		day.Format("2006-01-02"), clientID, keyID, provider, credits, requests, cacheHits)
	return err
}

type UsageRow struct {
	Day       string `json:"day"`
	ClientID  int    `json:"clientId"`
	Provider  string `json:"provider"`
	Credits   int    `json:"credits"`
	Requests  int    `json:"requests"`
	CacheHits int    `json:"cacheHits"`
}

// UsageSummary tarih araliginda gun+musteri+provider kiriliminda kullanim.
func (d *DB) UsageSummary(ctx context.Context, from, to time.Time, clientID *int) ([]UsageRow, error) {
	q := `SELECT to_char(day,'YYYY-MM-DD'), client_id, provider, sum(credits), sum(requests), sum(cache_hits) FROM usage_daily WHERE day BETWEEN $1 AND $2`
	args := []any{from.Format("2006-01-02"), to.Format("2006-01-02")}
	if clientID != nil {
		args = append(args, *clientID)
		q += ` AND client_id=$3`
	}
	q += ` GROUP BY day, client_id, provider ORDER BY day DESC, client_id, provider`
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UsageRow{}
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.Day, &r.ClientID, &r.Provider, &r.Credits, &r.Requests, &r.CacheHits); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreditsUsedToday bir musterinin bugunku toplam kredisi (Redis yoksa yedek).
func (d *DB) CreditsUsedToday(ctx context.Context, clientID int) (int, error) {
	var n *int
	err := d.Pool.QueryRow(ctx, `SELECT sum(credits) FROM usage_daily WHERE client_id=$1 AND day=CURRENT_DATE AND provider <> '*'`, clientID).Scan(&n)
	if err != nil || n == nil {
		return 0, err
	}
	return *n, nil
}

// ---- Istek loglari ----

type RequestLog struct {
	ID         int64     `json:"id"`
	At         time.Time `json:"at"`
	RequestID  *string   `json:"requestId"`
	ClientID   *int      `json:"clientId"`
	APIKeyID   *int      `json:"apiKeyId"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	DurationMs int       `json:"durationMs"`
	IP         *string   `json:"ip"`
	Error      *string   `json:"error"`
}

func (d *DB) InsertRequestLog(ctx context.Context, l RequestLog) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO request_logs (request_id, client_id, api_key_id, method, path, status, duration_ms, ip, error) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		l.RequestID, l.ClientID, l.APIKeyID, l.Method, l.Path, l.Status, l.DurationMs, l.IP, l.Error)
	return err
}

func (d *DB) ListRequestLogs(ctx context.Context, clientID *int, limit int) ([]RequestLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id, at, request_id, client_id, api_key_id, method, path, status, duration_ms, ip, error FROM request_logs`
	args := []any{}
	if clientID != nil {
		q += ` WHERE client_id=$1`
		args = append(args, *clientID)
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY at DESC LIMIT $%d`, len(args))
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RequestLog{}
	for rows.Next() {
		var l RequestLog
		if err := rows.Scan(&l.ID, &l.At, &l.RequestID, &l.ClientID, &l.APIKeyID, &l.Method, &l.Path, &l.Status, &l.DurationMs, &l.IP, &l.Error); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// PurgeRequestLogs verilen gunden eski loglari siler.
func (d *DB) PurgeRequestLogs(ctx context.Context, olderThanDays int) (int64, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM request_logs WHERE at < now() - ($1 || ' days')::interval`, fmt.Sprint(olderThanDays))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
