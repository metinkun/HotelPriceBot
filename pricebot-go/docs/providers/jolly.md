# Jolly Tur provider (`jolly`)

Durum: **duz HTTP, tarayici ve cerez gerekmez.** Fiyat + paket (GetReservationCompletePartial), URL cozumleme,
**statik detay** (detay HTML) ve **isimle arama** (Shared/Search).

## URL cozumleme (`ResolveFromURL`)

`GET https://www.jollytur.com/<slug>` -> gizli inputlar `SearchParameters.Id` (providerHotelId), `SearchParameters.HotelType`
(`Cyprus` / `Domestic`), `SearchParameters.HotelName`. Metadata: `hotelType`, `sourceUrl`.

## Fiyat / paket (ozet)

`POST /hotel/GetReservationCompletePartial` (form: `id, startDate yyyy.MM.dd, endDate, rooms "2-5-8", hotelType, searchType=Product`
+ paket icin `originId/originType=Airport/packageSearchType=0`) -> `{html, analyticsDataModel, isAvailableHotel}`.
Havalimani id: `GET /Shared/Search?key=<IATA>&type=FlightPlanner` (`destinationType == 7`).

## Statik detay (`GetDetails`)

Kaynak: detay sayfasi HTML'i (`pageHeaders`), goquery ile parse. Sayfa `#genel-bilgiler` altinda **sezon sekmeleri**
(`.tabs-item[data-index]` Kis/Yaz + tarih araligi; `.tab-content.info-content-area[data-index]`) icerir; birden fazla
sezon varsa section basliklari `"<Sezon> · <Baslik>"` olur, yapisal alanlar (aciklama, ozellikler, pansiyon, politikalar)
**ilk (aktif) sezondan** alinir.

| Sema | Kaynak |
|---|---|
| `name` | `h1.title` (yoksa `SearchParameters.HotelName`) |
| `address`, `location`, check-in/out | ld+json `@type: Hotel` (`address.streetAddress`, `geo`, `checkinTime/checkoutTime`) |
| `country`, `city`, `region` | `#breadCrumbBar` ("Kıbrıs Otelleri > Girne Otelleri > Çatalköy Otelleri" -> " Otelleri" eki atilir) |
| `description` | `#generalInfoBox article` |
| `images` | `#detailSliderGallery a.item[href]` (1024px cdng URL'leri), caption = `img[data-category]`; "Kampanyalar" bannerlari atlanir |
| `facilities` | "Otel Özellikleri" `ul.detail-list li` (popover `data-content="Ücretli"` -> "(Ücretli)"); "Aktiviteler" article'indaki `<strong>Baslik</strong><ul>` gruplari -> "Aktiviteler / <Baslik>" |
| `boards` | "Otel Konsepti" `.content-item[data-concept] .badge-title` |
| `rooms` | `GetReservationCompletePartial` sabit pencere (iki ay sonrasinin 1'i, 3 gece, 2 yetiskin): `[data-roomtype]` kartlari; `.room-title`, `.option-text` (aciklama), `.room-badges .badge` ("30 m2" -> `size`, digerleri ozellik), `.option-tag span` (+ "(Ücretli)"), `.image-gallery a[href]`; farkli pansiyondaki ayni oda birlesir, ada gore sirali |
| `policies` | ld+json saatleri (yan panel `.checkin-checkout` ile de doldurulur); `petPolicy` = "Önemli Bilgiler" rozetinde "Evcil..." ; `other` = "Önemli Bilgiler" paragraflari |
| `sections` | "Sezonlar" (pairs); sezon basina: "Otel Özellikleri" (list), "Genel Bilgiler" (pairs: Alan/Oda/Restaurant/Bar/Salon + text), "Genel Bilgiler · Toplantı Salonları" (text), "Otel Konsepti" (pairs: `Pansiyon – Restoran – Öğün` -> saat), "Aktiviteler" (text), "Önemli Bilgiler" (text + items), "Giriş / Çıkış" (pairs), yan panel: "Plaj ve Havuz Bilgisi", "Konum Bilgisi" (list: `Havalimanı: Ercan: 28 km`), "Yeme-İçme Bilgisi", "Balayı Bilgisi"; "Etkinlikler" (list: tum `.addon-modal .modal-title`, alfabetik) |

Alinmayanlar: `stars`/`rating` (sayfada gosterilmiyor; `#puan-yorum` bos), FAQ (yok), "En Son Baktığınız"/"Bunlara da Baktı".

Probe (Acapulco, 2026-10-26): `images=221 facilities=8 rooms=3 faq=0 sections=24`, ~1 sn; iki ardisik cagri `fetchedAt`
haric birebir ayni JSON. Oda sayisi pencere icindeki musaitlige baglidir (Aralik 2026 icin 3 oda tipi).

## Isimle arama (`SearchHotels`)

`GET https://www.jollytur.com/Shared/Search?key=<metin>&type=<Planner>` (header: UA + `x-requested-with`). Planner
tipleri siraya gore `CybrusHotelPlanner` (Kibris otelleri) ve `HotelPlanner` (yurt ici); sonuclar `id` ile tekillestirilir.
Yanit `[{id, value, adjustName, typeName, destinationBreadCrumb, categoryName}]`; sonuc yoksa tek satir
`categoryName: "Sonuç Bulunamadı", id: 0` gelir (atlanir). Sadece `typeName == "Otel"`. URL = `https://www.jollytur.com` +
`adjustName`; `providerHotelId` = `id`; `city` = `destinationBreadCrumb` ("Alsancak, Girne, Kıbrıs").
Sonuc yoksa sorgu son kelimesi atilarak tekrarlanir. Not: `HotelPlanner` bulanik eslesir ("Cratos Premium" -> diger
"Premium" oteller de gelir); Kibris sonuclari listenin basindadir.

Ornek: "Merit Royal" -> Merit Royal & Spa / Gardens / Diamond (3); "Elexus" -> Elexus Hotel Resort & Spa;
"Salamis Bay Conti" -> Salamis Bay Conti Hotel (Magosa) + yurt ici "Bay" otelleri.

## Bilinen sinirlar

- Yildiz ve misafir puani yok.
- Odalar fiyat yanitindan geldigi icin sadece pencerede musait oda tipleri listelenir; `hotelType` metadata'si yoksa odalar bos.
- Aktif sezon degistiginde (Nisan/Kasim) yapisal alanlar diger sezonun icerigine gecer (gunluk diff'te bir kerelik degisim).
