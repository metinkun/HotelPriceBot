package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"pricebot/internal/model"
)

// ErrNotFound kayit bulunamadi.
var ErrNotFound = errors.New("kayit bulunamadi")

// IsUniqueViolation Postgres 23505 (Prisma P2002 karsiligi).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

const mappingCols = `id, provider, internal_hotel_id, provider_hotel_id, hotel_name, metadata, is_active, created_at, updated_at`

func scanMapping(row pgx.Row) (*model.HotelMapping, error) {
	var m model.HotelMapping
	var meta []byte
	if err := row.Scan(&m.ID, &m.Provider, &m.InternalHotelID, &m.ProviderHotelID, &m.HotelName, &meta, &m.IsActive, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	if len(meta) > 0 && string(meta) != "null" {
		_ = json.Unmarshal(meta, &m.Metadata)
	}
	return &m, nil
}

func collectMappings(rows pgx.Rows) ([]model.HotelMapping, error) {
	defer rows.Close()
	out := []model.HotelMapping{}
	for rows.Next() {
		m, err := scanMapping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func metaArg(meta map[string]any) any {
	if meta == nil {
		return nil
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return nil
	}
	return b
}

// MappingFilter listeleme filtresi.
type MappingFilter struct {
	Provider        string
	InternalHotelID string
	Search          string
	Active          *bool
	OrderBy         string // "created_at DESC" (varsayilan) veya "provider ASC"
}

// ListMappings filtreye gore eslestirmeleri listeler.
func (d *DB) ListMappings(ctx context.Context, f MappingFilter) ([]model.HotelMapping, error) {
	where := []string{"1=1"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Provider != "" {
		add("provider = $%d", f.Provider)
	}
	if f.InternalHotelID != "" {
		add("internal_hotel_id = $%d", f.InternalHotelID)
	}
	if f.Active != nil {
		add("is_active = $%d", *f.Active)
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		n := len(args)
		where = append(where, fmt.Sprintf("(hotel_name ILIKE $%d OR internal_hotel_id LIKE $%d OR provider_hotel_id LIKE $%d)", n, n, n))
	}
	order := f.OrderBy
	if order == "" {
		order = "created_at DESC"
	}
	q := fmt.Sprintf(`SELECT %s FROM hotel_mappings WHERE %s ORDER BY %s`, mappingCols, strings.Join(where, " AND "), order)
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return collectMappings(rows)
}

// GetMapping provider + internalHotelId benzersiz anahtariyla tek kayit (yoksa ErrNotFound).
func (d *DB) GetMapping(ctx context.Context, provider, internalHotelID string) (*model.HotelMapping, error) {
	row := d.Pool.QueryRow(ctx, `SELECT `+mappingCols+` FROM hotel_mappings WHERE provider=$1 AND internal_hotel_id=$2`, provider, internalHotelID)
	m, err := scanMapping(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// ActiveMappingsForHotel bir otelin aktif eslestirmeleri; providers bos degilse filtreler.
func (d *DB) ActiveMappingsForHotel(ctx context.Context, internalHotelID string, providers []string) ([]model.HotelMapping, error) {
	q := `SELECT ` + mappingCols + ` FROM hotel_mappings WHERE internal_hotel_id=$1 AND is_active=true`
	args := []any{internalHotelID}
	if len(providers) > 0 {
		q += ` AND provider = ANY($2)`
		args = append(args, providers)
	}
	q += ` ORDER BY provider ASC`
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return collectMappings(rows)
}

// ActiveMappingsForHotels bir provider'in verilen otel listesi icin aktif eslestirmeleri.
func (d *DB) ActiveMappingsForHotels(ctx context.Context, provider string, internalHotelIDs []string) ([]model.HotelMapping, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+mappingCols+` FROM hotel_mappings WHERE provider=$1 AND internal_hotel_id = ANY($2) AND is_active=true`, provider, internalHotelIDs)
	if err != nil {
		return nil, err
	}
	return collectMappings(rows)
}

// MappingUpsert upsert girdisi.
type MappingUpsert struct {
	Provider        string
	InternalHotelID string
	ProviderHotelID string
	HotelName       *string
	Metadata        map[string]any
	IsActive        bool
}

const upsertSQL = `
INSERT INTO hotel_mappings (provider, internal_hotel_id, provider_hotel_id, hotel_name, metadata, is_active, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$7)
ON CONFLICT (provider, internal_hotel_id) DO UPDATE SET
  provider_hotel_id = EXCLUDED.provider_hotel_id,
  hotel_name = EXCLUDED.hotel_name,
  metadata = EXCLUDED.metadata,
  is_active = EXCLUDED.is_active,
  updated_at = EXCLUDED.updated_at
RETURNING ` + mappingCols

// UpsertMapping (provider, internalHotelId) uzerinden ekler/gunceller.
func (d *DB) UpsertMapping(ctx context.Context, in MappingUpsert) (*model.HotelMapping, error) {
	row := d.Pool.QueryRow(ctx, upsertSQL, in.Provider, in.InternalHotelID, in.ProviderHotelID, in.HotelName, metaArg(in.Metadata), in.IsActive, time.Now().UTC())
	return scanMapping(row)
}

// UpsertMappingsTx birden fazla upsert'i tek transaction'da yapar.
func (d *DB) UpsertMappingsTx(ctx context.Context, ins []MappingUpsert) ([]model.HotelMapping, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	out := make([]model.HotelMapping, 0, len(ins))
	now := time.Now().UTC()
	for _, in := range ins {
		row := tx.QueryRow(ctx, upsertSQL, in.Provider, in.InternalHotelID, in.ProviderHotelID, in.HotelName, metaArg(in.Metadata), in.IsActive, now)
		m, err := scanMapping(row)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateMapping yeni kayit ekler (benzersizlik ihlalinde IsUniqueViolation true).
func (d *DB) CreateMapping(ctx context.Context, in MappingUpsert) (*model.HotelMapping, error) {
	row := d.Pool.QueryRow(ctx, `
INSERT INTO hotel_mappings (provider, internal_hotel_id, provider_hotel_id, hotel_name, metadata, is_active, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$7) RETURNING `+mappingCols,
		in.Provider, in.InternalHotelID, in.ProviderHotelID, in.HotelName, metaArg(in.Metadata), in.IsActive, time.Now().UTC())
	return scanMapping(row)
}

// MappingPatch kismi guncelleme; nil alanlar dokunulmaz. MetadataSet true ise Metadata yazilir (nil => NULL).
type MappingPatch struct {
	InternalHotelID *string
	ProviderHotelID *string
	HotelName       *string
	HotelNameSet    bool
	Metadata        map[string]any
	MetadataSet     bool
	IsActive        *bool
}

// UpdateMappingByID id ile gunceller (yoksa ErrNotFound).
func (d *DB) UpdateMappingByID(ctx context.Context, id int, p MappingPatch) (*model.HotelMapping, error) {
	sets := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if p.InternalHotelID != nil {
		add("internal_hotel_id", *p.InternalHotelID)
	}
	if p.ProviderHotelID != nil {
		add("provider_hotel_id", *p.ProviderHotelID)
	}
	if p.HotelNameSet {
		add("hotel_name", p.HotelName)
	}
	if p.MetadataSet {
		add("metadata", metaArg(p.Metadata))
	}
	if p.IsActive != nil {
		add("is_active", *p.IsActive)
	}
	add("updated_at", time.Now().UTC())
	args = append(args, id)
	q := fmt.Sprintf(`UPDATE hotel_mappings SET %s WHERE id = $%d RETURNING %s`, strings.Join(sets, ", "), len(args), mappingCols)
	m, err := scanMapping(d.Pool.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// UpdateHotelName bir otelin tum satirlarinda adi gunceller.
func (d *DB) UpdateHotelName(ctx context.Context, internalHotelID string, name *string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE hotel_mappings SET hotel_name=$1, updated_at=$2 WHERE internal_hotel_id=$3`, name, time.Now().UTC(), internalHotelID)
	return err
}

// DeleteMappingByID (yoksa ErrNotFound).
func (d *DB) DeleteMappingByID(ctx context.Context, id int) error {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM hotel_mappings WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteMappingsForHotel otelin tum provider satirlarini siler; silinen sayisini doner.
func (d *DB) DeleteMappingsForHotel(ctx context.Context, internalHotelID string) (int64, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM hotel_mappings WHERE internal_hotel_id=$1`, internalHotelID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
