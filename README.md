# Etstur Price Bot

Etstur otel fiyatlarını kendi sisteminizdeki otel ID'leriyle sorgulayan bir REST API servisi.
[tatil-sepeti-bot](../tatil-sepeti-bot) mimarisinin Etstur'a uyarlanmış halidir.

## Nasıl Çalışır?

Etstur'un genel (public) arama endpoint'ini kullanır:

```
POST https://www.etstur.com/services/api/search/v2/hotels
```

- Arama **destinasyon bazlıdır** (`url: "Kemer-Otelleri"` gibi). Doğrudan tek otel
  sorgulanamaz; bir destinasyon aranıp sonuçlar `hotelId` ile filtrelenir.
- Endpoint **cookie/oturum gerektirmez** (TatilSepeti'den farkı budur — daha sağlamdır).
- Fiyatlar tek istekte `limit=100` ile çekilir; otel bulunana kadar sayfalanır.

### Eşleştirme (mapping) modeli

Her otel için 3 bilgi saklanır:

| Alan | Açıklama | Örnek |
|------|----------|-------|
| `internalHotelId` | Sizin sisteminizdeki ID | `hotel-1` |
| `etsturHotelId` | Etstur'un otel kodu | `swr922thyt19` |
| `destinationUrl` | Otelin aranacağı destinasyon slug'ı | `Kemer-Otelleri` |

> **Bir otelin `etsturHotelId` ve `destinationUrl` bilgisini bulmak için:**
> Etstur'da ilgili bölgeyi aç (örn. `etstur.com/Kemer-Otelleri`), DevTools → Network →
> `v2/hotels` isteğinin yanıtında oteli bul; `hotelId` alanı = `etsturHotelId`,
> sayfanın slug'ı = `destinationUrl`.

## Kurulum (Geliştirici / Local Test)

```bash
npm install
docker compose up -d          # PostgreSQL (port 5433)
cp .env.example .env          # değerleri ayarla (aşağıya bak)
npm run prisma:generate
npm run prisma:migrate        # tabloları oluşturur (boş başlar!)
npm run dev                   # http://localhost:3000/docs
```

`.env` içinde en azından şunlar olmalı:
```
PORT=3000
DATABASE_URL="postgresql://botuser:botpass123@localhost:5433/etstur_bot?schema=public"
ADMIN_API_KEY="test-api-key-123"   # admin uçları için; istediğini koy
ETSTUR_COOKIE=""                   # opsiyonel, boş bırakılabilir
```

### ⚠️ Önce otel eşleştirmesi ekle (yoksa "otel eslestirmesi bulunamadi" hatası alırsın)

Otel eşleştirmeleri **veritabanında** tutulur ve git ile gelmez. Yeni kurulan
veritabanı **boştur**; bu yüzden fiyat/paket sorgusundan **önce** eşleştirme eklemelisin.
Aksi halde `{"error":"Otel eslestirmesi bulunamadi: hotel-cy-1"}` döner.

Toplu örnek seed (admin, `x-api-key` gerekli):
```bash
curl -X POST http://localhost:3000/api/admin/mappings/bulk \
  -H "Content-Type: application/json" \
  -H "x-api-key: test-api-key-123" \
  -d '{"mappings":[
    {"internalHotelId":"hotel-1","etsturHotelId":"swr922thyt19","destinationUrl":"Kemer-Otelleri","hotelName":"Mirage Park Resort"},
    {"internalHotelId":"hotel-cy-1","etsturHotelId":"61b893df0b29","destinationUrl":"Kibris-Otelleri","hotelName":"Acapulco Resort Convention Spa"}
  ]}'
```

Ardından fiyat/paket sorguları çalışır:
```bash
curl "http://localhost:3000/api/prices?hotelId=hotel-1&checkIn=2026-07-07&checkOut=2026-07-10&adults=2"
curl "http://localhost:3000/api/packages?hotelId=hotel-cy-1&airportCode=IST&checkIn=2026-07-07&checkOut=2026-07-18&adults=2"
```

### Kimlik Doğrulama (Authentication)

| Uç nokta | Kimlik doğrulama |
|----------|------------------|
| `GET /api/prices`, `POST /api/prices/bulk`, `GET /api/packages` | **Yok (public)** |
| `GET/POST/PUT/DELETE /api/admin/mappings*` | **`x-api-key` header = `ADMIN_API_KEY`** |

Etstur'a giden dış istekler **cookie/oturum gerektirmez**. Yani uygulamanın kendi
kullanıcı girişi/JWT'si yoktur; sadece admin (eşleştirme yönetimi) uçları API key ile korunur.

## Uç Noktalar

**Fiyat (public):**
- `GET /api/prices?hotelId=hotel-1&checkIn=2026-06-24&checkOut=2026-06-25&adults=2&children=0`
- `POST /api/prices/bulk`

Tarih formatı: `yyyy-MM-dd`. `childAges` opsiyonel (GET'te `5,8` / POST'ta `[5,8]`).

**Paket — Otel + Uçak + Transfer (public):**
- `GET /api/packages?hotelId=hotel-cy-1&airportCode=IST&checkIn=2026-07-07&checkOut=2026-07-18&adults=2&children=1&childAges=0`

Kullanıcıdan **otel** (kendi ID'niz) + **kalkış havalimanı** (IATA, örn. `IST`, `ESB`, `ADB`) alır; paket oda/pansiyon fiyat listesini döner. Özellikle Kıbrıs otelleri için anlamlıdır. Paket sunulmayan otel/havalimanı kombinasyonunda `available:false` ve boş liste döner.

> **Nasıl çalışır (iki aşamalı, tek mapping ile):**
> 1. `POST /services/api/room` → otelin odalarını arar; **oturum cookie'si + `roomSearchId`** üretir.
> 2. `POST /services/api/room/package` → aynı oturum + `roomSearchId` + `airportCode` ile paket fiyatlarını döner.
>
> `roomSearchId` SESSION cookie'sine bağlı olduğu için iki çağrı aynı oturumu paylaşır (Set-Cookie → Cookie). Mevcut `etsturHotelId` (arama API'sindeki `hotelId`) bu akışta da doğrudan kullanılır — ek alan gerekmez.

**Admin — eşleştirme (x-api-key korumalı):**
- `GET/POST/PUT/DELETE /api/admin/mappings`
- `POST /api/admin/mappings/bulk`

### Örnek fiyat yanıtı

```json
{
  "internalHotelId": "hotel-1",
  "etsturHotelId": "swr922thyt19",
  "hotelName": "Mirage Park Resort",
  "found": true,
  "price": {
    "available": true,
    "roomName": "Park Bina Deluxe Oda Bahçe Manzaralı",
    "boardType": "Premium Her Şey Dahil",
    "currency": "TL",
    "listPrice": 28000,
    "price": 25200,
    "discountRate": 10,
    "bankCampaignPrice": 23184,
    "bankCampaignLabel": "Maximum'a özel %8 ek indirim",
    "minStayNights": 3
  }
}
```

## Notlar / Sınırlamalar

- Bir destinasyonda çok otel varsa (örn. Antalya ~1417) sayfalama gerekir; mümkün
  olan **en dar** destinasyon slug'ı kullanılırsa (Kemer, Belek...) çok daha hızlıdır.
- `MAX_PAGES` güvenlik sınırı 20 sayfadır (max 2000 otel). Otel daha geride ise
  daha dar bir `destinationUrl` kullanın.
- Etstur arama parametrelerini değiştirirse `buildSearchBody` güncellenmelidir.
