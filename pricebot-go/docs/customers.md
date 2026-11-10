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

## Paketler, limitler ve hesap kurallari (0005_accounts.sql)

- **Paket (plan)** alanlari: tur (ACENTE / OTEL / her ikisi), site sayisi, otel sayisi, kullanici sayisi, gunluk anlik arama,
  gunluk planli gorev, API kullanicisi, demo paketi (+ gun), cihaz basina sinir (bilgisayar/mobil), site cagrisi/gun (ic gosterim).
- **Kota birimi = arama sayisi**: bir arama kac siteye giderse gitsin 1 sayilir; musteri portalda bunu gorur. Site cagrisi
  (eski kredi) yalnizca admin panelde (Kullanim > Site cagrilari) izlenir. Sayaclar `client_daily` tablosunda.
- **Musteri limitleri**: paket atanirken paket degerleri musteriye kopyalanir (`clients.limits`). Admin tek tek
  degistirebilir (`limits_customized=true`). Paket degistiginde ozellestirilmis degerler KORUNUR (yazili anlasma olabilir);
  portalda "Paketinizde size ozel duzenlemeler var" uyarisi gosterilir. "Pakete sifirla" ile paket degerlerine donulur.
- **ACENTE / OTEL**: musteri turu. OTEL turunde "kendi oteli" secilir; portalda bu otel sabitlenir, rakip otellerin her
  sitedeki fiyat farki gosterilir. Paket turu 'both' degilse yalnizca ayni turdeki musteriye atanabilir.
- **Demo**: demo paketi atanan musteriye `demo_expires_at` yazilir; suresi dolunca portal ve API girisi reddedilir.
- **Cihaz dogrulama (WebAuthn)**: musteri ayari `require_device_auth` (varsayilan acik). Giris: sifre -> ilk cihazda
  kayit (Face ID / Touch ID / Windows Hello) -> sonraki girislerde biyometrik dogrulama. Kullanici basina sinir
  2 bilgisayar + 2 mobil (paketten). Kullanici gunde 1 cihaz kaldirabilir; fazlasi veya sinir doluyken yeni cihaz
  talebi "Onay Bekleyenler"e duser; admin onaylayinca eski cihaz kaldirilir. Admin tarafinda kaldirma sinirsiz.
  Test/sunum hesaplarinda ayar kapatilabilir (demo musterisinde kapali).
- **Giris loglari**: her portal giris denemesi (IP, tarayici, cihaz, sonuc) `login_logs`; admin > Giris Loglari ve musteri detayi.
- **Site yonetimi**: admin bir siteyi genel olarak kapatabilir (bakim); kapali site hicbir musteri icin sorgulanmaz,
  portalda "bakimda" gorunur. Gorunen ad ve sira degistirilebilir. Yeni site eklemek kod gelistirmesi gerektirir.
- **Otel kunyesi**: `hotels` tablosu (sehir, bolge, ulke, yildiz, zincir, dis kimlik, aktif). Pasif otel aramalarda
  atlanir. Admin bir sistem otelini dogrudan musteriye baglayabilir (otel limiti kontrol edilir).

## Kasım 2026 ikinci istek listesi (uygulanan kurallar)

- **Silme yok**: müşteri, portal kullanıcısı, otel ve planlı arama silinmez; pasife alınır (admin ve portalda silme düğmeleri kaldırıldı,
  `DELETE` uçları kapatıldı). API anahtarı iptali ve cihaz kaldırma (günlük hak + admin onayı) korunur.
- **Portaldan otel ekleme/çıkarma yok**: otel listesi admin > Müşteri > Oteller'den yönetilir ("Sistem otelinden ekle" / "Listeden çıkar").
- **Otel detayı iki katmanlı**: önce **Otel özeti** (künye, siteler, puanlar, yorumlar, etkinlikler, kampanyalar, değişiklikler; admin'de müşteriler),
  "Statik bilgiler" düğmesiyle acente bazlı statik ekran (sekme içerikleri kendi içinde kayar). Sarı vurgulu alana tıklanınca
  eski/yeni karşılaştırma modalı açılır (kelime bazlı renklendirme). Detay değişiklikleri sayfasında çift tıklama aynı modalı açar.
- **Yorum / puan / etkinlik / kampanya**: snapshot'taki acente bölümlerinden otomatik türetilir (`details.Enrich`), diff'te ayrı etiketlerle
  raporlanır ve ayrı bildirim türleri üretir (`review`, `score`, `event`, `campaign`; diğerleri `detail_change`).
- **Fiyat + müsaitlik**: sonuç tablosunda her hücrede fiyat ve yeşil "müsait" / kırmızı "müsait değil" etiketi; Tatilbudur'da düğmesiz
  (satılmış) yanıttaki fiyat da gösterilir (`price.note`). Müsait olmayan fiyat "en ucuz" vurgusuna girmez.
- **Şifre sıfırlama linki**: admin > Müşteri > Portal kullanıcıları > "Şifre sıfırlama linki" → tek kullanımlık, süreli link
  (`PASSWORD_RESET_TTL_MINUTES`, varsayılan 60). SMTP ayarlıysa (`SMTP_HOST/PORT/USER/PASSWORD/FROM`, `PUBLIC_PORTAL_URL`) e-posta gider;
  değilse link ekranda gösterilir. Kullanıcı `/portal/#reset/<token>` sayfasında yeni şifre belirler; giriş loguna `password_reset` düşer.
- **Kullanıcı bazında cihaz limiti**: portal kullanıcısında bilgisayar/mobil limiti paket/müşteri değerini ezer (boş = varsayılan).
- **Planlı arama tarih aralığı**: başlangıç/bitiş tarihi (boş = hemen / süresiz); zamanlayıcı aralık dışında çalıştırmaz.
- **Admin ekranları**: Site detayı (oteller, müşteriler, değişiklikler, sağlık), proxy/IP rotasyonu durumu (Site Yönetimi),
  planlı arama geçmişi (çalışmalar → sonuç matrisi), arama detayı modalı (Müşteri > Son aramalar), portal kullanıcısında "Giriş logları"
  düğmesi ve giriş loglarında kullanıcı filtresi, tüm tablolarda genel arama + sütun filtreleri, 50-100 siteye uygun site seçici.
- **IP rotasyonu**: `WEBSHARE_API_KEY` ile dönüşümlü havuz (5 dk'da bir senkron), yoksa `PROXY_HOST/PORT/USERNAME/PASSWORD` tek proxy;
  headless tarayıcı için `BROWSER_PROXY_URL` (http://user:pass@host:port, kimlik doğrulama CDP ile). Durum: `GET /api/admin/proxies/status`.
- Terminoloji: "plan" → **paket** (menü, formlar, mesajlar); "planlı arama" zamanlanmış arama anlamında kalır.
