# Setur (setur.com.tr)

Durum: **calisiyor** (fiyat + paket + statik detay + isimle arama). Duz HTTP, cerez/tarayici gerekmez.
Kod: `internal/providers/setur/` (`api.go` fiyat/paket + URL cozumleme, `details.go` statik detay + arama,
`rsc.go` Next.js RSC payload yardimcilari, `html.go` HTML -> metin).

## Uclar

| Uc | Amac |
|---|---|
| `GET https://www.setur.com.tr/<slug>?in=&out=&room=` | Fiyat: RSC payload icindeki `lowestPrice` (bkz. `api.go`) |
| `GET https://www.setur.com.tr/<slug>` (tarihsiz) | **Statik detay**: RSC payload'daki `initialData.data` |
| `POST /api/services/v4/SearchesService/searchesUrl` `{"args":[{"s":"<slug>"}]}` | URL -> otel id/ad (ResolveFromURL) |
| `POST /api/services/v4/SearchesService/searchesHotel` `{"args":[{"s":"<metin>"}]}` | **Isimle arama** (autocomplete) |

Slug yonlendirmesi olabilir (`merit-royal-premium-hotel` -> `merit-royal-premium-hotel-casino-spa`, 308); `httpx`
takip eder. Fiyatli sayfa yavas (~20-40 sn), tarihsiz detay sayfasi ~0.5 sn.

## Statik detay (`GetDetails`)

Sayfa Next.js App Router; veri `self.__next_f.push([1,"..."])` parcalarindaki React Flight akisinda.
`rsc.go`: parcalar JS string olarak cozulup birlestirilir, `"initialData":{` ile baslayan JSON `json.Decoder`
ile alinir; uzun metinler ayri `<id>:T<hexBayt>,<metin>` kayitlarindadir ve JSON'da `"$<id>"` olarak
referanslanir (`generalConditions`, kategori `description` alanlari) -> `rscTexts/rscRef` ile yerine konur.
Not: T kaydindan sonra satir sonu olmayabilir, uzunluk bayt cinsinden; akis sirali taranir.

`initialData.data` -> ortak sema:

| Sema | Kaynak |
|---|---|
| name, address, city, region, country, stars, location | `name`, `address`, `city`, `region`, `country`, `starCount`, `latitude/longitude` |
| description | `generalConditions` (Word HTML -> duz metin) |
| images (tam URL, kategori adi caption) | `images[].url` + `hotelImageCategoryName` (Genel, Plaj ve Havuz, Yeme - İçme, ...) |
| facilities | `seasonFeatures[0].categories[]` -> grup adi + `features[].name` (`charged` ise "(Ücretli)") |
| rooms | `rooms[]` ad, `minAreaSize` (m2), `images[]`; ayni adli kayitlar (farkli tedarikci) birlestirilir |
| policies | `checkinBeginTime`, `checkoutTime`; evcil hayvan = "Evcil Hayvan Kabul Şartları" kategorisi |
| sections | "Genel Bilgiler" + aciklamasi olan her kategori (Yiyecek & İçecek, Spor & Eğlence, Spa & Sağlık, Çocuk Hizmetleri, Balayı, Konum, Ödüller ve Sertifikalar, Evcil Hayvan...) + "Öne Çıkan Özellikler" (`mainFacilities`) + "Mesafeler" (`distanceList`, pairs) + "Etkinlikler" (`events[].title`) |

Sayfada **yok**: puan/yorum sayisi (`ratingInitialData:null`), SSS (`faqData:null`), pansiyon tipleri (sadece
tarihli sorguda `boardType`), telefon (JSON-LD'deki numaralar Setur cagri merkezi). Bunlar bos kalir.

Deterministiklik: gorsel URL'lerinde sorgu parametresi yok; `events` yalnizca baslik (tarih alanlari alinmaz);
yanit sirasi kaynak sirasi. Acapulco iki ardisik cekimde `fetchedAt` disinda birebir ayni JSON.

## Isimle arama (`SearchHotels`)

`searchesHotel` yaniti `[{id,text,type,url,city,country}]`; `type=="Hotel"` olanlar alinir (HotelCity/HotelChain/
HotelRegion atlanir). URL = `https://www.setur.com.tr/<url>` (ResolveFromURL `searchesUrl` ile cozer),
`providerHotelId` = `id`, `city` = "Girne, Kıbrıs". Yurt disi oteller de gelir (orn. "Elexus" -> Romanya);
secim cagiran tarafta. Buyuk/kucuk harf ve Turkce karakter farkina API dayanikli.

## Probe ciktisi (2026-10-26)

```
probe setur details https://www.setur.com.tr/acapulco-resort-convention-spa-hotel-casino
  details 0.7s images=194 facilities=7 rooms=10 faq=0 sections=12  (ikinci cekim: ayni JSON)
probe setur search "Merit Royal"        -> 20 otel (Merit Royal Diamond/Gardens/Premium/..., id 821 = merit-royal-premium-hotel)
probe setur search "Elexus"             -> 6 (ilk: Elexus Hotel Resort Casino Spa, Girne, id 1058)
probe setur search "Salamis Bay Conti"  -> 1 (id 506, Magosa)
```

## Sinirlar / notlar

- Fiyat/paket notlari icin `api.go` ve `provider.go` yorumlari (paket yalnizca "lowest" fiyat).
- RSC akis bicimi (Next.js surumu) degisirse `rsc.go` kirilir; hata "Setur sayfasinda RSC payload bulunamadi".
- Oda kapasitesi/ozellikleri statik sayfada yok (yalnizca ad, m2, gorseller).
