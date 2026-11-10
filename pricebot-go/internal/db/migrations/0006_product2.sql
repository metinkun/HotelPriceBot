-- Urun istekleri 2: sifre sifirlama linki, planli arama baslangic/bitis tarihi,
-- kullanici bazinda cihaz limiti, "silme yok -> pasif" icin ek alanlar.

-- Sifre sifirlama (admin uretir, e-posta ile gonderilir; tek kullanimlik, sureli)
CREATE TABLE IF NOT EXISTS password_resets (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES portal_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at TIMESTAMPTZ,
    sent_to TEXT,
    sent_at TIMESTAMPTZ,
    send_error TEXT
);
CREATE INDEX IF NOT EXISTS password_resets_user_idx ON password_resets(user_id, created_at DESC);

-- Planli arama gecerlilik araligi (bos = sinirsiz)
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS starts_on DATE;
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS ends_on DATE;

-- Kullanici bazinda cihaz limiti (NULL = musteri/paket degeri)
ALTER TABLE portal_users ADD COLUMN IF NOT EXISTS max_devices_pc INTEGER;
ALTER TABLE portal_users ADD COLUMN IF NOT EXISTS max_devices_mobile INTEGER;

-- Bildirim turu icin indeks (yorum/puan/kampanya/etkinlik bildirimleri)
CREATE INDEX IF NOT EXISTS notifications_kind_idx ON notifications(kind, created_at DESC);
