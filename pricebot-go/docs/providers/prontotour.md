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

## Statik detay (`GetDetails`) ve isimle arama (`SearchHotels`)

### Detay — tarihsiz `POST /Hotel/HotelDetail`

Ayni govde, `StartDate/EndDate` bos (2 yetiskin). Yanit otel kimligi disinda su statik alanlari da icerir:

- `HotelListResponse`: `Name`, `Address`, `Latitude/Longitude` ("35,33031" virgullu), `CheckinHours`/`ChecoutHours` (14:00/12:00), `ImgUrl`;
  `Star`, `City`, `Description`, `Phone` genelde null.
- `HotelDetailImageList[] {Original}` (7 gorsel, cdn.crtbooking.com) -> `images` (cache-buster sorgusu temizlenir).
- `HotelDetailFacilityList[] {Name, Type (YES|AVAILABLE|PAID|TEXT|CONTENT), Category, Value, IsPaid, IsActive}` — sayfa JS'i
  (`app.HotelDetail.Facilities`) ile ayni kurallar: kategori **Genel Bilgiler**'de `CONTENT` ("GENEL AÇIKLAMALAR", HTML) -> `description` +
  "Otel Hakkında" bolumu; adinda "Saat"/"Hayvan" gecenler -> "Tesis Kuralları" pairs (+ `policies` yedegi); **Konum Bilgileri** -> pairs
  (LATITUDE/LONGITUDE gizli; SEHİR -> `city`, ÜLKE -> `country`); diger kategoriler (Temalar, Spa & Fitness, Havuz & Plaj, Genel Özellikler,
  Bebek & Çocuk, Balayı Çiftleri, Aktiviteler) -> ozellik gruplari, `IsPaid` -> "(Ücretli)"; `TEXT` + dolu `Value` -> kategori adli pairs bolumu.
- `FacilityDistance[] {Name, Distance}` -> "Mesafeler" pairs. `Opportunities`, `ListPolicyStruct {Title, Descripiton}`, `GeneralInformation`
  (oda sayisi/insa yili) dolu gelirse bolum olur (Acapulco'da null).
- `HotelRoomDetailsList` (varsayilan arama satirlari, oda x pansiyon): ada gore birlestirilir ve **ad sirasina** gore siralanir;
  `TourLogicFacility` (kategori "Yeme & İçme Konsepti" haric, pansiyona bagli) -> oda ozellikleri (sirali); `Mealy` -> `boards`.
  `RoomSize/BedType/ImageList` API'de bos geliyor (m2/gorsel yok). Oda kumesi musaitlige gore degisebilir (gunluk diff'te goze alinmali).
- `HotelServiceType` yanlis ise `HotelListResponse` null -> hata.

Probe (2026-10-26, Acapulco h3234-8): `images=7 facilities=7 rooms=6 faq=0 sections=4` (Otel Hakkında, Tesis Kuralları, Konum Bilgileri,
Mesafeler), boards 2, konum 35.33031/33.420868, giris 14:00 / cikis 12:00; iki calistirma birebir ayni JSON. Sure ~0.8 sn.

### Arama — `POST /AutoComplete/Search` `{"Key":"Merit Royal","Type":"H","Id":0,"Source":1}`

- Yanit `[{code, name, type:"H", IdDestType (5 = otel), Parent1 (ilce), Parent2}]`; sadece `IdDestType == 5` alinir. Sonuc yoksa sorgu
  sondan kisaltilir.
- Autocomplete servis tipini vermez; detay URL'i icin tarihsiz `HotelDetail` `HotelServiceType` adaylari (once `8`, sonra 1-12) ile
  dogrulanir (Kibris otellerinde hepsi 8; yanlis tipte `HotelListResponse` null). URL = `https://www.prontotour.com/<Seo-Ad>-h<code>-<tip>`
  (slug sitenin `getSeoUrl` mantigi; ResolveFromURL yalnizca `-h<kod>-<tip>` ekine bakar). Ad `HotelDetail` yanitindan (bosluklar sadelestirilir).
- Probe: "Merit Royal" -> 4 otel (Diamond h3232-8, Gardens h24-8 Iskele, Royal h925-8, Premium h39-8), 3.6 sn (otel basina 1 HotelDetail);
  "Cratos Premium" -> h2-8; "Salamis Bay Conti" -> h23-8; "Elexus" -> h52-8. Prontotour Turkiye otellerini listelemiyor ("Rixos" bos).
