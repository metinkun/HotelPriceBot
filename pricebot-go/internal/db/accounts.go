package db

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- Limitler: paketten kopyalanir, musteri bazinda ozellestirilebilir ----

// Limits bir musterinin gecerli kisitlari. 0 = sinirsiz.
type Limits struct {
	MaxSites           int  `json:"maxSites"`
	MaxHotels          int  `json:"maxHotels"`
	MaxUsers           int  `json:"maxUsers"`
	DailySearches      int  `json:"dailySearches"`      // arama sayisi (kac siteye giderse gitsin 1)
	DailyScheduledRuns int  `json:"dailyScheduledRuns"` // gunde calisabilecek planli arama
	DailyQuota         int  `json:"dailyQuota"`         // site cagrisi (ic gosterim)
	RatePerMinute      int  `json:"ratePerMinute"`
	MaxConcurrency     int  `json:"maxConcurrency"`
	AllowAPI           bool `json:"allowApi"`
	AllowPackages      bool `json:"allowPackages"`
	AllowFresh         bool `json:"allowFresh"`
	MaxDevicesPC       int  `json:"maxDevicesPc"`
	MaxDevicesMobile   int  `json:"maxDevicesMobile"`
}

// UnlimitedLimits plansiz musteri icin varsayilan.
func UnlimitedLimits() Limits {
	return Limits{AllowAPI: true, AllowPackages: true, MaxDevicesPC: 2, MaxDevicesMobile: 2, RatePerMinute: 0}
}

// LimitsFromPlan paket degerlerini limit yapisina kopyalar.
func LimitsFromPlan(p *Plan) Limits {
	if p == nil {
		return UnlimitedLimits()
	}
	return Limits{
		MaxSites: p.MaxSites, MaxHotels: p.MaxHotels, MaxUsers: p.MaxUsers, DailySearches: p.DailySearches, DailyScheduledRuns: p.DailyScheduledRuns,
		DailyQuota: p.DailyQuota, RatePerMinute: p.RatePerMinute, MaxConcurrency: p.MaxConcurrency,
		AllowAPI: p.AllowAPI, AllowPackages: p.AllowPackages, AllowFresh: p.AllowFresh,
		MaxDevicesPC: p.MaxDevicesPC, MaxDevicesMobile: p.MaxDevicesMobile,
	}
}

// EffectiveLimits musterinin gecerli limitleri: ozellestirilmis/kopyalanmis degerler varsa onlar, yoksa paket.
func EffectiveLimits(c *Client, p *Plan) Limits {
	if c != nil && c.Limits != nil {
		return *c.Limits
	}
	return LimitsFromPlan(p)
}

// SetClientLimits musteri limitlerini yazar (admin ozellestirmesi veya pakete sifirlama).
func (d *DB) SetClientLimits(ctx context.Context, clientID int, l Limits, customized bool) error {
	b, _ := json.Marshal(l)
	tag, err := d.Pool.Exec(ctx, `UPDATE clients SET limits=$1, limits_customized=$2, updated_at=now() WHERE id=$3`, b, customized, clientID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Gunluk sayaclar ----

type ClientDaily struct {
	Day           time.Time `json:"day"`
	ClientID      int       `json:"clientId"`
	Searches      int       `json:"searches"`
	ScheduledRuns int       `json:"scheduledRuns"`
	APIJobs       int       `json:"apiJobs"`
	SiteCalls     int       `json:"siteCalls"`
}

var dailyCols = map[string]bool{"searches": true, "scheduled_runs": true, "api_jobs": true, "site_calls": true}

// IncClientDaily bugunun sayacini artirir (col: searches | scheduled_runs | api_jobs | site_calls).
func (d *DB) IncClientDaily(ctx context.Context, clientID int, col string, n int) error {
	if !dailyCols[col] {
		return fmt.Errorf("gecersiz sayac: %s", col)
	}
	_, err := d.Pool.Exec(ctx, fmt.Sprintf(`INSERT INTO client_daily (day, client_id, %s) VALUES (CURRENT_DATE, $1, $2)
ON CONFLICT (day, client_id) DO UPDATE SET %s = client_daily.%s + EXCLUDED.%s`, col, col, col, col), clientID, n)
	return err
}

// ClientDailyToday musterinin bugunku sayaclari.
func (d *DB) ClientDailyToday(ctx context.Context, clientID int) ClientDaily {
	c := ClientDaily{ClientID: clientID, Day: time.Now()}
	_ = d.Pool.QueryRow(ctx, `SELECT searches, scheduled_runs, api_jobs, site_calls FROM client_daily WHERE day=CURRENT_DATE AND client_id=$1`, clientID).
		Scan(&c.Searches, &c.ScheduledRuns, &c.APIJobs, &c.SiteCalls)
	return c
}

// ClientDailyRange tarih araligi (admin kullanim raporu).
func (d *DB) ClientDailyRange(ctx context.Context, from, to time.Time, clientID *int) ([]ClientDaily, error) {
	q := `SELECT day, client_id, searches, scheduled_runs, api_jobs, site_calls FROM client_daily WHERE day BETWEEN $1 AND $2`
	args := []any{from, to}
	if clientID != nil {
		args = append(args, *clientID)
		q += ` AND client_id=$3`
	}
	q += ` ORDER BY day DESC, client_id`
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ClientDaily{}
	for rows.Next() {
		var c ClientDaily
		if err := rows.Scan(&c.Day, &c.ClientID, &c.Searches, &c.ScheduledRuns, &c.APIJobs, &c.SiteCalls); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountClientHotels / CountPortalUsers limit kontrolleri icin.
func (d *DB) CountClientHotels(ctx context.Context, clientID int) int {
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM client_hotels WHERE client_id=$1`, clientID).Scan(&n)
	return n
}
func (d *DB) CountPortalUsers(ctx context.Context, clientID int) int {
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM portal_users WHERE client_id=$1`, clientID).Scan(&n)
	return n
}

// ---- Giris loglari ----

type LoginLog struct {
	ID         int64     `json:"id"`
	At         time.Time `json:"at"`
	ClientID   *int      `json:"clientId"`
	UserID     *int      `json:"userId"`
	Email      *string   `json:"email"`
	IP         *string   `json:"ip"`
	UserAgent  *string   `json:"userAgent"`
	DeviceID   *int      `json:"deviceId"`
	DeviceKind *string   `json:"deviceKind"`
	Result     string    `json:"result"`
	Detail     *string   `json:"detail"`
}

func (d *DB) InsertLoginLog(ctx context.Context, l LoginLog) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO login_logs (client_id, user_id, email, ip, user_agent, device_id, device_kind, result, detail) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		l.ClientID, l.UserID, l.Email, l.IP, l.UserAgent, l.DeviceID, l.DeviceKind, l.Result, l.Detail)
	return err
}

func (d *DB) ListLoginLogs(ctx context.Context, clientID, userID *int, limit int) ([]LoginLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	q := `SELECT id, at, client_id, user_id, email, ip, user_agent, device_id, device_kind, result, detail FROM login_logs WHERE 1=1`
	args := []any{}
	if clientID != nil {
		args = append(args, *clientID)
		q += fmt.Sprintf(` AND client_id=$%d`, len(args))
	}
	if userID != nil {
		args = append(args, *userID)
		q += fmt.Sprintf(` AND user_id=$%d`, len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY at DESC LIMIT $%d`, len(args))
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LoginLog{}
	for rows.Next() {
		var l LoginLog
		if err := rows.Scan(&l.ID, &l.At, &l.ClientID, &l.UserID, &l.Email, &l.IP, &l.UserAgent, &l.DeviceID, &l.DeviceKind, &l.Result, &l.Detail); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ---- Cihazlar (WebAuthn) ----

type Device struct {
	ID           int        `json:"id"`
	UserID       int        `json:"userId"`
	ClientID     int        `json:"clientId"`
	Name         string     `json:"name"`
	Kind         string     `json:"kind"` // pc | mobile
	CredentialID []byte     `json:"-"`
	PublicKey    []byte     `json:"-"`
	SignCount    int64      `json:"-"`
	AAGUID       []byte     `json:"-"`
	Transports   []string   `json:"transports"`
	UserAgent    *string    `json:"userAgent"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastUsedAt   *time.Time `json:"lastUsedAt"`
	RemovedAt    *time.Time `json:"removedAt"`
	RemovedBy    *string    `json:"removedBy"`
}

const deviceCols = `id, user_id, client_id, name, kind, credential_id, public_key, sign_count, aaguid, transports, user_agent, created_at, last_used_at, removed_at, removed_by`

func scanDevice(row pgx.Row) (*Device, error) {
	var x Device
	if err := row.Scan(&x.ID, &x.UserID, &x.ClientID, &x.Name, &x.Kind, &x.CredentialID, &x.PublicKey, &x.SignCount, &x.AAGUID, &x.Transports, &x.UserAgent, &x.CreatedAt, &x.LastUsedAt, &x.RemovedAt, &x.RemovedBy); err != nil {
		return nil, err
	}
	if x.Transports == nil {
		x.Transports = []string{}
	}
	return &x, nil
}

// EnsureWebauthnID kullanicinin WebAuthn user handle'ini (rastgele 32 bayt) doner, yoksa uretir.
func (d *DB) EnsureWebauthnID(ctx context.Context, userID int) ([]byte, error) {
	var id []byte
	if err := d.Pool.QueryRow(ctx, `SELECT webauthn_id FROM portal_users WHERE id=$1`, userID).Scan(&id); err != nil {
		return nil, err
	}
	if len(id) > 0 {
		return id, nil
	}
	id = make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	_, err := d.Pool.Exec(ctx, `UPDATE portal_users SET webauthn_id=$1 WHERE id=$2 AND webauthn_id IS NULL`, id, userID)
	if err != nil {
		return nil, err
	}
	_ = d.Pool.QueryRow(ctx, `SELECT webauthn_id FROM portal_users WHERE id=$1`, userID).Scan(&id)
	return id, nil
}

func (d *DB) CreateDevice(ctx context.Context, x Device) (*Device, error) {
	return scanDevice(d.Pool.QueryRow(ctx, `INSERT INTO portal_devices (user_id, client_id, name, kind, credential_id, public_key, sign_count, aaguid, transports, user_agent, last_used_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now()) RETURNING `+deviceCols,
		x.UserID, x.ClientID, x.Name, x.Kind, x.CredentialID, x.PublicKey, x.SignCount, x.AAGUID, x.Transports, x.UserAgent))
}

// ListDevices kullanicinin cihazlari (aktif; includeRemoved ile kaldirilanlar da).
func (d *DB) ListDevices(ctx context.Context, userID int, includeRemoved bool) ([]Device, error) {
	q := `SELECT ` + deviceCols + ` FROM portal_devices WHERE user_id=$1`
	if !includeRemoved {
		q += ` AND removed_at IS NULL`
	}
	q += ` ORDER BY created_at`
	rows, err := d.Pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		x, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

func (d *DB) GetDevice(ctx context.Context, id int) (*Device, error) {
	x, err := scanDevice(d.Pool.QueryRow(ctx, `SELECT `+deviceCols+` FROM portal_devices WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return x, err
}

func (d *DB) GetDeviceByCredential(ctx context.Context, cred []byte) (*Device, error) {
	x, err := scanDevice(d.Pool.QueryRow(ctx, `SELECT `+deviceCols+` FROM portal_devices WHERE credential_id=$1`, cred))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return x, err
}

func (d *DB) TouchDevice(ctx context.Context, id int, signCount int64) {
	_, _ = d.Pool.Exec(ctx, `UPDATE portal_devices SET last_used_at=now(), sign_count=$1 WHERE id=$2`, signCount, id)
}

// RemoveDevice cihazi pasife alir (by: user | admin | approval).
func (d *DB) RemoveDevice(ctx context.Context, id int, by string) error {
	tag, err := d.Pool.Exec(ctx, `UPDATE portal_devices SET removed_at=now(), removed_by=$1 WHERE id=$2 AND removed_at IS NULL`, by, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UserDeviceRemovalsToday kullanicinin bugun kendi kaldirdigi cihaz sayisi (gunluk sinir kontrolu).
func (d *DB) UserDeviceRemovalsToday(ctx context.Context, userID int) int {
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM portal_devices WHERE user_id=$1 AND removed_by='user' AND removed_at::date = CURRENT_DATE`, userID).Scan(&n)
	return n
}

// ---- Onay bekleyen islemler ----

type Approval struct {
	ID        int             `json:"id"`
	ClientID  int             `json:"clientId"`
	UserID    *int            `json:"userId"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Status    string          `json:"status"`
	Note      *string         `json:"note"`
	CreatedAt time.Time       `json:"createdAt"`
	DecidedAt *time.Time      `json:"decidedAt"`
	DecidedBy *string         `json:"decidedBy"`
	// Gosterim alanlari (join)
	ClientName *string `json:"clientName,omitempty"`
	UserEmail  *string `json:"userEmail,omitempty"`
}

const approvalCols = `a.id, a.client_id, a.user_id, a.type, a.payload, a.status, a.note, a.created_at, a.decided_at, a.decided_by, c.name, u.email`
const approvalFrom = ` FROM approval_requests a LEFT JOIN clients c ON c.id=a.client_id LEFT JOIN portal_users u ON u.id=a.user_id `

func scanApproval(row pgx.Row) (*Approval, error) {
	var a Approval
	if err := row.Scan(&a.ID, &a.ClientID, &a.UserID, &a.Type, &a.Payload, &a.Status, &a.Note, &a.CreatedAt, &a.DecidedAt, &a.DecidedBy, &a.ClientName, &a.UserEmail); err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *DB) CreateApproval(ctx context.Context, clientID int, userID *int, typ string, payload any) (*Approval, error) {
	b, _ := json.Marshal(payload)
	var id int
	if err := d.Pool.QueryRow(ctx, `INSERT INTO approval_requests (client_id, user_id, type, payload) VALUES ($1,$2,$3,$4) RETURNING id`, clientID, userID, typ, b).Scan(&id); err != nil {
		return nil, err
	}
	return d.GetApproval(ctx, id)
}

func (d *DB) GetApproval(ctx context.Context, id int) (*Approval, error) {
	a, err := scanApproval(d.Pool.QueryRow(ctx, `SELECT `+approvalCols+approvalFrom+` WHERE a.id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// ListApprovals status bos = hepsi; clientID/userID filtreleri opsiyonel.
func (d *DB) ListApprovals(ctx context.Context, status string, clientID, userID *int, limit int) ([]Approval, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT ` + approvalCols + approvalFrom + ` WHERE 1=1`
	args := []any{}
	if status != "" {
		args = append(args, status)
		q += fmt.Sprintf(` AND a.status=$%d`, len(args))
	}
	if clientID != nil {
		args = append(args, *clientID)
		q += fmt.Sprintf(` AND a.client_id=$%d`, len(args))
	}
	if userID != nil {
		args = append(args, *userID)
		q += fmt.Sprintf(` AND a.user_id=$%d`, len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY a.created_at DESC LIMIT $%d`, len(args))
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Approval{}
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (d *DB) CountPendingApprovals(ctx context.Context) int {
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM approval_requests WHERE status='pending'`).Scan(&n)
	return n
}

// DecideApproval bekleyen istegi karara baglar (approved | rejected).
func (d *DB) DecideApproval(ctx context.Context, id int, status string, note *string, by string) (*Approval, error) {
	tag, err := d.Pool.Exec(ctx, `UPDATE approval_requests SET status=$1, note=$2, decided_at=now(), decided_by=$3 WHERE id=$4 AND status='pending'`, status, note, by, id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return d.GetApproval(ctx, id)
}

// ---- Site (provider) ayarlari ----

type SiteSetting struct {
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	DisplayName *string   `json:"displayName"`
	SortOrder   int       `json:"sortOrder"`
	Note        *string   `json:"note"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (d *DB) ListSiteSettings(ctx context.Context) (map[string]SiteSetting, error) {
	rows, err := d.Pool.Query(ctx, `SELECT name, enabled, display_name, sort_order, note, updated_at FROM provider_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SiteSetting{}
	for rows.Next() {
		var s SiteSetting
		if err := rows.Scan(&s.Name, &s.Enabled, &s.DisplayName, &s.SortOrder, &s.Note, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out[s.Name] = s
	}
	return out, rows.Err()
}

func (d *DB) UpsertSiteSetting(ctx context.Context, name string, enabled *bool, displayName *string, sortOrder *int, note *string) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO provider_settings (name, enabled, display_name, sort_order, note)
VALUES ($1, COALESCE($2, true), $3, COALESCE($4, 100), $5)
ON CONFLICT (name) DO UPDATE SET enabled=COALESCE($2, provider_settings.enabled), display_name=COALESCE($3, provider_settings.display_name),
 sort_order=COALESCE($4, provider_settings.sort_order), note=COALESCE($5, provider_settings.note), updated_at=now()`,
		strings.TrimSpace(name), enabled, displayName, sortOrder, note)
	return err
}

// ClientsUsingProvider bir siteyi etkin kullanan musteri sayisi (bos liste = tumu acik sayilir).
func (d *DB) ClientsUsingProvider(ctx context.Context, name string) int {
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM clients WHERE is_active AND (cardinality(enabled_providers)=0 OR $1 = ANY(enabled_providers))`, name).Scan(&n)
	return n
}

// ---- Otel kunyesi ----

type HotelMeta struct {
	InternalHotelID string    `json:"internalHotelId"`
	Name            *string   `json:"name"`
	City            *string   `json:"city"`
	Region          *string   `json:"region"`
	Country         *string   `json:"country"`
	Stars           *int      `json:"stars"`
	Chain           *string   `json:"chain"`
	ExternalID      *string   `json:"externalId"`
	IsActive        bool      `json:"isActive"`
	Notes           *string   `json:"notes"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

const hotelMetaCols = `internal_hotel_id, name, city, region, country, stars, chain, external_id, is_active, notes, created_at, updated_at`

func scanHotelMeta(row pgx.Row) (*HotelMeta, error) {
	var h HotelMeta
	if err := row.Scan(&h.InternalHotelID, &h.Name, &h.City, &h.Region, &h.Country, &h.Stars, &h.Chain, &h.ExternalID, &h.IsActive, &h.Notes, &h.CreatedAt, &h.UpdatedAt); err != nil {
		return nil, err
	}
	return &h, nil
}

// EnsureHotelMeta eslestirme olusurken otel ana kaydini acar (varsa adi bos ise doldurur).
func (d *DB) EnsureHotelMeta(ctx context.Context, internalID string, name *string) {
	_, _ = d.Pool.Exec(ctx, `INSERT INTO hotels (internal_hotel_id, name) VALUES ($1, $2)
ON CONFLICT (internal_hotel_id) DO UPDATE SET name=COALESCE(hotels.name, EXCLUDED.name), updated_at=now()`, internalID, name)
}

func (d *DB) GetHotelMeta(ctx context.Context, internalID string) (*HotelMeta, error) {
	h, err := scanHotelMeta(d.Pool.QueryRow(ctx, `SELECT `+hotelMetaCols+` FROM hotels WHERE internal_hotel_id=$1`, internalID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return h, err
}

func (d *DB) ListHotelMeta(ctx context.Context) (map[string]HotelMeta, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+hotelMetaCols+` FROM hotels`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]HotelMeta{}
	for rows.Next() {
		h, err := scanHotelMeta(rows)
		if err != nil {
			return nil, err
		}
		out[h.InternalHotelID] = *h
	}
	return out, rows.Err()
}

type HotelMetaInput struct {
	Name       *string `json:"name"`
	City       *string `json:"city"`
	Region     *string `json:"region"`
	Country    *string `json:"country"`
	Stars      *int    `json:"stars"`
	Chain      *string `json:"chain"`
	ExternalID *string `json:"externalId"`
	IsActive   *bool   `json:"isActive"`
	Notes      *string `json:"notes"`
}

// UpdateHotelMeta verilen alanlari gunceller (nil = dokunma). Kayit yoksa olusturur.
func (d *DB) UpdateHotelMeta(ctx context.Context, internalID string, in HotelMetaInput) (*HotelMeta, error) {
	d.EnsureHotelMeta(ctx, internalID, in.Name)
	sets := []string{"updated_at=now()"}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if in.Name != nil {
		add("name", in.Name)
	}
	if in.City != nil {
		add("city", in.City)
	}
	if in.Region != nil {
		add("region", in.Region)
	}
	if in.Country != nil {
		add("country", in.Country)
	}
	if in.Stars != nil {
		add("stars", in.Stars)
	}
	if in.Chain != nil {
		add("chain", in.Chain)
	}
	if in.ExternalID != nil {
		add("external_id", in.ExternalID)
	}
	if in.IsActive != nil {
		add("is_active", *in.IsActive)
	}
	if in.Notes != nil {
		add("notes", in.Notes)
	}
	args = append(args, internalID)
	if _, err := d.Pool.Exec(ctx, fmt.Sprintf(`UPDATE hotels SET %s WHERE internal_hotel_id=$%d`, strings.Join(sets, ", "), len(args)), args...); err != nil {
		return nil, err
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		_, _ = d.Pool.Exec(ctx, `UPDATE hotel_mappings SET hotel_name=$1, updated_at=now() WHERE internal_hotel_id=$2`, strings.TrimSpace(*in.Name), internalID)
	}
	return d.GetHotelMeta(ctx, internalID)
}

// InactiveHotels pasif otel kimlikleri (aramalarda atlanir).
func (d *DB) InactiveHotels(ctx context.Context, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := d.Pool.Query(ctx, `SELECT internal_hotel_id FROM hotels WHERE NOT is_active AND internal_hotel_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ClientsOfHotel bir otele bagli musteriler (admin otel detayi).
func (d *DB) ClientsOfHotel(ctx context.Context, internalID string) ([]Client, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+clientCols+clientFrom+` JOIN client_hotels ch ON ch.client_id=c.id WHERE ch.internal_hotel_id=$1 ORDER BY c.name`, internalID)
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
