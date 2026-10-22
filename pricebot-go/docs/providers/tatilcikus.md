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
