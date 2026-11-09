# Ürün ekibine sorular (admin panel istek listesi, Kasım 2026)

Geliştirme bu sorular netleşmeden, aşağıda yazılı varsayımlarla ilerledi. Cevaplar farklı çıkarsa ayrı istek olarak değiştirilir.

## Kota ve sayım

1. **Günlük arama sayısı birimi.** Varsayım: bir arama kaç siteye giderse gitsin 1 sayılır; müşteri yalnızca arama sayısını görür, site çağrısı sayısı admin panelde kalır. Onaylıyor musunuz? Paket fiyatlaması site çağrısına göre yapılacaksa müşteriye de gösterilmeli mi?
2. **Planlı aramalar günlük haktan düşer mi?** Varsayım: hayır, planlı aramalar ayrı sayılır ("günlük planlı görev"), anlık aramalar ayrı.
3. **Günlük planlı görev** tanımı: Varsayım: gün içinde çalışabilecek planlı arama adedi (günde 20 çalışma). Alternatif: tanımlanabilecek plan adedi (en fazla 5 plan). Hangisi? İkisi birden isteniyorsa söyleyin.
4. **API aramaları** günlük arama hakkından düşer mi? Varsayım: evet, API üzerinden açılan her iş 1 arama sayılır (API müşterisi için ayrı bir hak gerekiyorsa belirtin).
5. Hak dolduğunda davranış: Varsayım: arama reddedilir, "yarın 00:00'da yenilenir" mesajı gösterilir. Ek ücretli aşım (overage) istiyor musunuz?

## ACENTE / OTEL ayrımı

6. **OTEL türü müşterinin amacı** kendi otelini rakiplerle karşılaştırmak. Varsayım: admin müşteriye "kendi oteli"ni atar; portalda bu otel sabitlenir, her sitede rakip otellerin fiyat farkı (+/-) gösterilir. Rakip seti müşteri tarafından mı seçilsin (bugün "Otellerim" listesi = rakipler), yoksa biz mi tanımlayalım?
7. OTEL türünde **birden fazla kendi oteli** (zincir) olabilir mi? Varsayım: tek otel. Zincir ise OTEL yerine ACENTE gibi mi davranmalı?
8. **ACENTE türü** için farklı bir ekran/rapor bekleniyor mu (örneğin "diğer acentelerle farkım" için acentenin kendi sitesinin işaretlenmesi)? Varsayım: ACENTE bugünkü genel görünümü kullanır; kendi sitesini işaretleme yok.
9. Tür, **paket satışını kısıtlasın mı**? Varsayım: paketin türü "her ikisi" değilse yalnızca aynı türdeki müşteriye atanabilir.
10. Türe göre **fiyatlandırma veya limit farkı** olacak mı (OTEL paketlerinde otel sayısı küçük, site sayısı büyük gibi)? Varsayılan paketleri buna göre hazırlayalım mı?

## Kullanıcı, cihaz ve güvenlik

11. **Biyometri şifreye ek** olarak uygulandı (önce şifre, sonra cihaz doğrulaması). Şifre tamamen kaldırılsın mı (passkey ile şifresiz giriş)?
12. **Cihaz sınırı** kullanıcı başına 2 bilgisayar + 2 mobil (paketten değiştirilebilir). Sınır hesap başına mı olmalı, kullanıcı başına mı? Varsayım: kullanıcı başına.
13. Kullanıcı **günde 1 cihaz** kaldırabilir; fazlası yönetici onayına düşer. Onay bekleyen talep için müşteriye e-posta/bildirim gitsin mi? Onayı kim verir (bizim ekip mi, müşterinin kendi yöneticisi mi)? Varsayım: bizim admin ekibi.
14. Cihaz doğrulamasını desteklemeyen tarayıcı/cihaz (eski Android, kurumsal kısıtlı PC) için ne yapılsın? Varsayım: giriş engellenir ve güncel tarayıcı önerilir; müşteri bazında ayar kapatılabilir (test/sunum hesapları).
15. **Şifre yönetimi** tamamen bizde: "şifremi unuttum" akışı olmayacak, kullanıcı şifre değiştiremeyecek. Doğru mu? Şifre politikası (uzunluk, süre) var mı?
16. **Giriş logları** saklama süresi? Varsayım: istek logları ile aynı (LOG_RETENTION_DAYS, 90 gün). KVKK gereği IP maskeleme gerekiyor mu?

## Paket ve demo

17. **Demo paketi** varsayımı: 14 gün, 5 site, 5 otel, 1 kullanıcı, günde 10 arama, 2 planlı çalışma, API yok, 1 bilgisayar + 1 mobil. Süre dolunca hesap girişi reddedilir ve admin'e "demo doldu" rozeti düşer. Süre ve limitler uygun mu? Süre sonunda otomatik dönüşüm (ücretli pakete geçiş teklifi) isteniyor mu?
18. **Paket değişince özelleştirilmiş limitler korunur**, portalda "paketinizde size özel düzenlemeler var, yetkilinizle görüşün" uyarısı çıkar. Uyarı metni ve nerede görüneceği (özet sayfası) uygun mu?
19. "**Site sayısı**" limiti: hangi sitelerin seçileceğine müşteri mi karar verir, biz mi? Varsayım: biz (admin, müşteri detayı > Siteler); müşteri yalnızca görür.
20. **Kullanıcı sayısı** dolunca yeni kullanıcı eklenemez; kullanıcı pasife alınınca sayılmaya devam eder mi? Varsayım: pasif kullanıcı da sayılır (silinince düşer).

## Site ve otel yönetimi

21. **Site yönetimi**nden beklenen, mevcut 13 sitenin genel açık/kapalı (bakım), görünen ad ve sıra yönetimi olarak uygulandı. Arayüzden yeni site ekleme beklentisi var mı? (Her site ayrı entegrasyon geliştirmesi gerektirir.)
22. **Otel künyesi** alanları: ad, şehir, bölge, ülke, yıldız, zincir, dış kimlik (Excel ID), aktif/pasif, not. Eksik alan var mı (konum koordinatı, kategori, sözleşme durumu)?
23. Pasif otel aramalarda atlanır; müşteri portalında görünmeye devam etsin mi, gizlensin mi? Varsayım: görünür ama "pasif" etiketiyle.
24. Admin bir oteli müşteriye doğrudan bağlayabiliyor. Müşteri kendi eklediği oteli silerse ve başka müşteri kullanmıyorsa sistemden de siliniyor; bu davranış korunsun mu?
