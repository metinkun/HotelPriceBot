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
