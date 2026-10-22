# PriceBot (Go)

Birlesik cok-saglayicili otel fiyat botu. `priceBot/` altindaki TypeScript surumunun Go'ya cevrilmis ve
production icin sertlestirilmis halidir. API yollari, JSON alan adlari ve veritabani semasi TS ile uyumludur.

Provider'lar (siteler) ve calisma sekli:

| Site | Fiyat | Paket | Tarayici | Not |
|---|---|---|---|---|
| etstur | ✓ | ✓ | yok | `room/multi` uclari |
| tatilsepeti | ✓ | ✓ | yedek | oturum otomatik (HTTP); engellenirse Chromium'dan cf_clearance |
| jolly | ✓ | ✓ | yok | |
| tatilbudur | ✓ | ✓ | yok | Akamai + CSRF token (HTTP) |
| touristica | ✓ | ✓ | evet | Cloudflare; kalici sekme havuzu (~2-6 sn) |
| setur | ✓ | ✓ (tek en uygun) | yok | yavas sayfa (20-40 sn) |
| gezinomi | ✓ | ✓ | hafif | x-client-key WASM worker'da uretilir: tek kalici sekme sadece anahtar icin, istekler HTTP |
| enuygun | ✓ | — | yok | JSON API; paket yok (dinamik paket) |
| mngturizm | ✓ | ✓ | yok | Referer sart |
| tatilsitesi | ✓ | ✓ | yok | Methods.aspx JSON; ucus+transfer toplami hesaplanir |
| prontotour | ✓ | ✓ | yok | webapi JSON |
| tatilcikus | ✓ | — | yok | api.tatilcikus.com; tedarikci (Akgunler/Coral/Gezinomi) proxy; paket canli satilmiyor |
| tatildukkani | ✓ | — | yok | api.tatildukkani.com + Socket.IO oda listesi; paket misafire kapali |

Her sitenin ayrintisi: `docs/providers/<site>.md`; yeni site ekleme: `docs/provider-authoring.md`.
Iki musteri tipi (API / web portal) ve admin paneli: `docs/customers.md`.

## Hizli baslangic (Docker)

```bash
cp .env.example .env      # ADMIN_API_KEY, ADMIN_EMAIL/ADMIN_PASSWORD, WEBSHARE_API_KEY, cookie'ler, CANARY_HOTEL_ID
docker compose up -d --build
curl http://localhost:3000/health
open http://localhost:3000/admin/     # admin paneli
open http://localhost:3000/docs       # Swagger
```

Production: bkz. [deploy/README.md](deploy/README.md) (Caddy TLS, kapali portlar, gunluk yedek).

## Mimari

```
cmd/server             giris noktasi
cmd/capture            gelistirme araclari (fixture yakalama, ag trafigi kaydi)
internal/api           router, middleware (request id, CORS, auth), handler'lar, Swagger, admin panel embed
internal/services      hotel / price / package is mantigi, Fetcher (cache + sinirlayici), usage (kredi/kota)
internal/jobs          asenkron is kuyrugu (Redis durum, in-process worker havuzu, tekillestirme, kismi sonuc)
internal/providers     etstur, tatilsepeti, jolly, tatilbudur, touristica, setur, gezinomi + registry (+ fixture testleri)
internal/provider      ortak Provider arayuzu ve sonuc tipleri
internal/cache         Redis cache (provider basina, 5 dk) + singleflight
internal/limiter       provider basina eszamanlilik, devre kesici, saglik istatistikleri
internal/canary        gunluk kanarya sorgusu (site degisikligi erken uyarisi)
internal/db            pgx, sema + migration'lar (hotel_mappings, proxies, plans, clients, api_keys, admins, usage_daily, request_logs)
internal/auth          kimlik context'i, HS256 JWT, anahtar hash
internal/browser       chromedp tarayici yoneticisi (sekme siniri, periyodik yeniden baslatma)
internal/httpx         proxy rotasyonlu HTTP istemcisi (4xx/5xx -> hata)
internal/proxy         Webshare senkronu + round-robin
ui/                    React/Vite admin paneli (Docker'da derlenip Go'ya gomulur, /admin/)
```

## Kimlik dogrulama ve planlar

- **Musteri uclari** (`/api/prices*`, `/api/packages`, `/api/v1/*`) `X-API-Key` ister
  (`REQUIRE_API_KEY=false` ile gelistirmede anonim erisim acilabilir). `ADMIN_API_KEY` super anahtar olarak da gecer.
- **Admin uclari** (`/api/admin/*`) `Authorization: Bearer <JWT>` (panel girisi) veya `x-api-key: ADMIN_API_KEY`.
- Ilk admin `ADMIN_EMAIL` / `ADMIN_PASSWORD` ile olusturulur (admin tablosu bossa).
- **Plan**: gunluk kredi kotasi (0 = sinirsiz), izinli provider listesi (bos = hepsi), paket izni, fresh (cache atlama) izni, istek/dk.
- **Kredi**: bir siteye atilan her gercek sorgu 1 kredi; cache'ten (5 dk) donen sonuc 0 kredi. Yanitta `cached` + `fetchedAt`,
  basliklarda `X-Quota-Limit` / `X-Quota-Used`.
- Anahtarlar `pb_live_...` formatinda; DB'de sadece SHA-256 ozeti saklanir, duz metin yalnizca olusturma yanitinda gosterilir.

## API ozeti

| Yontem | Yol | Aciklama |
|---|---|---|
| GET | /health, /ready | canlilik / hazirlik (Postgres zorunlu, Redis opsiyonel) |
| POST | /api/v1/jobs | **onerilen**: is olustur -> `requestId` (202; ayni istek -> 200 deduplicated) |
| GET | /api/v1/jobs/{id}?wait=20 | durum, provider bazli ilerleme, kismi/tam sonuc (1 saat saklanir) |
| GET | /api/v1/me | anahtar sahibinin plani ve bugunku kullanimi |
| GET | /api/prices, /api/prices/all, POST /api/prices/bulk, GET /api/packages | senkron (geriye uyumlu), `fresh=true` destekler |
| POST | /api/admin/auth/login | admin JWT |
| CRUD | /api/admin/plans, /api/admin/clients, /api/admin/clients/{id}/keys, DELETE /api/admin/keys/{id} | musteri/plan/anahtar yonetimi |
| GET | /api/admin/usage, /api/admin/logs | kullanim raporu, istek loglari |
| GET | /api/admin/providers/health | eszamanlilik, basari/hata, devre kesici, kanarya, cache |
| POST | /api/admin/providers/{name}/reset-circuit, /api/admin/canary/run | operasyon |
| CRUD | /api/admin/hotels, /api/admin/mappings | otel ve eslestirme yonetimi |

Tam sema: `internal/api/openapi.json` (Swagger `/docs`). Postman koleksiyonu: `postman/` (OpenAPI'den uretilir, bkz. `postman/README.md`).

## Gelistirme

```bash
# Go kurulu degilse Docker ile derle/test et
docker run --rm -v "$PWD":/src -w /src golang:1.23-bookworm sh -c 'go vet ./... && go test ./...'
# Admin paneli
cd ui && npm install && npm run dev        # http://localhost:5173/admin/ (API /api -> :3000 proxy)
```

Testler provider normalizer'larini `testdata/` altindaki gercek site yanitlariyla dogrular; bir site degisince once bu testler
ve gunluk kanarya (`CANARY_HOTEL_ID`) uyarir.

## Ortam degiskenleri

Bkz. [.env.example](.env.example). Onemliler: `DATABASE_URL`, `REDIS_URL`, `ADMIN_API_KEY`, `ADMIN_EMAIL/PASSWORD`, `JWT_SECRET`,
`REQUIRE_API_KEY`, `CACHE_TTL_SECONDS`, `PROVIDER_CONCURRENCY`, `BROWSER_MAX_TABS`, `CANARY_HOTEL_ID`, `WEBSHARE_API_KEY`.

## Bilinen durumlar

- Etstur paket API'si (Ekim 2026) `room/multi` + `room/multi/package` uclarina tasindi; odalar `result.roomGroups[].rooms`
  altinda. Kod guncellendi ve canli dogrulandi (bkz. `docs/phase3-research.md`).
- Gezinomi fiyat API'si sunucudan "Oda bulunamadi" donuyor (oturum/IP bagli); resolve calisiyor.
- Touristica Cloudflare arkasinda; headless Chromium ile calisir (`BROWSER_MAX_TABS` ile sinirli). Tarayicisiz hibrit calisma arastirma asamasinda.
