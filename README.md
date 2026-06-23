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

## Kurulum

```bash
npm install
docker compose up -d          # PostgreSQL (port 5433)
cp .env.example .env          # gerekirse düzenle (cookie opsiyonel)
npm run prisma:generate
npm run prisma:migrate
npm run dev                   # http://localhost:3000/docs
```

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
