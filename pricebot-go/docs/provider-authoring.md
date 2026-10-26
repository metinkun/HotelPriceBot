# Yeni provider (site) ekleme rehberi

Bu dokuman, bir otel satis sitesini bota baglamak icin izlenecek yolu ve kurallari anlatir.
Hedef: **tarayici bagimliligi minimum** — once duz HTTP (JSON API / HTML parse), olmuyorsa hibrit
(tarayici sadece cerez/oturum icin), en son care tam tarayici akisi.

## 1. Kesif (reverse engineering)

1. Sitede bir otel detay sayfasi bul (tercihen *Acapulco Resort Convention Spa*, Girne/Kibris; yoksa herhangi bir otel).
2. Tarayici header seti ile sayfayi cek ve icerigi incele:
   ```bash
   curl -s -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36" \
     -H "accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "accept-language: tr-TR,tr;q=0.9" \
     -H 'sec-ch-ua: "Chromium";v="150", "Google Chrome";v="150", "Not;A=Brand";v="8"' -H "sec-ch-ua-mobile: ?0" -H 'sec-ch-ua-platform: "macOS"' \
     -H "sec-fetch-dest: document" -H "sec-fetch-mode: navigate" -H "sec-fetch-site: none" -H "upgrade-insecure-requests: 1" --compressed "<URL>" -o page.html -w "%{http_code}\n"
   ```
   - `__NEXT_DATA__`, `window.__INITIAL_STATE__`, RSC payload, `data-*` attribute'lari, gizli input'lar (`hotelId`, `_token`) ara.
   - 403/503 "Just a moment"/Akamai/Cloudflare ise header setini tamamla; yine olmuyorsa tarayici gerekir.
3. Fiyat/paket cagrilarini bul. En guvenilir yol tarayicidan **ag trafigi yakalama**:
   ```bash
   # app imaji icinde Chromium var; bin/capture host'ta derlenir (linux, CGO_ENABLED=0) ve mount edilir
   docker run --rm -v "$PWD":/src -v pricebot_gomodcache:/go/pkg/mod -v pricebot_gocache:/root/.cache/go-build -w /src golang:1.23-bookworm \
     sh -c 'CGO_ENABLED=0 go build -o bin/capture ./cmd/capture'
   docker run --rm --shm-size=512m -v "$PWD/bin:/tools:ro" -v "$PWD/out:/out" -e REDIS_URL= -e DATABASE_URL=x \
     --entrypoint /tools/capture pricebot-go-app net "<otelURL>" 2026-12-05 2026-12-08 /out/site_net.json ["tiklaMetni"]
   ```
   `net` modu: sayfayi acar, tarih alanlarini (`#txtCheckInDate` benzeri) doldurmayi dener, "Oda Ara" tiklar ve XHR/fetch
   isteklerini (URL, method, govde base64, yanit basliklari) kaydeder. Sitenin form alanlari farkliysa
   `js` modu ile kendi JS'inizi sayfada calistirin: `capture js "<url>" script.js out.json` (async destekli).
4. Bulunan istegi `curl` ile **tarayici disinda** tekrar oynat. Cerez/token gerekiyorsa hangileri sart, ne kadar sure gecerli not et.
5. Cocuk yasi, oda formati, tarih formati, para birimi, indirimli/liste fiyat alanlarini dogrula.

## 2. Uygulama

Dizin: `internal/providers/<site>/` — uc dosya + testdata:

- `api.go` — HTTP cagrilari (`internal/httpx`: `httpx.Do/Get/Post`, `httpx.FormValues` sirali form, `resp.SetCookies()`),
  tarih donusumleri, `resolveHotelFromURL` (detay URL -> providerHotelId, otel adi, metadata).
- `normalizer.go` — ham yanit -> `types.NormalizedPrice` (lead/en ucuz teklif) ve `[]types.NormalizedPackageRoom`.
  Yardimcilar: `provider.ParseTRPrice`, `provider.DecodeEntities`, `provider.FirstMatch`, `types.DiscountRateOf`, `types.NightsBetween`.
- `provider.go` — `provider.Provider` arayuzu: `Name()`, `Capabilities()`, `GetPrice`, `GetBulkPrices` (genelde
  `provider.DefaultBulk`), opsiyonel `GetPackagePrices`, `ValidateMappingInput`, `RequiredMetadataFields`, `ResolveFromURL`.
  Metadata'da mutlaka `sourceUrl` sakla.
- `testdata/` — gercek yanitlardan fixture (JSON/HTML; kisaltilmis, kisisel veri yok) + `normalizer_test.go`.

Ornekler: `jolly` (en basit: form POST + JSON), `tatilbudur` (token + cookie + HTML attribute parse),
`setur` (sayfa icindeki escape'li JSON), `gezinomi` (JSON API), `touristica` (tarayici: kalici sekme havuzu `pool.go`).

Kayit: `internal/providers/registry.go` icinde `r.register(<site>.New(...))`; cookie/anahtar gerekiyorsa `internal/config`'e
ortam degiskeni ekle (`.env.example`'a da yaz). Siteye ozel sabit URL/ID'leri metadata'da degil kodda tut.

Kurallar:
- Her dis istek `httpx` uzerinden gitmeli (proxy rotasyonu, 4xx/5xx -> hata).
- Zaman asimi makul (20–45 sn); yavas siteler icin belirt.
- Tarayici gerekiyorsa `browser.Manager` kullan; sayfa basina degil, havuz/oturum mantigiyla (touristica/pool.go).
- `docker compose` CALISTIRMA (port cakismasi). Test icin `bin/probe` kullan:
  ```bash
  docker run --rm -v "$PWD":/src -v pricebot_gomodcache:/go/pkg/mod -v pricebot_gocache:/root/.cache/go-build -w /src golang:1.23-bookworm \
    sh -c 'go vet ./... && go test ./internal/providers/<site>/ && CGO_ENABLED=0 go build -o bin/probe ./cmd/probe'
  docker run --rm -v "$PWD/bin:/tools:ro" -e REDIS_URL= -e DATABASE_URL=x --entrypoint /tools/probe pricebot-go-app <site> price "<otelURL>" 2026-12-05 2026-12-08 2
  docker run --rm -v "$PWD/bin:/tools:ro" -e REDIS_URL= -e DATABASE_URL=x --entrypoint /tools/probe pricebot-go-app <site> package "<otelURL>" 2026-12-05 2026-12-08 2 SAW
  ```

## 3. Teslim

- `docs/providers/<site>.md`: uclar, header/cerez gereksinimleri, govde formatlari, tarih/kisi formatlari, bilinen
  sinirlar (orn. paket yok, cocuk yasi desteklenmiyor), probe ciktisi ozeti, tarayici gerekip gerekmedigi ve nedeni.
- `go vet ./...` ve `go test ./...` temiz; `gofmt -l .` bos.
- Degisiklikleri commit et (mesaj: `<site>: provider eklendi`).

## 4. Statik detay (`DetailProvider`) ve isimle arama (`HotelSearcher`)

`internal/provider/details.go` iki opsiyonel arayuz tanimlar:

- `GetDetails(ctx, mapping) (*HotelDetails, error)` — acentenin otel detay sayfasini ORTAK semaya normalize eder:
  ad, aciklama, yildiz/puan, adres/sehir/konum, iletisim, **gorseller** (tam URL), **ozellik gruplari** (acentedeki grup
  adiyla), **odalar** (ad, m2, kapasite, ozellikler, gorseller), pansiyon tipleri, **politikalar** (giris/cikis saati, cocuk,
  evcil, iptal), **soru-cevap**, ve `Sections`: acentenin kendi basliklariyla tum metin/listeler (ham gorunum icin;
  `kind` text|list|pairs|html; HTML temizlenmis olmali, script/style yok). Bos alanlari atla ama dizi alanlarini `nil`
  degil bos dizi birak. Metinleri `provider.DecodeEntities` ile temizle. Mapping metadata'da `sourceUrl` varsa onu kullan.
  Veri her gun yeniden cekilip onceki snapshot ile karsilastirilir: **deterministik** ol (siralama sabit, tarih/sayac gibi
  her istekte degisen alanlari koyma, gorsel URL'lerinde cache-buster parametresi varsa temizle).
- `SearchHotels(ctx, query) ([]HotelCandidate, error)` — acentenin arama/autocomplete API'siyle otel adina gore aday
  listesi: `Name`, `URL` (ResolveFromURL'in kabul ettigi detay URL'i), `City`, `ProviderHotelID`. Sadece OTEL sonuclari
  (destinasyon/bolge degil). Toplu otel ekleme (78 KKTC oteli) bu arayuzle yapilir; Turkce karakter/buyuk-kucuk harf
  farklarina dayanikli ol (acente API'si izin veriyorsa kisaltilmis sorgu da dene).

Test: `bin/probe <site> details <url>` ve `bin/probe <site> search "<otel adi>"` (ornek: "Merit Royal", "Elexus").
