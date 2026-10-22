# mngturizm — MNG Turizm (www.mngturizm.com)

Duz HTTP; **tarayici gerekmez, cerez gerekmez**. Site PHP + jQuery; otel detay sayfasindaki
"Fiyat Hesapla" butonu `index.php?s=otelprice` ucuna form POST atar ve HTML parcasi alir.
Oteller ve otel+ucak(+transfer) paketleri (ozellikle Kibris) destekleniyor.

## Uclar

| Amac | Istek | Not |
|---|---|---|
| Otel detay (resolve) | `GET https://www.mngturizm.com/<otel-slug>` | Slug kokte (`/acapulco-resort-hotel-convention-spa`); sondaki `/` 404 verir |
| Oda fiyatlari | `POST https://www.mngturizm.com/index.php?s=otelprice` (form-urlencoded) | HTML parcasi (`<section class="otelfiyatsablon">` bloklari) |
| Ucus arama (paket) | `GET https://www.mngturizm.com/index.php?s=flightsearch&...` | HTML; `flight-box` kutulari + gizli `mintoplam`/`pax` |
| Havalimani autocomplete | `POST index.php?s=flightac&c=tr` body `search=<metin>` | `rel="air-SAW"` formati; kodda kullanilmiyor (IATA dogrudan yetiyor) |

### Header / cerez

- Cloudflare onunde: XHR uclarina **`Referer: https://www.mngturizm.com/...`** yoksa 403 "Just a moment" (JS challenge) donuyor.
  Referer varsa cerezsiz 200. Mapping'deki `sourceUrl` referer olarak gonderilir (yoksa `https://www.mngturizm.com/`).
- Diger basliklar (UA, `x-requested-with`, `accept`) tarayici seti; zorunlu olani sadece Referer.
- Detay sayfasi GET icin referer gerekmez (`sec-fetch-site: none` tarayici header seti yeterli).
- `lang` / `kur` cerezleri (dil/para birimi) set ediliyor ama gonderilmese de TRY fiyat donuyor.

### otelprice govdesi

```
otel=13791&giris=05.12.2026&cikis=08.12.2026&yetiskin=2
&cocuk1=12&cocuk2=-1&cocuk3=-1&cocuk4=-1&cocuksayi=1
&transfervar=0&mintoplam=&gidisUcusDeger=&donusUcusDeger=&gidisrezbilgi=&donusrezbilgi=&pax=
&toid=air-ECN
```

- Tarih `dd.MM.yyyy`. Yetiskin 1–4. Cocuk: en fazla 4; `cocukN` = yas (0 = bebek, 1–17), `-1` = yok; `cocuksayi` = adet.
  Cocuk yasi dikkate aliniyor (12 yas fiyati artirdi, 5 yas ucretsiz cikti — otelin cocuk politikasina bagli).
- Paket: `transfervar=1` + flightsearch'ten alinan `mintoplam`, `pax`, secilen gidis/donus radyo dugmelerinin
  `data-fiyat` (-> `gidisUcusDeger`/`donusUcusDeger`) ve `data-book` (-> `gidisrezbilgi`/`donusrezbilgi`) blob'lari.
- `toid` otel sayfasindaki gizli `#toid` (Kibris: `air-ECN`, Antalya: `air-AYT`); metadata `toId` olarak saklanir.

### flightsearch parametreleri

`otelid, from=SAW, to=ECN, departuredate, returndate (dd.MM.yyyy), adult, child, infant=0, fromid=air-SAW, toid=air-ECN,
s=flightsearch, search=otel, ou=otel+ucak, type=round, transfervar=1, childrenages=-1,-1,-1,-1`
`from`/`to` icin IATA kodu yeterli (sitede havalimani adi gonderiliyor ama zorunlu degil).

Ucus secimi (site JS'i ilk gidisi ve ona uygun ilk donusu secer; biz **en dusuk ucus farkini** (`data-total`) seciyoruz):
gidis `CyprusRoyal` ise donus de `CyprusRoyal` olmali (degilse olmamali); `KHY` tek yon biletlenemez (iki yon de KHY).
Secilen ucus `PackageResult.Note` icinde raporlanir.

## Yanit formati (HTML parse)

Her oda+pansiyon bir `<section class="otelfiyatsablon otel ...">`:
- oda: ilk `<div class="title">`, pansiyon: `<div class="title hidden-xs">`
- kesin fiyat: `class="taksit" rel="33150.00/TRY/OTEL/13791"` (birincil), yoksa `<div class="new"><span class="a">33.150</span><span class="b">,00</span>`
- liste fiyati: `<div class="old">` (paket modunda yok)
- kampanya: `<div class="note">%35 Erken Rezervasyon ... ,<span ...>` (span oncesi metin)
- paket isareti: `<div class="ucakfiyatadahil">Toplam 3 gece otel+uçak+ Transfer paket fiyat</div>`
- musait degil: `"Seçtiğiniz tarihler için oda fiyat bilgisi bulunamadı."` (section yok)

Lead fiyat = en dusuk `Price` teklif. Para birimi TRY -> `TL`.

## Mapping

- `providerHotelId`: `#rezotelid` (yedek: `data-id`). `hotelName`: dataLayer `CD_ProdName` (yedek `<title>`). 
- Metadata: `sourceUrl` (referer icin), `toId` (paket icin zorunlu), `city` (`CD_Area`, orn. "Kıbrıs Girne").
- Zorunlu metadata alani yok; `toId` eksikse paket sorgusu hata verir (URL'den yeniden cozun).
- Online fiyat formu (`#fiyat-hesapla`) olmayan "Talep Et" otelleri (orn. `/grand-kartal-hotel`) resolve'da hata ile reddedilir.

## Probe (2026-10-22, Acapulco Resort Hotel & Convention & SPA, 05–08.12.2026)

```
probe mngturizm price   <url> 2026-12-05 2026-12-08 2        -> Bungalow Oda / Yarım Pansiyon, 33.150 TL (liste 51.000, %35), 5.5s
probe mngturizm price   <url> 2026-12-05 2026-12-08 2 1 12   -> 45.581,25 TL (liste 70.125, %35) — cocuk 12 yas fiyati degistirdi
probe mngturizm package <url> 2026-12-05 2026-12-08 2 SAW    -> 6 oda / 12 board; en ucuk Bungalow YP 46.683,58 TL (otel+ucak+transfer, 3 gece)
                                                               Ucus: SAW 05:45 Pegasus (fark 0) / ECN 14:35 AJET (fark 0); 7.7s
```

## Bilinen sinirlar

- Paket fiyatinda liste fiyati/indirim yok (site sadece net paket toplami veriyor).
- Oda sayisi 1 (site formu tek oda). Cocuk 4 ile sinirli. Yetiskin 4 ile sinirli (site select'i).
- Aralik 2026'da Antalya otelleri kapali ("fiyat bilgisi bulunamadı") — normal; Kibris otelleri fiyat veriyor.
- `{{PASIF}}` / `{{BITTARIH}}` gibi sablon artiklari yanitta kaliyor; parse etkilenmez.
- Cloudflare kurali degisirse (Referer yetmez, cf_clearance ister) hibrit tarayiciya gecilmesi gerekir; su an gerekmiyor.
- `data-total` sadece ucus fark tutaridir; iptal kosulu (`cancellation`) yanitta yok.
