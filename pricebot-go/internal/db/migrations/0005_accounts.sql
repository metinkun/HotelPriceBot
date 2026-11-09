-- Urun istekleri: paket limitleri (tur, site/otel/kullanici sayisi, gunluk arama ve planli gorev),
-- musteri bazinda ozellestirilebilir limitler, ACENTE/OTEL turu, demo suresi, cihaz dogrulama (WebAuthn),
-- giris loglari, onay bekleyen islemler, site (provider) ayarlari, otel kunyesi.

-- ---- Paketler ----
ALTER TABLE plans ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'both';               -- acente | otel | both
ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_sites INTEGER NOT NULL DEFAULT 0;            -- 0 = sinirsiz
ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_hotels INTEGER NOT NULL DEFAULT 0;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_users INTEGER NOT NULL DEFAULT 0;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS daily_searches INTEGER NOT NULL DEFAULT 0;       -- bir arama kac siteye giderse gitsin 1
ALTER TABLE plans ADD COLUMN IF NOT EXISTS daily_scheduled_runs INTEGER NOT NULL DEFAULT 0; -- gunde calisabilecek planli arama
ALTER TABLE plans ADD COLUMN IF NOT EXISTS allow_api BOOLEAN NOT NULL DEFAULT true;         -- API anahtari uretilebilir
ALTER TABLE plans ADD COLUMN IF NOT EXISTS is_demo BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS demo_days INTEGER NOT NULL DEFAULT 14;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_devices_pc INTEGER NOT NULL DEFAULT 2;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_devices_mobile INTEGER NOT NULL DEFAULT 2;

-- ---- Musteriler ----
ALTER TABLE clients ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'acente';           -- acente | otel
ALTER TABLE clients ADD COLUMN IF NOT EXISTS limits JSONB;                                  -- NULL = paket degerleri; dolu = atamada kopyalanan (+ ozellestirilen)
ALTER TABLE clients ADD COLUMN IF NOT EXISTS limits_customized BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS own_hotel_id TEXT;                             -- OTEL turu: musterinin kendi oteli
ALTER TABLE clients ADD COLUMN IF NOT EXISTS demo_expires_at TIMESTAMPTZ;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS require_device_auth BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS plan_assigned_at TIMESTAMPTZ;

-- Gunluk sayaclar (musteri bazinda): arama, planli calisma, API isi, site cagrisi
CREATE TABLE IF NOT EXISTS client_daily (
    day DATE NOT NULL,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    searches INTEGER NOT NULL DEFAULT 0,
    scheduled_runs INTEGER NOT NULL DEFAULT 0,
    api_jobs INTEGER NOT NULL DEFAULT 0,
    site_calls INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (day, client_id)
);

-- ---- Giris loglari (portal) ----
CREATE TABLE IF NOT EXISTS login_logs (
    id BIGSERIAL PRIMARY KEY,
    at TIMESTAMPTZ NOT NULL DEFAULT now(),
    client_id INTEGER,
    user_id INTEGER,
    email TEXT,
    ip TEXT,
    user_agent TEXT,
    device_id INTEGER,
    device_kind TEXT,                 -- pc | mobile
    result TEXT NOT NULL,             -- ok | bad_password | inactive | expired | device_required | device_limit | webauthn_failed | removed_device
    detail TEXT
);
CREATE INDEX IF NOT EXISTS login_logs_at_idx ON login_logs(at DESC);
CREATE INDEX IF NOT EXISTS login_logs_client_idx ON login_logs(client_id, at DESC);

-- ---- Kayitli cihazlar (WebAuthn / passkey) ----
ALTER TABLE portal_users ADD COLUMN IF NOT EXISTS webauthn_id BYTEA;
CREATE TABLE IF NOT EXISTS portal_devices (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES portal_users(id) ON DELETE CASCADE,
    client_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'pc',  -- pc | mobile
    credential_id BYTEA NOT NULL,
    public_key BYTEA NOT NULL,
    sign_count BIGINT NOT NULL DEFAULT 0,
    aaguid BYTEA,
    transports TEXT[] NOT NULL DEFAULT '{}',
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    removed_by TEXT                   -- user | admin | approval
);
CREATE UNIQUE INDEX IF NOT EXISTS portal_devices_cred_idx ON portal_devices(credential_id);
CREATE INDEX IF NOT EXISTS portal_devices_user_idx ON portal_devices(user_id, removed_at);

-- ---- Onay bekleyen islemler ----
CREATE TABLE IF NOT EXISTS approval_requests (
    id SERIAL PRIMARY KEY,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    user_id INTEGER,
    type TEXT NOT NULL,               -- device_remove | device_replace
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending', -- pending | approved | rejected
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ,
    decided_by TEXT
);
CREATE INDEX IF NOT EXISTS approval_requests_status_idx ON approval_requests(status, created_at DESC);

-- ---- Site (provider) ayarlari: genel ac/kapa, gorunen ad, siralama ----
CREATE TABLE IF NOT EXISTS provider_settings (
    name TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT true,
    display_name TEXT,
    sort_order INTEGER NOT NULL DEFAULT 100,
    note TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---- Otel kunyesi (eslestirmelerden bagimsiz ana kayit) ----
CREATE TABLE IF NOT EXISTS hotels (
    internal_hotel_id TEXT PRIMARY KEY,
    name TEXT,
    city TEXT,
    region TEXT,
    country TEXT,
    stars SMALLINT,
    chain TEXT,
    external_id TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO hotels (internal_hotel_id, name, external_id, country)
SELECT m.internal_hotel_id, max(m.hotel_name),
       CASE WHEN m.internal_hotel_id LIKE 'ext_%' THEN substr(m.internal_hotel_id, 5) END,
       CASE WHEN m.internal_hotel_id LIKE 'ext_%' THEN 'TRNC' END
FROM hotel_mappings m GROUP BY m.internal_hotel_id
ON CONFLICT (internal_hotel_id) DO NOTHING;
