# TatilSepeti provider (`internal/providers/tatilsepeti`)

Durum (Ekim 2026): **duz HTTP, cerez elle girilmez**. Oturum (cerez + VerificationToken) otomatik
edinilir; duz HTTP engellenirse (Cloudflare) tarayici yalnizca cerez/token okumak icin bir kez acilir (hibrit).
Fiyat ve paket sorgulari dogrulandi (bkz. probe ozeti).

## Uclar

| Amac | Istek | Govde (form-urlencoded) | Yanit |
|---|---|---|---|
| Liste (lead) fiyati | `POST https://www.tatilsepeti.com/hotel/GetHotelListPrice/` | `AdultCount=2&ChildCount=0&CampaignType=0&CampaignId=&checkinDate=05.12.2026+&checkoutDate=08.12.2026&HotelIds=7741[,229,...]` | JSON dizi; otel basina `HotelId, Price, DiscountRate, DiscountPrice, Accommodation, CampaignName, HasPrice, HasFlyingPackage` |
| Paket (otel+ucak+transfer) | `POST https://www.tatilsepeti.com/Hotel/GetFlyingPackages` | `Search=oda:2;tarih:05.12.2026,08.12.2026;click:true&Id=7741&IsFlightPocket=true&depatureCode=saw&destinationCode=ecnc` (cocuklu: `oda:2,5,8`) | JSON: `roomList` (HTML), `flightListResult` (HTML), `transferList`, `detailMessage`, `IsFlyingPackageOnline`, `CityCode` |
| URL cozumleme | `GET https://www.tatilsepeti.com/<otel-slug>` | - | HTML: `#hidHotelId`, `#hidHotelName`, `input#nereye[data-search-value]` (destinasyon kodu, orn. `ecnc`), `#forgeryToken`, `#hidCityCode` (99 = Kibris) |

Tarih formati `dd.MM.yyyy` (checkinDate sonunda bosluk sitenin JS'i gibi). Kisi: `AdultCount`/`ChildCount`
(liste fiyati cocuk yasi almaz); paket `Search` icinde yaslar virgulle. Para birimi TL (yanitta `Currency` null gelir).

Sitenin kendi detay sayfasi oda listesi icin `POST /<otel-slug>` (`Search=oda:2;tarih:...;click:true`) kullanir; biz
liste fiyati (GetHotelListPrice) ve paket (GetFlyingPackages) uclarini kullaniyoruz.

### Basliklar

XHR istekleri: `x-requested-with: XMLHttpRequest`, `content-type: application/x-www-form-urlencoded; charset=UTF-8`,
`origin: https://www.tatilsepeti.com`, `referer: <otel sayfasi>`, `user-agent` = `browser.UA` (cf_clearance UA'ya baglidir),
`cookie: <oturum>`, `verificationtoken: <#forgeryToken>` (sitenin JS'i `VerificationToken` adiyla ekler).

Detay sayfasi GET'i **tarayici-gezinme** basliklariyla yapilmali (`sec-fetch-mode: navigate`, `sec-fetch-dest: document`,
`accept: text/html...`). `x-requested-with`/`origin` ile GET yapilirsa site parca (partial) HTML doner (~12 KB, `hidHotelId` yok).

## Oturum mekanigi (`session.go`)

Olcum (22.10.2026, konut IP + Docker): her iki uc **cerezsiz ve token'siz de JSON donuyor**. Yine de site Cloudflare bot
yonetimi arkasinda (sayfa yuklenirken `/cdn-cgi/challenge-platform/.../jsd/oneshot`, tarayicida `cf_clearance` cerezi olusuyor);
sunucu/proxy IP'lerinden 403/503 "Just a moment" gorulebilir. Bu nedenle:

1. **Duz HTTP** (varsayilan): detay sayfasi (yoksa ana sayfa) GET -> `Set-Cookie` (`ASP.NET_SessionId`,
   `HotelSearchCriteriaCookie`, `MyVisits`) + HTML'deki `#forgeryToken`. `ResolveFromURL` sirasinda cekilen sayfadan da
   oturum alinir (ek istek yok).
2. **Tarayici** (`WithBrowser(bm)` verildiyse ve duz HTTP engellendiyse ya da `WithForceBrowser(true)`): `bm.NewPage` ile
   sayfa acilir, `browser.Navigate` challenge'in gecmesini bekler, `network.GetCookies` ile tatilsepeti.com cerezleri
   (`cf_clearance` dahil) ve `#forgeryToken` okunur, sekme kapatilir. Tarayici sorgu basina degil, **oturum basina** kullanilir.
3. Onbellek: tek paylasilan oturum (cerezler otele bagli degil), **20 dk** sonra yenilenir (`WithSessionAge`).
4. Yenileme tetikleri: yanit JSON degil, 403/429/503 veya challenge HTML'i -> oturum gecersiz sayilir, bir kez yeniden
   edinilip istek tekrarlanir. Engel goruldu ve tarayici varsa bir sonraki edinim tarayici ile yapilir.
5. `TATILSEPETI_COOKIE` / `TATILSEPETI_VERIFICATION_TOKEN` (env) **istege bagli override**: doluysa once onlar kullanilir;
   bir istek basarisiz olursa otomatik edinime gecilir (`Capabilities.RequiresCookie=false`).

Token omru: `#forgeryToken` her sayfa yuklemesinde degisir (istek basina uretilir) ve sunucu su an token'i dogrulamiyor
gibi gorunuyor (token'siz istek de 200 JSON). Cerez omru: `ASP.NET_SessionId` oturum cerezi (sure yok),
`HotelSearchCriteriaCookie` 1 gun, `cf_clearance` tipik 30 dk-1 saat ve IP+UA'ya bagli. 20 dk'lik yenileme bunlarin icinde kalir.

`Stats()` saglik sayaclari: `sessionHttp`, `sessionBrowser`, `sessionRefresh`, `blocked`, `sessionFailures`,
`sessionSource` (1 env, 2 http, 3 browser).

### Kayit (registry)

`registry.go` su an `tatilsepeti.New(cfg.TatilSepetiCookie, cfg.TatilSepetiVerificationToken)` ile kaydeder; bu haliyle yalnizca
duz HTTP calisir (engelde hata). Tarayici yedegini acmak icin:

```go
r.register(tatilsepeti.New(cfg.TatilSepetiCookie, cfg.TatilSepetiVerificationToken, tatilsepeti.WithBrowser(b)))
```

## Normalize

- Liste fiyati: API `Price` = indirimsiz liste fiyati, `DiscountPrice` = `DiscountRate` uygulanmis odenecek fiyat
  (orn. 67200 / %15 / 57120). Normalize: `price=57120, listPrice=67200, discountRate=15, discountPrice=57120`.
  `Price=0`/`HasPrice=0` (orn. 4 yetiskin tek oda) -> `available=false`. `hotelName` yanitta null gelir, mapping'den doldurulur.
- Paket: `roomList` HTML'inde `#roomName_flight_<RoomTypeId>` kartlari; pansiyon basina `getInstallment(... FlightPocketPrice: '65533,58' ...)`
  (TR formati, `provider.ParseTRPrice`). Toplam fiyat = oda + ucak + transfer (`TotalRoomPrice`, `FlightPrice`, `TotalTransferPrice` gizli inputlari).
  `free-cancelation` blogu -> `FREE_CANCELLATION`. Paket yoksa `detailMessage` (`Konaklama yapacağınız tarihlerde uçaklı paket bulunamadı.`) `note`'a yazilir.

## Bilinen sinirlar

- Liste fiyati ucu cocuk **yasi** almaz (sadece sayi); oda tipi/oda adi donmez (lead fiyat).
- Paket icin mapping metadata'da `destinationCode` gerekir (URL cozumlemede `#nereye[data-search-value]`'dan dolar; Kibris otelleri `ecnc`).
  `depatureCode` kucuk harf IATA (`saw`, `ist`, `adb`...). Site "giris tarihi bugunden en az 7 gun sonra" kurali uygular.
- Tarayici ile alinan `cf_clearance` IP'ye baglidir: `httpx` proxy rotasyonu aciksa tarayici (proxy'siz) cerezi proxy IP'sinde gecersiz olabilir.
- `GetFlyingPackages` yaniti buyuk (~650 KB, ucus listesi dahil); zaman asimi 40 sn.

## Probe ozeti (22.10.2026, app imaji icinde)

```
probe tatilsepeti price   https://www.tatilsepeti.com/kaya-palazzo-resort 2026-12-05 2026-12-08 2
  resolve 0.8s -> 7741 "Kaya Palazzo Resort" destinationCode=ecnc
  price   1.0s -> found=true  price=57120 TL  listPrice=67200  %15  Oda Kahvaltı  "Son Dakika Fırsatı %15 İndirim + 3 Taksit"
probe tatilsepeti package https://www.tatilsepeti.com/kaya-palazzo-resort 2026-12-05 2026-12-08 2 SAW
  package 9.4s -> available=true roomCount=5
  46 Kara Manzaralı Deluxe Oda   OK 65.540,86  TP+ 78.140,86   (3 gece, ucak+transfer dahil)
  45 Deniz Manzaralı Deluxe Oda  OK 82.640,86  TP+ 100.190,86
  218 Grand Suit                 OK 105.140,86 TP+ 134.240,86  ... (5 oda)
probe tatilsepeti price   https://www.tatilsepeti.com/acapulco-resort-convention-spa-hotel 2026-12-05 2026-12-08 2
  resolve 0.9s -> 229 "Acapulco Resort Convention Spa Hotel" destinationCode=ecnc
  price   0.9s -> found=true  price=30600 TL  listPrice=51000  %40  Yarım Pansiyon
```

Tarayici yolu dogrulamasi (gecici olarak `WithBrowser(b)` + `WithForceBrowser(true)` ile): oturum ~3-4 sn'de alindi
(`cf_clearance`, `ASP.NET_SessionId`, `HotelSearchCriteriaCookie`, token), ardindan duz HTTP ile fiyat 57120 TL ve Acapulco paketi
(5 oda, 45.740,86 TL'den) basarili.

Tarayici gerekli mi: **normalde hayir**. Yalnizca duz HTTP Cloudflare tarafindan engellenirse, oturum basina (~20 dk'da bir)
tek sayfa acilisi icin.
