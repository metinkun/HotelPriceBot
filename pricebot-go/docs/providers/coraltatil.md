# Coral Tatil (coraltatil.com) provider

Durum: **tarayici ZORUNLU.** Site Imperva/Incapsula bot korumasi arkasinda: duz HTTP ile hicbir sayfa acilmaz
(212 baytlik JS challenge, `visid_incap_*`/`incap_ses_*` cerezleri + TLS parmak izi). Cerezler HTTP istemcisine
tasinamadigi icin tum istekler headless Chromium icinde, sitenin kendi origin'inden `fetch` ile yapilir.
Kod: `internal/providers/coraltatil/` (`api.go` sekme + JS, `normalizer.go`, `provider.go`).

Sekme modeli: **tek kalici sekme**, ana sayfa (`https://www.coraltatil.com/`) acik tutulur; challenge gecildikten
sonra (`__NEXT_DATA__` + govde metni) tum sorgular ayni sekmede `fetch` ile yapilir (istekler serilestirilir,
`PROVIDER_CONCURRENCY` varsayilani `coraltatil=1`). Sekme `CORAL_SESSION_MINUTES` (20) sonra veya 40 kullanimda
yenilenir. Site Next.js; `buildId` sayfadaki `__NEXT_DATA__`'dan okunur, deploy sonrasi veri rotasi 404 verirse
sayfa yenilenip bir kez daha denenir.

## Uclar (sira ile, hepsi sayfa icinden)

Ortak basliklar: `Accept: application/json`, `Content-Type: application/json`, `CurrencyCode: TRY`,
`DepartureId: 63-5` (Istanbul), `LanguageCode: tr-TR`, `MobileType: Web`, `X-Correlation-ID: <uuid>`.

### 1. Arama kimligi — `POST /endpoints/OnlyHotelProduct/PriceSearchEncrypt`

```json
{"reservationType":2,"beginDates":["2026-12-15","2026-12-15"],
 "roomCriterias":[{"passengers":[{"passengerType":0,"birthDate":null,"age":20,"isChildOrInfant":false},...,
                                 {"passengerType":1,"birthDate":null,"age":5,"isChildOrInfant":true}],
                   "adultPassengerCount":2,"childOrInfantCount":1,"childOrInfantPassengers":[...],"adultPassengers":[...]}],
 "nights":[{"value":3}],
 "arrivalLocations":[{"id":"3787-7","locationId":"3787","locationUniqueId":"3787-7","type":7,"name":"Salamis Bay Conti","friendlyUrl":"salamis-bay-conti","countryIds":["61"]}],
 "paging":{"pageNumber":0,"pageSize":20,"sortType":3},"imageSizes":[4],"additionalFilters":[]}
```

Yanit `result.queryParams.sidp` (uuid). Otel kimligi = `arrivalLocations[].locationId` (type 7 = otel). Yetiskin
`passengerType 0` (age 20), cocuk `passengerType 1` + yas. Tek yetiskin/cocuklu sorgular dogrulandi.

### 2. Teklifler — `GET /_next/data/<buildId>/<slug>.json?sidp=<sidp>&availability=2&p=2&pagePath=<slug>`

(`x-nextjs-data: 1`). `sidp`'siz cagri sitenin varsayilan aramasini (yarin+7 gece) doner; `p=2` zorunlu.
`pageProps.layout` icinde `widgetType=12` ("Hotel Detail Widget") -> `widgetData.hotelDetailData.onlyHotelPriceSearchResponse`:

- `hotelDetail {id, name, friendlyUrl, locationSummary, place{name}, area, region, country}`
- `rooms {roomKey: {name, roomSize, maxPax, images...}}`, `meals {mealKey: {name}}`, `accommodations {key: {name "2 ADL"}}`
- `products[]`: `rooms[0].{roomKey, mealKey, stopSale}`, `availability` (1 = satista), `checkInDate/checkOutDate/stayNights`,
  `price {oldAmount (liste), amount (satis), discountPercent, currency "TRY"}`, `offerId` (rezervasyon icin).

Normalizasyon: her product bir oda+pansiyon teklifi (`price.rooms`); lead = en ucuz `availability=1 && !stopSale`;
hic yoksa "sorulacak" (`on_request`) tekliflerin en ucuzu `available:false` + Note. `stopSale` -> `sold_out`.
Satista olmayan odalar genelde yanitta hic yer almaz (musait olmayan tarihlerde oda sayisi azalir).

### 3. Otel cozumleme — ayni veri rotasi `sidp`'siz: `hotelDetail.id/name/friendlyUrl` (resolve). Slug URL'den:
`https://www.coraltatil.com/<slug>/[?sidp=...]`.

**Dikkat (slug cakismasi):** ayni `friendlyUrl` birden fazla otelde olabilir (orn. `colony-hotel` = Girne 3765 ve Roma 39779);
`sidp`'siz veri rotasi bunlardan birini (Roma) doner. Fiyat akisinda `sidp` otel kimligini tasidigi icin dogru otel gelir;
bu yuzden eslestirmede kimlik tercihen isimle aramadan (`ListArrivalLocations`, sehir bilgisiyle) alinmali, URL ile
cozumlemede sehir/ad kontrol edilmelidir. KKTC listesinde Colony (ext_8) kimligi elle 3765 yapildi.

### 4. Isimle arama — `POST /endpoints/OnlyHotelProduct/ListArrivalLocations {"text":"salamis"}`
-> `result.locations[]` (`type 7` otel: `id "3787-7"`, `name`, `friendlyUrl`, `parent.name "Magusa, Kuzey Kıbrıs, Kıbrıs"`).
Diger tipler (ulke/bolge/sehir) elenir. Parametre adi `text` (digerleri tum ulke listesini doner).

## Probe ciktisi (16 Kas 2026)

- `probe coraltatil search Salamis` -> 2 aday (Salamis Bay Conti 3787, Crown Salamis Hotel 77985), 2.8 sn.
- `probe coraltatil price https://www.coraltatil.com/salamis-bay-conti/ 2026-12-15 2026-12-18 2` -> 7 oda, lead 32.400 TL
  (liste 60.000, %46), Ultra Her Sey Dahil.
- Acapulco 2 yetiskin + 1 cocuk (5): 12 teklif (6 oda x YP/TPP). Resolve `the-arkin-iskele-hotel` -> 50808.

## Bilinen sinirlar

- Paket (otel+ucak) yok (`PackageTourHotelProduct/*` uclari var, uygulanmadi).
- Statik detay (`GetDetails`) uygulanmadi (hotelDetail.marketingText + rooms sozlugu ile eklenebilir).
- Tek sekme: ayni anda bir Coral sorgusu; her sorgu ~2-4 sn. Imperva oturumu ~20 dk sonra yenilenir.
- Keşif betikleri: tarayici ile `capture js` (bkz. provider-authoring.md); duz `curl` her zaman challenge sayfasi alir.
