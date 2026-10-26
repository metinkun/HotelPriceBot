# Tatil Dukkani (tatildukkani.com) provider

Durum: **tarayici gerekmez; duz HTTP (JSON API) + bir Socket.IO (WebSocket) adimi.** Cerez/CSRF yok;
tek gereken, herkese acik `GET /guest` ile alinan misafir `apiToken`'i (sabit gorunuyor, kodda 6 saat
cache'lenir). Kod: `internal/providers/tatildukkani/` (`api.go` HTTP, `socket.go` ws akisi,
`normalizer.go`, `provider.go`).

Site Nuxt 2 SSR (`window.__NUXT__`); sayfa HTML'inde fiyat yok, her sey `https://api.tatildukkani.com`
JSON API'sinden ve oda/fiyat listesi de `wss://ws.tatildukkani.com` uzerinden geliyor. Bu yuzden HTML
parse yapilmiyor; API cagri sirasi sitenin `fetchDataBySlug` + `fillSocketData` akisinin aynisi.

## Uclar (sira ile)

Tum API istekleri `content-type: application/json` + `x-auth-token: <apiToken>` + `tokenType: GUEST`
header'lari ile (kodda ayrica tarayici UA, `origin`, `referer`). CORS `*`, cerez gerekmez.

### 0. Misafir token — `GET https://api.tatildukkani.com/guest`

`data.apiToken` (32 hex, orn. `48ad20ec...`), `data.type = "GUEST"`. Sitenin kendisi bunu `loggedUser`
cerezine koyuyor (23 saat). 401/403 gelirse kod token'i bir kez yenileyip tekrar dener.

### 1. Otel sayfasi — `POST /api/seo_page/slug`

```json
{"slug":"acapulco-resort-convention-spa","checkIn":"2026-12-05","checkOut":"2026-12-08",
 "origin":"","periods":[],"destination":[],"adt":"1970-01-01,1970-01-01","chd":"2021-10-29"}
```

Yanit `data.type` (`HOTEL` olmali; `HOTEL_DESTINATION`, `SEO_CONTENT_PAGE` vb. otel degil),
`data.typeId` (otel id, orn. 1404), `data.title`, `data.airportCode` (ECN), `data.result.{name,
hotelDestination.name, destination[].name, isAvailable, isPrice, avabilityCriteria}`.
**`avabilityCriteria` nesnesi (hotelId, destinationId, categorys[], hotelProviderReference,
providerCode, puzzleReferenceCode, market) bir sonraki adima aynen gonderilmek zorunda** — yalnizca
`{hotelId}` ile socket sonuc gondermiyor (denendi). `lowPrice`/`rooms` bu yanitta hep `null`.
Tarihsiz (`checkIn:""`) cagri yalnizca URL cozumleme icin kullanilir. ~0.5 sn, ~110 KB.

### 2. Arama kaydi — `POST /api/searchlog/hotel/init`

```json
{"id":1404,"type":"HOTEL","userId":null,"checkIn":"2026-12-05","checkOut":"2026-12-08",
 "adt":"1970-01-01,1970-01-01","chd":"","hotelList":[<avabilityCriteria>]}
```

Yanit `data.uid` (`UID_...`). Tek kullanimlik arama kimligi.

### 3. Socket oturumu — `POST /api/socket-login` (govde `{}`) -> `data.socketToken` (`STK_...`).

### 4. Oda/fiyat listesi — Socket.IO `wss://ws.tatildukkani.com/socket.io/?EIO=4&transport=websocket`

Engine.IO v4 / Socket.IO v4, varsayilan namespace; veri **JSON string** olarak tasiniyor:

```
<- 0{"sid":...}                                    -> 40
<- 40{"sid":...}                                   -> 42["login","{\"socketToken\":\"STK_..\",\"available\":[{\"type\":\"HOTEL\",\"token\":null}]}"]
<- 42["is_login", ...]                             -> 42["available-hotel","{\"uid\":\"UID_..\",\"socketToken\":\"STK_..\"}"]
<- 42["available-hotel-token", {hotelAvailableToken}]
<- 42["available-hotel", "{...status, rooms[]...}"] -> 42["available-hotel-callback","{\"hotelAvailableToken\":<hashCode>,\"refererId\":..}"]
```

Sunucu ping `2` -> `3`. Yanit ~1-2 sn icinde geliyor; kodda ust sinir 40 sn. Istemci icin yeni modul
eklenmedi: chromedp'nin bagimliligi olan `github.com/gobwas/ws` dogrudan kullaniliyor (`go.mod`'da
indirect -> direct). **Not:** ws baglantisi `httpx` proxy rotasyonundan gecmez (dogrudan baglanir).

`available-hotel` govdesi: `status` (`AVAILABLE`), `hotelId`, `hashCode`, `rooms[]`:

```
roomTitle, conceptTitle (pansiyon: "Yarım pansiyon", "Tam pansiyon plus"), status (AVAILABLE | ON_REQUEST | STOP_SALE | NO_ROOMS)
fee (sayi, satis, konaklama toplami TRY) = realFee (string) = discountPrice
posterPrice (liste/afis fiyati), posterRate (afis indirim %, orn 35), currency "TRY"
roomCampaign[] {name:"(D) +10 EB +5ilave", rate, endDate}, days[] {date, fee, normalFeeDay, discountFeeDay}
priceCode / roomCode / conceptCode (rezervasyon icin), cost/realCost (acente maliyeti, kullanilmiyor)
```

Normalizasyon: lead = once `status=AVAILABLE` odalarin en ucuzu (`fee>0`); hic yoksa `ON_REQUEST`
odalarin en ucuzu (site bu odalarda fiyati gosterip sari "talep" butonu koyar; otel-duzeyi `status` da
`ON_REQUEST` olur). `STOP_SALE` / `NO_ROOMS` ("Dolu") ve fiyatsiz odalar sayilmaz. `price=fee`,
`listPrice=posterPrice` (fee'den buyukse), `discountRate` list/price'tan (= siteyle ayni %35),
`currency` TRY -> `TL`, `campaignName` = `roomCampaign[0].name`. Oda yoksa / hepsi satis disi ->
`available:false`. Yanitta `source` tedarikciyi soyler (PUZZLE, TATILSEPETI, TATILBUDUR...).

### Kullanilmayan uclar

- `POST /api/hotel/get-room-price {hotelId, checkIn}` — fiyat degil, donem bazli **cocuk politikasi**
  tablosu (0-11 yas FREE vb.). `POST /api/hotel/minacc-upper-available` — min konaklama/ust oda bilgisi.
- `POST /api/flight/available`, `/api/transfer/available` — ucus/transfer; misafire kapali/ayri urun.

## Tarih / kisi formatlari

- Tarihler `yyyy-MM-dd` (bot standardi ile ayni).
- Yetiskin sayisi: `adt` = yetiskin basina `1970-01-01`, virgulle (`"1970-01-01,1970-01-01"`).
- Cocuk: `chd` = cocuk basina **dogum tarihi**, virgulle. Sitenin `$calcChildAge`'i:
  `checkOut - yas yil - 1 ay - 10 gun` (moment ay-sonu kirpmasi dahil; `childBirthDate`).
  Orn. checkOut 2026-12-08, 5 yas -> `2021-10-29`.
- Dogrulama (Acapulco, 05-08 Aralik 2026, 2 yetiskin): 33.150 TL; +1 cocuk 5 yas: 33.150 TL
  (0-11 ucretsiz, politika tablosuyla uyumlu); +1 cocuk 13 yas: 45.581,25 TL (liste 70.125, %35).

## URL cozumleme

`https://www.tatildukkani.com/<slug>` -> slug (ilk yol parcasi; query/hash atilir) -> adim 1 tarihsiz.
`type != "HOTEL"` ise hata (`antalya-otelleri` -> `HOTEL_DESTINATION`), bilinmeyen slug 404 -> "otel bulunamadi".

Mapping: `providerHotelId` = `typeId` (orn. `1404`), metadata `slug` (zorunlu; yoksa `sourceUrl`'den
turetilir), `sourceUrl`, `city`, `airportCode` (bilgi). Fiyat sorgusunda slug donen `typeId` mapping ile
eslesmezse hata verilir.

## Paket (otel + ucak)

**Desteklenmiyor** (`SupportsPackages=false`). Sitede "Otel + Ucak + Transfer" sekmesi var ama:
`tabProductSettings.flight.showToGuest=false` (ucus misafire kapali; `Flights` bileseni yalnizca
`PASSENGERBOOKING` sahibi icin render ediliyor, `fetchFlights` o kosulda cagriliyor). `origin:"SAW"` ile
slug + init + socket akisi denendi: **oda listesi ve fiyatlar otel-only ile birebir ayni**, yanitta ucus
yok; yalnizca `packageProfit` (kar orani) alani ekleniyor ve sayfada `Transfer` secimi aciliyor. Tek bir
"paket fiyati" veren uc yok. Probe `package` modu "provider paket desteklemiyor" der.

## Probe ciktisi (2026-10-23)

```
probe tatildukkani price https://www.tatildukkani.com/acapulco-resort-convention-spa 2026-12-05 2026-12-08 2
  resolve 1.0s -> 1404 "Acapulco Resort Convention Spa", Girne, ECN
  price   1.6-5.6s -> Acapulco villa / Yarım pansiyon, 33.150 TL (liste 51.000, %35), kampanya "(D) +10 EB +5ilave"
... 2 1 5   -> 33.150 TL (cocuk ucretsiz)
... 2 1 13  -> 45.581,25 TL (liste 70.125, %35)
probe tatildukkani package ... SAW -> HATA: provider paket desteklemiyor
probe tatildukkani resolve https://www.tatildukkani.com/antalya-otelleri -> HATA: otel detay sayfasi degil (tip: "HOTEL_DESTINATION")
probe tatildukkani price https://www.tatildukkani.com/merit-park-hotel 2026-12-05 2026-12-08 2
  resolve 0.4s -> 1425 "Merit Park Hotel", Girne; price 4.7s -> Standart oda / Ultra her şey dahil,
  71.250 TL (liste 95.000, %25), kampanya "PARTNER (TD)+ 10+2 ilave"
probe tatildukkani price https://www.tatildukkani.com/crystal-admiral-resort-suites-spa 2026-12-05 2026-12-08 2
  resolve 0.4s -> 308 "Crystal Admiral Resort Suites Spa", Kızılot; price 1.0s -> available:false
  (socket: status NO_ROOMS, source TATILSEPETI, rooms []; 2027-06 icin de NO_ROOMS -> otel satista degil)
probe tatildukkani price https://www.tatildukkani.com/sidemarin-kirman-premium 2027-06-05 2027-06-08 2
  resolve 0.5s -> 165 "Sidemarin Kirman Premium", Manavgat; price ~2s -> otel status ON_REQUEST (source TATILBUDUR),
  6 oda hepsi ON_REQUEST -> Kara manzaralı standart oda / Ultra her şey dahil 63.000 TL (liste 126.000, %50)
```

## Bilinen sinirlar / notlar

- Fiyatlar konaklama toplami (3 gece), gecelik degil. Tek oda.
- Otel basina 4 istek (slug ~0.5 sn, socket-login, init, ws ~1-2 sn); toplam ~2-6 sn. Bulk = `DefaultBulk`.
- `avabilityCriteria` her sorguda yeniden cekiliyor (metadata'ya yazilmiyor; kategori listesi degisebilir).
- ws baglantisi proxy kullanmaz; site tarafinda oran siniri/engelleme gozlenmedi (Cloudflare/Akamai yok, nginx).
- `socketToken` her sorguda yeniden aliniyor (ucuz; cache'lenmedi). Misafir `apiToken` 6 saat cache.
- `available-hotel` olayi tek parcada geldi; `status` bos ve oda yoksa kod ikinci olayi bekler (40 sn).
- Tedarikci "PUZZLE" (Puzzle Travel); Jolly ile ayni fiyatlar gozlendi (33.150), ayni tedarik zinciri olabilir.

## Statik detay (`GetDetails`) ve isimle arama (`SearchHotels`)

Kod: `details.go`, `search.go`, `html.go`. Yalnizca misafir token'li JSON istekleri; tarayici/socket gerekmez.

### Kaynak: `POST /api/seo_page/slug` (tarihsiz, ~110 KB) -> `data.result`

| Alan | Kaynak |
|---|---|
| name, stars, address | `result.name`, `result.star` ("5"), `result.address` |
| city / region / country | `hotelDestination.name` (Girne); `destination[]` ilk = ulke (Kıbrıs), son = bolge (sehirle ayniysa bos) |
| location | `latitude`/`longitude` (string) |
| rating, ratingCount | `reviews.totalPoint` (5 uzerinden -> x2, 1 ondalik) ve `reviews.totalComment`; yorum metinleri/isimler alinmaz |
| description | `facality` icindeki "Genel Bilgi" zengin metni (HTML -> metin) |
| images | `singleImage` (kapak) + `images[]` (hotelType, line, id sirali; `fullPath` tam URL, sorgu parametresi atilir); caption = `imageHotelTypes[hotelType]` (Genel, Plaj ve Havuz, Restaurant ve Bar, Oda, Lobi, Aktivite, Spa, Balayı, Çocuk ve Bebek, Toplantı ve Kongre) |
| facilities | `facality[]` gruplari (sequence, id sirali; Genel Özellikler, Havuz & Plaj, Temalar, Bebek & Çocuklara Özel, Konum Bilgisi, Balayı Çiftleri İçin, Spa, Aktiviteler). Satir etiketi: `inputNumber`/`inputSelect`/`inputText` -> "Ad: deger" (orn. "Açılış Yılı: 1982", "Plaj: Kum"); `inputPaidFree` -> "(Ücretli)/(Ücretsiz)"; `inputYesNo=true` -> ad; `inputRichtext` satirlari ozellik degil, bolum olur. Bos gruplar (Yeme İçme Konsepti) atlanir |
| boards | `result.concepts[]` |
| policies | "Genel Uyarılar" zengin metni: giris/cikis saati regex (15:00 / 11:00), "evcil hayvan" cumlesi; tum satirlar `other`. "Bebek Ve Çocuk Bilgisi" -> childPolicy |
| sections | Her grup: `list` (ozellik satirlari) + grubun zengin metinleri kendi adiyla (`Genel Bilgi`, `Genel Uyarılar`, `Havuz Plaj Bilgisi`, `Konum Bilgisi (Açıklama)`, `Spa Bilgisi`, `Aktivite Bilgisi`...; `text`/`list`/`html`); "Etkinlikler" (`eventGroups` aktif: "tarih — baslik", sirali); "Misafir Değerlendirmeleri" (pairs: genel puan, yorum sayisi, kategori puanlari) |

- **Odalar yok**: sayfa tarihsiz acildiginda "Odaları görüntülemek için lütfen tarih giriniz!"; oda listesi yalnizca socket
  fiyat akisinda gelir. `POST /api/hotel/get-room-price {hotelId, checkIn}` oda adlarini + donem bazli cocuk politikasini verir ama
  `checkIn` zorunlu ve donem bazli (zamanla degisir) oldugu icin deterministik statik veri sayilmadi -> `rooms: []`.
- FAQ: `data.faqCategories` Acapulco'da bos; sema bilinmediginden bos dizi.
- Deterministik: siralamalar sequence/line/id/ad ile sabit; `isAvailable`/`lowPrice` gibi dinamik alanlar kullanilmaz. Iki calistirma birebir ayni.

### Arama: `POST /api/autocomplate/hotel`

Sitenin arama kutusu (`fetchHotels` store aksiyonu; yazim hatasi sitede boyle). Govde `{"term":"merit royal","limit":20,"type":"HOTEL"}`
(`type` zorunlu; yoksa `"Bu alan, eksik"`), misafir token header'lari. Yanit `data.hotel[]` (`id`, `name`, `seoPage.slug`,
`destination[]`), `data.destination[]`/`category[]` alinmaz. Sunucu Turkce karakter duyarsiz ("merıt" -> Merit ...). Aday:
`URL = https://www.tatildukkani.com/<slug>`, `City` = son destinasyon (Girne / Gazimağusa), `ProviderHotelID` = id (typeId ile ayni).
Site 3 karakterden kisa sorgu gondermiyor; sonuc yoksa son kelimeler atilarak tekrar denenir.

### Probe (26.10.2026)

```
probe tatildukkani details https://www.tatildukkani.com/acapulco-resort-convention-spa
  0.2 sn -> images=161 facilities=8 rooms=0 faq=0 sections=18; 5 yildiz; rating 8.6 (40 yorum); Girne / Kıbrıs; (35.3299, 33.4210)
  policies: giris 15:00, cikis 11:00, "Evcil hayvan kabul edilmemektedir."; iki calistirma birebir ayni
probe tatildukkani search "Merit Royal"    -> 4 (Diamond 1433, Gardens 1379 Gazimağusa, Hotel 1429, Premium 1434)
probe tatildukkani search "Elexus"         -> Elexus Hotel & Resort & Spa (1440)
probe tatildukkani search "Cratos Premium" -> Cratos Premium Hotel Port Spa (1420)
probe tatildukkani search "Salamis Bay Conti" -> Salamis Bay Conti Resort Hotel (1383)
```
