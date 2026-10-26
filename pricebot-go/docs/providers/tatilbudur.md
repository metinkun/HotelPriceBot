# Tatilbudur provider (`tatilbudur`)

Durum: **duz HTTP, tarayici gerekmez; Akamai onunde.** Tam tarayici header seti (`pageHeaders`: UA + `sec-ch-ua*` +
`sec-fetch-*` + `upgrade-insecure-requests`) olmadan sayfa ve XHR uclari 403 "Access Denied" doner. Laravel CSRF
`_token` + oturum cerezleri (`ak_bmsc`, `bm_sv`, `XSRF-TOKEN`, `tbcom`) detay sayfasindan alinir.
Fiyat + paket, URL cozumleme, **statik detay** ve **isimle arama**.

## URL cozumleme (`ResolveFromURL`)

`GET https://www.tatilbudur.com/<slug>` -> `hotelId=(\d+)` (providerHotelId, orn. 6713), `h1.hotel-name`. Metadata: `sourceUrl`.
Yanlis slug 404 sayfasi doner (hotelId yok).

## Fiyat / paket (ozet)

`POST /hotel/calculate-room-price` (form `#allHotelTabForm`: `_token, hotelId, checkInDate d.M.yyyy, checkOutDate,
daterange-1 dd/MM/yyyy - dd/MM/yyyy, adult, child, ...`; header `x-csrf-token`, `cookie`) -> `{view, alternativeHtml}`.
Paket: `POST /hotel/calculate-package-room-price` + `to` (sayfadaki `&to=ECN&from=`) + `from`.

## Statik detay (`GetDetails`)

Kaynak: detay sayfasi HTML'i (`fetchTokenAndCookies`), goquery ile parse.

| Sema | Kaynak |
|---|---|
| `name` | `h1.hotel-name` |
| `stars`, `address`, `city`, `region`, `country`, `location`, `contact.phone`, check-in/out | ld+json `WebPage.mainEntity` (Hotel): `starRating.ratingValue`, `address.streetAddress / addressRegion (Girne) / addressLocality (Çatalköy) / addressCountry.name`, `geo`, `telephone`, `checkInTime/checkOutTime` |
| `rating`, `ratingCount` | `.comments-number` ("7.7/10"), `.comment-detail-title li` ("407 Onaylı Misafir Yorumu") |
| `description` | "Konsept Özellikleri" karti (`.hd-info-card .hd-info-desc`); sayfada ayri genel tanitim metni yok |
| `images` | `#main-carousel a.carousel__slide img` (productcdn `/Otel/gallery/`), caption = `galleryCategory_N` sinifi -> `.hotel__modal__filter li[data-filter]` adi ("Genel", "Yeme & İçme", "Oda", ...); `.hotel_room_modal` carousel'indeki ek gorseller gorsel id'sine (`_1113588.jpg`) gore tekillestirilerek eklenir |
| `facilities` | `.hotel-amenities-wrapper .hf-inner`: sekme (`.hf-tab-title-text`) / alt grup (`.hf-subgroup-title`) -> "Sekme / Alt grup"; madde `.hf-item-text` + rozet `.hf-item-badge .text` -> "(Ücretli)"/"(Ücretsiz)"; ayrica `.highlights .hotel-properties li` -> "Öne Çıkan Özellikler" |
| `rooms`, `boards` | `calculate-room-price` sabit pencere (iki ay sonrasinin 1'i, 3 gece, 2 yetiskin): `.room-type-new` karti (`h3.room-type-title`, `.room-gallery img[data-src]`, `.features-check-list li span`, `.meal-type-desc` -> boards). Ardindan **ayni oturumda** `GET /room-detail-modal/<hotelId>/<roomTypeId>` (calc yanitinin cerezleri eklenir; fiyat hesaplanmadan cagrilirsa `{"success":false}` doner): `.room-property li` tum oda ozellikleri (`" *"` eki = Ücretli), `.hotel-images-2 img` oda galerisi |
| `policies` | ld+json saatleri; `childPolicy` = "Aile & Çocuk" sekme aciklamasi (varsa); `petPolicy` = ozelliklerde "evcil" gecen madde; `other` = "Önemli Notlar" paragraflari (varsa) |
| `sections` | "Misafir Değerlendirmesi" (pairs: Genel/Oda/Yemekler/Hizmet/Fiyat Performans/Yorum Sayısı), sekme aciklamalari `.hf-tab-desc-text` (text, orn. "Havuz & Plaj", "Balayı"), `.hd-info-card` kartlari ("Konsept Özellikleri", "Önemli Notlar"), "Konum Bilgileri" (pairs: Mevkii / Denize Mesafe / Havaalanına Mesafe / Şehir Merkezine Mesafe), "Etkinlikler" (list: `.activity-card` baslik + tarih), "KKTC Giriş Kuralları" (`#cyprusInfoModal`, Kibris otelleri) |

Alinmayanlar: tekil yorumlar (`#comment-container`), kampanya modal'lari, "Popüler Bölgeler". Oda m2/aciklama sitede yok.
"Önemli Notlar" karti `data-area="HotelDescription"` bos geliyor (Acapulco'da).

Probe (Acapulco, 2026-10-26): `images=135 facilities=16 rooms=6 faq=0 sections=7`, ~5 sn (sayfa + calc + 6 oda modali);
iki ardisik cagri `fetchedAt` haric birebir ayni JSON.

## Isimle arama (`SearchHotels`)

`GET https://www.tatilbudur.com/hotel/search?term=<metin>` (header: `xhrHeaders` = tarayici UA + `sec-ch-ua*` +
`sec-fetch-dest: empty / mode: cors / site: same-origin` + `x-requested-with`; cerez gerekmez, eksik header'da Akamai 403).
Yanit `[{value, id, type, url, country, city, county, description, category}]`; sadece `type == "Hotel"`.
URL = `https://www.tatilbudur.com/<url>`; `providerHotelId` = `id`; `city` = `county, city`. Sonuc yoksa sorgu son kelimesi
atilarak tekrarlanir (min 2 karakter). Site arama kutusu da bu ucu kullanir (`hotelSearch(term)`).

Ornek: "Merit Royal" -> Premium / Gardens / Diamond (+14) / Hotel Casino Spa (+14) (4); "Elexus" -> Elexus Hotel Resort
Casino (9032); "Cratos Premium" -> Cratos Premium Hotel Casino & SPA (6741); "Salamis Bay Conti" -> Salamis Bay Conti
Resort Hotel (6716, Gazimağusa).

## Bilinen sinirlar

- Oda buyuklugu (m2) ve oda aciklamasi sitede sunulmuyor.
- Odalar fiyat gorunumunden geldigi icin pencerede musait olmayan oda tipleri gorunmez.
- Akamai cerezleri ~1 saat gecerli; her GetDetails cagrisi sayfayi yeniden ceker (token+cerez taze).
