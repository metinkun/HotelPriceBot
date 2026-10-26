# tatilcikus (tatilcikus.com)

Durum: **duz HTTP JSON API, cerez/token/tarayici gerekmez.** Paket (otel+ucak+transfer) canli sorgulanamiyor (bkz. Sinirlar).

## Site mimarisi

- `www.tatilcikus.com` React tek sayfa uygulamasi; HTML kabugu bos (`<div id="app">`), tum veri `https://api.tatilcikus.com/<Servis>/` (ASP.NET) JSON API'sinden gelir.
- Otel detay yolu: `https://www.tatilcikus.com/<slug>` (orn. `acapulco-resort-hotel-spa`). Arama parametreleri: `?start=DD.MM.YYYY&end=DD.MM.YYYY&room-0-adult=2`.
- Tatilcikus kendi fiyat uretmez; otelin `priceOperator` alanina gore **tedarikci** API'sine proxy yapar:

| priceOperator | Tedarikci | Uc | hotelId | Otel sayisi (Ekim 2026) |
|---|---|---|---|---|
| 2 | Coral | `POST Coral/GetAvailableRooms` | `coralId` | 210 |
| 4 | Akgunler (Kibris) | `POST Akgunler/GetAvailableRooms` | `akgunlerId` (orn. `ACAPULCO`) | 46 |
| 3 | Gezinomi | `POST Gezinomi/GetAvailableRooms` | `gezinomiId` | 13 |
| -1 | yok | — | — | 51 ("Sizi Arayalim", online fiyat yok -> hata doner) |
| 0 / 1 | Periods / Puzzle | sitede kullanilmiyor | | 0 |

## Uclar

1. **Otel detayi / URL cozme**: `GET Hotels/GetByUrlWithDetail?url=<slug>` -> `data.hotel` (id, name, priceOperator, coralId/gezinomiId/akgunlerId,
   commissionRate, isActiveCustomerNetPriceCommission, customerNetPriceCommission, firstChildAge, hasPackage, packagePrice, cityId),
   `data.rooms[]` ve `data.concepts[]` (id, name, isShown, openForSale, coralIds/gezinomiIds/akgunlerIds). 10 dk bellek cache.
2. **Musaitlik**: yukaridaki tedarikci ucuna JSON govde:
   ```json
   {"hotelId":"ACAPULCO","startDate":"05.12.2026","endDate":"08.12.2026","adultCount":2,"childrenBirths":["05.06.2020"]}
   ```
   - Tarih **DD.MM.YYYY** zorunlu (ISO -> 500 Internal Server Error).
   - Cocuklar **dogum tarihi** (DD.MM.YYYY) olarak verilir; bot yas -> `giris - yas yil - 6 ay` cevirir. Sitede oda basina en fazla 2 cocuk.
3. Yardimci (opsiyonel): `GET Cities/` (sehir adi), `GET CustomerCommissions/` (net-fiyat otelleri icin tarih bazli komisyon override, hotelId ile filtrelenir).
4. Header: `accept: application/json`, `content-type: application/json`, `origin/referer: https://www.tatilcikus.com`. `Authorization: Bearer` sadece giris yapilmis kullanicida eklenir; anonim calisir. `Hotels/GetAllByFilter` gibi yonetim uclari 401 (gerekmiyor).

## Fiyat normalizasyonu (sitenin `cartInfo` mantigi, bundle'dan cikarildi)

Her teklif detaydaki oda/pansiyon ile tedarikci id'si uzerinden eslenir; `isShown && openForSale` olmayanlar atilir. Sitede gosterilen fiyat:

| Tedarikci | Satis (price) | Liste (ustu cizili) | Indirim % | Durum |
|---|---|---|---|---|
| Akgunler | `SalePrice` | `GrossPrice` | `CommissionPercentEB` | `StopSale/Release` -> STOP_SALE, `AskSale` -> ON_REQUEST |
| Coral | `price` | `originalPrice` | `discount` | `saleStatus` 0 musait, 2 sorulacak, 1/3 stop; para birimi `currency.sname` |
| Gezinomi | `afise_fiyat` | `brut_fiyat` | `intDiscountPercent` | `reasons[]`: Stop/Realesa/Min Geceleme/... -> STOP_SALE, Kota/Sot-Sat -> ON_REQUEST |

- Lead fiyat = en ucuz **AVAILABLE** teklif. Hic yoksa en ucuz ON_REQUEST teklif fiyatiyla doner ama `available=false` (+ `campaignName` notu). Stop-sale'ler alinmaz.
- `isActiveCustomerNetPriceCommission=true` (17 Akgunler oteli: Merit, Elexus, Lord's Palace...) icin sitenin "Mt" formulu uygulanir:
  `price = GrossPrice - SUM(gun) trunc(GrossAmount * (komisyon + CommissionPercentEB) / 100)`; komisyon = o gun icin CustomerCommissions override, yoksa `customerNetPriceCommission`. `CommissionPercentEB=0` ise fiyat `GrossPrice`.
- Sinir yasi ustu cocuk: `firstChildAge` (Acapulco: 11) ustu cocuk icin tedarikci **bos liste** doner (12 yas -> `data: []`). Bot bos sonucta bu cocuklari yetiskin sayarak bir kez daha sorar (sitede kullanicinin yapacagi gibi).

## Mapping

- `providerHotelId` = Tatilcikus otel id (orn. `5`), `metadata.slug` zorunlu (`acapulco-resort-hotel-spa`), `sourceUrl` saklanir.
  `ResolveFromURL` ayrica bilgi amacli `priceOperator`, `supplierId`, `packageFromPricePerNight`, `packageMinNight` yazar.

## Probe ozeti (23.10.2026, 05-08 Aralik 2026)

```
probe tatilcikus price https://www.tatilcikus.com/acapulco-resort-hotel-spa 2026-12-05 2026-12-08 2
  resolve 0.6s -> id 5, Akgunler/ACAPULCO
  price 3.4s   -> Villa Oda / Yarım Pansiyon, 28.050 TL (liste 51.000, %45), Girne, available=true
... 2 1 6   -> ayni (6 yas cocuk ucretsiz): 28.050 TL
... 2 1 12  -> tedarikci bos; fallback 3 yetiskin -> Villa Oda 38.568,78 TL (liste 70.125,03)
abant-aden-boutique-hotel-spa (Coral)  -> Aden Suit / Oda Kahvaltı 23.797,50 TL (liste 25.050, %5) saleStatus=2 -> available=false (Talep Et)
ng-enjoy (Gezinomi, 13.7s)             -> Deluxe Oda / Yarım Pansiyon 52.440 TL (liste 57.000, %8), Sakarya
probe ... package ... SAW              -> "provider paket desteklemiyor"
```
Sureler: Akgunler/Coral 1-4 sn, Gezinomi 10-15 sn olabilir (timeout 40 sn).

## Sinirlar

- **Paket yok**: `hasPackage=true` otellerde detay sayfasi sadece statik "N GECE" kartlari gosterir: `packagePrice * gece + siteSettings.packageExtraPrice`
  (Acapulco: 7.680 TL/gece + 3.550) ve "Sizi Arayalim" butonu. Canli ucak sorgusu (`Akgunler/GetAvailableFlights`) yalnizca rezervasyon akisinda (temp sale) kullaniliyor;
  kalkis havalimani secimi yok. Bu nedenle `SupportsPackages=false`; baslangic fiyati metadata'da (`packageFromPricePerNight`) tutulur.
- `priceOperator=-1` oteller (51 adet) online fiyat vermez -> `GetPrice` hata doner.
- Cocuk yasi dogum tarihine cevrilir (giris - yas - 6 ay); sinir yasi ustu cocuk yetiskin sayilir (yukarida).
- Coral yanitindaki `night` / min konaklama bilgisi su an `minStayNights` olarak doldurulmuyor.
- Tarayici dogrulamasi: headless Chromium'da detay sayfasi URL parametreleriyle acildiginda oda listesi render edilmedi (SPA store'u sadece arama bileseninden tetikleniyor);
  gosterilen fiyat alanlari bundle'daki `cartInfo` kodundan birebir cikarildi (SalePrice/price/afise_fiyat) ve `%45` indirim rozeti `CommissionPercentEB` ile tutarli.
- Cloudflare onunde ama anonim JSON istekleri engellenmiyor; proxy rotasyonu `httpx` uzerinden calisir.

## Statik detay (`GetDetails`) ve isimle arama (`SearchHotels`)

Kod: `details.go`, `search.go`, `html.go` (HTML -> metin/liste yardimcilari). Tarayici/cerez gerekmez.

### Kaynaklar

- `GET Hotels/GetByUrlWithDetail?url=<slug>` (fiyatta da kullanilan yanit, ~210 KB): `hotel.{name, generalText, childText,
  marriedText, callCenterText, tagGroupHotelTexts[], score, googleMapsUrl, folderName, cityId/districtId/countryId,
  tripadvisor*, cancelOption, moneyOption}`, `highlights[]`, `hotelTags[]` (tagGroupId, tagId, isPaid), `roomTags[]`,
  `hotelImages[]` (groupId, roomId, url, isShown), `hotelVideos[]`, `rooms[]` (name, description HTML), `concepts[]` (name, description HTML).
- `GET Tags/` (255 tag: id, tagGroupId, name, isShown, status) ve `GET TagGroups/` (9 grup: Genel, Plaj & Havuz, Spa, Aktivite,
  Balayı, Çocuk, Konum, Kurumsal, Oda) — tag adlari ve grup basliklari; 1 saat bellek cache. `isShown=false`/`status=false` tag'ler atilir.
- `GET Cities/`, `Districts/`, `Countries/` — sehir/ilce/ulke adi (1 saat cache; alinamazsa bos kalir).
- Gorsel URL'i (bundle'dan): `https://www.tatilcikus.com/images/hotels/<folderName>/gallery/optimized/<url>.jpg` (`.webp` de var).
  Galeri grup etiketleri: groupId 1 Genel, 2 Odalar, 3 Yeme İçme, 4 Çocuk, 5 Aktivite. `roomId>0` olanlar ilgili odaya da yazilir.

### Normalizasyon

| Alan | Kaynak |
|---|---|
| name, description | `hotel.name`, `generalText` (HTML -> metin) |
| rating | `hotel.score` (10 uzerinden; Tripadvisor puani ayri bolumde) |
| city / region / country, location | Cities/Districts/Countries; `googleMapsUrl` icindeki `@lat,lng` |
| images | `isShown` gorseller, (groupId, id) sirali; caption = grup etiketi veya `description` |
| facilities | TagGroups sirasiyla; grupta `hotelTags` tag id sirali; ucretli ise "(Ücretli)" eki |
| rooms | `isShown` odalar (id sirali): aciklama metni, m2 (`\d+ m²` regex, ilk sayi), kapasite ("Maksimum ... konaklayabilmektedir"), `roomTags` ozellikleri, `hotelImages[roomId]` |
| boards | `concepts[].name` (`isShown`) |
| policies | childText -> childPolicy; Other: tedarikci cocuk yas siniri (`firstChildAge`), iptal/para iade secenegi |
| sections | "Öne Çıkanlar" (highlights), "<Otel> Hakkında" (generalText), tag grubu metinleri (grup adiyla, orn. "Plaj & Havuz"), "Çocuk Bilgileri", "Balayı Tesisi", "Çağrı Merkezi Bilgilendirmesi", "Pansiyon: <ad>" (tablo iceren HTML -> `html`, temizlenmis), "Tripadvisor" (pairs), "Video" |

Bolum turu: yalnizca `<li>` -> `list`; `<table>` var -> `html` (script/style/attribute'siz) + `text`; aksi `text`.
FAQ sitede yok (bos dizi). Yildiz bilgisi API'de yok. Iletisim: sitedeki telefonlar Tatilcikus cagri merkezi oldugu icin alinmaz.
Deterministik: tum listeler id/grup sirali; `dateOf*` alanlari ve fiyat (`packagePrice`) kullanilmaz.

### Arama

Ayri autocomplete ucu yok; sitenin arama kutusu gibi `GET Hotels/GetSummary` (tum oteller: id, countryId, name, url; ~320 kayit,
10 dk cache) istemci tarafinda filtrelenir: Turkce karakter/buyuk-kucuk harf duyarsiz, sorgunun tum kelimeleri ad veya slug icinde.
Sira: tam ad, ad sorguyla basliyor, digerleri; esitlikte ada gore. Sonuc yoksa son kelime atilarak tekrar denenir.
`City` alaninda **ulke adi** doner (liste sehir vermiyor). URL: `https://www.tatilcikus.com/<url>`.

### Probe (26.10.2026)

```
probe tatilcikus details https://www.tatilcikus.com/acapulco-resort-hotel-spa
  0.8-5 sn -> images=397 facilities=8 rooms=14 faq=0 sections=14; rating 8.9; Girne / Çatalköy / Kıbrıs; (35.330273, 33.4209)
  Villa Oda 30 m2, 14 ozellik, 4 gorsel; boards [Tam Pansiyon Plus, Yarım Pansiyon]; iki calistirma birebir ayni (fetchedAt haric)
probe tatilcikus search "Merit Royal"       -> 4 (Diamond, Gardens, Hotel & Spa, Premium)
probe tatilcikus search "Elexus"            -> Elexus Hotel Resort & Spa (49)
probe tatilcikus search "Salamis Bay Conti" -> Salamis Bay Conti Hotel (75)
```
