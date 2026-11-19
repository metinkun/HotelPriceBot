# SmartRate API Referansı

Entegrasyon geliştiricileri için uç nokta, istek ve yanıt referansı. Tüm örnekler canlı sistemden alınmıştır
(16 Kasım 2026); fiyatlar o günkü değerlerdir.

## 1. Genel kurallar

| Konu | Değer |
|---|---|
| Taban adres | `{{BASE_URL}}` |
| Kimlik doğrulama | Her istekte `X-API-Key: pb_live_...` başlığı. Alternatifler: `Authorization: apikey pb_live_...` veya `?api_key=` sorgu parametresi (önerilmez, loglara düşer). |
| İçerik tipi | İstek ve yanıt `application/json; charset=utf-8`. |
| Tarih biçimi | `yyyy-MM-dd` (ör. `2026-12-15`). `checkOut` > `checkIn` olmalı. |
| Otel kimliği | `hotelId` SmartRate iç kimliğidir (ör. `ext_3248`); `GET /api/v1/hotels` listesinden alınır. Listede olmayan kimlik 404 döner. |
| Site kodları | `etstur, tatilsepeti, jolly, tatilbudur, touristica, setur, gezinomi, enuygun, mngturizm, tatilsitesi, prontotour, tatilcikus, tatildukkani, coraltatil`. Güncel liste `GET /api/v1/me` → `providers`. |
| Para birimi | Fiyatlar konaklama toplamıdır (gecelik değil), `currency` genelde `"TL"`. |
| Önbellek | Aynı otel + tarih + kişi sorgusu 5 dakika önbellekte tutulur (`cached: true`, `fetchedAt` çekilme zamanı). Paketinizde `allowFresh` açıksa `fresh=true` ile atlanır. |
| İzleme | Her yanıtta `X-Request-Id` başlığı; hata gövdesinde de `requestId`. Destek talebinde bu değeri iletin. |

### Kota başlıkları

Asenkron iş oluşturma (`POST /api/v1/jobs`) günlük **arama hakkından** düşer; senkron uçlar (`/api/prices*`, `/api/packages`)
site çağrısı kredisinden düşer. İlgili sayaçlar yanıt başlıklarında gelir:

```
X-Quota-Limit: 100      # günlük hak
X-Quota-Used: 3         # bugün kullanılan
Retry-After: 60         # yalnızca 429 yanıtında (saniye)
```

### Hata gövdesi

Tüm hatalar aynı biçimdedir:

```json
{"error": "Otel listenizde yok: ext_3200 (GET /api/v1/hotels ile tanimli otelleri gorebilirsiniz)", "requestId": "6f41c70606a8fae1"}
```

| HTTP | Ne zaman | Örnek `error` |
|---|---|---|
| 400 | Eksik/hatalı parametre | `checkOut, checkIn'den sonra olmali` · `Eksik parametre. provider, hotelId, checkIn, checkOut, adults zorunlu.` · `Bilinmeyen provider: x. Desteklenen: etstur, ...` |
| 401 | Anahtar yok/geçersiz/iptal, hesap pasif, demo süresi dolmuş | `API anahtari gerekli (X-API-Key)` · `gecersiz veya iptal edilmis API anahtari` · `Deneme suresi doldu; ...` |
| 403 | Paket izni yok | `Bu paket API erisimi icermiyor` · `Planiniz 'setur' saglayicisini kapsamiyor. Izinli: ...` · `Planiniz paket (otel+ucak+transfer) sorgusunu kapsamiyor` |
| 404 | Otel listenizde değil / iş bulunamadı | `Otel listenizde yok: ext_3200 (...)` · `Is bulunamadi: olmayan123 (suresi dolmus olabilir)` |
| 429 | Dakikalık sınır veya günlük hak | `Dakikalik istek siniri asildi (60/dk)` · `Gunluk arama hakki doldu (100/100). Hak her gun 00:00'da yenilenir.` |
| 503 | Site bakımda / kuyruk kapalı | `'tatilbudur' sitesi gecici olarak kapali (bakimda)` |

---

## 2. Uç noktalar

### 2.1 `GET /health` — servis ayakta mı (anahtar gerekmez)

```http
GET {{BASE_URL}}/health
```

```json
{
  "status": "ok",
  "version": "1.1.0",
  "commit": "e60c3fe",
  "builtAt": "2026-11-16T13:58:23Z",
  "uptimeSeconds": 812,
  "providers": ["etstur", "tatilsepeti", "jolly", "tatilbudur", "touristica", "setur", "gezinomi", "enuygun", "mngturizm", "tatilsitesi", "prontotour", "tatilcikus", "tatildukkani", "coraltatil"]
}
```

### 2.2 `GET /api/v1/me` — anahtar sahibi, paket, limitler

```http
GET {{BASE_URL}}/api/v1/me
X-API-Key: pb_live_...
```

```json
{
  "admin": false,
  "clientId": 3,
  "clientName": "Örnek API Müşterisi",
  "plan": "standard",
  "providers": ["etstur", "tatilsepeti", "jolly", "tatilbudur", "touristica", "setur", "gezinomi", "enuygun", "mngturizm", "tatilsitesi", "prontotour", "tatilcikus", "tatildukkani", "coraltatil"],
  "allowPackages": true,
  "allowFresh": false,
  "dailyQuota": 2000,
  "ratePerMinute": 60,
  "usedToday": 0,
  "hotelCount": 3,
  "hotelsUrl": "/api/v1/hotels"
}
```

| Alan | Açıklama |
|---|---|
| `providers` | Sorgulayabileceğiniz siteler (paket + hesap ayarı). İstekte `providers` boş bırakılırsa bu liste kullanılır. |
| `allowPackages` | `type: "packages"` (otel + uçak + transfer) sorgusu izni. |
| `allowFresh` | `fresh: true` ile önbelleği atlama izni. |
| `dailyQuota` / `usedToday` | Günlük site çağrısı kredisi ve bugünkü kullanım. |
| `ratePerMinute` | Dakikada izin verilen istek sayısı (aşılırsa 429). |
| `hotelCount` | Size tanımlı otel sayısı. |

### 2.3 `GET /api/v1/hotels` — size tanımlı oteller

Sorgu parametresi: `search` (isteğe bağlı; ad, kimlik veya şehirde arar).

```http
GET {{BASE_URL}}/api/v1/hotels?search=club
X-API-Key: pb_live_...
```

```json
{
  "count": 1,
  "hotels": [
    {
      "hotelId": "ext_3246",
      "name": "Club Di Mare Hotel",
      "city": null,
      "region": null,
      "country": "TRNC",
      "stars": null,
      "chain": null,
      "isActive": true,
      "providers": ["touristica", "gezinomi", "mngturizm"],
      "missingProviders": ["etstur", "tatilsepeti", "jolly", "tatilbudur", "setur", "enuygun", "tatilsitesi", "prontotour", "tatilcikus", "tatildukkani", "coraltatil"],
      "linkedAt": "2026-11-12T16:56:15+03:00"
    }
  ]
}
```

| Alan | Açıklama |
|---|---|
| `hotelId` | Fiyat sorgularında kullanılacak kimlik. Sabittir. |
| `providers` | Bu otel için sonuç dönebilecek siteler (eşleşmesi olan + açık + izinli). |
| `missingProviders` | İzinli olup bu otelde eşleşmesi olmayan siteler; bu siteler istekte verilse bile sonuçta yer almaz. |
| `isActive` | Pasif otel sorgulanamaz. |
| `label` | Hesap yöneticinizin verdiği etiket (varsa). |

### 2.4 `POST /api/v1/jobs` — fiyat sorgusu başlat (asenkron, önerilen)

İstek gövdesi:

| Alan | Tip | Zorunlu | Açıklama |
|---|---|---|---|
| `type` | string | hayır | `prices` (varsayılan, sadece otel) veya `packages` (otel + uçak + transfer). |
| `hotelId` | string | evet | `GET /api/v1/hotels` → `hotelId`. |
| `checkIn`, `checkOut` | string | evet | `yyyy-MM-dd`. |
| `adults` | int | evet | ≥ 1. |
| `children` | int | hayır | Çocuk sayısı. |
| `childAges` | int[] | hayır | Çocuk yaşları (`children` ile aynı uzunlukta). |
| `providers` | string[] | hayır | Boş = izinli tüm siteler. Eşleşmesi olmayan site sessizce atlanır. |
| `airportCode` | string | paket için evet | Kalkış havalimanı IATA kodu (ör. `SAW`, `IST`, `ADB`). |
| `fresh` | bool | hayır | Önbelleği atla (`allowFresh` gerekir; yoksa yok sayılır). |
| `maxWaitSeconds` | int | hayır | 0–120. Verilirse iş en geç bu süre sonunda `done` olur; yanıt vermeyen siteler `progress`'te `timeout`, `result.errors` içinde "Zaman asimi..." olarak listelenir ve o ana kadarki sonuç döner. 0 (varsayılan) = tüm siteler bitene kadar bekle. **Yanıt süresini garanti etmek için önerilir** (ör. 8). |

```http
POST {{BASE_URL}}/api/v1/jobs
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
  "providers": ["etstur", "tatildukkani", "coraltatil"]
}
```

Yanıt `202 Accepted` (`Location: /api/v1/jobs/<requestId>`, `X-Quota-Limit`, `X-Quota-Used` başlıkları):

```json
{
  "requestId": "ce11d35ef7c8fb4dc4fe8711",
  "status": "queued",
  "deduplicated": false,
  "pollAfterMs": 1000,
  "statusUrl": "/api/v1/jobs/ce11d35ef7c8fb4dc4fe8711",
  "expiresInSeconds": 3600
}
```

Aynı sorgu hâlâ sürerken tekrar gönderilirse `200` ve `deduplicated: true` ile mevcut işin kimliği döner; hak düşülmez.
Sonuç `expiresInSeconds` (1 saat) boyunca saklanır.

### 2.5 `GET /api/v1/jobs/{requestId}` — iş durumu ve sonucu

Sorgu parametresi: `wait` (0–30 saniye). Verilirse sunucu iş bitene kadar en fazla bu süre bekler (uzun bekleme),
böylece tek istekle sonuç alınır. Bitmemiş işte `Retry-After: 1` başlığı döner.

```http
GET {{BASE_URL}}/api/v1/jobs/ce11d35ef7c8fb4dc4fe8711?wait=20
X-API-Key: pb_live_...
```

Sürerken (`wait` verilmeden):

```json
{
  "requestId": "ce11d35ef7c8fb4dc4fe8711",
  "type": "prices",
  "status": "running",
  "clientId": "3",
  "createdAt": "2026-11-16T14:11:37.870582584Z",
  "startedAt": "2026-11-16T14:11:37.871616251Z",
  "request": {
    "type": "prices", "hotelId": "ext_3248", "checkIn": "2026-12-15", "checkOut": "2026-12-18",
    "adults": 2, "children": 1, "childAges": [6], "providers": ["etstur", "tatildukkani", "coraltatil"]
  },
  "progress": {"etstur": "running", "tatildukkani": "running"}
}
```

Tamamlanınca (`status: "done"`); `coraltatil` bu otelde eşleşmesi olmadığından `progress` ve `results` içinde yoktur:

```json
{
  "requestId": "ce11d35ef7c8fb4dc4fe8711",
  "type": "prices",
  "status": "done",
  "clientId": "3",
  "createdAt": "2026-11-16T14:11:37.870582584Z",
  "startedAt": "2026-11-16T14:11:37.871616251Z",
  "finishedAt": "2026-11-16T14:11:39.23682446Z",
  "request": {
    "type": "prices", "hotelId": "ext_3248", "checkIn": "2026-12-15", "checkOut": "2026-12-18",
    "adults": 2, "children": 1, "childAges": [6], "providers": ["etstur", "tatildukkani", "coraltatil"]
  },
  "progress": {"etstur": "done", "tatildukkani": "done"},
  "result": {
    "hotelId": "ext_3248",
    "checkIn": "2026-12-15",
    "checkOut": "2026-12-18",
    "results": [
      {
        "provider": "etstur",
        "internalHotelId": "ext_3248",
        "providerHotelId": "sy0pgcybz9ku",
        "hotelName": "Ekor Elegance Hotel",
        "found": true,
        "price": {
          "available": false,
          "hotelName": "Ekor Elegance Hotel",
          "city": "Gazimağusa",
          "roomName": null, "boardType": null, "currency": null,
          "listPrice": null, "price": null, "discountRate": 0
        },
        "durationMs": 1309,
        "cached": false,
        "fetchedAt": "2026-11-16T17:11:39.18270821+03:00"
      },
      {
        "provider": "tatildukkani",
        "internalHotelId": "ext_3248",
        "providerHotelId": "7944",
        "hotelName": "Ekor Elegance Hotel",
        "found": true,
        "price": {
          "available": true,
          "hotelName": "Ekor Elegance Hotel",
          "city": "Gazimağusa",
          "roomName": "Superior oda",
          "boardType": "Oda kahvaltı",
          "currency": "TL",
          "listPrice": 25522.5,
          "price": 21694.13,
          "discountRate": 15,
          "rooms": [
            {"roomName": "Superior oda", "boardType": "Oda kahvaltı", "currency": "TL", "listPrice": 25522.5, "price": 21694.13, "discountRate": 15, "available": true, "status": "available"},
            {"roomName": "Deluxe oda", "boardType": "Oda kahvaltı", "currency": "TL", "listPrice": 28710, "price": 24403.5, "discountRate": 15, "available": true, "status": "available"},
            {"roomName": "Superior oda", "boardType": "Yarım pansiyon", "currency": "TL", "listPrice": 46402.5, "price": 39442.13, "discountRate": 15, "available": true, "status": "available"},
            {"roomName": "Deluxe oda", "boardType": "Yarım pansiyon", "currency": "TL", "listPrice": 49302, "price": 41906.7, "discountRate": 15, "available": true, "status": "available"}
          ]
        },
        "durationMs": 1363,
        "cached": false,
        "fetchedAt": "2026-11-16T17:11:39.236354793+03:00"
      }
    ],
    "errors": []
  }
}
```

Bir site hata verdiğinde `errors` dolu gelir, örneğin `"errors": [{"provider": "tatilbudur", "error": "Request failed with status code 429"}]`.
Satışta olmayan bir odanın örneği: `{"roomName": "Salamis Park", "boardType": "Ultra her şey dahil", "currency": "TL", "listPrice": 49500, "price": 27225, "discountRate": 45, "available": false, "status": "sold_out"}`.

`status` değerleri: `queued` → `running` → `done` | `failed`. `failed` durumunda `result` yerine `error` alanı gelir.
`progress` site bazında `pending | running | done | error | timeout`.

**Süre beklentisi:** siteler paralel sorgulanır; JSON API'li siteler 1–3 sn, tarayıcı gerektiren siteler (touristica,
gezinomi, coraltatil) 10–30 sn sürebilir. İş, en yavaş site bitince tamamlanır. Sonuç `running` iken de kısmidir:
`GET /api/v1/jobs/{id}` biten sitelerin sonuçlarını `result.results[]` içinde o anda döner; `maxWaitSeconds` ile
tamamlanma süresini sınırlayabilir ya da `providers` ile yalnızca hızlı siteleri seçebilirsiniz. Bir site hata verirse diğerleri etkilenmez; hata
`result.errors[]` içinde `{provider, error}` olarak listelenir.

### 2.6 Paket sorgusu (`type: "packages"`)

```json
{"type": "packages", "hotelId": "ext_3248", "airportCode": "SAW", "checkIn": "2026-12-15", "checkOut": "2026-12-18", "adults": 2, "providers": ["etstur"]}
```

Tamamlanmış iş (`result.results[]` paket sonuçlarıdır; aşağıda otelin o tarihte paketi yoktur):

```json
{
  "requestId": "798ee63a355b7c51f26f1555",
  "type": "packages",
  "status": "done",
  "request": {"type": "packages", "hotelId": "ext_3248", "checkIn": "2026-12-15", "checkOut": "2026-12-18", "adults": 2, "children": 0, "providers": ["etstur"], "airportCode": "SAW"},
  "progress": {"etstur": "done"},
  "result": {
    "hotelId": "ext_3248",
    "results": [
      {
        "provider": "etstur",
        "internalHotelId": "ext_3248",
        "providerHotelId": "sy0pgcybz9ku",
        "hotelName": "Ekor Elegance Hotel",
        "airportCode": "SAW",
        "checkIn": "2026-12-15",
        "checkOut": "2026-12-18",
        "available": false,
        "roomCount": 0,
        "rooms": [],
        "durationMs": 0,
        "cached": true,
        "fetchedAt": "2026-11-16T17:11:42.165980878+03:00"
      }
    ]
  }
}
```

Paket bulunduğunda `rooms[]` dolu gelir:

```json
{
  "available": true,
  "roomCount": 2,
  "rooms": [
    {
      "roomId": "6490",
      "roomName": "Park Standart Oda",
      "roomSize": 28,
      "nightCount": 3,
      "boards": [
        {"boardType": "Ultra Her Şey Dahil", "currency": "TL", "listPrice": 60000, "price": 42590, "discountRate": 29, "cancellation": "FREE_CANCELLATION"}
      ]
    }
  ],
  "note": "Touristica otel sayfasinda kalkis havalimani secimi yoktur; paket fiyatlari sitenin varsayilan kalkisi ile dondurulur (airportCode dikkate alinmaz)."
}
```

Paket destekleyen siteler: etstur, tatilsepeti, jolly, tatilbudur, touristica, setur, gezinomi, mngturizm, tatilsitesi,
prontotour, tatilcikus, tatildukkani (enuygun ve coraltatil yalnızca otel). `note` site özelinde uyarıdır (ör. kalkış
havalimanının dikkate alınmadığı siteler).

### 2.7 `GET /api/prices` — tek site, senkron

Parametreler: `provider`, `hotelId`, `checkIn`, `checkOut`, `adults` (zorunlu); `children`, `childAges` (virgülle: `5,8`), `fresh`.

```http
GET {{BASE_URL}}/api/prices?provider=etstur&hotelId=ext_3248&checkIn=2026-12-15&checkOut=2026-12-18&adults=2
X-API-Key: pb_live_...
```

```json
{
  "internalHotelId": "ext_3248",
  "providerHotelId": "sy0pgcybz9ku",
  "hotelName": "Ekor Elegance Hotel",
  "found": true,
  "price": {
    "available": false,
    "hotelName": "Ekor Elegance Hotel",
    "city": "Gazimağusa",
    "roomName": null, "boardType": null, "currency": null,
    "listPrice": null, "price": null, "discountRate": 0
  },
  "durationMs": 1107,
  "cached": false,
  "fetchedAt": "2026-11-16T17:11:40.439478919+03:00"
}
```

Yanıt, site cevap verene kadar bekler (genelde 1–5 sn, tarayıcı gerektiren sitelerde 10–30 sn). `found: false` otelin
o sitede eşleşmesi yok demektir; `price.available: false` eşleşme var ama o tarihte satış yok demektir.

### 2.8 `GET /api/prices/all` — tek otel, birden fazla site, senkron

Parametreler: `hotelId`, `checkIn`, `checkOut`, `adults` (zorunlu); `provider` (virgülle liste; boş = izinli tüm siteler), `children`, `childAges`, `fresh`.

```http
GET {{BASE_URL}}/api/prices/all?hotelId=ext_3248&checkIn=2026-12-15&checkOut=2026-12-18&adults=2&provider=etstur,jolly
X-API-Key: pb_live_...
```

```json
{
  "hotelId": "ext_3248",
  "checkIn": "2026-12-15",
  "checkOut": "2026-12-18",
  "results": [
    {
      "provider": "etstur",
      "internalHotelId": "ext_3248",
      "providerHotelId": "sy0pgcybz9ku",
      "hotelName": "Ekor Elegance Hotel",
      "found": true,
      "price": {
        "available": false,
        "hotelName": "Ekor Elegance Hotel",
        "city": "Gazimağusa",
        "roomName": null,
        "boardType": null,
        "currency": null,
        "listPrice": null,
        "price": null,
        "discountRate": 0
      },
      "durationMs": 1033,
      "cached": false,
      "fetchedAt": "2026-11-16T17:15:55.319550009+03:00"
    },
    {
      "provider": "jolly",
      "internalHotelId": "ext_3248",
      "providerHotelId": "1054405",
      "hotelName": "Ekor Elegance Hotel",
      "found": false,
      "price": {
        "available": false,
        "hotelName": "Ekor Elegance Hotel",
        "city": "Gazi Mağusa",
        "roomName": null,
        "boardType": null,
        "currency": "TL",
        "listPrice": null,
        "price": null,
        "discountRate": 0
      },
      "durationMs": 310,
      "cached": false,
      "fetchedAt": "2026-11-16T17:15:54.596767509+03:00"
    }
  ]
}
```

Yapı, asenkron işin `result` alanıyla aynıdır (`results[]` + `errors[]`); müsait otelde `price` ve `rooms` 2.5'teki gibi dolu gelir.
Yukarıdaki örnekte otel her iki sitede de o tarihte satışta değildir (`available: false`, fiyat yok).

### 2.9 `POST /api/prices/bulk` — tek site, birden fazla otel, senkron

Gövde: `provider`, `hotelIds` (dizi), `checkIn`, `checkOut`, `adults` (zorunlu); `children`, `childAges`.

```http
POST {{BASE_URL}}/api/prices/bulk
X-API-Key: pb_live_...
Content-Type: application/json

{"provider": "etstur", "hotelIds": ["ext_3246", "ext_3248"], "checkIn": "2026-12-15", "checkOut": "2026-12-18", "adults": 2}
```

```json
{
  "provider": "etstur",
  "durationMs": 955,
  "results": [
    {"internalHotelId": "ext_3248", "providerHotelId": "sy0pgcybz9ku", "hotelName": "Ekor Elegance Hotel", "found": true,
     "price": {"available": false, "hotelName": "Ekor Elegance Hotel", "city": "Gazimağusa", "roomName": null, "boardType": null, "currency": null, "listPrice": null, "price": null, "discountRate": 0},
     "cached": false}
  ],
  "notFound": ["ext_3246"]
}
```

`notFound` o sitede eşleşmesi olmayan otel kimlikleridir. Önbellek kullanılmaz; her otel sitede sorgulanır.

### 2.10 `GET /api/packages` — paket (otel + uçak + transfer), senkron

Parametreler: `hotelId`, `airportCode`, `checkIn`, `checkOut`, `adults` (zorunlu); `provider` (tek site; boş = paket destekleyen tüm izinli siteler), `children`, `childAges`.

```http
GET {{BASE_URL}}/api/packages?hotelId=ext_3248&airportCode=SAW&checkIn=2026-12-15&checkOut=2026-12-18&adults=2&provider=etstur
X-API-Key: pb_live_...
```

Tek site verildiğinde yanıt doğrudan paket sonucudur:

```json
{
  "internalHotelId": "ext_3248",
  "providerHotelId": "sy0pgcybz9ku",
  "hotelName": "Ekor Elegance Hotel",
  "airportCode": "SAW",
  "checkIn": "2026-12-15",
  "checkOut": "2026-12-18",
  "available": false,
  "roomCount": 0,
  "rooms": [],
  "durationMs": 211,
  "cached": false,
  "fetchedAt": "2026-11-16T17:11:42.165980878+03:00"
}
```

Birden fazla site istendiğinde `{"hotelId", "airportCode", "checkIn", "checkOut", "results": [paket sonuçları], "errors": []}` döner.

---

## 3. Veri modelleri

### 3.1 Site sonucu (`results[]` elemanı)

| Alan | Tip | Açıklama |
|---|---|---|
| `provider` | string | Site kodu (yalnızca çoklu site yanıtlarında). |
| `internalHotelId` | string | SmartRate kimliği (`hotelId` ile aynı). |
| `providerHotelId` | string | Sitenin kendi otel kimliği. |
| `hotelName` | string | Otel adı. |
| `found` | bool | Sitede eşleşme bulundu ve sorgu yapıldı. |
| `price` | NormalizedPrice | Fiyat özeti (aşağıda). `found=false` ise `null`. |
| `durationMs` | int | Sitenin yanıt süresi. |
| `cached` | bool | Önbellekten geldi. |
| `fetchedAt` | datetime | Siteden çekilme zamanı. |
| `error` | string | Yalnızca hata durumunda (çoğunlukla `errors[]` içinde gelir). |

### 3.2 `NormalizedPrice` — fiyat özeti

Özet alanlar **en ucuz müsait teklifi** gösterir; `rooms` sitenin listelediği tüm teklifleri içerir.

| Alan | Tip | Açıklama |
|---|---|---|
| `available` | bool | Otel o tarihte satışta mı. `false` iken bazı siteler fiyatı yine de verir (`note` açıklar). |
| `hotelName`, `city` | string\|null | Sitenin verdiği ad ve şehir. |
| `roomName`, `boardType` | string\|null | Lider teklifin oda adı ve pansiyonu. |
| `currency` | string\|null | `"TL"`, nadiren `"EUR"`. |
| `listPrice` | number\|null | Üstü çizili liste fiyatı (varsa). |
| `price` | number\|null | Satış fiyatı (konaklama toplamı). |
| `discountRate` | number | İndirim yüzdesi (0 = yok). |
| `discountPrice` | number | Kampanyalı fiyat (bazı siteler). |
| `campaignName` | string | Kampanya adı (bazı siteler). |
| `bankCampaignPrice`, `bankCampaignLabel` | number, string | Banka kartı kampanyası (bazı siteler). |
| `minStayNights` | int | Minimum geceleme şartı (bazı siteler). |
| `note` | string | Uyarı (ör. "Musait degil; sitede gosterilen fiyat"). |
| `rooms` | RoomOffer[] | Tüm oda + pansiyon teklifleri; müsait olanlar önce, fiyata göre artan. |

### 3.3 `RoomOffer` — oda + pansiyon teklifi

| Alan | Tip | Açıklama |
|---|---|---|
| `roomName` | string | Oda adı. |
| `boardType` | string | Pansiyon (Her Şey Dahil, Yarım Pansiyon, Oda Kahvaltı...). Boş olabilir. |
| `currency` | string | Para birimi. |
| `listPrice` | number | Liste fiyatı; satış fiyatından büyükse gelir, yoksa alan yok. |
| `price` | number | Satış fiyatı (konaklama toplamı). |
| `discountRate` | number | İndirim yüzdesi. |
| `available` | bool | `status == "available"` kısayolu. |
| `status` | string | `available` (anında satış) · `on_request` (site fiyat gösterir, "talep et" ile sorulur) · `sold_out` (satışta değil; fiyat bilgi amaçlı). |
| `campaignName` | string | Kampanya metni (varsa). |
| `minStayNights` | int | Minimum geceleme (varsa). |

### 3.4 Paket sonucu

| Alan | Tip | Açıklama |
|---|---|---|
| `airportCode` | string | Kalkış havalimanı. |
| `available` | bool | Paket bulundu. |
| `roomCount` | int | Oda sayısı. |
| `rooms[]` | PackageRoom | `roomId`, `roomName`, `roomSize` (m², olabilir), `nightCount`, `boards[]`. |
| `rooms[].boards[]` | PackageBoard | `boardType`, `currency`, `listPrice`, `price`, `discountRate`, `cancellation` (`FREE_CANCELLATION` vb. veya `null`). Fiyat otel + uçak (+ transfer) toplamıdır. |
| `note` | string | Site özelinde uyarı. |

---

## 4. Önerilen akış

```
1. GET /api/v1/me                      → bağlantı testi, izinli siteler, hak
2. GET /api/v1/hotels                  → hotelId listesi (günde bir kez yenilemek yeterli)
3. POST /api/v1/jobs                   → her otel/tarih için bir iş (aynı anda en fazla 5 iş)
4. GET /api/v1/jobs/{id}?wait=20       → status "done" olana kadar (genelde tek istek yeter)
5. result.results[].price.rooms        → tüm oda fiyatları; price.price lider fiyat
```

Örnek (bash):

```bash
BASE={{BASE_URL}}; KEY=pb_live_...
RID=$(curl -s -X POST $BASE/api/v1/jobs -H "X-API-Key: $KEY" -H 'Content-Type: application/json' \
  -d '{"hotelId":"ext_3248","checkIn":"2026-12-15","checkOut":"2026-12-18","adults":2}' | jq -r .requestId)
curl -s "$BASE/api/v1/jobs/$RID?wait=20" -H "X-API-Key: $KEY" | jq '.status, .result.results[] | {provider, price: .price.price, rooms: (.price.rooms|length)}'
```

- Hızlı yanıt gerekiyorsa `maxWaitSeconds` verin (ör. 8): iş en geç 8 sn'de `done` olur, geç kalan siteler `timeout` olarak işaretlenir.
  Sayfayı aşamalı dolduracaksanız `maxWaitSeconds` vermeden `GET /api/v1/jobs/{id}` ile 1 sn aralıkla sorgulayın; biten siteler hemen gelir.
- `429` alındığında `Retry-After` kadar bekleyip tekrar deneyin; dakikada `ratePerMinute` istek sınırı vardır.
- Tarayıcı gerektiren siteler (touristica, gezinomi, coraltatil) 10–30 sn sürebilir; `wait=30` ile tek istekte alın veya
  `pollAfterMs` aralığıyla sorgulayın.
- Sonuçları kendi tarafınızda 5 dakikadan sık yenilemeyin; aynı sorgu önbellekten döner (`cached: true`).
- OpenAPI/Swagger: `{{BASE_URL}}/docs` (makine tarafından okunabilir şema buradan indirilebilir).
