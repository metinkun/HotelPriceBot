import swaggerJsdoc from "swagger-jsdoc";

const options: swaggerJsdoc.Options = {
  definition: {
    openapi: "3.0.0",
    info: {
      title: "Hotel Price Bot API",
      version: "1.0.0",
      description:
        "Unified hotel price bot - multiple providers (etstur, tatilsepeti, tatilbudur...)",
    },
    servers: [
      { url: "https://hotel.alpkun.com", description: "Production" },
    ],
    components: {
      securitySchemes: {
        ApiKeyAuth: {
          type: "apiKey",
          in: "header",
          name: "x-api-key",
        },
      },
      schemas: {
        HotelMapping: {
          type: "object",
          properties: {
            id: { type: "integer" },
            provider: { type: "string", example: "etstur" },
            internalHotelId: { type: "string", example: "hotel-1" },
            providerHotelId: { type: "string", example: "swr922thyt19" },
            hotelName: { type: "string", nullable: true },
            metadata: {
              type: "object",
              nullable: true,
              example: { destinationUrl: "Kemer-Otelleri" },
            },
            isActive: { type: "boolean" },
            createdAt: { type: "string", format: "date-time" },
            updatedAt: { type: "string", format: "date-time" },
          },
        },
      },
    },
    paths: {
      "/health": {
        get: {
          tags: ["System"],
          summary: "Saglik kontrolu + provider listesi",
          responses: {
            "200": {
              description: "Servis calisiyor",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      status: { type: "string", example: "ok" },
                      providers: {
                        type: "array",
                        items: { type: "string" },
                        example: ["etstur", "tatilsepeti"],
                      },
                    },
                  },
                },
              },
            },
          },
        },
      },
      "/api/prices": {
        get: {
          tags: ["Fiyat Sorgulama"],
          summary: "Tek otel fiyat sorgula",
          parameters: [
            {
              name: "provider",
              in: "query",
              required: true,
              schema: { type: "string", enum: ["etstur", "tatilsepeti"] },
              example: "etstur",
            },
            {
              name: "hotelId",
              in: "query",
              required: true,
              schema: { type: "string" },
              example: "hotel-1",
            },
            {
              name: "checkIn",
              in: "query",
              required: true,
              schema: { type: "string" },
              example: "2026-07-07",
            },
            {
              name: "checkOut",
              in: "query",
              required: true,
              schema: { type: "string" },
              example: "2026-07-10",
            },
            {
              name: "adults",
              in: "query",
              required: true,
              schema: { type: "integer" },
              example: 2,
            },
            {
              name: "children",
              in: "query",
              required: false,
              schema: { type: "integer", default: 0 },
            },
            {
              name: "childAges",
              in: "query",
              required: false,
              schema: { type: "string" },
              description: "Virgulle ayrilmis cocuk yaslari",
            },
          ],
          responses: {
            "200": { description: "Fiyat bilgisi" },
            "400": { description: "Eksik parametre veya gecersiz provider" },
            "404": { description: "Otel eslestirmesi bulunamadi" },
          },
        },
      },
      "/api/prices/bulk": {
        post: {
          tags: ["Fiyat Sorgulama"],
          summary: "Toplu otel fiyat sorgula",
          requestBody: {
            required: true,
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  required: ["provider", "hotelIds", "checkIn", "checkOut", "adults"],
                  properties: {
                    provider: { type: "string", example: "etstur" },
                    hotelIds: {
                      type: "array",
                      items: { type: "string" },
                      example: ["hotel-1", "hotel-2"],
                    },
                    checkIn: { type: "string", example: "2026-07-07" },
                    checkOut: { type: "string", example: "2026-07-10" },
                    adults: { type: "integer", example: 2 },
                    children: { type: "integer", default: 0 },
                    childAges: {
                      type: "array",
                      items: { type: "integer" },
                    },
                  },
                },
              },
            },
          },
          responses: {
            "200": { description: "Toplu fiyat bilgisi" },
            "400": { description: "Eksik parametre" },
          },
        },
      },
      "/api/packages": {
        get: {
          tags: ["Paket (Otel + Ucak + Transfer)"],
          summary: "Paket fiyatlari (tek, coklu veya tum provider'lar)",
          description:
            "Otel + Ucak + Transfer paket fiyatlarini sorgular.\n\n" +
            "**Kullanim sekilleri:**\n" +
            "- `?provider=etstur` — Tek provider'dan sorgula\n" +
            "- `?provider=etstur,tatilbudur` — Birden fazla provider'dan paralel sorgula\n" +
            "- Provider parametresi bos birakilirsa paket destekleyen **tum** provider'lar sorgulanir\n\n" +
            "Tek provider verildiginde dogrudan sonuc doner. Birden fazla veya hepsi secildiginde `{ hotelId, results: [...], errors?, notMapped? }` formatinda doner.",
          parameters: [
            {
              name: "provider",
              in: "query",
              required: false,
              schema: { type: "string" },
              description:
                "Provider adi. Tek (etstur), coklu (etstur,tatilbudur) veya bos (tum provider'lar)",
              examples: {
                tek: { summary: "Tek provider", value: "etstur" },
                coklu: {
                  summary: "Birden fazla provider",
                  value: "etstur,tatilsepeti",
                },
                hepsi: {
                  summary: "Tum provider'lar (bos birak)",
                  value: "",
                },
              },
            },
            {
              name: "hotelId",
              in: "query",
              required: true,
              schema: { type: "string" },
              example: "hotel-cy-1",
            },
            {
              name: "airportCode",
              in: "query",
              required: true,
              schema: { type: "string" },
              description: "Kalkis havalimani IATA kodu",
              example: "IST",
            },
            {
              name: "checkIn",
              in: "query",
              required: true,
              schema: { type: "string" },
              example: "2026-07-07",
            },
            {
              name: "checkOut",
              in: "query",
              required: true,
              schema: { type: "string" },
              example: "2026-07-18",
            },
            {
              name: "adults",
              in: "query",
              required: true,
              schema: { type: "integer" },
              example: 2,
            },
            {
              name: "children",
              in: "query",
              required: false,
              schema: { type: "integer", default: 0 },
            },
            {
              name: "childAges",
              in: "query",
              required: false,
              schema: { type: "string" },
              description: "Virgulle ayrilmis cocuk yaslari",
            },
          ],
          responses: {
            "200": {
              description: "Paket fiyat listesi",
              content: {
                "application/json": {
                  examples: {
                    tekProvider: {
                      summary: "Tek provider sonucu",
                      value: {
                        internalHotelId: "hotel-cy-1",
                        providerHotelId: "61b893df0b29",
                        hotelName: "Acapulco Resort",
                        airportCode: "IST",
                        available: true,
                        roomCount: 2,
                        rooms: [
                          {
                            roomId: "6530",
                            roomName: "Villa - Yarim Pansiyon",
                            roomSize: 30,
                            nightCount: 11,
                            boards: [
                              {
                                boardType: "Yarim Pansiyon",
                                currency: "TL",
                                listPrice: 352000,
                                price: 186027,
                                discountRate: 40,
                                cancellation: "FREE_CANCELLATION",
                              },
                            ],
                          },
                        ],
                      },
                    },
                    multiProvider: {
                      summary: "Coklu provider sonucu",
                      value: {
                        hotelId: "hotel-cy-1",
                        results: [
                          {
                            provider: "etstur",
                            available: true,
                            roomCount: 10,
                            rooms: ["..."],
                          },
                          {
                            provider: "tatilsepeti",
                            available: true,
                            roomCount: 5,
                            rooms: ["..."],
                          },
                        ],
                        notMapped: ["tatilbudur"],
                      },
                    },
                  },
                },
              },
            },
            "400": { description: "Eksik parametre veya provider desteklemiyor" },
            "404": { description: "Otel eslestirmesi bulunamadi" },
          },
        },
      },
      "/api/admin/mappings": {
        get: {
          tags: ["Admin - Otel Eslestirme"],
          summary: "Eslestirmeleri listele / ara",
          security: [{ ApiKeyAuth: [] }],
          parameters: [
            {
              name: "provider",
              in: "query",
              required: false,
              schema: { type: "string" },
              description: "Provider'a gore filtrele",
            },
            {
              name: "search",
              in: "query",
              required: false,
              schema: { type: "string" },
            },
            {
              name: "active",
              in: "query",
              required: false,
              schema: { type: "string", enum: ["true", "false"] },
            },
          ],
          responses: {
            "200": {
              description: "Eslestirme listesi",
              content: {
                "application/json": {
                  schema: {
                    type: "array",
                    items: { $ref: "#/components/schemas/HotelMapping" },
                  },
                },
              },
            },
            "401": { description: "Gecersiz API anahtari" },
          },
        },
        post: {
          tags: ["Admin - Otel Eslestirme"],
          summary: "Tek eslestirme ekle",
          security: [{ ApiKeyAuth: [] }],
          requestBody: {
            required: true,
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  required: ["provider", "internalHotelId", "providerHotelId"],
                  properties: {
                    provider: { type: "string", example: "etstur" },
                    internalHotelId: { type: "string", example: "hotel-99" },
                    providerHotelId: { type: "string", example: "swr922thyt19" },
                    hotelName: { type: "string" },
                    metadata: {
                      type: "object",
                      example: { destinationUrl: "Kemer-Otelleri" },
                    },
                    isActive: { type: "boolean", default: true },
                  },
                },
              },
            },
          },
          responses: {
            "201": { description: "Eslestirme olusturuldu" },
            "400": { description: "Eksik parametre veya validasyon hatasi" },
            "401": { description: "Gecersiz API anahtari" },
            "409": { description: "Bu provider + internalHotelId zaten mevcut" },
          },
        },
      },
      "/api/admin/mappings/bulk": {
        post: {
          tags: ["Admin - Otel Eslestirme"],
          summary: "Toplu eslestirme ekle/guncelle (upsert)",
          security: [{ ApiKeyAuth: [] }],
          requestBody: {
            required: true,
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  required: ["provider", "mappings"],
                  properties: {
                    provider: { type: "string", example: "etstur" },
                    mappings: {
                      type: "array",
                      items: {
                        type: "object",
                        required: ["internalHotelId", "providerHotelId"],
                        properties: {
                          internalHotelId: { type: "string" },
                          providerHotelId: { type: "string" },
                          hotelName: { type: "string" },
                          metadata: { type: "object" },
                          isActive: { type: "boolean", default: true },
                        },
                      },
                    },
                  },
                },
              },
            },
          },
          responses: {
            "201": { description: "Eslestirmeler olusturuldu" },
            "400": { description: "Eksik parametre veya validasyon hatasi" },
            "401": { description: "Gecersiz API anahtari" },
          },
        },
      },
      "/api/admin/mappings/{id}": {
        put: {
          tags: ["Admin - Otel Eslestirme"],
          summary: "Eslestirme guncelle",
          security: [{ ApiKeyAuth: [] }],
          parameters: [
            {
              name: "id",
              in: "path",
              required: true,
              schema: { type: "integer" },
            },
          ],
          requestBody: {
            required: true,
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    internalHotelId: { type: "string" },
                    providerHotelId: { type: "string" },
                    hotelName: { type: "string" },
                    metadata: { type: "object" },
                    isActive: { type: "boolean" },
                  },
                },
              },
            },
          },
          responses: {
            "200": { description: "Eslestirme guncellendi" },
            "401": { description: "Gecersiz API anahtari" },
            "404": { description: "Eslestirme bulunamadi" },
          },
        },
        delete: {
          tags: ["Admin - Otel Eslestirme"],
          summary: "Eslestirme sil",
          security: [{ ApiKeyAuth: [] }],
          parameters: [
            {
              name: "id",
              in: "path",
              required: true,
              schema: { type: "integer" },
            },
          ],
          responses: {
            "200": { description: "Eslestirme silindi" },
            "401": { description: "Gecersiz API anahtari" },
            "404": { description: "Eslestirme bulunamadi" },
          },
        },
      },
    },
  },
  apis: [],
};

export const swaggerSpec = swaggerJsdoc(options);
