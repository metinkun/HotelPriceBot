# Prontotour (`prontotour`)

Site: https://www.prontotour.com — ASP.NET WebForms kabuk + tum veri `https://webapi.prontotour.com/corewebapi`
JSON API'sinden (tarayicida JS ile cekiliyor). **Tarayici gerekmez, cerez/token gerekmez**: API acik CORS ile
yanit veriyor; sadece `content-type: application/json` + tarayici UA/origin/referer header'lari gonderiliyor.
Yanit sureleri: HotelDetail ~0.3-1 sn, BundleSearch (ucus aramasi dahil) ~4-6 sn.

## Uclar

| Amac | Istek | Not |
|---|---|---|
| Otel / havalimani arama | `POST /AutoComplete/Search` `{"Key":"acapulco","Type":"H","Id":0,"Source":1}` | `Type`: `H` otel/destinasyon, `F` havalimani, `HF` ucakli paket. Yanit: `code` (otel kodu), `IdDest`, `AirPortCode`, `IdDestType` (5 = otel, 4 = havalimani) |
| Oda/fiyat listesi | `POST /Hotel/HotelDetail` | govde asagida; `HotelRoomDetailsList` oda x pansiyon satirlari |
| Paket (otel+ucak) | `POST /Bundle/BundleSearch` | `SourceType:"HF"`, `HotelRequest.FirstHotel:true`, `HotelCode:"<kod>-<tip>"` |
| Dolar kuru (gerekmiyor) | `POST https://www.prontotour.com/Ws.aspx/GetUsdKur` | site CurrencyRequest.Dolar'i buradan dolduruyor; API Dolar bos olsa da calisiyor |

### HotelDetail govdesi (otel-only)

```json
{"Code":"3234","CityCode":"","Room":[{"HotelTravelers":[{"Type":0,"Age":"35"},{"Type":0,"Age":"35"}]}],
 "HotelServiceType":"8","StartDate":"2026-12-05","EndDate":"2026-12-08","SearchKey":"",
 "CurrencyRequest":{"Dolar":"","Euro":"","Currency":"TL"},"SourceType":"H","Packing":false,"Bundle":false,"S_GUID":""}
```

- Tarih: `yyyy-MM-dd` (standart ile ayni).
- Yolcu: yetiskin `{"Type":0,"Age":"35"}` (string!), cocuk `{"Type":1,"Age":<yas>}` (1-14), bebek `{"Type":2,"Age":0}`.
  Cocuk yasi fiyati etkiliyor (5 yas ucretsiz, 12 yas: 30.345 -> 41.725 TL). Yas verilmeyen cocuk 7 yas sayilir.
- `HotelServiceType`: detay URL'inin sonundaki ek (`-h3234-8` -> kod 3234, tip 8). Yanlis tip -> bos yanit.
- Tarihler bos verilirse otel kimligi (ad, adres) gelir, fiyat gelmez — `ResolveFromURL` bunu kullanir.
- Yanit: `HotelListResponse` (tek nesne; `PriceTL` en ucuz liste, `DiscountPriceTL` cogu zaman 0),
  `HotelRoomDetailsList[]`: `RoomName`, `Mealy` (pansiyon), `PriceTl`, `DiscountPriceTl`, `Allotment`, `FreeCancel`.
  Lead fiyat = `Allotment:true` odalar arasinda en ucuk `DiscountPriceTl||PriceTl` (site ile ayni).

### Paket akisi (detay sayfasindaki `HotelDetailSearch`)

1. `AutoComplete/Search` `Type:"F"` ile IATA -> `IdDest` (SAW=25078, IST=23709, ADB=22276, ESB=23237).
2. `Bundle/BundleSearch`:
   ```json
   {"FlightRequest":{"IsTurkey":true,"DepartureDate":"2026-12-05","ArrivalDate":"2026-12-08","RouteType":"1",
      "Travelers":[{"Type":0,"Age":"35"},{"Type":0,"Age":"35"}],"MultiFlightRequest":[]},
    "LocationRequest":{"FromType":"F","FromId":"25078","ToType":"","ToId":0,"TourCode":""},
    "HotelRequest":{"Checkin":"2026-12-05","Checkout":"2026-12-08","Room":[{"HotelTravelers":[...]}],
      "Packing":true,"HotelPackage":"NO","FirstHotel":true,"HotelCode":"3234-8","TagName":"","IsTag":false,"RCI":false},
    "TourRequest":null,"CurrencyRequest":{"Dolar":"","Euro":"","Currency":"TL"},"SourceType":"HF"}
   ```
   **`CurrencyRequest.Currency:"TL"` sart** — bos birakilirsa `ErrorText: "Sonuc bulunamadi"`. `LocationRequest.DestinationDetail`
   GONDERILMEMELI (gonderilirse tum destinasyon listesi, ~1.8 MB, doner).
   Yanit: `HotelListResponse[0]` = otel; `PriceTL/DiscountPriceTL` = **en ucuz oda + secili gidis/donus ucuslari (+transfer)**,
   `FlightResponse[]` (Selected:true olanlar secili ucuslar), `S_GUID` (oturum), `TotalPrice` (en ucuz oda + ucus).
3. `Hotel/HotelDetail` ile `Packing:true,Bundle:true,SourceType:"HF",S_GUID:<2. adimdan>`: oda listesi **en ucuz odaya gore
   fark** olarak gelir (en ucuz oda 0/0). Site her odayi `taban(DiscountPriceTL) + fark` olarak gosterir; provider da ayni
   formulu uygular. S_GUID gecersiz/eski ise bos yanit gelir; o durumda yalnizca taban paket tek satir ("En uygun paket") doner.

## Mapping

- `providerHotelId`: otel kodu (`3234`). Metadata: `serviceType` (`"8"`), `sourceUrl`.
- `ResolveFromURL`: `https://www.prontotour.com/<Seo-Ad>-h<kod>-<tip>[?...]` — kod/tip URL'den, ad tarihsiz HotelDetail'den.
  Liste URL'leri (`/Kibris-Otelleri-H26059`) kabul edilmez.
- Otel URL'i bulmak icin: `AutoComplete/Search` `Type:"H"` ile ara, `code` + tip ile detay URL'i kur
  (tip liste yanitindaki `HotelServiceType`; Kibris otellerinde 8).

## Probe ciktisi (2026-10-23, Acapulco Resort Hotel, 05-08 Aralik 2026)

- `price ... 2` -> Villa Oda / Yarim Pansiyon, liste 51.000 TL, satis **30.345 TL** (%41), 0.7 sn.
- `price ... 2 1 12` (2 yetiskin + 12 yas cocuk) -> ayni oda, liste 70.125 TL, satis **41.725 TL**.
- `package ... 2 SAW` -> 3 oda x 2 pansiyon; en ucuz **41.597 TL** (Villa Oda YP, liste 61.341), 4.1 sn.
  Note: `Secili ucuslar: SAW→ECN 05.12 05:45 PC1912 / ECN→SAW 08.12 14:35 VF158 (AJET)`.
  Kontrol: otel 30.345 + ucus 2 x (2.764 + 2.403) = 40.679 = API `TotalPrice`; otel `DiscountPriceTL` 41.597 (transfer farki ~900 TL).

## Bilinen sinirlar

- Sehir alani HotelDetail'de null (Address serbest metin); `city` bos doner.
- Paket fiyati API'nin otomatik sectigi en ucuz ucus ciftine gore; ucus secimi/transfer opsiyonu yok.
  Ucus fiyatlari aramadan aramaya birkac TL oynayabiliyor (fixture 41.597 vs ilk olcum 41.590).
- `Allotment:false` odalar (kontenjan yok) lead fiyat ve paket listesine alinmaz; hic kontenjanli oda yoksa tumu kullanilir.
- Tek oda sorgulanir (coklu oda yok). Bebek (0 yas) fiyati etkilemiyor.
- Donus tarihli ucus olmayan tarihlerde BundleSearch `ErrorText` doner; sonuc `available:false` + `note` olarak iletilir.
