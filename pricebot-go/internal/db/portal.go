package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- Portal kullanicilari ----

type PortalUser struct {
	ID           int        `json:"id"`
	ClientID     int        `json:"clientId"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Name         *string    `json:"name"`
	IsActive     bool       `json:"isActive"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	// Kullanici bazinda cihaz limiti (nil = musteri/paket degeri)
	MaxDevicesPC     *int `json:"maxDevicesPc"`
	MaxDevicesMobile *int `json:"maxDevicesMobile"`
}

const portalUserCols = `id, client_id, email, password_hash, name, is_active, created_at, last_login_at, max_devices_pc, max_devices_mobile`

func scanPortalUser(row pgx.Row) (*PortalUser, error) {
	var u PortalUser
	if err := row.Scan(&u.ID, &u.ClientID, &u.Email, &u.PasswordHash, &u.Name, &u.IsActive, &u.CreatedAt, &u.LastLoginAt, &u.MaxDevicesPC, &u.MaxDevicesMobile); err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) CreatePortalUser(ctx context.Context, clientID int, email, passwordHash string, name *string) (*PortalUser, error) {
	return scanPortalUser(d.Pool.QueryRow(ctx, `INSERT INTO portal_users (client_id, email, password_hash, name) VALUES ($1,$2,$3,$4) RETURNING `+portalUserCols,
		clientID, strings.ToLower(strings.TrimSpace(email)), passwordHash, name))
}

func (d *DB) GetPortalUserByEmail(ctx context.Context, email string) (*PortalUser, error) {
	u, err := scanPortalUser(d.Pool.QueryRow(ctx, `SELECT `+portalUserCols+` FROM portal_users WHERE lower(email)=lower($1)`, strings.TrimSpace(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (d *DB) GetPortalUser(ctx context.Context, id int) (*PortalUser, error) {
	u, err := scanPortalUser(d.Pool.QueryRow(ctx, `SELECT `+portalUserCols+` FROM portal_users WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (d *DB) ListPortalUsers(ctx context.Context, clientID int) ([]PortalUser, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+portalUserCols+` FROM portal_users WHERE client_id=$1 ORDER BY id`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PortalUser{}
	for rows.Next() {
		u, err := scanPortalUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (d *DB) UpdatePortalUser(ctx context.Context, id int, passwordHash *string, isActive *bool, name *string) error {
	sets := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if passwordHash != nil {
		add("password_hash", *passwordHash)
	}
	if isActive != nil {
		add("is_active", *isActive)
	}
	if name != nil {
		add("name", *name)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	tag, err := d.Pool.Exec(ctx, fmt.Sprintf(`UPDATE portal_users SET %s WHERE id=$%d`, strings.Join(sets, ", "), len(args)), args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetPortalUserDeviceLimits kullanici bazinda cihaz limiti (nil = varsayilana don).
func (d *DB) SetPortalUserDeviceLimits(ctx context.Context, id int, pc, mobile *int) error {
	tag, err := d.Pool.Exec(ctx, `UPDATE portal_users SET max_devices_pc=$1, max_devices_mobile=$2 WHERE id=$3`, pc, mobile, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Sifre sifirlama ----

type PasswordReset struct {
	ID        int        `json:"id"`
	UserID    int        `json:"userId"`
	ExpiresAt time.Time  `json:"expiresAt"`
	CreatedBy *string    `json:"createdBy"`
	CreatedAt time.Time  `json:"createdAt"`
	UsedAt    *time.Time `json:"usedAt"`
	SentTo    *string    `json:"sentTo"`
	SentAt    *time.Time `json:"sentAt"`
	SendError *string    `json:"sendError"`
}

const resetCols = `id, user_id, expires_at, created_by, created_at, used_at, sent_to, sent_at, send_error`

func scanReset(row pgx.Row) (*PasswordReset, error) {
	var x PasswordReset
	if err := row.Scan(&x.ID, &x.UserID, &x.ExpiresAt, &x.CreatedBy, &x.CreatedAt, &x.UsedAt, &x.SentTo, &x.SentAt, &x.SendError); err != nil {
		return nil, err
	}
	return &x, nil
}

// CreatePasswordReset ayni kullanicinin kullanilmamis eski linklerini gecersiz kilar ve yeni kayit acar.
func (d *DB) CreatePasswordReset(ctx context.Context, userID int, tokenHash string, expires time.Time, createdBy string) (*PasswordReset, error) {
	_, _ = d.Pool.Exec(ctx, `UPDATE password_resets SET used_at=now() WHERE user_id=$1 AND used_at IS NULL`, userID)
	return scanReset(d.Pool.QueryRow(ctx, `INSERT INTO password_resets (user_id, token_hash, expires_at, created_by) VALUES ($1,$2,$3,$4) RETURNING `+resetCols, userID, tokenHash, expires, createdBy))
}

func (d *DB) GetPasswordReset(ctx context.Context, tokenHash string) (*PasswordReset, error) {
	x, err := scanReset(d.Pool.QueryRow(ctx, `SELECT `+resetCols+` FROM password_resets WHERE token_hash=$1`, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return x, err
}

func (d *DB) UsePasswordReset(ctx context.Context, id int) error {
	_, err := d.Pool.Exec(ctx, `UPDATE password_resets SET used_at=now() WHERE id=$1`, id)
	return err
}

func (d *DB) MarkPasswordResetSent(ctx context.Context, id int, to string, sendErr *string) {
	if sendErr == nil {
		_, _ = d.Pool.Exec(ctx, `UPDATE password_resets SET sent_to=$1, sent_at=now(), send_error=NULL WHERE id=$2`, to, id)
		return
	}
	_, _ = d.Pool.Exec(ctx, `UPDATE password_resets SET sent_to=$1, send_error=$2 WHERE id=$3`, to, *sendErr, id)
}

// LatestPasswordReset kullanicinin son linki (yoksa nil).
func (d *DB) LatestPasswordReset(ctx context.Context, userID int) *PasswordReset {
	x, err := scanReset(d.Pool.QueryRow(ctx, `SELECT `+resetCols+` FROM password_resets WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1`, userID))
	if err != nil {
		return nil
	}
	return x
}

func (d *DB) DeletePortalUser(ctx context.Context, id int) error {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM portal_users WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) TouchPortalLogin(ctx context.Context, id int) {
	_, _ = d.Pool.Exec(ctx, `UPDATE portal_users SET last_login_at=now() WHERE id=$1`, id)
}

// ---- Musteri: etkin provider'lar / tip ----

// ClientSettings musteri tipi ve etkin site listesi.
type ClientSettings struct {
	ClientType       string   `json:"clientType"`       // api | portal | both
	EnabledProviders []string `json:"enabledProviders"` // bos = hepsi (plan sinirlari icinde)
}

func (d *DB) GetClientSettings(ctx context.Context, clientID int) (*ClientSettings, error) {
	var s ClientSettings
	err := d.Pool.QueryRow(ctx, `SELECT client_type, enabled_providers FROM clients WHERE id=$1`, clientID).Scan(&s.ClientType, &s.EnabledProviders)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if s.EnabledProviders == nil {
		s.EnabledProviders = []string{}
	}
	return &s, err
}

func (d *DB) UpdateClientSettings(ctx context.Context, clientID int, clientType *string, enabled []string) error {
	sets := []string{"updated_at=now()"}
	args := []any{}
	if clientType != nil {
		args = append(args, *clientType)
		sets = append(sets, fmt.Sprintf("client_type=$%d", len(args)))
	}
	if enabled != nil {
		args = append(args, enabled)
		sets = append(sets, fmt.Sprintf("enabled_providers=$%d", len(args)))
	}
	args = append(args, clientID)
	tag, err := d.Pool.Exec(ctx, fmt.Sprintf(`UPDATE clients SET %s WHERE id=$%d`, strings.Join(sets, ", "), len(args)), args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Musteri otelleri ----

type ClientHotel struct {
	ID              int       `json:"id"`
	ClientID        int       `json:"clientId"`
	InternalHotelID string    `json:"internalHotelId"`
	Label           *string   `json:"label"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (d *DB) LinkClientHotel(ctx context.Context, clientID int, internalHotelID string, label *string) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO client_hotels (client_id, internal_hotel_id, label) VALUES ($1,$2,$3)
ON CONFLICT (client_id, internal_hotel_id) DO UPDATE SET label=COALESCE(EXCLUDED.label, client_hotels.label)`, clientID, internalHotelID, label)
	return err
}

func (d *DB) UnlinkClientHotel(ctx context.Context, clientID int, internalHotelID string) error {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM client_hotels WHERE client_id=$1 AND internal_hotel_id=$2`, clientID, internalHotelID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) ListClientHotels(ctx context.Context, clientID int) ([]ClientHotel, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id, client_id, internal_hotel_id, label, created_at FROM client_hotels WHERE client_id=$1 ORDER BY created_at DESC`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ClientHotel{}
	for rows.Next() {
		var h ClientHotel
		if err := rows.Scan(&h.ID, &h.ClientID, &h.InternalHotelID, &h.Label, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (d *DB) ClientOwnsHotel(ctx context.Context, clientID int, internalHotelID string) (bool, error) {
	var ok bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM client_hotels WHERE client_id=$1 AND internal_hotel_id=$2)`, clientID, internalHotelID).Scan(&ok)
	return ok, err
}

// ---- Aramalar ----

type Search struct {
	ID         int64           `json:"id"`
	ClientID   int             `json:"clientId"`
	UserID     *int            `json:"userId"`
	ScheduleID *int            `json:"scheduleId"`
	Kind       string          `json:"kind"`
	Request    json.RawMessage `json:"request"`
	JobID      *string         `json:"jobId"`
	Status     string          `json:"status"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      *string         `json:"error"`
	CreatedAt  time.Time       `json:"createdAt"`
	FinishedAt *time.Time      `json:"finishedAt"`
}

const searchCols = `id, client_id, user_id, schedule_id, kind, request, job_id, status, result, error, created_at, finished_at`

func scanSearch(row pgx.Row) (*Search, error) {
	var s Search
	var req, res []byte
	if err := row.Scan(&s.ID, &s.ClientID, &s.UserID, &s.ScheduleID, &s.Kind, &req, &s.JobID, &s.Status, &res, &s.Error, &s.CreatedAt, &s.FinishedAt); err != nil {
		return nil, err
	}
	s.Request = json.RawMessage(req)
	if len(res) > 0 {
		s.Result = json.RawMessage(res)
	}
	return &s, nil
}

func (d *DB) CreateSearch(ctx context.Context, clientID int, userID, scheduleID *int, kind string, request any) (*Search, error) {
	b, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return scanSearch(d.Pool.QueryRow(ctx, `INSERT INTO searches (client_id, user_id, schedule_id, kind, request) VALUES ($1,$2,$3,$4,$5) RETURNING `+searchCols,
		clientID, userID, scheduleID, kind, b))
}

func (d *DB) UpdateSearch(ctx context.Context, id int64, status string, result any, errMsg *string, finished bool) error {
	var res []byte
	if result != nil {
		res, _ = json.Marshal(result)
	}
	var fin any
	if finished {
		fin = time.Now().UTC()
	}
	_, err := d.Pool.Exec(ctx, `UPDATE searches SET status=$1, result=COALESCE($2, result), error=$3, finished_at=COALESCE($4, finished_at) WHERE id=$5`, status, res, errMsg, fin, id)
	return err
}

func (d *DB) GetSearch(ctx context.Context, clientID int, id int64) (*Search, error) {
	s, err := scanSearch(d.Pool.QueryRow(ctx, `SELECT `+searchCols+` FROM searches WHERE id=$1 AND client_id=$2`, id, clientID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// GetSearchByID musteri kisiti olmadan (admin) tek arama, sonuc govdesiyle.
func (d *DB) GetSearchByID(ctx context.Context, id int64) (*Search, error) {
	s, err := scanSearch(d.Pool.QueryRow(ctx, `SELECT `+searchCols+` FROM searches WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// ListSearches musterinin aramalari (sonuc govdesi haric, hafif liste). scheduleID verilirse filtreler.
func (d *DB) ListSearches(ctx context.Context, clientID int, scheduleID *int, limit int) ([]Search, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, client_id, user_id, schedule_id, kind, request, job_id, status, NULL::jsonb, error, created_at, finished_at FROM searches WHERE client_id=$1`
	args := []any{clientID}
	if scheduleID != nil {
		args = append(args, *scheduleID)
		q += fmt.Sprintf(` AND schedule_id=$%d`, len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, len(args))
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Search{}
	for rows.Next() {
		s, err := scanSearch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// UnfinishedSearches acilista yarim kalan aramalar (status queued/running).
func (d *DB) UnfinishedSearches(ctx context.Context) ([]int64, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id FROM searches WHERE status IN ('queued','running')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (d *DB) FailSearches(ctx context.Context, ids []int64, msg string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := d.Pool.Exec(ctx, `UPDATE searches SET status='failed', error=$1, finished_at=now() WHERE id = ANY($2)`, msg, ids)
	return err
}

// ---- Planli aramalar ----

type Schedule struct {
	ID        int             `json:"id"`
	ClientID  int             `json:"clientId"`
	UserID    *int            `json:"userId"`
	Name      string          `json:"name"`
	Kind      string          `json:"kind"`
	Request   json.RawMessage `json:"request"`
	Weekdays  []int16         `json:"weekdays"`
	RunTime   string          `json:"runTime"`
	Timezone  string          `json:"timezone"`
	IsActive  bool            `json:"isActive"`
	LastRunAt *time.Time      `json:"lastRunAt"`
	NextRunAt *time.Time      `json:"nextRunAt"`
	StartsOn  *string         `json:"startsOn"` // YYYY-MM-DD (bos = hemen)
	EndsOn    *string         `json:"endsOn"`   // YYYY-MM-DD (bos = suresiz)
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

const scheduleCols = `id, client_id, user_id, name, kind, request, weekdays, run_time, timezone, is_active, last_run_at, next_run_at, to_char(starts_on,'YYYY-MM-DD'), to_char(ends_on,'YYYY-MM-DD'), created_at, updated_at`

func scanSchedule(row pgx.Row) (*Schedule, error) {
	var s Schedule
	var req []byte
	if err := row.Scan(&s.ID, &s.ClientID, &s.UserID, &s.Name, &s.Kind, &req, &s.Weekdays, &s.RunTime, &s.Timezone, &s.IsActive, &s.LastRunAt, &s.NextRunAt, &s.StartsOn, &s.EndsOn, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	s.Request = json.RawMessage(req)
	if s.Weekdays == nil {
		s.Weekdays = []int16{}
	}
	return &s, nil
}

type ScheduleInput struct {
	Name     string
	Kind     string
	Request  any
	Weekdays []int16
	RunTime  string
	Timezone string
	IsActive bool
	NextRun  *time.Time
	StartsOn *string // YYYY-MM-DD
	EndsOn   *string
}

func (d *DB) CreateSchedule(ctx context.Context, clientID int, userID *int, in ScheduleInput) (*Schedule, error) {
	b, err := json.Marshal(in.Request)
	if err != nil {
		return nil, err
	}
	return scanSchedule(d.Pool.QueryRow(ctx, `INSERT INTO schedules (client_id, user_id, name, kind, request, weekdays, run_time, timezone, is_active, next_run_at, starts_on, ends_on)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::date,$12::date) RETURNING `+scheduleCols,
		clientID, userID, in.Name, in.Kind, b, in.Weekdays, in.RunTime, in.Timezone, in.IsActive, in.NextRun, in.StartsOn, in.EndsOn))
}

func (d *DB) UpdateSchedule(ctx context.Context, clientID, id int, in ScheduleInput) (*Schedule, error) {
	b, err := json.Marshal(in.Request)
	if err != nil {
		return nil, err
	}
	s, err := scanSchedule(d.Pool.QueryRow(ctx, `UPDATE schedules SET name=$1, kind=$2, request=$3, weekdays=$4, run_time=$5, timezone=$6, is_active=$7, next_run_at=$8, starts_on=$11::date, ends_on=$12::date, updated_at=now()
WHERE id=$9 AND client_id=$10 RETURNING `+scheduleCols,
		in.Name, in.Kind, b, in.Weekdays, in.RunTime, in.Timezone, in.IsActive, in.NextRun, id, clientID, in.StartsOn, in.EndsOn))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (d *DB) DeleteSchedule(ctx context.Context, clientID, id int) error {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM schedules WHERE id=$1 AND client_id=$2`, id, clientID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetScheduleActive planli aramayi pasife alir / aktifler (silme yerine).
func (d *DB) SetScheduleActive(ctx context.Context, clientID, id int, active bool, next *time.Time) error {
	tag, err := d.Pool.Exec(ctx, `UPDATE schedules SET is_active=$1, next_run_at=COALESCE($2, next_run_at), updated_at=now() WHERE id=$3 AND client_id=$4`, active, next, id, clientID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetScheduleByID musteri kisiti olmadan (admin).
func (d *DB) GetScheduleByID(ctx context.Context, id int) (*Schedule, error) {
	s, err := scanSchedule(d.Pool.QueryRow(ctx, `SELECT `+scheduleCols+` FROM schedules WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (d *DB) GetSchedule(ctx context.Context, clientID, id int) (*Schedule, error) {
	s, err := scanSchedule(d.Pool.QueryRow(ctx, `SELECT `+scheduleCols+` FROM schedules WHERE id=$1 AND client_id=$2`, id, clientID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// ListSchedules clientID 0 ise tum musteriler (admin).
func (d *DB) ListSchedules(ctx context.Context, clientID int) ([]Schedule, error) {
	q := `SELECT ` + scheduleCols + ` FROM schedules`
	args := []any{}
	if clientID > 0 {
		q += ` WHERE client_id=$1`
		args = append(args, clientID)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// DueSchedules calisma zamani gelmis aktif planlar.
func (d *DB) DueSchedules(ctx context.Context, now time.Time) ([]Schedule, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+scheduleCols+` FROM schedules WHERE is_active AND next_run_at IS NOT NULL AND next_run_at <= $1
  AND (starts_on IS NULL OR starts_on <= ($1 AT TIME ZONE 'Europe/Istanbul')::date)
  AND (ends_on IS NULL OR ends_on >= ($1 AT TIME ZONE 'Europe/Istanbul')::date) ORDER BY next_run_at`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (d *DB) MarkScheduleRun(ctx context.Context, id int, ranAt, next time.Time) error {
	_, err := d.Pool.Exec(ctx, `UPDATE schedules SET last_run_at=$1, next_run_at=$2, updated_at=now() WHERE id=$3`, ranAt, next, id)
	return err
}
