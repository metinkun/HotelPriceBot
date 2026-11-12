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

## Surum

Tek kaynak `VERSION` dosyasidir (semver, orn. `1.1.0`). Dockerfile bunu ve `GIT_COMMIT` build arg'ini binary'ye gomer;
`/health` yaniti `version`, `commit`, `builtAt` dondurur ve admin/portal arayuzleri sol menunun altinda ve giris ekraninda
silik olarak `v1.1.0` gosterir (commit yalnizca `/health` yanitinda). Yeni surum cikarirken:

```bash
echo 1.2.0 > VERSION && git commit -am "v1.2.0" && git tag v1.2.0 && git push && git push --tags
GIT_COMMIT=$(git rev-parse --short HEAD) docker compose up -d --build   # commit bos birakilirsa yalnizca surum gorunur
curl -s http://localhost:3000/health | grep -o '"version":"[^"]*"'
```

## Mimari

```
cmd/server             giris noktasi
cmd/capture            gelistirme araclari (fixture yakalama, ag trafigi kaydi)
internal/api           router, middleware (request id, CORS, auth), handler'lar, Swagger, admin panel embed
internal/services      hotel / price / package is mantigi, Fetcher (cache + sinirlayici), usage (kredi/kota), importer (toplu otel)
internal/details       statik otel detayi: gunluk cekim, hash/diff, degisiklik bildirimi, ilk goruntulemede vurgu
internal/portal        musteri portali: aramalar (job uzerinden) ve planli aramalar (30 sn zamanlayici)
internal/jobs          asenkron is kuyrugu (Redis durum, in-process worker havuzu, tekillestirme, kismi sonuc)
internal/providers     13 site (etstur, tatilsepeti, jolly, tatilbudur, touristica, setur, gezinomi, enuygun, mngturizm,
                       tatilsitesi, prontotour, tatilcikus, tatildukkani) + registry; fiyat, paket, statik detay, isimle arama
internal/provider      ortak Provider arayuzu ve sonuc tipleri
internal/cache         Redis cache (provider basina, 5 dk) + singleflight
internal/limiter       provider basina eszamanlilik, devre kesici, saglik istatistikleri
internal/canary        gunluk kanarya sorgusu (site degisikligi erken uyarisi)
internal/db            pgx, sema + migration'lar (hotel_mappings, proxies, plans, clients, api_keys, admins, usage_daily, request_logs)
internal/auth          kimlik context'i, HS256 JWT, anahtar hash
internal/browser       chromedp tarayici yoneticisi (sekme siniri, periyodik yeniden baslatma)
internal/httpx         proxy rotasyonlu HTTP istemcisi (4xx/5xx -> hata)
internal/proxy         Webshare senkronu + round-robin
ui-kit/                SmartRate tasarim sistemi (tokenlar, acik/koyu tema, AppShell, DataTable, modal/toast, mobil); iki uygulama @kit ile kullanir
ui/                    React/Vite admin paneli (Docker'da derlenip Go'ya gomulur, /admin/)
ui-portal/             React/Vite musteri portali (/portal/)
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
| GET/POST | /api/admin/hotels/{id}/details[?provider=], .../details/refresh | acente bazli statik detay (ilk goruntulemede `highlightPaths`), yenile |
| GET/POST | /api/admin/details/changes, /api/admin/details/run | degisiklik gecmisi, gunluk kontrolu simdi calistir (`DETAILS_RUN_HOUR`, varsayilan 04) |
| GET/POST | /api/admin/notifications, .../read | admin bildirimleri (portal esdegeri `/api/portal/notifications`) |
| POST/GET | /api/admin/hotels/import, /api/admin/imports/{id} | isim listesinden toplu otel ekleme (asenkron; `apply:false` = rapor) |

Tam sema: `internal/api/openapi.json` (Swagger `/docs`). Postman koleksiyonu: `postman/` (OpenAPI'den uretilir, bkz. `postman/README.md`).

## Arayuz (UI)

Admin paneli ve musteri portali ayni tasarim dilini `ui-kit/` uzerinden paylasir (marka: SmartRate; bkz. `ui-kit/README.md`).
Listeler satir sonu aksiyon ikonlu tablolardir, satira cift tiklama detaya goturur; kisa formlar modal, uzun formlar sayfa
icinde karttir; 860 px altinda menu alt cubuga iner ve tablolar karta donusur. Yerel gelistirme:

```bash
cd ui && npm install && npx vite --port 5173        # http://localhost:5173/admin/  (API: localhost:3000'e proxy)
cd ui-portal && npm install && npx vite --port 5174 # http://localhost:5174/portal/
node ui-kit/shot.mjs "http://localhost:5173/admin/#hotels" out.png --token pricebot_admin_token=<jwt> [--mobile] [--dark]
```

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
`REQUIRE_API_KEY`, `CACHE_TTL_SECONDS`, `PROVIDER_CONCURRENCY`, `BROWSER_MAX_TABS`, `CANARY_HOTEL_ID`, `WEBSHARE_API_KEY`,
`DETAILS_RUN_HOUR` (statik detay gunluk kontrol saati, Europe/Istanbul), `BROWSER_PROXY_URL` (headless tarayici proxy'si),
`SMTP_HOST/PORT/USER/PASSWORD/FROM`, `PUBLIC_PORTAL_URL`, `PASSWORD_RESET_TTL_MINUTES` (sifre sifirlama linki e-postasi).

## Bilinen durumlar

- Etstur paket API'si (Ekim 2026) `room/multi` + `room/multi/package` uclarina tasindi; odalar `result.roomGroups[].rooms`
  altinda. Kod guncellendi ve canli dogrulandi (bkz. `docs/phase3-research.md`).
- Gezinomi fiyat API'si sunucudan "Oda bulunamadi" donuyor (oturum/IP bagli); resolve calisiyor.
- Touristica Cloudflare arkasinda; headless Chromium ile calisir (`BROWSER_MAX_TABS` ile sinirli). Tarayicisiz hibrit calisma arastirma asamasinda.
