-- Otel statik verisi (acente detay sayfalari): snapshot'lar, degisiklik kayitlari, goruntuleme izleri, bildirimler.

CREATE TABLE IF NOT EXISTS hotel_details (
    id BIGSERIAL PRIMARY KEY,
    internal_hotel_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    content_hash TEXT NOT NULL,                 -- normalize JSON'in sha256'si
    snapshot JSONB NOT NULL,                    -- provider.HotelDetails
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    changed_at TIMESTAMPTZ,                     -- iceriginin onceki surumden farklilastigi an
    error TEXT,                                 -- son cekim hatasi (snapshot eski kalir)
    UNIQUE (internal_hotel_id, provider)
);

-- Her degisiklik: alan bazli diff (path, old, new) + ozet
CREATE TABLE IF NOT EXISTS hotel_detail_changes (
    id BIGSERIAL PRIMARY KEY,
    internal_hotel_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    version INTEGER NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    diff JSONB NOT NULL,                        -- [{path, kind: added|removed|changed, old, new}]
    summary TEXT NOT NULL                        -- "3 gorsel eklendi, aciklama degisti"
);
CREATE INDEX IF NOT EXISTS hotel_detail_changes_hotel_idx ON hotel_detail_changes(internal_hotel_id, provider, changed_at DESC);

-- Degisikligi kim gordu: ilk goruntulemede vurgulanir, sonra vurgu kalkar
CREATE TABLE IF NOT EXISTS hotel_detail_views (
    viewer_kind TEXT NOT NULL,                  -- admin | portal
    viewer_id INTEGER NOT NULL,                 -- admin id veya portal_user id
    internal_hotel_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    seen_version INTEGER NOT NULL,              -- gorulen son surum
    seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (viewer_kind, viewer_id, internal_hotel_id, provider)
);

-- Bildirimler (acente bazli degisiklik bildirimi; admin icin client_id NULL)
CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    client_id INTEGER,                          -- NULL = admin bildirimi
    kind TEXT NOT NULL,                         -- detail_change | ...
    provider TEXT,
    internal_hotel_id TEXT,
    title TEXT NOT NULL,
    body TEXT,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS notifications_client_idx ON notifications(client_id, created_at DESC);
