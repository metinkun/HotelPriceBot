# Postman

- `pricebot.postman_collection.json` — `internal/api/openapi.json`'dan uretilir (tek kaynak OpenAPI).
- `pricebot.local.postman_environment.json`, `pricebot.production.postman_environment.json` — ortamlar (`baseUrl`, `apiKey`, `adminToken`, `requestId`).

Yeniden uretmek icin (openapi.json degistiginde):

```bash
npx -y openapi-to-postmanv2@4 -s internal/api/openapi.json -o postman/pricebot.postman_collection.json -p \
  -O folderStrategy=Tags,requestParametersResolution=Example,exampleParametersResolution=Example
```

Akislar:
1. **Admin**: `POST /api/admin/auth/login` -> test scripti `adminToken`'i ortama yazar; admin istekleri Bearer ile gider.
2. **Musteri (asenkron)**: `POST /api/v1/jobs` -> `requestId` ortama yazilir -> `GET /api/v1/jobs/{{requestId}}?wait=20`
   test scripti is bitmediyse ayni istegi tekrar calistirir (Collection Runner'da otomatik polling).
3. Musteri istekleri koleksiyon seviyesinde `X-API-Key: {{apiKey}}` kullanir.
