# Musteri tipleri ve uygulamalar

## 1) API entegrasyonu musterisi (`clientType = api`)

- Admin panelden musteri ve API anahtari olusturulur (`pb_live_...`, yalnizca bir kez gosterilir).
- Istekler `X-API-Key` ile `/api/v1/jobs` (asenkron, onerilen) veya `/api/prices*`, `/api/packages` (senkron) uclarina gider.
- Plan: gunluk kredi, izinli siteler, paket ve `fresh` izni, istek/dk. Ek olarak **musteri basina etkin siteler**
  (admin > Musteri > "Siteler & portal") plan izinlerini daha da daraltir; kapali site icin 403 doner.
- Dokuman: `/docs` (Swagger), `postman/`.

## 2) Web portal musterisi (`clientType = portal` veya `both`)

Uygulama: **`/portal/`** (React, Go'ya gomulu). Giris: admin tarafindan acilan portal kullanicisi (e-posta + sifre).

Ozellikler:
- **Otellerim**: otel adi + etkin sitelerdeki detay linkleri -> otel kimlikleri otomatik cozulur. Oteller musteriye ozeldir
  (`client_hotels`); baska musteri ayni oteli kullanmiyorsa kaldirildiginda eslestirmeler de silinir.
- **Fiyat Ara**: otel(ler) + tarih + kisi (+ paket icin kalkis havalimani) -> her otel icin bir job; sonuc tablosu
  satir=otel, sutun=site, en ucuz hucre vurgulu; sayfa 1.5 sn'de bir yenilenir, kismi sonuclar aninda gorunur.
- **Planli Aramalar**: ad, haftanin gunleri (bos = her gun), saat (Europe/Istanbul), "kac gun sonrasi icin" + gece sayisi,
  kisi, siteler, oteller. Zamanlayici 30 sn'de bir vadesi gelen planlari calistirir, sonuc `searches` tablosuna
  `scheduleId` ile yazilir. "Simdi calistir" ile aninda tetiklenir. Ornek: her Pazartesi 14:00, 30 gun sonrasina 3 gece.
- **Gecmis**: anlik ve planli aramalarin tamami; detayda ayni karsilastirma tablosu.
- **Siteler**: hesap icin etkin/kapali siteler (salt okunur; admin yonetir).
- **Otel detayi** (Otellerim > Detay): her acentenin otel icin yayinladigi statik bilgiler ortak basliklar altinda
  (Genel, Gorseller, Ozellikler, Odalar, Politikalar, Soru-Cevap) + acentenin kendi gorunumu; acente dugmeleriyle gecis.
  Sistem her gun (`DETAILS_RUN_HOUR`) tum acenteleri yeniden ceker, degisiklikleri (eklenen/silinen gorsel, degisen
  aciklama, oda, SSS...) kaydeder ve **bildirim** uretir. Degisiklik sonrasi ILK goruntulemede degisen alanlar farkli
  renkle vurgulanir; sonraki goruntulemelerde vurgu kalkar (kullanici bazli "goruldu" surumu).
- **Bildirimler**: acente bazli "neler degisti" ozetleri; okunmamis sayisi menude rozet olarak gorunur.
- Kota ve kredi API musterisiyle ayni mantikla islenir (site cagrisi basina 1 kredi, cache 0).

Portal API'si (`/api/portal/*`, Bearer JWT, rol `portal`): `auth/login`, `me`, `hotels` (GET/POST/PUT/DELETE),
`searches` (POST/GET/GET {id}), `schedules` (GET/POST/PUT/DELETE, POST {id}/run),
`hotels/{id}/details[?provider=&peek=]`, `hotels/{id}/details/refresh`, `notifications`, `notifications/read`.

## Admin paneli (`/admin/`)

- Musteriler: olustur, plan ata, API anahtari uret/iptal, **Siteler & portal** ekrani: musteri tipi, site ac/kapa,
  portal kullanicilari (olustur, sifre sifirla, pasife al, sil), oteller, planli aramalar ve son aramalar.
- Planli Aramalar: tum musterilerin planlari ve sonraki calisma zamanlari.
- Ozet: provider sagligi, kanarya, kuyruk, cache.
- Oteller > Detay: acente bazli statik detay (portal ile ayni ekran, tum acenteler), "Simdi yenile".
- Toplu Otel Ekle: `ID[tab]Ad` listesi (orn. KKTC 78 otel) -> acentelerde isimle arama -> puanli eslesme raporu
  ("Sadece ara") -> "Ara ve ekle". Ulke alani `TRNC` ise Turkiye/yurt disi sehirlerindeki adaylar elenir; belirsiz
  (ikinci aday cok yakin) eslesmeler eklenmez, elle URL girilir.
- Detay Degisiklikleri / Bildirimler: tum otellerdeki degisiklik gecmisi, son gunluk kontrol raporu, admin bildirimleri.

## Veri modeli (0003_portal.sql, 0004_details.sql)

`clients.client_type`, `clients.enabled_providers`, `portal_users`, `client_hotels`, `searches`, `schedules`;
`hotel_details` (acente basina son snapshot + surum), `hotel_detail_changes` (diff + ozet), `hotel_detail_views`
(goruntuleyen bazli goruldu surumu), `notifications` (`client_id` NULL = admin).
