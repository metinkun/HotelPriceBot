# Faz 3 arastirma notlari (2026-10-16)

Arac: `cmd/capture net <url> <checkIn> <checkOut> <out.json> [tiklaMetni]` — headless Chromium ile sayfayi acar,
tarih doldurup "Oda Ara" tetikler ve XHR/fetch trafigini kaydeder. App imajinda calistirma:

```bash
docker run --rm -v "$PWD/bin:/tools:ro" -v /tmp/out:/out -e REDIS_URL= -e DATABASE_URL=postgresql://x:x@localhost:1/x \
  --entrypoint /tools/capture pricebot-go-app net "https://www.touristica.com.tr/<slug>" 2026-11-20 2026-11-23 /out/t.json
```

## Touristica: tarayicisiz calisma analizi

Bulgu (capture `touristica_net2.json`):

1. Sayfa acilisinda Cloudflare challenge cozuluyor; oturumda `cf_clearance`, `__cflb`, `ASP.NET_SessionId`, `NTWSC*` cerezleri olusuyor.
2. "Oda Ara" tiklamasi su XHR'lari yapiyor (hepsi ayni oturum cerezleriyle, `X-Requested-With: XMLHttpRequest`):
   - `POST /Ajax/Main.asmx/HotelSearchNow?y=4c` — JSON govde `{"request":{"xc":"<base64>"}}`. `xc` istemci tarafinda
     **sifrelenmis** arama parametreleri (tarih, kisi, otel). Yanit `application/json`.
   - `GET /<slug>/odalar?rpd=True&cc=&det=False&y=1c&_=<ts>` — oda kartlari HTML (fiyatlar burada).
   - `GET /<slug>/otel-indirim?...`, `/otel-indirim-tab?...` — kampanya HTML'leri.
   - `GET /<slug>/ucak-ve-odalar?rpd=true&ojr=true&cc=&det=false` — paket (otel+ucak) HTML.
3. Yani fiyat verisi sunucu oturumuna (session) yazilan arama parametrelerine bagli; `odalar` tek basina tarihsiz doner
   (onceki gozlemle tutarli).

Tarayicisiz (hibrit) plan — uygulanabilir, iki on kosul var:

- **TLS parmak izi:** `cf_clearance` Chrome'un JA3/JA4 parmak izine baglidir. Go'da `github.com/bogdanfinn/tls-client`
  (utls tabanli, Chrome profili) ile ayni cerez tekrar oynatilabilir. Standart `net/http` ile 403 alinir (dogrulandi).
- **`xc` sifrelemesi:** HotelSearchNow govdesi sayfadaki JS tarafindan sifreleniyor. Anahtar/algoritma sayfanin JS
  paketinden cikarilmali (buyuk olasilikla AES-CBC, `{"iv":...,"value":...}` benzeri yapi). Alternatif: sifrelemeyi
  tarayicida bir kez yaptirip (page.evaluate ile `xc` uretmek) sonraki istekleri tls-client ile atmak.

Ikinci sondaj (`cmd/capture js` + `touristica_fns.js`) ile `xc`'nin nasil uretildigi goruldu:

- Global `sendSearchRequest(i, r, u)` fonksiyonu (obfuscate edilmis `bundle.min.js`, ~900 KB) `r` arama parametrelerinden
  (`CheckInDateString`, `CheckOutDateString`, `AdultCount`, `ChildCount`, `FirstChildAge`, `SecondChildAge`, otel/URL alanlari)
  bir dizi olusturur; sayfadaki gizli `hdnEncArray` input'larindaki indeks listelerine gore alanlari birlestirir ve
  **CryptoJS AES-CBC (PKCS7)** ile sifreler. Anahtar ve IV, kodda gomulu kisa sabit parcalarin (`"aQ","!p","-d","8f","nS","*1","Jl",".w"`)
  farkli siralarda birlestirilmesiyle uretilir; ikinci bir AES turu daha var (ic ice sifreleme).
- `xcPre` sayfa basina sabit bir GUID (`1901d69b-...`), `window.HotelSearchParameters` varsayilan parametre nesnesi,
  `HOTELONLINERESERVATIONROOT = https://book.touristica.com.tr/Hotel`.
- Sonuc: `xc` tamamen istemci tarafinda ve deterministik uretiliyor; Go'da yeniden yazilabilir ama anahtar parcalari ve
  `hdnEncArray` siralamasi her bundle surumunde degisebilir (kirilgan). Daha dayanikli yol: sifrelemeyi sayfanin kendi
  fonksiyonuna yaptirmak.

### Adim 1 UYGULANDI (2026-10-17): kalici sekme havuzu

`internal/providers/touristica/pool.go` + `api.go`:

- `TOURISTICA_TABS` kadar sekme surekli acik (varsayilan 1); `TOURISTICA_SESSION_MINUTES` (20) sonra veya 10 sorguda bir
  sekme yenilenir. Oturum/cerez (cf_clearance) sekmede kalir; ayni otel acik ise navigasyon yok.
- Arama: tarih/kisi doldur + "Oda Ara"; sayfaya kurulan jQuery `ajaxSuccess` kancasiyla `/odalar` yanitinin gelmesi beklenir
  (metin tabanli bekleme sadece jQuery yoksa). Onceki aramanin bloklari `data-pb-stale` ile isaretlenir ve okunmaz
  (bayat fiyat okunmasini onler); uygulanan tarih sayfadan dogrulanir.
- Paket: ARAMA ile birlikte otomatik gelir (`/ucak-ve-odalar?ojr=true`). Sekmeye tiklamak gereksizdir ve zararlidir:
  menudeki "ÖZEL DÖNEM(OTEL+UÇAK)" linkiyle karisip `/kibris-turlari`'na gidiyordu; dogru sekme
  `a.nav-hotel-detail-room-and-flight-tab` olsa bile tiklamak paneli gizliyor ve sonraki aramada otomatik yuklemeyi bozuyor.
  Bu yuzden paket sorgusu sayfayi yeniden yukler (oturum sayesinde ~3 sn), aramadan sonra `pkg` sayacini bekler, tiklamaz.
- Olcum (Acapulco, lokal Docker): fiyat 2.3–6 sn (eski ~15 sn), paket 4.8–5.6 sn (eski ~26 sn), 6/6 basarili.
  Bellek: 6 sorgu sonrasi konteyner ~1 GB -> sekme 10 sorguda bir yenilenir, Chromium bellek bayraklari eklendi
  (`renderer-process-limit`, `imagesEnabled=false`, `max-old-space-size=256`).
- Gelistirme araci: `capture touristica-debug <url> <in> <out> [in2 out2]` gercek havuz akisini adim adim izler.

Siradaki (adim 2, opsiyonel): tam tarayicisiz calisma icin asagidaki plan gecerli.

Onerilen uygulama sirasi:
1. **Kalici sekme + sayfa ici fetch (en az riskli, ilk adim):** Touristica icin tek bir sekme acik tutulur (oturum ve
   `cf_clearance` tarayicida kalir). Her sorguda sayfaya gidilmez; `page.evaluate` ile `window.HotelSearchParameters`
   doldurulup `sendSearchRequest` cagrilir (veya mevcut UI tiklama akisi), ardindan `fetch("/<slug>/odalar?rpd=True...")`
   HTML'i okunur ve goquery ile parse edilir. Kazanim: navigasyon + render + 15 sn bekleme yerine ~3–5 sn; sekme bellegi
   bir kez odenir. Hotel degisimi: `HotelSearchParameters.Url`/`HotelId` alanlari sorguya gore set edilir (dogrulanmali).
2. **Tam tarayicisiz (ikinci adim):** `xc` uretimini Go'ya tasimak (AES-CBC, anahtar parcalari + `hdnEncArray`) ve
   istekleri tls-client (Chrome JA3 profili) + tarayicidan alinan `cf_clearance`/`ASP.NET_SessionId` cerezleriyle atmak.
   Cerez ve anahtar parcalari 20–30 dk'da bir / bundle surumu degisince tarayiciyla yenilenir. 403 veya challenge HTML
   gelirse otomatik olarak 1. adima geri dusulur.
3. Her iki adimda da kanarya testi Touristica'yi gunluk izler; bundle surumu (`bundle.min.js?v=`) degisince uyari uretilir.

Beklenen kazanim: 1. adimla istek suresi ~15 sn -> ~4 sn ve es zamanli sorgu siniri 1 -> 3–4; 2. adimla tarayici yuku
neredeyse sifirlanir. Tahmini efor: 1. adim 1–2 gun, 2. adim 3–5 gun (kirilganlik riski yuksek).

## Etstur paket: yeni uclar

Capture (`etstur_net.json`) detay sayfasinin su cagrilari yaptigini gosterdi:

- `POST /services/api/room/multi` govde `{"hotelId","checkIn":"yyyy-MM-dd","checkOut","rooms":[{adultCount,childCount,childAges,infantCount}]}` -> `result.roomSearchId`
- `POST /services/api/room/multi/package` govde `{"roomSearchId","airportCode":"SAW"}`
- `GET /services/api/hotel/detail/<slug>?check_in=dd.MM.yyyy&check_out=&adult_1=&child_1=` (sayfa verisi)

Kod bu uclara gecirildi (`internal/providers/etstur/api.go`). Dogrudan replay `success:true` donuyor ancak `result.rooms`
bu otel/tarih icin bos geldi; `result` icinde `flightPackage`, `minPrice`, `noDates`, `promotions` alanlari var. Olasiliklar:
paket odalari artik `hotel/detail` yanitinda tasiniyor olabilir veya `multi/package` yaniti gecikmeli (tarayicida 15 sn'de
tamamlanmadi). Sonraki adim: `hotel/detail` JSON'unu ve `multi/package` yanitinin tamamini (tarayici icinde `fetch` ile)
kaydedip oda listesinin nerede oldugunu bulmak.

## Gezinomi

`GetHotelRoomPrices` sunucudan "Oda bulunamadi" donmeye devam ediyor; tarayici capture ile `x-session-id` uretimi incelenmeli
(ayni arac `net` modu kullanilabilir).
