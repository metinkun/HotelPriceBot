# Tatilsitesi.com provider (`tatilsitesi`)

Durum: **duz HTTP, tarayici gerekmez, cerez/token gerekmez.** Fiyat + paket (otel+ucak+transfer) + URL cozumleme.

Site ASP.NET WebForms; tum veri `POST /Methods.aspx/<Method>` WebMethod'lari ile JSON olarak geliyor
(`content-type: application/json`, yanit `{"d": ...}`). Cloudflare onunde ama challenge yok; `ASP.NET_SessionId`
cerezi set ediliyor fakat hic bir cagri icin gerekli degil (hepsi cerezsiz test edildi). Ek header olarak sadece
`x-requested-with: XMLHttpRequest`, `origin`, `referer` ve tarayici UA gonderiyoruz.

## Otel detay sayfasi / ResolveFromURL

- URL: `https://www.tatilsitesi.com/<slug>` (orn. `/acapulco-resort-hotel--convention--spa`). Yanlis slug -> 301 `/pagenotfound`.
- Sayfa icinde inline script degiskenleri:
  ```js
  var HotelName = 'Acapulco Resort Hotel & Convention & SPA';
  var hotelCODE = 'ACAPULCO';   // providerHotelId
  var CityCode  = 'ECN';        // ucus varis sehri (GetAirSearch.arrivalCity)
  var CityID    = '58';         // transfer sorgusu (GetPackageTransfer.CityCode)
  var TownID    = '232';
  ```
- Mapping: `providerHotelId = hotelCODE`, metadata `sourceUrl`, `cityCode`, `cityId`, `townId`.
- Otel listesi/autocomplete: `POST /Methods.aspx/GetGeneralList` `{}` -> `d.Hotels[] {text, gotoid(slug), code}`.

## Fiyat (sadece otel)

`POST /Methods.aspx/hotelDetailSearch`
```json
{"Parameter":{"hotelname":"...","hotelCode":"ACAPULCO","checkInSTR":"05/12/2026","checkOutSTR":"08/12/2026",
  "adultCount":2,"childCount":1,"childAge1":5,"childAge2":0,"childAge3":0,"childAge4":-1,"HotelsProCoralAllRoom":true},
 "Type":"Hotel"}
```
- Tarih `dd/MM/yyyy`. Cocuk: `childCount` + `childAge1..3` (kullanilmayan 0, `childAge4` daima -1). Site en fazla 3 cocuk
  destekliyor (0-12 yas); daha fazlasi hata doner. `childAges` verilmeden `children>0` gonderilirse hata (yas zorunlu).
- Yanit `d[]`: oda x pansiyon satirlari, **konaklama toplami** (tum indirimler uygulanmis):
  `SaleAmount`, `NotDiscountAmount` (liste), `SaleCurrency` ("TL"), `RoomCategoryName`, `PansionName`,
  `Release/StopSale/AskSale` (biri true ise sayfada "Talep Et"), `NonRefundable`,
  `PriceDetailInfos[] {inType, stMessage}` (1 = kampanya metni, 6 = min. geceleme), `DailyPricesFC[]` (gunluk).
- Lead fiyat: rezervasyona acik (`!Release && !StopSale && !AskSale`) en ucuz satir. `discountRate` liste/satis orani,
  `campaignName` inType=1 mesaji, `minStayNights` inType=6 mesajindan.
- Sure ~2-4 sn.

## Paket (otel + ucak + transfer)

Sayfa paket toplamini istemcide hesapliyor (`Package.Calc`: oda + ucus + transfer). Ayni mantik, 3 cagri paralel:

1. `hotelDetailSearch` ayni govde, `"Type":"Package"` -> paket sozlesmesi oda fiyatlari (farkli ContractID; otel-only'den
   ucuz, orn. Villa Oda YP 30.273,60 vs 35.700 TL).
2. `POST /Methods.aspx/GetAirSearch`
   ```json
   {"flightSearchParams":{"departure":"SAW","arrival":"-1","departureCity":-1,"depSearchType":"City","retSearchType":"Apt",
     "arrivalCity":"ECN","departureDate":"05/12/2026","returnDate":"08/12/2026","adultCount":2,"childCount":0,
     "CAge1":-1,"CAge2":-1,"CAge3":-1,"CAge4":-1,"CAge5":-1,"CAge6":-1,"provider":16,"CAge7":-1,"CAge8":-1,
     "isRoundTrip":true,"isInternational":false,"classType":"Economy","parityControl":"","asGuid":"<uuid4>"}}
   ```
   Yanit `d.Status` ("Ready" degilse ayni `asGuid` ile 2 sn arayla tekrar, en fazla 10), `d.Data.FlightOptions[]`
   (`OptionFlag` OUT/IN, `TotalFare` = tum yolcular toplami, `Currency`, `Segments[0].OperatingAirline/Cabin`),
   `d.Data.Recommendations[]` (gidis-donus kombinasyon; doluysa en ucuzu kullanilir). Pratikte ilk cagri ~35 sn surup
   `Ready` donuyor. Secim: sayfadaki otomatik secim gibi en ucuz OUT + en ucuz IN; anlasmali (`Cabin=contracted`)
   Pegasus bir yonde secilirse diger yon de Pegasus olmali kurali uygulanir.
3. `POST /Methods.aspx/GetPackageTransfer`
   `{"parameter":{"HotelCode":"ACAPULCO","CityCode":"58","CheckIn":"05/12/2026","CheckOut":"08/12/2026","Adult":2,"Child":0,
   "ChildAge1":0,"ChildAge2":0,"ChildAge3":0,"ChildAge4":-1,"ChildAge5":-1}}` ->
   `d.RoundTripTransferList / DepartureTransferList / ArrivalTransferList[] {SaleAmount, SaleCurrencyName, Description}`.
   Secim: gidis-donus paketi varsa ve tekli ciftten ucuzsa o, degilse en ucuz gidis + en ucuz donus. Transfer yoksa 0.

Paket oda fiyati = `SaleAmount + ucus + transfer`; liste = `NotDiscountAmount + ucus + transfer`. `Note` alaninda ucus/transfer
bilesenleri yaziyor. Sayfadaki `HotelDetailLoginMethod {Type:'Package'}` cagrisi oturum durumu icin; sonucu etkilemiyor,
atlandi. Ek hizmetler (`GetExtraServicesPackDetail`) varsayilan secili degil, pakete eklenmiyor. Kalkis havalimani
`q.AirportCode` (IATA) dogrudan `departure` alanina veriliyor (sayfa `getAirportsNew` listesinden ayni kodu kullaniyor).

## Probe ciktisi (Acapulco Resort, 05-08.12.2026, 2 yetiskin)

```
probe tatilsitesi price <url> 2026-12-05 2026-12-08 2       # resolve 0.7s, price 1.8s
  Villa Oda / Yarim Pansiyon: 35.700 TL (liste 51.000, %30, "%30 Erken Rezervasyon Indirimi"), minStayNights 1
probe tatilsitesi price <url> 2026-12-05 2026-12-08 2 1 5   # 2 yetiskin + 1 cocuk (5)
  ayni fiyat 35.700 TL (cocuk ucretsiz; satir sayisi 12 -> 9, yani yas parametresi isleniyor)
probe tatilsitesi package <url> 2026-12-05 2026-12-08 2 SAW # 12-40 sn (ilk ucus aramasi ~35 sn, tekrarinda ~10 sn)
  6 oda / 12 pansiyon; Villa Oda YP 45.614,46 TL (30.273,60 oda + 10.340,86 ucus + 5.000 transfer), liste 66.340,86
  Note: "Ucus (en ucuz gidis+donus, SAW): 10340.86 TL; transfer: 5000.00 (Eko. Transfer + Eko. Transfer)"
```

## Bilinen sinirlar

- En fazla 3 cocuk; cocuk yasi zorunlu (`childAges`).
- Ucus fiyatlari aramadan aramaya birkac TL oynuyor (canli tedarikci); paket fiyatlari da buna gore degisir.
- `Recommendations` dolu yanit gorulmedi (kodda destekleniyor ama canli test edilmedi).
- Talep Et (`StopSale/Release/AskSale`) satirlari lead fiyatta atlanir; pakette `cancellation: "Talep üzerine"` ile listelenir.
- `city` alani bos (sayfa sehir adini vermiyor; sadece CityCode/CityID).
- Tarayici gerekmedi: tum uclar duz HTTP ile cerezsiz calisiyor; Cloudflare challenge gozlenmedi.

## Statik detay (`GetDetails`) ve isimle arama (`SearchHotels`)

### Detay — Methods.aspx WebMethod'lari (cerezsiz JSON)

Sayfa JS'i (HotelDetail.js) ile ayni cagrilar, hepsi `POST /Methods.aspx/<Method>`:

| Method | Govde | Kullanim |
|---|---|---|
| `GetHotelInfo` | `{"HotelCode":"ACAPULCO"}` | `d.Info`: `HotelName`, `Star`, `CityName` (Kıbrıs -> region/country), `TownName` (Girne -> city), `Url` (slug), `PageDescription`, `ImageList[] {stFilePath, inActive, inOrder, stImageType, inID}`, ve HTML metin alanlari: `GeneralInfo` (tanitim + `<details>` SSS), `BriefInfo`, `HotelFeatures`, `Facilities`, `HotelLocation`, `HotelTransportation`, `HotelFoodConcept`, `ChildConcept`, `BeachInfo`, `RoomInfo`, `HoneymoonInfo`, `FreeServices`, `PaidServices`, `ConceptDates`, `Winter*`, `GeneralNotice`, `Notes`, `AdditionalInfo` (~200 KB yanit) |
| `getHDDetailContent` | `{"hotelCode":"ACAPULCO"}` | one cikan ikon listesi -> ozellik grubu "Öne Çıkanlar" |
| `getHDChildAges` | `{"hotelCode":...}` | `{stHeader:"1. Çocuk", stContent:"0-11,99 Yaş Ücretsiz"}` -> `policies.childPolicy` + "Çocuk Yaş Politikası" pairs |
| `getHDAccomodation` | `{"hotelCode":...}` | pansiyon tipleri -> `boards` |
| `Get_HotelCoordinat` | `{"hotelCode":...}` | `inGoogleX` = enlem, `inGoogleY` = boylam |

- HTML alanlari duz metne cevrilir (style/script/img atilir, blok etiketleri satir); bos (`<br>`) alanlar bolum olmaz.
  Bolum basliklari sitedeki sekme adlari: Hakkında, Öne Çıkanlar, Tesis Özellikleri, Genel Özellikler, Konum, Ulaşım, Yeme & İçme, Çocuk,
  Plaj & Havuz, Odalar, Balayı, Ücretsiz/Ücretli Hizmetler, Konsept Tarihleri, Kış *, Önemli Bilgiler, Notlar, Ek Bilgiler, Pansiyon Tipleri,
  Çocuk Yaş Politikası. SSS `GeneralInfo` icindeki `<details><summary>` ciftlerinden.
- Gorseller: aktif olanlar `(inOrder, inID)` sirali, caption = `stImageType` ("Ana Fotoğraf", "Genel", "Oda", "Havuz" ...).
- `petPolicy`: "evcil" gecen cumle (Notlar). Giris/cikis saati metinde yoksa bos (Acapulco'da yok).
- `rooms` bos: oda listesi yalnizca tarihli `hotelDetailSearch` ile geliyor (musaitlige bagli); oda ozellikleri "Odalar" bolumunde metin.
  `getRoomPhotos {hotelCode, roomID, provider}` oda fotograflari da arama satirina bagli oldugu icin kullanilmadi.
- `GetHotelInfo` zorunlu; diger 4 cagri hata verirse detay yine uretilir (ilgili alanlar bos kalir).

Probe (2026-10-26, Acapulco): `images=25 facilities=1 (Öne Çıkanlar 5) rooms=0 faq=7 sections=18`, boards 2, konum 35.330304/33.421408;
iki calistirma birebir ayni JSON. Sure ~1.2-1.5 sn.

### Arama — GetGeneralList

- `POST /Methods.aspx/GetGeneralList` `{}` -> `d.Hotels[] {text, gotoid (slug), code (hotelCODE)}` (65 otel; sitenin tum oteli, ~500 KB yanit).
- Eslesme: Turkce karakterler sadelestirilir (ı->i, ş->s ...), sorgunun her kelimesi otel adindaki bir kelimenin on eki olmali;
  sonuc yoksa sorgu sondan kisaltilir. URL = `https://www.tatilsitesi.com/<gotoid>`, `providerHotelId = code`. Liste sehir vermiyor (`city` bos).
- Probe: "Merit Royal" -> 4 (Gardens MERITGARDENS `/merit-cyprus-gardens-holiday-village`, Premium MERITROYALPRM `/merit-royal-premium-hotel--`,
  Diamond MERROYIMPHOTSPA, Royal MRTRYL); "Elexus" -> ELEHOTRESCAS `/elexus-hotel-resort-`; "Cratos Premium" -> CRATOSPREMIUM. ~0.6 sn.
