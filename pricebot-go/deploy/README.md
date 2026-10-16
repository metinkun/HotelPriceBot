# Deploy (tek sunucu, Docker)

Gereksinimler: Docker + Compose v2, 80/443 acik, DNS A kaydi (hotel.alpkun.com -> sunucu IP).
Onerilen: 2 vCPU / 4 GB RAM (2 GB ile calisir; tarayici sekmesi 2 ile sinirli).

```bash
git clone <repo> && cd HotelPriceBot/pricebot-go
cp .env.example .env            # ADMIN_API_KEY, WEBSHARE_API_KEY, cookie'ler, CANARY_HOTEL_ID
DOMAIN=hotel.alpkun.com docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
curl -s https://hotel.alpkun.com/health
```

- Guncelleme: `git pull && docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build`
- Loglar: `docker compose logs -f app` (JSON, her satirda reqId)
- Yedekler: `backups` volume'unda gunluk `pricebot-YYYYMMDD-HHMMSS.sql.gz`, 14 gun saklanir.
  Geri yukleme: `gunzip -c yedek.sql.gz | docker compose exec -T postgres psql -U botuser -d pricebot`
- Postgres/Redis portlari disa kapali; sadece Caddy 80/443 dinler.
