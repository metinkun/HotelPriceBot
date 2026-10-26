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

// ---- Statik detay snapshot'lari ----

type HotelDetailRow struct {
	ID              int64           `json:"id"`
	InternalHotelID string          `json:"internalHotelId"`
	Provider        string          `json:"provider"`
	Version         int             `json:"version"`
	ContentHash     string          `json:"contentHash"`
	Snapshot        json.RawMessage `json:"snapshot"`
	FetchedAt       time.Time       `json:"fetchedAt"`
	ChangedAt       *time.Time      `json:"changedAt"`
	Error           *string         `json:"error"`
}

const detailCols = `id, internal_hotel_id, provider, version, content_hash, snapshot, fetched_at, changed_at, error`

func scanDetail(row pgx.Row) (*HotelDetailRow, error) {
	var r HotelDetailRow
	var snap []byte
	if err := row.Scan(&r.ID, &r.InternalHotelID, &r.Provider, &r.Version, &r.ContentHash, &snap, &r.FetchedAt, &r.ChangedAt, &r.Error); err != nil {
		return nil, err
	}
	r.Snapshot = json.RawMessage(snap)
	return &r, nil
}

func (d *DB) GetDetail(ctx context.Context, hotelID, provider string) (*HotelDetailRow, error) {
	r, err := scanDetail(d.Pool.QueryRow(ctx, `SELECT `+detailCols+` FROM hotel_details WHERE internal_hotel_id=$1 AND provider=$2`, hotelID, provider))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

// ListDetailsForHotel bir otelin tum acente snapshot'lari (govde haric ozet).
func (d *DB) ListDetailsForHotel(ctx context.Context, hotelID string) ([]HotelDetailRow, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id, internal_hotel_id, provider, version, content_hash, '{}'::jsonb, fetched_at, changed_at, error FROM hotel_details WHERE internal_hotel_id=$1 ORDER BY provider`, hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HotelDetailRow{}
	for rows.Next() {
		r, err := scanDetail(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// SaveDetail yeni/degisen snapshot yazar. changed true ise version artar ve changed_at guncellenir.
func (d *DB) SaveDetail(ctx context.Context, hotelID, provider, hash string, snapshot any, changed bool) (*HotelDetailRow, error) {
	b, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var r *HotelDetailRow
	if changed {
		r, err = scanDetail(d.Pool.QueryRow(ctx, `INSERT INTO hotel_details (internal_hotel_id, provider, version, content_hash, snapshot, fetched_at, changed_at, error)
VALUES ($1,$2,1,$3,$4,$5,$5,NULL)
ON CONFLICT (internal_hotel_id, provider) DO UPDATE SET version=hotel_details.version+1, content_hash=EXCLUDED.content_hash, snapshot=EXCLUDED.snapshot, fetched_at=EXCLUDED.fetched_at, changed_at=EXCLUDED.fetched_at, error=NULL
RETURNING `+detailCols, hotelID, provider, hash, b, now))
	} else {
		r, err = scanDetail(d.Pool.QueryRow(ctx, `UPDATE hotel_details SET fetched_at=$3, error=NULL WHERE internal_hotel_id=$1 AND provider=$2 RETURNING `+detailCols, hotelID, provider, now))
	}
	return r, err
}

func (d *DB) SetDetailError(ctx context.Context, hotelID, provider, msg string) {
	_, _ = d.Pool.Exec(ctx, `INSERT INTO hotel_details (internal_hotel_id, provider, version, content_hash, snapshot, error)
VALUES ($1,$2,0,'', '{}'::jsonb, $3)
ON CONFLICT (internal_hotel_id, provider) DO UPDATE SET error=EXCLUDED.error`, hotelID, provider, msg)
}

// ---- Degisiklikler ----

type DetailChange struct {
	ID              int64           `json:"id"`
	InternalHotelID string          `json:"internalHotelId"`
	Provider        string          `json:"provider"`
	Version         int             `json:"version"`
	ChangedAt       time.Time       `json:"changedAt"`
	Diff            json.RawMessage `json:"diff"`
	Summary         string          `json:"summary"`
}

func (d *DB) AddDetailChange(ctx context.Context, hotelID, provider string, version int, diff any, summary string) (*DetailChange, error) {
	b, _ := json.Marshal(diff)
	var c DetailChange
	var raw []byte
	err := d.Pool.QueryRow(ctx, `INSERT INTO hotel_detail_changes (internal_hotel_id, provider, version, diff, summary) VALUES ($1,$2,$3,$4,$5)
RETURNING id, internal_hotel_id, provider, version, changed_at, diff, summary`, hotelID, provider, version, b, summary).
		Scan(&c.ID, &c.InternalHotelID, &c.Provider, &c.Version, &c.ChangedAt, &raw, &c.Summary)
	c.Diff = json.RawMessage(raw)
	return &c, err
}

// LatestDetailChange otel+acente icin son degisiklik (yoksa ErrNotFound).
func (d *DB) LatestDetailChange(ctx context.Context, hotelID, provider string) (*DetailChange, error) {
	var c DetailChange
	var raw []byte
	err := d.Pool.QueryRow(ctx, `SELECT id, internal_hotel_id, provider, version, changed_at, diff, summary FROM hotel_detail_changes WHERE internal_hotel_id=$1 AND provider=$2 ORDER BY version DESC LIMIT 1`, hotelID, provider).
		Scan(&c.ID, &c.InternalHotelID, &c.Provider, &c.Version, &c.ChangedAt, &raw, &c.Summary)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	c.Diff = json.RawMessage(raw)
	return &c, err
}

// ListDetailChanges hotelID bos ise tum oteller; limit ile.
func (d *DB) ListDetailChanges(ctx context.Context, hotelID string, hotelIDs []string, limit int) ([]DetailChange, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id, internal_hotel_id, provider, version, changed_at, diff, summary FROM hotel_detail_changes`
	args := []any{}
	where := []string{}
	if hotelID != "" {
		args = append(args, hotelID)
		where = append(where, fmt.Sprintf("internal_hotel_id=$%d", len(args)))
	}
	if hotelIDs != nil {
		args = append(args, hotelIDs)
		where = append(where, fmt.Sprintf("internal_hotel_id = ANY($%d)", len(args)))
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY changed_at DESC LIMIT $%d", len(args))
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DetailChange{}
	for rows.Next() {
		var c DetailChange
		var raw []byte
		if err := rows.Scan(&c.ID, &c.InternalHotelID, &c.Provider, &c.Version, &c.ChangedAt, &raw, &c.Summary); err != nil {
			return nil, err
		}
		c.Diff = json.RawMessage(raw)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---- Goruntuleme izleri ----

// SeenVersion kullanicinin bu otel+acente icin gordugu son surum (0 = hic gormedi).
func (d *DB) SeenVersion(ctx context.Context, viewerKind string, viewerID int, hotelID, provider string) int {
	var v int
	err := d.Pool.QueryRow(ctx, `SELECT seen_version FROM hotel_detail_views WHERE viewer_kind=$1 AND viewer_id=$2 AND internal_hotel_id=$3 AND provider=$4`, viewerKind, viewerID, hotelID, provider).Scan(&v)
	if err != nil {
		return 0
	}
	return v
}

func (d *DB) MarkSeen(ctx context.Context, viewerKind string, viewerID int, hotelID, provider string, version int) error {
	_, err := d.Pool.Exec(ctx, `INSERT INTO hotel_detail_views (viewer_kind, viewer_id, internal_hotel_id, provider, seen_version, seen_at) VALUES ($1,$2,$3,$4,$5,now())
ON CONFLICT (viewer_kind, viewer_id, internal_hotel_id, provider) DO UPDATE SET seen_version=EXCLUDED.seen_version, seen_at=now()`, viewerKind, viewerID, hotelID, provider, version)
	return err
}

// ---- Bildirimler ----

type Notification struct {
	ID              int64           `json:"id"`
	ClientID        *int            `json:"clientId"`
	Kind            string          `json:"kind"`
	Provider        *string         `json:"provider"`
	InternalHotelID *string         `json:"internalHotelId"`
	Title           string          `json:"title"`
	Body            *string         `json:"body"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	ReadAt          *time.Time      `json:"readAt"`
}

func (d *DB) AddNotification(ctx context.Context, clientID *int, kind, provider, hotelID, title, body string, payload any) error {
	var pb []byte
	if payload != nil {
		pb, _ = json.Marshal(payload)
	}
	_, err := d.Pool.Exec(ctx, `INSERT INTO notifications (client_id, kind, provider, internal_hotel_id, title, body, payload) VALUES ($1,$2,$3,$4,$5,$6,$7)`, clientID, kind, provider, hotelID, title, body, pb)
	return err
}

// ListNotifications clientID nil ise admin bildirimleri (client_id IS NULL).
func (d *DB) ListNotifications(ctx context.Context, clientID *int, unreadOnly bool, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, client_id, kind, provider, internal_hotel_id, title, body, payload, created_at, read_at FROM notifications WHERE `
	args := []any{}
	if clientID == nil {
		q += `client_id IS NULL`
	} else {
		args = append(args, *clientID)
		q += `client_id=$1`
	}
	if unreadOnly {
		q += ` AND read_at IS NULL`
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, len(args))
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		var n Notification
		var pb []byte
		if err := rows.Scan(&n.ID, &n.ClientID, &n.Kind, &n.Provider, &n.InternalHotelID, &n.Title, &n.Body, &pb, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, err
		}
		if len(pb) > 0 {
			n.Payload = json.RawMessage(pb)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (d *DB) CountUnread(ctx context.Context, clientID *int) int {
	var n int
	if clientID == nil {
		_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE client_id IS NULL AND read_at IS NULL`).Scan(&n)
	} else {
		_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE client_id=$1 AND read_at IS NULL`, *clientID).Scan(&n)
	}
	return n
}

// MarkNotificationsRead ids bos ise hepsini okundu yapar.
func (d *DB) MarkNotificationsRead(ctx context.Context, clientID *int, ids []int64) error {
	var err error
	switch {
	case clientID == nil && len(ids) == 0:
		_, err = d.Pool.Exec(ctx, `UPDATE notifications SET read_at=now() WHERE client_id IS NULL AND read_at IS NULL`)
	case clientID == nil:
		_, err = d.Pool.Exec(ctx, `UPDATE notifications SET read_at=now() WHERE client_id IS NULL AND id = ANY($1)`, ids)
	case len(ids) == 0:
		_, err = d.Pool.Exec(ctx, `UPDATE notifications SET read_at=now() WHERE client_id=$1 AND read_at IS NULL`, *clientID)
	default:
		_, err = d.Pool.Exec(ctx, `UPDATE notifications SET read_at=now() WHERE client_id=$1 AND id = ANY($2)`, *clientID, ids)
	}
	return err
}

// ClientsForHotel oteli listesinde bulunduran musteri id'leri.
func (d *DB) ClientsForHotel(ctx context.Context, hotelID string) ([]int, error) {
	rows, err := d.Pool.Query(ctx, `SELECT client_id FROM client_hotels WHERE internal_hotel_id=$1`, hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
