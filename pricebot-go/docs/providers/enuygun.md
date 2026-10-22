# Enuygun (enuygun.com) provider

Durum: **duz HTTP, cerez/token/tarayici gerekmez.** Site Cloudflare arkasinda ama hem HTML sayfalar
hem de JSON API'ler tarayici header seti olmadan bile 200 donuyor (sadece `__cf_bm`/`_cfuvid`
cerezleri set ediliyor, geri gonderilmesi sart degil). Kod: `internal/providers/enuygun/`.

## Uclar

### 1. Fiyat / teklif listesi — `POST https://www.enuygun.com/otel/search-core/detail-search`

Detay sayfasinin (`/otel/detay/<slug>/`) oda-teklif listesini dolduran API (Next.js bundle'inda
`detailSearch` route'u; `hotel/_next/.../pages/_app.js` icindeki `x=e=>{...hotelId:[t],hos:!0,...}`).

Istek govdesi (JSON):

```json
{
  "hotelId": [580512],
  "hos": true,
  "checkInDate": "05.12.2026",
  "checkOutDate": "08.12.2026",
  "rooms": [{"adults": 2, "children": [12]}],
  "locale": "tr_TR",
  "currency": "TRY",
  "nationality": "tr",
  "pagination": {"page": 1, "limit": 30}
}
```

- Tarih formati **DD.MM.YYYY**. `rooms[].children` = cocuk **yas** dizisi (tam sayi); bos ise `[]` gonder.
- `pagination` zorunlu (yoksa 400 `pagination isteği eksik`). Diger alanlar sitenin gonderdigi set.
- Header: yalnizca `content-type: application/json` yeterli. Kodda yine de tarayici UA, `origin`,
  `referer`, `x-app-market: TR` gonderiliyor (site JS'i `X-App-Market` ve `Accept-Language` yolluyor).
- Yanit ~1 sn. Tek oda destekleniyor (bot zaten tek oda sorguluyor; `rooms` dizisine birden fazla oda
  verilebilir ama normalizer ilk odayi/teklifleri birlestirilmis olarak isler).

Yanit (kullanilan alanlar):

```
status: true
hotels[0].id, .data.name, .data.address.city.name, .availabilityState.Availability
hotels[0].rooms[].type.name                 -> oda adi
hotels[0].rooms[].offers[].concept.description -> pansiyon ("Yarım Pansiyon", "Her Şey Dahil"...)
hotels[0].rooms[].offers[].tryPrice.total.amount (string, TRY, yuvarlanmis) -> satis fiyati (konaklama toplami)
hotels[0].rooms[].offers[].price.total.amount  (string, ham, orn "33149.99999...") -> yedek
hotels[0].rooms[].offers[].discountFrom (number) -> liste fiyati (0 ise indirim yok)
hotels[0].rooms[].offers[].discountTags[] {tag:"provider", discountString:"%35"} -> indirim etiketi
hotels[0].rooms[].offers[].saleable, hiddenState.hidden, roomClass.{code,refundable}, adultCount, childrenCount
```

- Musait degil / otel kapali: `status:true, hotels:[]` (hata degil) -> `available:false`.
- Lead fiyat = `saleable && !hidden` tekliflerin en ucuzu (tum odalar). `listPrice` = `discountFrom`
  (satis fiyatindan buyukse). `discountRate` list/price'tan hesaplanir (siteyle ayni: %35).
- `currency` her zaman TRY -> `TL`. (Ayrica `currency_price` USD karsiligi var, kullanilmiyor.)
- `discountTags[].tag == "provider"` olanlar kampanya adi olarak **yazilmiyor** (standart tedarikci
  indirimi); baska tag gelirse `campaignName` = `label + discountString`.

### 2. URL cozumleme — `GET https://www.enuygun.com/otel/api-detail/<slug>.json?slug=<slug>&type=hotel`

Detay sayfasinin SSR verisi (`__NEXT_DATA__.props.pageProps.detailInfo` ile ayni icerik).
`data.defaultParameters.hotel.{id,slug,name}`, `data.region.name`, `data.hotel.details.{domestic,isCyprus}`.

- Slug sonundaki sayi otel id'si **olabilir** (`acapulco-resort-convention-spa-580512`) ama her otelde
  yok (`club-simena`, `dome-hotel`), bu yuzden id her zaman api-detail'den alinir; api-detail cevap
  vermezse ve slug'da sayi varsa onunla devam edilir.
- Kabul edilen URL bicimleri: `/otel/detay/<slug>/`, `/otel/<slug>/`, `/en/hotels/detail/<slug>/`.
  `/otel/bolge/...`, `/otel/tema/...` otel degildir (hata).

Mapping: `providerHotelId` = sayisal otel id'si (zorunlu, tek gereken). Metadata: `slug`, `sourceUrl`,
`region`, `isDomestic`, `isCyprus` (bilgi amacli).

## Kisi / cocuk

- Yetiskin: `rooms[0].adults`. Cocuk: `rooms[0].children` yas listesi. Dogrulandi:
  Acapulco 2 yetiskin = 33.150 TL; +1 cocuk (5 yas) = 33.150 TL (ucretsiz, `childrenCount:1`);
  +1 cocuk (12 yas) = 45.581,25 TL (ucretli). Negatif yas 0'a cekilir.

## Paket (otel + ucak)

**Desteklenmiyor.** Enuygun'un "Otel + Ucak" urunu (`/otel/paket-arama/`, `/otel/paket-detay/`,
`isDynamicPackageEligible`) tur operatoru paketi degil, **dinamik paket**: ayni `detail-search`
(`source:"dynamic_package"` ile, ayni otel fiyatlari doner) + Enuygun'un kendi ucus arama API'sinin
tarayicida birlestirilmesi. Tek bir "paket fiyati" veren API yok; transfer de yok. Bu yuzden
`SupportsPackages=false`. (Istenirse otel fiyati + ayri ucus aramasi olarak ileride eklenebilir.)

## Probe ciktisi (2026-10-22)

```
probe enuygun price https://www.enuygun.com/otel/detay/acapulco-resort-convention-spa-580512/ 2026-12-05 2026-12-08 2
  resolve 0.5s -> 580512 "Acapulco Resort Convention & Spa", Girne, isCyprus
  price   1.0s -> Acapulco Villa / Yarım Pansiyon, 33.150 TL (liste 51.000, %35), TL
... 2 1 12   -> 45.581,25 TL (liste 70.125, %35)        # 2 yetiskin + 12 yas cocuk
... club-simena/ 2 1 5 -> 580531 "Club Simena" cozuldu; bu tarihlerde hotels:[] -> available:false
... ela-excellence-resort-belek-all-inclusive-22494/ 2 -> 22494, Antalya (domestic),
     Superior Bahçe Manzaralı Oda / Excellence Her Şey Dahil 45.290,40 TL (liste 56.613, %20)
```

## Bilinen sinirlar / notlar

- Paket yok (yukarida). Bulk = `DefaultBulk` (otel basina 1 POST, ~1 sn).
- `hotelId` dizisi birden fazla otel alabiliyor (`hotelId:[a,b]`) — ileride gercek bulk icin kullanilabilir
  (denenmedi).
- Cloudflare bugun engellemiyor; engellerse ilk adim tarayici header setini (zaten gonderiliyor) ve
  `__cf_bm` cerezini tasimak olur (`httpx` cerez saklamaz; gerekirse `resp.SetCookies()` ile).
- Sayfa HTML'i CDN'de UA bazli cache'leniyor (tarihsiz detay sayfasi `parsedUa` Amazonbot gostermisti);
  fiyat icin HTML degil, `cf-cache-status: DYNAMIC` olan `detail-search` kullanildigindan etkilenmez.
- Fiyatlar konaklama toplami (3 gece), gecelik degil. `offerType:"TOTAL"`.
