-- Musteri tipi 2: web portal. Portal kullanicilari, musteri basina etkin siteler, musteri otelleri,
-- aramalar (gecmis) ve planli aramalar.

-- Musteri basina etkin provider listesi (bos = plan varsayilani / tumu). Admin panelden ac/kapa.
ALTER TABLE clients ADD COLUMN IF NOT EXISTS enabled_providers TEXT[] NOT NULL DEFAULT '{}';
-- Musteri tipi: api | portal | both
ALTER TABLE clients ADD COLUMN IF NOT EXISTS client_type TEXT NOT NULL DEFAULT 'api';

CREATE TABLE IF NOT EXISTS portal_users (
    id SERIAL PRIMARY KEY,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    name TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ,
    UNIQUE (email)
);
CREATE INDEX IF NOT EXISTS portal_users_client_idx ON portal_users(client_id);

-- Musterinin kendi otel listesi (hotel_mappings.internal_hotel_id'ye referans; otel mapping'leri globaldir)
CREATE TABLE IF NOT EXISTS client_hotels (
    id SERIAL PRIMARY KEY,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    internal_hotel_id TEXT NOT NULL,
    label TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (client_id, internal_hotel_id)
);

-- Arama gecmisi: anlik veya planli aramanin istegi + sonucu
CREATE TABLE IF NOT EXISTS searches (
    id BIGSERIAL PRIMARY KEY,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    user_id INTEGER,
    schedule_id INTEGER,
    kind TEXT NOT NULL DEFAULT 'prices',          -- prices | packages
    request JSONB NOT NULL,
    job_id TEXT,
    status TEXT NOT NULL DEFAULT 'queued',         -- queued | running | done | failed
    result JSONB,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS searches_client_idx ON searches(client_id, created_at DESC);
CREATE INDEX IF NOT EXISTS searches_schedule_idx ON searches(schedule_id, created_at DESC);

-- Planli aramalar: haftalik gun(ler) + saat (Europe/Istanbul) veya gunluk
CREATE TABLE IF NOT EXISTS schedules (
    id SERIAL PRIMARY KEY,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    user_id INTEGER,
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'prices',
    request JSONB NOT NULL,                        -- hotelIds[], providers[], nightsAhead/checkIn modu, adults...
    weekdays SMALLINT[] NOT NULL DEFAULT '{1}',    -- 0=Pazar ... 6=Cumartesi; bos = her gun
    run_time TEXT NOT NULL DEFAULT '09:00',        -- HH:MM (Europe/Istanbul)
    timezone TEXT NOT NULL DEFAULT 'Europe/Istanbul',
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS schedules_due_idx ON schedules(is_active, next_run_at);
