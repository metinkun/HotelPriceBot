# SmartRate API Entegrasyon Rehberi

Bu doküman SmartRate fiyat karşılaştırma servisine API ile bağlanacak müşteriler içindir.

## 1. Erişim bilgileri

| Bilgi | Değer |
|---|---|
| Taban adres | `{{BASE_URL}}` |
| Kimlik | Her istekte `X-API-Key: <anahtarınız>` başlığı |
| Dokümantasyon | `{{BASE_URL}}/docs` (Swagger, OpenAPI indirilebilir) |
| Postman | `SmartRate-API.postman_collection.json` + `SmartRate-API.postman_environment.json` |

Anahtar `pb_live_` ile başlar, yalnızca size iletilir ve sunucuda düz metin olarak saklanmaz. Anahtarınızı kaybederseniz
yeni anahtar üretilir, eskisi iptal edilir.

## 2. Postman kurulumu

1. Postman > Import > iki dosyayı birlikte sürükleyin (koleksiyon + ortam).
2. Sağ üstten "SmartRate API" ortamını seçin.
3. Ortam dosyasında `baseUrl` ve `apiKey` değerleri size özel hazırlanmıştır; koleksiyondaki tüm istekler bu iki değişkeni kullanır.
   (`apiKey` boşsa size iletilen anahtarı buraya yazın.)
4. "1. Hesap, oteller ve fiyat sorgusu" klasöründeki istekleri sırayla çalıştırın. İş oluşturma isteği `requestId`
   değişkenini otomatik yazar; durum isteği iş bitene kadar kendini tekrarlar (Collection Runner ile).

## 3. Uçlar ve ne işe yaradıkları

| Uç | Ne yapar |
|---|---|
| `GET /api/v1/me` | Anahtarın sahibi: paket, izinli siteler, günlük hak, bugünkü kullanım, tanımlı otel sayısı. İlk bağlantı testi için kullanın. |
| `GET /api/v1/hotels` | Size tanımlı oteller. Her otel için `hotelId` (sorgularda kullanılacak kimlik), ad, künye, `providers` (bu otel için sonuç dönebilecek siteler) ve `missingProviders` (izinli olup eşleşmesi olmayan siteler). `?search=` ile ad/şehir araması. |
| `POST /api/v1/jobs` | Fiyat sorgusu başlatır (asenkron). 202 ve `requestId` döner. |
| `GET /api/v1/jobs/{requestId}?wait=20` | İşin durumu ve sonucu. `wait` saniye kadar bekleyip sonucu döner (en fazla 30). |
| `GET /api/prices`, `/api/prices/all`, `POST /api/prices/bulk`, `GET /api/packages` | Senkron alternatifler; yanıt siteler cevaplayana kadar bekler. Küçük hacimli entegrasyonlar için. |
| `GET /health` | Servis ayakta mı. Anahtar gerekmez. |

Yalnızca size tanımlı otelleri görebilir ve sorgulayabilirsiniz. Listede olmayan bir `hotelId` ile istek 404 döner.
Yeni otel eklenmesi için hesap yöneticinizle iletişime geçin.

## 4. Fiyat sorgusu örneği

```http
POST /api/v1/jobs
X-API-Key: pb_live_...
Content-Type: application/json

{
  "type": "prices",
  "hotelId": "ext_3248",
  "checkIn": "2026-12-15",
  "checkOut": "2026-12-18",
  "adults": 2,
  "children": 1,
  "childAges": [6],
  "providers": ["etstur", "jolly"]
}
```

- `type`: `prices` (otel) veya `packages` (otel + uçak + transfer; `airportCode` zorunlu, paket izni gerekir).
- `providers` boş bırakılırsa otelin eşleşmesi olan tüm izinli siteler sorgulanır. Eşleşmesi olmayan site sessizce atlanır.
- Yanıt: `202 {"requestId":"...","status":"queued","statusUrl":"/api/v1/jobs/...","pollAfterMs":1000}`.
  Aynı sorgu kısa sürede tekrar gelirse 200 ve `deduplicated: true` döner, hak harcanmaz.

Sonuç (`GET /api/v1/jobs/{requestId}?wait=20`):

```json
{
  "requestId": "e184cf83...",
  "status": "done",
  "progress": { "etstur": "done", "jolly": "done" },
  "result": {
    "hotelId": "ext_3248", "checkIn": "2026-12-15", "checkOut": "2026-12-18",
    "results": [
      { "provider": "etstur", "found": true, "cached": false, "durationMs": 1091,
        "price": { "available": true, "price": 48500, "listPrice": 52000, "currency": "TRY",
                   "roomName": "Standart Oda", "boardType": "Her Şey Dahil", "discountRate": 7 } },
      { "provider": "jolly", "found": true,
        "price": { "available": false, "price": null, "note": "Müsait değil" } }
    ]
  }
}
```

`status` değerleri: `queued`, `running`, `done`, `failed`. `price.available=false` ise otel o sitede o tarihte satışta değildir;
bazı sitelerde fiyat yine de dolu gelebilir (`note` alanı açıklar). Sonuç 1 saat saklanır.

## 5. Limitler ve hata kodları

Her iş 1 arama sayılır. Yanıt başlıklarında `X-Quota-Limit` ve `X-Quota-Used` bugünkü hakkınızı gösterir.

| Kod | Anlam | Ne yapmalı |
|---|---|---|
| 400 | Eksik/hatalı parametre | Mesajdaki alanı düzeltin |
| 401 | Anahtar geçersiz, süresi dolmuş, hesap pasif | Hesap yöneticinizle görüşün |
| 403 | Paketiniz bu siteyi veya paket sorgusunu kapsamıyor | `GET /api/v1/me` ile izinleri kontrol edin |
| 404 | Otel listenizde yok | `GET /api/v1/hotels` |
| 429 | Dakikalık sınır veya günlük hak doldu | `Retry-After` başlığı kadar bekleyin |
| 503 | Site bakımda | Daha sonra tekrar deneyin |

Hata gövdesi her zaman `{"error": "...", "requestId": "..."}` biçimindedir; destek talebinde `requestId` değerini iletin.

## 6. Öneriler

- Sorguları `POST /api/v1/jobs` ile başlatıp `wait=20` ile bekleyin; dakikalık sınırı aşmamak için aynı anda en fazla
  5 iş açın.
- Otel listesini günde bir kez `GET /api/v1/hotels` ile yenileyin; `hotelId` değerleri sabittir, değişmez.
- Fiyatlar önbellekten dönebilir (`cached: true`, en fazla 5 dakika). Paketinizde `allowFresh` açıksa `"fresh": true`
  ile önbelleği atlayabilirsiniz.
