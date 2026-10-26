# Touristica provider (`internal/providers/touristica`)

Durum (Ekim 2026): **tarayici zorunlu**. Site Cloudflare challenge arkasinda; `cf_clearance` cerezi TLS parmak izine
bagli oldugu icin HTTP istemcisine tasinamaz. Tum istekler kalici sekme havuzundaki (`pool.go`) sayfalarin icinden
yapilir. Fiyat/paket akisi ve havuz mekanigi `docs/phase3-research.md` icinde anlatilir; bu dokuman ozellikle statik
detay (`DetailProvider`) ve isimle arama (`HotelSearcher`) uclarini kapsar.

## Uclar

| Amac | Istek | Notlar |
|---|---|---|
| Otel detay sayfasi | `GET https://www.touristica.com.tr/<slug>` | Tarayici navigasyonu (Cloudflare). `window.HotelId` (sayisal id), `window.PageURL` (slug), `window.PageURLDetail.IsHotelDetailPage`, ld+json `@type: Hotel` (adres, puan), `.map-canvas[data-lat][data-lng]` (virgullu ondalik) |
| Oda listesi (statik) | `GET /<slug>/odalar?rpd=True&cc=&det=False&y=1c` (`#HotelRoomListPlaceHolder[data-place-holder-target]`) | Sayfa ici `fetch` (`X-Requested-With: XMLHttpRequest`, cerezler dahil). Tarihsiz cagrida fiyat yoktur ama oda kartlari ve "Detayli Bilgi" popup'lari (m2, ozellikler, gorseller) tam gelir. ~335 KB HTML |
| Otomatik tamamlama | `GET /anasayfa/otomatik-tamamlama?SearchType=hotel&SearchText=<q>` | Ana sayfa arama kutusunun ("Nerede Kalmak İstiyorsunuz?", `#HotelQueryText`) XHR'i. HTML doner: `<h3>KATEGORİLER</h3>` (bolge: `*-otelleri`) ve `<h3>TÜM OTELLER</h3>` bolumleri, `<li><a data-link='<slug>'>Ad</a></li>` |
| Fiyat listesi | `GET /<slug>/fiyat-listesi?cc=&det=False&y=` | Kullanilmiyor (sezon fiyat tablosu, ~600 KB; fiyat verisi oldugundan statik detaya alinmadi) |

## GetDetails (`details.go`, `details_parse.go`)

Akis: `withTab(sourceUrl)` sekmede otel sayfasini hazir eder (acik ise navigasyon yok) -> `jsReadDetailPage`
yalnizca gereken kapsayicilarin `outerHTML`'ini toplar (DOM degistirilmez, **fiyat aramasi tetiklenmez**) ->
`/odalar` sayfa ici `fetch` ile alinir -> `parseDetails(pageHTML, roomsHTML)` goquery ile ayristirir.
Sekmenin `t.url`'i otel sayfasinda kalir; ardindan gelen fiyat sorgusu ayni sekmeyi navigasyonsuz kullanir.

Toplanan kapsayicilar: `h1`'in satiri (ad, `.hotel-location` "Kıbrıs, Girne", `ul.amenities` etiketleri + `li.icon-add[title]`
gizli etiketler), ld+json Hotel, `#hotel-map`, `.detail-slider-wrapper` (gizli tam galeri: `.slider-img-wrapper[data-picture-id]`
`data-src` + `data-category`), `.gallery-v5` (yedek), `#hotel-details-tab` (acilir basliklar `section.check-list-box` + ozellik
gruplari `.other-check-list`), `#HotelDetailActionTabPlaceHolder` (kampanyalar), `#activities-tab` (etkinlik takvimi),
`#review-tab .review-wrapper > .row` (yalnizca puan satiri; yorum metinleri alinmaz).

Esleme:

- `name` h1; `address/city/country` ld+json `address` (streetAddress/addressLocality/addressCountry); `region` konum metninin
  ilk parcasi sehir ve ulkeden farkliysa (Kibris'ta bos kalir); `location` `data-lat/data-lng` (`35,33029` -> 35.33029).
- `rating` ld+json `aggregateRating.ratingValue` (10 uzerinden normalize), `ratingCount` `reviewCount`; yoksa `.review-wrapper .score/.reviewcount`.
  `stars` **yok** (site yildiz gostermiyor) -> nil. `contact` yok (ld+json telefonu acentenin cagri merkezidir, otelin degil).
- `description` "Genel Tanıtım" bolumu (yoksa ld+json description).
- `images` gizli galerideki tum resimler (video atlanir), `caption` = `data-category` (+ `data-caption`). URL normalizasyonu:
  sorgu parametresi atilir, `/<w>x<h>/` boyut segmenti kaldirilir (orijinal boyut), tekrarlar ilk gorunumde tutulur.
- `facilities` `.other-check-list` gruplari sitenin basligiyla ("Ücretsiz Aktiviteler", "Ücretli Aktiviteler", "Otel Özellikleri");
  ust bant etiketleri "Otel Özellikleri" grubuna eklenir (tekrarsiz).
- `rooms` `/odalar` icindeki `.availability-item` kartlari **oda adina gore tekil** (ayni oda her pansiyon icin ayri kart gelir):
  `h4.accommodation-type`, `.room-capacity-text`, popup `.room-check-list-box` (aciklama -> `size` icin ilk `NN m²`, `ul.check-list` ozellikler,
  `.big-image-slider img[data-src]` gorseller). Gala/ozel donem varyantlari ("... (31 Aralık ... GALASI)") sitede ayri oda oldugu icin ayri kayit olarak kalir.
- `boards` `h5.hotel-pension-type` degerleri gorunum sirasiyla.
- `policies` "Genel Uyarılar" metninden `checkIn` ("yerleşme saati en erken 15:00") ve `checkOut` ("en geç saat 11:00") regex ile
  (para tutarlari `10.000` saat sanilmaz); `other` = uyari maddeleri; `cancellation` = basliginda "iptal" gecen kampanya metni;
  `childPolicy` yalnizca basligi cocuk politikasi/kurali olan bir bolum varsa (Acapulco'da "Çocuk" bolumu kids club tanitimidir, alinmaz).
- `faq` sitede yok -> `[]`.
- `sections` sitenin basliklariyla sirayla: acilir basliklar (`<li>` iceren govde -> `list`, aksi `text`; `<br>` satir sonu),
  ozellik gruplari (`list`), "Kampanyalar" (`pairs`: baslik -> metin + tarih araliklari), "Etkinlik Takvimi" (`pairs`: tarih/sanatci -> aciklama),
  "Yorumlar" (`pairs`: kategori -> puan).

Deterministiklik: siralama DOM sirasidir, sayac/zaman alanlari yoktur (`fetchedAt` haric), gorsel URL'leri temizlenir.
Iki ardisik probe cagrisinin JSON'u `fetchedAt` disinda bire bir ayni cikti.

## SearchHotels

`withAnyTab`: havuzdan sekme alir; sekmede bir touristica sayfasi acik ise **navigasyon yapmaz** (otel sayfasi korunur),
sekme bos ise ana sayfayi acar. Sonra `otomatik-tamamlama` ucu sayfa ici `fetch` ile cekilir ve `parseAutocomplete` yalnizca
"OTEL" bolumlerindeki (`KATEGORİLER` haric, `*-otelleri` slug'lari atlanir) sonuclari doner.

Sorgu denemeleri (`searchAttempts`): tam sorgu -> ASCII'ye indirgenmis (ç/ğ/ı/ö/ş/ü) -> son kelime atilarak kisaltilmis
(en az 1 kelime). Ilk sonuc veren deneme kullanilir. Site on-ek/alt-dizi eslemesi yapar ("Merit Roy" -> 4 Merit Royal oteli).

Donen aday: `name`, `url` (`https://www.touristica.com.tr/<slug>`, `ResolveFromURL` kabul eder), `providerHotelId` = **slug**
(otomatik tamamlama sayisal id vermez; `ResolveFromURL` sayisal `HotelId`'yi doner, mapping olustururken o kullanilir), `city` **bos**
(ucta sehir bilgisi yok).

## Probe ozeti (26.10.2026, lokal Docker)

```
probe touristica details https://www.touristica.com.tr/acapulco-resort-convention-spa
# resolve (3.7s)
# details (2.3s) images=155 facilities=3 rooms=14 faq=0 sections=16
probe touristica search "Merit Royal"      -> 4 sonuc (2.1s): Merit Royal Diamond/Gardens/Hotel/Premium
probe touristica search "Elexus"           -> 1 sonuc: Elexus Hotel Resort & Spa
probe touristica search "Acapulco"         -> 1 sonuc
probe touristica search "Cratos Premium"   -> 1 sonuc: Cratos Premium Hotel & Port & Spa
```

Acapulco: rating 8.1 / 621 yorum, konum 35.33029,33.420895, 2 pansiyon tipi, giris 15:00 / cikis 11:00, 16 bolum.

## Bilinen sinirlar

- Yildiz sayisi, iletisim ve SSS sitede bulunmaz; `childPolicy` cogu otelde bos kalir (cocuk kurallari fiyat listesinde periyot bazindadir).
- `/odalar` yaniti sekmedeki oturumun son arama tarihlerine gore fiyat/musaitlik icerebilir; fiyatlar okunmaz, ancak sitenin
  belirli tarihlerde bazi odalari gizlemesi durumunda oda sayisi gunler arasinda degisebilir.
- Arama sonucunda sehir yok, kimlik slug'tir; bolge kategorileri ("Merit Otelleri") bilerek atlanir.
- Detay cagrisi sekme havuzunu kullanir: `TOURISTICA_TABS=1` iken fiyat sorgulariyla sirali calisir (~2-3 sn sayfa acik ise, ~6 sn navigasyonla).
- Testler: `details_test.go` fixture'lari `testdata/detail_page.html` (kisaltilmis galeri/kampanya/etkinlik), `testdata/odalar.html`
  (ilk 3 kart), `testdata/autocomplete_*.html`.
