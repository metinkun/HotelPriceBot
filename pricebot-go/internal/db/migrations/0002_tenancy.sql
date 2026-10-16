-- Coklu musteri: planlar, musteriler, API anahtarlari, adminler, kullanim ve istek loglari.
CREATE TABLE IF NOT EXISTS plans (
    id SERIAL PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    daily_quota INTEGER NOT NULL DEFAULT 0,            -- 0 = sinirsiz (kredi/gun; 1 kredi = 1 site cagrisi)
    allowed_providers TEXT[] NOT NULL DEFAULT '{}',    -- bos = tum provider'lar
    allow_packages BOOLEAN NOT NULL DEFAULT true,
    allow_fresh BOOLEAN NOT NULL DEFAULT false,        -- cache'i atlama (fresh=true) yetkisi
    max_concurrency INTEGER NOT NULL DEFAULT 5,
    rate_per_minute INTEGER NOT NULL DEFAULT 60,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS clients (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT,
    notes TEXT,
    plan_id INTEGER REFERENCES plans(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS api_keys (
    id SERIAL PRIMARY KEY,
    client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    key_hash TEXT UNIQUE NOT NULL,                     -- sha256(hex); duz metin saklanmaz
    key_prefix TEXT NOT NULL,                          -- gosterim icin ilk 12 karakter
    label TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS api_keys_client_idx ON api_keys(client_id);

CREATE TABLE IF NOT EXISTS admins (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    name TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS usage_daily (
    day DATE NOT NULL,
    client_id INTEGER NOT NULL,
    api_key_id INTEGER NOT NULL,
    provider TEXT NOT NULL,                            -- '*' = istek sayaci (provider'siz)
    credits INTEGER NOT NULL DEFAULT 0,
    requests INTEGER NOT NULL DEFAULT 0,
    cache_hits INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (day, client_id, api_key_id, provider)
);
CREATE INDEX IF NOT EXISTS usage_daily_client_idx ON usage_daily(client_id, day);

CREATE TABLE IF NOT EXISTS request_logs (
    id BIGSERIAL PRIMARY KEY,
    at TIMESTAMPTZ NOT NULL DEFAULT now(),
    request_id TEXT,
    client_id INTEGER,
    api_key_id INTEGER,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    status INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    ip TEXT,
    error TEXT
);
CREATE INDEX IF NOT EXISTS request_logs_at_idx ON request_logs(at DESC);
CREATE INDEX IF NOT EXISTS request_logs_client_idx ON request_logs(client_id, at DESC);

INSERT INTO plans (code, name, daily_quota, allowed_providers, allow_packages, allow_fresh, max_concurrency, rate_per_minute)
VALUES
  ('free',      'Deneme',    100,  '{}', false, false, 2, 20),
  ('standard',  'Standart',  2000, '{}', true,  false, 5, 60),
  ('unlimited', 'Sinirsiz',  0,    '{}', true,  true,  10, 300)
ON CONFLICT (code) DO NOTHING;
