# Etstur provider (`etstur`)

Durum: **duz HTTP, tarayici gerekmez.** Fiyat (destinasyon arama), paket (room/multi), URL cozumleme, **statik detay** ve **isimle arama**.
Site Next.js; detay sayfasinda `__NEXT_DATA__` var, ayni veri JSON API'den de aliniyor. Cerez zorunlu degil
(`ETSTUR_COOKIE` varsa fiyat aramasina eklenir).

## URL cozumleme (`ResolveFromURL`)

`GET https://www.etstur.com/<Slug>` -> `__NEXT_DATA__.props.pageProps.data`: `hotelId` (hash, providerHotelId), `name`,
`url` (slug), `location.locationBreadCrumbs[0].url` (destinasyon listesi, orn. `Kibris-Otelleri`).
Metadata: `destinationUrl`, `slug`, `sourceUrl`.

## Fiyat / paket (ozet)

- `POST /services/api/search/v2/hotels` — destinasyon listesi sayfalari (`url`, `checkIn/Out` `yyyy-MM-ddT00:00:00.000`,
  `rooms[]`, `offset/limit`); otel `hotelId` ile eslenir. Bulk sorgular destinasyona gore gruplanir.
- `POST /services/api/room/multi` (`hotelId, checkIn, checkOut, rooms[]`) -> `roomSearchId` + `roomGroups[].rooms[]`;
  `POST /services/api/room/multi/package` (`roomSearchId, airportCode`) -> paket odalari (ayni oturum cookie'si ile).

## Statik detay (`GetDetails`)

Kaynak: `GET https://www.etstur.com/services/api/hotel/detail/<slug>` (header: `defaultHeaders`, cookie gerekmez) ->
`{success, result}`. Slug metadata'dan, yoksa `sourceUrl`'in son yol parcasindan. API basarisiz olursa detay sayfasi
`__NEXT_DATA__` ile ayni semaya dusulur. Alan eslemesi (`result`):

| Sema | Kaynak |
|---|---|
| `name`, `stars` | `name`, `starRating` (Kibris otellerinde genelde null) |
| `description` | aktif `infos[].generalInfo.generalInfoText` (HTML temizlenir) |
| `rating` / `ratingCount` | `reviews.average.score` (0-100 -> /10), `reviewCount` |
| `address`, `city`, `region`, `country` | `contact.addressLines`, `location.city`, `location.customRegion`, `location.country` |
| `location` | `poiMap.geoLocation` |
| `contact` | `contact.email`, `contact.phone` |
| `images` | `images[]` (imgproxy URL'leri; sorgu parametresi yok; site sirasi korunur) |
| `facilities` | `generalInfo.facilities[]` (grup: `mainTitle` yoksa "Genel Özellikler") + `facilityCategories[].title.label` gruplari; madde = `label (description) (paidType)` |
| `rooms`, `boards` | `POST /services/api/room/multi` sabit pencere (bkz. asagi); `roomName` pansiyon eki atilir (`" - Yarım Pansiyon"`), ayni oda birlesir, ada gore sirali; `roomSize`, `maxAdultCapacity`, `description` (cift escape'li HTML cozulur), `facilities[].label` (+ "(Ücretli)"), `images[]`; `boards` = `subBoards[].boardType.label` |
| `policies` | `importantInfo.checkInTime/checkOutTime`; `childPolicy` = "Çocuk" kategorisinin metni; `petPolicy` = metinlerde "evcil" gecen ilk cumle; `other` = `importantInfo.desc` paragraflari |
| `faq` | `faqs[]` |
| `sections` | "Genel Bilgiler" (text), "Temalar" (list), her `facilityCategories` kategorisi (text+items), "Önemli Bilgiler" (text), "Etkinlik Takvimi" (list, `events[]` tarihe gore sirali), "Konum / Önemli Noktalar" (pairs, `poiMap.landmarks` tur+ada gore sirali — API her istekte farkli sirada donuyor), "Misafir Değerlendirmesi" (pairs) |

Alinmayanlar: `promotions` (banka kampanyalari, surekli degisir), `reviews.userReviews` (tekil yorumlar), `banners`.

Aktif sezon: `infos[]` icinde `active=true` olan (yoksa ilk). Sezon degisiminde (Yaz/Kis) icerik dogal olarak degisir.

Oda penceresi: `staticStayWindow` = iki ay sonrasinin 1'i, 3 gece (ay icinde sabit). Odalar musaitlige bagli
oldugundan pencere/ay degisiminde oda listesi degisebilir. room/multi hata verirse odalar bos liste olur.

Probe (Acapulco, 2026-10-26): `images=183 facilities=8 rooms=6 faq=17 sections=14`, ~1 sn; iki ardisik cagri `fetchedAt`
haric birebir ayni JSON.

## Isimle arama (`SearchHotels`)

`GET https://www.etstur.com/v2/autocomplete?q=<metin>` (cerez yok) -> `result[] {title, type, url, city, country, region}`.
Sadece `type == "HOTEL"` alinir; `title` "Ad, Semt" bicimindeyse semt ayrilip `city`'ye eklenir
(`"Merit Royal Premium Hotel & Spa, Alsancak"` -> ad + `Alsancak, Girne`). URL = `https://www.etstur.com/<url>`
(ResolveFromURL kabul eder). `providerHotelId` autocomplete'te yok (hash sayfadan cozulur). Sonuc yoksa sorgu son
kelimesi atilarak tekrarlanir (min 3 karakter).

Ornek: "Merit Royal" -> Diamond / Premium / Gardens (3); "Elexus" -> Elexus Hotel Resort; "Cratos Premium" -> Cratos
Premium Hotel Port Spa; "Salamis Bay Conti" -> Salamis Bay Conti Resort Hotel (Yeni Boğaziçi, Gazimağusa).

`/v2/autocomplete/suggestions?type=SEARCH` sorgu almaz (populer oneriler), kullanilmaz.

## Bilinen sinirlar

- `starRating` Kibris otellerinde null -> `stars` yok.
- Odalar fiyat aramasindan geldigi icin musait olmayan oda tipleri listede gorunmez.
- Telefon alani cogunlukla null.
