# Gezinomi (gezinomi.com)

Durum: **calisiyor** (fiyat + paket). Hibrit akis: fiyat istekleri duz HTTP, tarayici yalnizca
`x-client-key` uretimi icin (kalici, hafif bir sekme). Kod: `internal/providers/gezinomi/`
(`session.go` oturum/anahtar, `api.go` uclar, `normalizer.go`, `provider.go`).

## Uclar

| Uc | Amac | Govde |
|---|---|---|
| `GET https://www.gezinomi.com/otel/<bolge>/<sehir>/<slug>` | Otel detay (Next.js RSC). `"hotelId":932`, `"hotelName":"..."`, `"hotelLocationId":14361,...,"parentId":14360` escape'li JSON icinde | - |
| `POST https://apigezinomi.gezinomi.com/api/Hotel/GetHotelRoomPrices` | Oda fiyatlari; ucus verilirse ayni yanitta paket alanlari | asagida |
| `POST .../api/Hotel/GetHotelAirports` | Otelin varis havalimani (paket icin) | `{"hotelId":932,"hotelLocationId":14361,"parentId":14360}` -> `data:[{"airportCode":"ECN","isCity":false}]` |
| `POST .../api/Common/GetGezinomiContactNumber` | Ucuz "bootstrap" cagrisi: anahtarsiz atilir, yanit basligindan `x-session-id` alinir | `{"contactTypeId":1}` |

`GetHotelRoomPrices` govdesi (sitenin JS'inden birebir):

```json
{"checkInDate":"2026-12-05T00:00:00","checkOutDate":"2026-12-08T00:00:00","hotelId":932,"apiId":0,
 "searchTypeId":1,"apiProviderName":"",
 "roomDetails":[{"roomId":1,"persons":[{"isChild":false,"isMen":false,"age":0,"roomOrderId":0},
                                       {"isChild":false,"isMen":false,"age":0,"roomOrderId":0}]}],
 "flightSearch":{"flights":[],"passengers":{"adult":0,"child":0,"infant":0},"isMultiTrip":false,"directFlightsOnly":false,"cabinCodeType":""}}
```

- Tarih: `yyyy-MM-ddT00:00:00`. Cocuk: `{"isChild":true,"age":5}` (`isChild = yas < 16`).
- **Paket** (otel+ucak): `searchTypeId:2`, `passengers:{adult,child,infant}` dolu ve
  `flights:[{originCode:"SAW",isOriginCity:false,destinationCode:"ECN",isDestinationCity:false,flightDate:"2026-12-05T03:00:00.000Z"}, {donus}]`.
  Yanitta `reservationDetails[].flightPackageTotalPrice / flightPackageTotalDiscountedPrice / flightPackageDiscountRate`
  dolar (varsayilan/en ucuz ucus kombinasyonuyla). `data.flights` 40 civari ucus secenegi iceriyor (kullanilmiyor).
- Yanit: `data.roomDetails[]` (oda basina bir kayit; ayni `roomId` farkli pansiyonlarla tekrarlanir),
  `reservationDetails[].totalPrice` (liste), `totalDiscountedPrice` (satis), `discountRate`, `conceptName` (pansiyon),
  `absent` (true ise teklif yok, atlanir). Para birimi TL.

## Oturum mekanigi (asil zorluk)

API her POST'ta iki basligi dogrular; yanlis/eksikse **200** ile `{"data":null,"status":"Invalid","message":"Oda bulunamadı"}`
doner (gercek "oda yok" ile ayni gorunur, yaniltici).

1. **`x-session-id`** — sunucu uretir. Herhangi bir API yanitinin `x-session-id` basligindan alinir
   (`Access-Control-Expose-Headers: X-Session-ID`); sitede `xSessionId` cerezine yazilir. Baslik gonderilmez/gecersizse
   yanitta yeni bir kimlik doner. Sayfa yuklemesinde Set-Cookie yok, `__NEXT_DATA__`/localStorage'da degil.
2. **`x-client-key`** — 64 hex. Sitenin Web Worker'i (`/_next/static/chunks/1196.<hash>.js`) `/obfus.enc` dosyasini
   (xorshift128+ akis sifreli WASM) acar ve `k32(sid, path, dakikaEpoch)` fonksiyonunu cagirir:
   `path` = `"Hotel/GetHotelRoomPrices"` gibi uc yolu, `dakikaEpoch = floor(Date.now()/60000)`.
   Dogrulanan davranis: anahtar sid'e, yola ve dakikaya bagli; sunucu **±2-3 dakika** tolere eder (-30 dk reddedildi);
   baska yol icin uretilen anahtar, rastgele anahtar, sid'siz anahtar reddedildi.
3. Anahtar **tarayiciya bagli degil**: tarayicida uretilen anahtar + sid, `curl`/Go `net/http` ile aynen gecerli
   (TLS parmak izi, cerez, Cloudflare yok). `Authorization: Bearer` sadece giris yapan kullanicilar icin, gerekmez.

Uygulama (`session.go`):

- `browser.Manager.NewPersistentPage` ile tek kalici sekme, `https://www.gezinomi.com/robots.txt` acilir (hafif, ayni origin;
  CSP `worker-src 'self'`). Sayfada `jsSetup` calisir: ana sayfa HTML'inden chunk listesi + webpack runtime okunur,
  `new Worker(...u(<id>))` ile worker chunk id'si ve runtime'daki hash bulunur (hash her deploy'da degisir; son URL ipucu
  olarak saklanir ve HEAD ile dogrulanir), worker `init` edilir, `window.__gz.key(sid, path)` tanimlanir.
  Kurulum ~200 ms, anahtar uretimi ~1 ms. Sekme 6 saatte bir yenilenir; tarayici duserse otomatik yeniden kurulur.
- Anahtar yol basina dakikada bir hesaplanir (onbellek). Oturum kimligi Go bellegindedir (tarayiciya bagli degil).
- `post()`: sid yoksa bootstrap -> anahtar -> HTTP POST. Yanit `Invalid` ise: sunucu yeni sid verdiyse onunla, sid 2 dk'dan
  eskiyse yeniden bootstrap edip **bir kez** tekrar dener (gercek "oda yok" durumunda en fazla 1-2 ekstra ucuz istek).

## Mapping / metadata

`ResolveFromURL` detay sayfasindan `providerHotelId` (hotelId), otel adi ve metadata `sourceUrl`, `hotelLocationId`,
`parentId` uretir. Paket icin varis havalimani `GetHotelAirports` ile bulunur ve bellekte otel basina onbelleklenir;
metadata'da `hotelLocationId` yoksa `sourceUrl` yeniden cozulur.

## Probe ciktisi (2026-10-23, Acapulco Resort, 5-8 Aralik 2026)

```
probe gezinomi price ... 2026-12-05 2026-12-08 2       -> 1.4 s, found=true
  Acapulco Villa Odası / Yarım Pansiyon, liste 51.000 TL, fiyat 30.600 TL, %40 (3 gece)
probe gezinomi price ... 2 1 5  (2 yetiskin + 5 yas)   -> 1.4 s, 30.600 TL (cocuk ucretsiz)
probe gezinomi package ... 2 SAW                        -> 5.9 s, available=true, 6 oda x 2 pansiyon
  Villa / YP: liste 60.553,35 TL, fiyat 40.153,35 TL (SAW -> ECN gidis-donus, Pegasus)
```

(Bir gun once ayni sorgu %38 indirimle 31.620 TL / paket 41.166,09 TL idi; fixture'lar o yanitlardan kisaltildi.)

## Sinirlar / notlar

- Tarayici gerekir (sadece anahtar icin, tek hafif sekme). `Capabilities.RequiresBrowser=true`. Tarayicisiz calisma icin
  `k32` WASM'inin Go'ya tasinmasi gerekir (obfuscate; yapilmadi). Alternatif: wazero ile ayni WASM'i calistirmak (yeni bagimlilik).
- Anahtar dakikaya bagli: sunucu saati ile konteyner saati arasinda >2-3 dk fark varsa tum istekler "Oda bulunamadı" doner.
- `x-session-id`'nin IP'ye bagli olup olmadigi test edilmedi (proxy rotasyonu aktifse her istekte farkli IP -> olasi
  `Invalid`; `post()` yeni sid ile bir kez tekrar dener). Proxy'siz lokal testte sorun yok.
- Paket: `airportCode` kalkis olarak kullanilir; donus ayni havalimanina. Varis otelin ilk havalimani (Kibris: ECN).
  Fiyat sitenin varsayilan ucus secimine gore; farkli ucus secilirse degisir.
- Tek oda sorgulanir; cocuk yasi `age` ile verilir, bebek (`infant`) desteklenmiyor.
- Chunk/worker yapisi (webpack `u(1196)` kalibi, `k32Hex` mesaj tipi) degisirse `jsSetup` kirilir -> hata mesaji
  "Gezinomi anahtar worker'i kurulamadi"; `capture js` ile `out/gez_setup_test.js` benzeri bir betikle yeniden incelenir.

## Statik detay (`GetDetails`) ve isimle arama (`SearchHotels`)

Kod: `details.go` (+ `rsc.go` RSC payload, `html.go` HTML -> metin). Tarayici/anahtar gerekmez.

**Detay**: `GET <sourceUrl>` (detay sayfasi, ~0.3 sn). Next.js RSC akisindaki `{"hotelDetail":{...}}` nesnesi
`json.Decoder` ile alinir; `descriptions[].text` alanlari `"$<id>"` referansi olabilir -> `<id>:T<hexBayt>,` metin
kayitlarindan cozulur (bkz. setur.md, ayni mekanizma).

| Sema | Kaynak |
|---|---|
| name | `hotelName` |
| region / city / country | `navigationName` "Kıbrıs/Girne" -> region=Kıbrıs, city=Girne; `hotelLocation=="Kıbrıs"` ise country=Kıbrıs |
| stars | `metaKeyword` icindeki "5 Yıldız" (ayri alan yok) |
| rating / ratingCount | `commentsRatingTotal.avgRate` (10 uzerinden, 2 ondalik) / `commentsRatignGroup[].count` toplami |
| location | `latitude`/`longitude` (string) |
| images | `pictures[]` (`isCampaignPicture` haric; oda gorselleri de dahil). URL = `https://images.gezinomi.com/fit-in/1600x900/filters:quality(100)/filters:format(webp)/assets/<name>-b0.jpg` (`name` uzantisiz; uzantili adlar oldugu gibi; `externalImageUrl` varsa o) |
| facilities | `propertyList[]` -> `hotelPropertyGroupName` gruplari (Konum Bilgileri, Otel Özellikleri, Plaj ve Havuz Özellikleri, Aktiviteler, Çocuklar İçin, Spa - Sağlık Hizmetleri); madde = ad + `value` + birim (unitId 6=m, 7=km, 13=m²) + `(description)` + "(Ücretli)" |
| rooms | `hotelRoomCacheModels[]`: `roomName`, `description` (m2 regex ile size), `capacity`/`adultCount` -> "4 kişi (en fazla 3 yetişkin)", `hotelRoomProperties[]` -> features, `pictures[]` |
| policies | giris/cikis saati: aciklama metinlerinden regex ("giriş saati 15:00, çıkış saati ise en geç 11:00"); childPolicy = `childDesc` + `childAge1/2Min-Max`; petPolicy = "Evcil Hayvan Politikası" aciklamasi; cancellation = `cancelDescription`; `isAdultHotel` -> other |
| sections | her `descriptions[]` kaydi kendi basligiyla (Tesis Hakkında Genel Bilgiler, Yeme - İçme, Tesis Hakkında Genel Uyarı..., Balayı Ayrıcalıkları, Bebek/Çocuk, Spa, Plaj - Havuz, Evcil Hayvan, Animasyon - Eğlence; bos olanlar atlanir) + "Öne Çıkan Özellikler" (`hotelPropertyProminentCacheModels`) + "Kategoriler" (`hotelCategories`) + "Etkinlikler" (`hotelActivities[].title`) |
| description | ilk aciklama ("Tesis Hakkında Genel Bilgiler") duz metni |

Atlananlar: `hotelCampaignForAgaveRedis` (saat/gune gore degisen indirim oranlari -> gunluk diff gurultusu),
`commentsHotelRating` (kisi adli yorumlar), `campaigns` (genel kampanya metinleri). SSS/pansiyon listesi sayfada yok.
Deterministiklik: Acapulco iki ardisik cekimde `fetchedAt` disinda birebir ayni JSON.

**Arama**: `POST https://apigezinomi.gezinomi.com/api/Hotel/SearchAutoComplete` `{"query":"<metin>"}`. Imzasiz
(x-session-id/x-client-key'siz) calisir; yanit `Invalid` donerse ve tarayici varsa `signer.post` ile bir kez daha
denenir. 3 karakterden kisa sorgu bos liste. Yanit `data[] {id,text,url,location,resultType,routeLink,routeMain}`;
`resultType=="Otel" && routeMain=="hotel-detail"` alinir, URL = `https://www.gezinomi.com/otel/<routeLink>/<url>`,
city = `location`'in son parcasi. **`providerHotelId` bos**: autocomplete `id`'si (orn. 10868) mapping'de kullanilan
`hotelId` (932) degildir; URL cozumlemesi (`ResolveFromURL`) doldurur.

```
probe gezinomi details https://www.gezinomi.com/otel/kibris/girne/acapulco-resort-convention-spa-hotel
  details 0.1s images=43 facilities=6 rooms=18 faq=0 sections=12 (ikinci cekim ayni)
probe gezinomi search "Merit Royal"     -> 22 (Merit Royal Diamond/Gardens/Hotel/Premium ... Girne; sonra "Royal" iceren TR otelleri)
probe gezinomi search "Elexus"          -> 1 (Elexus Hotel & Resort, Girne)
probe gezinomi search "Cratos Premium"  -> 24 (ilk: Cratos Premium Hotel & Port & Spa, Girne)
```
