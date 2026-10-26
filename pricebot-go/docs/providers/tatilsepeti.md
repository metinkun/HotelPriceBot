# TatilSepeti provider (`internal/providers/tatilsepeti`)

Durum (Ekim 2026): **duz HTTP, cerez elle girilmez**. Oturum (cerez + VerificationToken) otomatik
edinilir; duz HTTP engellenirse (Cloudflare) tarayici yalnizca cerez/token okumak icin bir kez acilir (hibrit).
Fiyat ve paket sorgulari dogrulandi (bkz. probe ozeti).

## Uclar

| Amac | Istek | Govde (form-urlencoded) | Yanit |
|---|---|---|---|
| Liste (lead) fiyati | `POST https://www.tatilsepeti.com/hotel/GetHotelListPrice/` | `AdultCount=2&ChildCount=0&CampaignType=0&CampaignId=&checkinDate=05.12.2026+&checkoutDate=08.12.2026&HotelIds=7741[,229,...]` | JSON dizi; otel basina `HotelId, Price, DiscountRate, DiscountPrice, Accommodation, CampaignName, HasPrice, HasFlyingPackage` |
| Paket (otel+ucak+transfer) | `POST https://www.tatilsepeti.com/Hotel/GetFlyingPackages` | `Search=oda:2;tarih:05.12.2026,08.12.2026;click:true&Id=7741&IsFlightPocket=true&depatureCode=saw&destinationCode=ecnc` (cocuklu: `oda:2,5,8`) | JSON: `roomList` (HTML), `flightListResult` (HTML), `transferList`, `detailMessage`, `IsFlyingPackageOnline`, `CityCode` |
| URL cozumleme | `GET https://www.tatilsepeti.com/<otel-slug>` | - | HTML: `#hidHotelId`, `#hidHotelName`, `input#nereye[data-search-value]` (destinasyon kodu, orn. `ecnc`), `#forgeryToken`, `#hidCityCode` (99 = Kibris) |

Tarih formati `dd.MM.yyyy` (checkinDate sonunda bosluk sitenin JS'i gibi). Kisi: `AdultCount`/`ChildCount`
(liste fiyati cocuk yasi almaz); paket `Search` icinde yaslar virgulle. Para birimi TL (yanitta `Currency` null gelir).

Sitenin kendi detay sayfasi oda listesi icin `POST /<otel-slug>` (`Search=oda:2;tarih:...;click:true`) kullanir; biz
liste fiyati (GetHotelListPrice) ve paket (GetFlyingPackages) uclarini kullaniyoruz.

### Basliklar

XHR istekleri: `x-requested-with: XMLHttpRequest`, `content-type: application/x-www-form-urlencoded; charset=UTF-8`,
`origin: https://www.tatilsepeti.com`, `referer: <otel sayfasi>`, `user-agent` = `browser.UA` (cf_clearance UA'ya baglidir),
`cookie: <oturum>`, `verificationtoken: <#forgeryToken>` (sitenin JS'i `VerificationToken` adiyla ekler).

Detay sayfasi GET'i **tarayici-gezinme** basliklariyla yapilmali (`sec-fetch-mode: navigate`, `sec-fetch-dest: document`,
`accept: text/html...`). `x-requested-with`/`origin` ile GET yapilirsa site parca (partial) HTML doner (~12 KB, `hidHotelId` yok).

## Oturum mekanigi (`session.go`)

Olcum (22.10.2026, konut IP + Docker): her iki uc **cerezsiz ve token'siz de JSON donuyor**. Yine de site Cloudflare bot
yonetimi arkasinda (sayfa yuklenirken `/cdn-cgi/challenge-platform/.../jsd/oneshot`, tarayicida `cf_clearance` cerezi olusuyor);
sunucu/proxy IP'lerinden 403/503 "Just a moment" gorulebilir. Bu nedenle:

1. **Duz HTTP** (varsayilan): detay sayfasi (yoksa ana sayfa) GET -> `Set-Cookie` (`ASP.NET_SessionId`,
   `HotelSearchCriteriaCookie`, `MyVisits`) + HTML'deki `#forgeryToken`. `ResolveFromURL` sirasinda cekilen sayfadan da
   oturum alinir (ek istek yok).
2. **Tarayici** (`WithBrowser(bm)` verildiyse ve duz HTTP engellendiyse ya da `WithForceBrowser(true)`): `bm.NewPage` ile
   sayfa acilir, `browser.Navigate` challenge'in gecmesini bekler, `network.GetCookies` ile tatilsepeti.com cerezleri
   (`cf_clearance` dahil) ve `#forgeryToken` okunur, sekme kapatilir. Tarayici sorgu basina degil, **oturum basina** kullanilir.
3. Onbellek: tek paylasilan oturum (cerezler otele bagli degil), **20 dk** sonra yenilenir (`WithSessionAge`).
4. Yenileme tetikleri: yanit JSON degil, 403/429/503 veya challenge HTML'i -> oturum gecersiz sayilir, bir kez yeniden
   edinilip istek tekrarlanir. Engel goruldu ve tarayici varsa bir sonraki edinim tarayici ile yapilir.
5. `TATILSEPETI_COOKIE` / `TATILSEPETI_VERIFICATION_TOKEN` (env) **istege bagli override**: doluysa once onlar kullanilir;
   bir istek basarisiz olursa otomatik edinime gecilir (`Capabilities.RequiresCookie=false`).

Token omru: `#forgeryToken` her sayfa yuklemesinde degisir (istek basina uretilir) ve sunucu su an token'i dogrulamiyor
gibi gorunuyor (token'siz istek de 200 JSON). Cerez omru: `ASP.NET_SessionId` oturum cerezi (sure yok),
`HotelSearchCriteriaCookie` 1 gun, `cf_clearance` tipik 30 dk-1 saat ve IP+UA'ya bagli. 20 dk'lik yenileme bunlarin icinde kalir.

`Stats()` saglik sayaclari: `sessionHttp`, `sessionBrowser`, `sessionRefresh`, `blocked`, `sessionFailures`,
`sessionSource` (1 env, 2 http, 3 browser).

### Kayit (registry)

`registry.go` su an `tatilsepeti.New(cfg.TatilSepetiCookie, cfg.TatilSepetiVerificationToken)` ile kaydeder; bu haliyle yalnizca
duz HTTP calisir (engelde hata). Tarayici yedegini acmak icin:

```go
r.register(tatilsepeti.New(cfg.TatilSepetiCookie, cfg.TatilSepetiVerificationToken, tatilsepeti.WithBrowser(b)))
```

## Normalize

- Liste fiyati: API `Price` = indirimsiz liste fiyati, `DiscountPrice` = `DiscountRate` uygulanmis odenecek fiyat
  (orn. 67200 / %15 / 57120). Normalize: `price=57120, listPrice=67200, discountRate=15, discountPrice=57120`.
  `Price=0`/`HasPrice=0` (orn. 4 yetiskin tek oda) -> `available=false`. `hotelName` yanitta null gelir, mapping'den doldurulur.
- Paket: `roomList` HTML'inde `#roomName_flight_<RoomTypeId>` kartlari; pansiyon basina `getInstallment(... FlightPocketPrice: '65533,58' ...)`
  (TR formati, `provider.ParseTRPrice`). Toplam fiyat = oda + ucak + transfer (`TotalRoomPrice`, `FlightPrice`, `TotalTransferPrice` gizli inputlari).
  `free-cancelation` blogu -> `FREE_CANCELLATION`. Paket yoksa `detailMessage` (`Konaklama yapacağınız tarihlerde uçaklı paket bulunamadı.`) `note`'a yazilir.

## Bilinen sinirlar

- Liste fiyati ucu cocuk **yasi** almaz (sadece sayi); oda tipi/oda adi donmez (lead fiyat).
- Paket icin mapping metadata'da `destinationCode` gerekir (URL cozumlemede `#nereye[data-search-value]`'dan dolar; Kibris otelleri `ecnc`).
  `depatureCode` kucuk harf IATA (`saw`, `ist`, `adb`...). Site "giris tarihi bugunden en az 7 gun sonra" kurali uygular.
- Tarayici ile alinan `cf_clearance` IP'ye baglidir: `httpx` proxy rotasyonu aciksa tarayici (proxy'siz) cerezi proxy IP'sinde gecersiz olabilir.
- `GetFlyingPackages` yaniti buyuk (~650 KB, ucus listesi dahil); zaman asimi 40 sn.

## Probe ozeti (22.10.2026, app imaji icinde)

```
probe tatilsepeti price   https://www.tatilsepeti.com/kaya-palazzo-resort 2026-12-05 2026-12-08 2
  resolve 0.8s -> 7741 "Kaya Palazzo Resort" destinationCode=ecnc
  price   1.0s -> found=true  price=57120 TL  listPrice=67200  %15  Oda Kahvaltı  "Son Dakika Fırsatı %15 İndirim + 3 Taksit"
probe tatilsepeti package https://www.tatilsepeti.com/kaya-palazzo-resort 2026-12-05 2026-12-08 2 SAW
  package 9.4s -> available=true roomCount=5
  46 Kara Manzaralı Deluxe Oda   OK 65.540,86  TP+ 78.140,86   (3 gece, ucak+transfer dahil)
  45 Deniz Manzaralı Deluxe Oda  OK 82.640,86  TP+ 100.190,86
  218 Grand Suit                 OK 105.140,86 TP+ 134.240,86  ... (5 oda)
probe tatilsepeti price   https://www.tatilsepeti.com/acapulco-resort-convention-spa-hotel 2026-12-05 2026-12-08 2
  resolve 0.9s -> 229 "Acapulco Resort Convention Spa Hotel" destinationCode=ecnc
  price   0.9s -> found=true  price=30600 TL  listPrice=51000  %40  Yarım Pansiyon
```

Tarayici yolu dogrulamasi (gecici olarak `WithBrowser(b)` + `WithForceBrowser(true)` ile): oturum ~3-4 sn'de alindi
(`cf_clearance`, `ASP.NET_SessionId`, `HotelSearchCriteriaCookie`, token), ardindan duz HTTP ile fiyat 57120 TL ve Acapulco paketi
(5 oda, 45.740,86 TL'den) basarili.

Tarayici gerekli mi: **normalde hayir**. Yalnizca duz HTTP Cloudflare tarafindan engellenirse, oturum basina (~20 dk'da bir)
tek sayfa acilisi icin.

## Statik detay (`GetDetails`) ve isimle arama (`SearchHotels`)

Kod: `details.go` (HTML parse, regex tabanli), `search.go`, `html.go`. Detay sayfasi `pageHeaders` ile GET edilir (engelde
`browser.Manager` varsa sayfa tarayicida acilip HTML alinir); ayni GET'ten cikan cerez/token oturum olarak saklanir.

### Kaynak: otel detay HTML'i (sunucu tarafli render, ~600 KB)

| Alan | Kaynak |
|---|---|
| providerHotelId, name | `#hidHotelId`, `<h1 class="hotel__name">` |
| country / city / region | `.hotel__region` metni "Kıbrıs - Girne-Çatalköy" |
| address, checkIn/checkOut, petPolicy, rating, ratingCount | JSON-LD `LodgingBusiness`: `address[0].streetAddress`, `checkinTime` (14:00), `checkoutTime` (11:00), `petsAllowed` (No), `aggregateRating.ratingValue` (8.4 / 10), `reviewCount` (1786) |
| description | "<Otel> Genel Bilgiler" bolumu (`.ts-card.descriptions`, HTML -> metin) |
| images | `.photo-gallery` icindeki `img[src|data-src]` (`cdn.tatilsepeti.com/Files/Images/Tesis/<id>/1050X700/...jpg`; sorgu parametresi atilir; caption = `title`) |
| facilities | "Otel Özellikleri" `<li>` listesi; "Ücretsiz Aktiviteler"; "Ücretli Aktiviteler"; "Çocuklar İçin" (tablo satirlari -> "Çocuk Kulubü (Ücretsiz)", "Bebek Bakımı (Ücretli)") |
| rooms | `.hotel-detail-cards[data-roomtypeid]` kartlari (sayfa sirasi, roomtypeid benzersiz; "… Gala Dahil" satis varyantlari da kart oldugu icin dahil): `<h3>` ad, kart `data-src` gorseli (TesisOda), kapasite metni ("En fazla 4 yetişkin + 1 çocuk …"), kart ozellikleri + modal `.room-properties` (`Oda metrakaresi: 30 - 32 m2` -> size 30, `<li>` ozellikleri). Oda gorselleri: `GET /hotel/gethotelroomimages/?hotelId=&roomTypeId=&ImageSizeType=3` (parca HTML, XHR basliklariyla JSON string olarak sarili doner; oda basina 1 istek, hata olursa kart gorseli kalir) |
| boards | `var ttl_pension_type = "tam pansiyon plus"` -> "Tam Pansiyon Plus" (tek deger; oda bazli pansiyonlar fiyat akisinda) |
| policies.childPolicy / other | "Diğer Notlar: Çocuklara Özel" satirlari; giris saati / iptal / provizyon iceren notlar |
| sections | "<Otel> Genel Bilgiler" (text), "Konum Bilgileri" (pairs: Hava Alanına Uzaklığı, Kurulu Alan, Plaj…), "Otel Özellikleri" (pairs: yapim yili, kapasite…) + "(Liste)", aktiviteler (list), "Çocuklar İçin" (pairs), "Diğer Notlar: <alt baslik>" (her `<b>` basligi ayri `list`: Plaj Bilgisi, Havuz Bilgisi, Aquapark, Çocuklara Özel, Spa Merkezi, Ücretsiz/Ücretli Hizmetler, Toplantı Salonları, Ek Bilgiler, KKTC Seyahati…), "<Otel> Yorumları" (pairs: Tatilsepeti puani, degerlendirme/yorum sayisi, kategori puanlari) |

- Yildiz ve koordinat sayfada yok (harita iframe'i tiklamayla yuklenir). Iletisim: sayfadaki telefon Tatilsepeti cagri merkezi, alinmaz. FAQ yok.
- Yorumcu adlari/yorum metinleri alinmaz; `hiddenEnrtyDate`/`ttl_sessionId`/`forgeryToken` gibi istek bazli alanlar kullanilmaz.
- **Site tarafli degiskenlik**: "Genel Bilgiler" metni sunucu tarafinda cumle varyantlariyla uretiliyor (ayni URL, farkli web
  sunucusu `WEB8`/`WEB17`: "en son 2010 yılında yenilenmiştir" vs "1982 yılında yapılan … bulunmaktadır"). Parser deterministik;
  `description` ve bu bolum iki cekim arasinda farkli cikabilir. Diger tum alanlar (141 gorsel, 12 oda, 4 ozellik grubu, 19 bolum) birebir ayni.

### Arama: `GET /common/hotelsearch?value=<sorgu>`

Sitenin arama kutusu (jQuery UI autocomplete, `tsAutocomplete({selector:"#bolge",url:"/common/hotelsearch"})`; parametre `value`).
Cerez/token gerekmez; XHR basliklari ve varsa mevcut oturum cerezi eklenir (oturum edinmek icin ek istek yapilmaz). Yanit JSON
dizi: `Title`, `Type`/`TypeId` (1 Yurt İçi Oteller, 2 kampanya/tema, 5 bolge, 6 Yurt Dışı Oteller, 7-8 yer), `Id`, `Link` ("/<slug>"),
`AreaPlace` ("Çatalköy, Girne, Kıbrıs"). Yalnizca `TypeId=1` alinir (KKTC otelleri yurt ici sayilir; yurt disi otel sayfalari farkli
sablon). Sunucu bulanik eslestirir (orn. "Merit" icin "Mert Seaside", "Meri Hotel"); sonuc sorgunun tum kelimelerini (Turkce duyarsiz)
icerenler one alinarak siralanir, kalan sunucu sirasinda. Sonuc yoksa son kelimeler atilarak tekrar denenir.

### Probe (26.10.2026)

```
probe tatilsepeti details https://www.tatilsepeti.com/acapulco-resort-convention-spa-hotel
  2.3-3.2 sn -> images=141 facilities=4 rooms=12 faq=0 sections=19; rating 8.4 (1786); Kıbrıs / Girne / Çatalköy
  Villa Oda 30 m2, 13 ozellik, 5 gorsel; policies: giris 14:00, cikis 11:00, evcil hayvan yok; board Tam Pansiyon Plus
  iki calistirma: yalnizca "Genel Bilgiler" metni farkli (site varyanti, yukarida), gerisi birebir ayni
probe tatilsepeti search "Merit Royal Premium" -> 17 (ilk: Merit Royal Premium Hotel Spa 4518, Alsancak, Girne, Kıbrıs)
probe tatilsepeti search "Acapulco"            -> Acapulco Resort Convention Spa Hotel (229)
probe tatilsepeti search "Cratos Premium"      -> 22 (ilk: Cratos Premium Hotel 2750)
probe tatilsepeti search "Salamis Bay Conti"   -> 12 (ilk: Salamis Bay Conti Resort Hotel 212, Magosa)
```
