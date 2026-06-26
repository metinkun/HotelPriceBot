import swaggerJsdoc from "swagger-jsdoc";

const options: swaggerJsdoc.Options = {
  definition: {
    openapi: "3.0.0",
    info: {
      title: "TatilSepeti Price Bot API",
      version: "1.0.0",
      description: "TatilSepeti fiyat sorgulama ve otel eslestirme servisi",
    },
    servers: [{ url: "http://localhost:3000" }],
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
            internalHotelId: { type: "string" },
            tatilsepetiHotelId: { type: "string" },
            hotelName: { type: "string", nullable: true },
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
          summary: "Saglik kontrolu",
          responses: {
            "200": {
              description: "Servis calisiyor",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: { status: { type: "string", example: "ok" } },
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
            { name: "hotelId", in: "query", required: true, schema: { type: "string" }, example: "hotel-1" },
            { name: "checkIn", in: "query", required: true, schema: { type: "string" }, example: "19.05.2026" },
            { name: "checkOut", in: "query", required: true, schema: { type: "string" }, example: "30.05.2026" },
            { name: "adults", in: "query", required: true, schema: { type: "integer" }, example: 3 },
            { name: "children", in: "query", required: false, schema: { type: "integer", default: 0 }, example: 0 },
          ],
          responses: {
            "200": { description: "Fiyat bilgisi" },
            "400": { description: "Eksik parametre" },
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
                  required: ["hotelIds", "checkIn", "checkOut", "adults"],
                  properties: {
                    hotelIds: { type: "array", items: { type: "string" }, example: ["hotel-1", "hotel-2"] },
                    checkIn: { type: "string", example: "19.05.2026" },
                    checkOut: { type: "string", example: "30.05.2026" },
                    adults: { type: "integer", example: 3 },
                    children: { type: "integer", default: 0, example: 0 },
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
      "/api/admin/mappings": {
        get: {
          tags: ["Admin - Otel Eslestirme"],
          summary: "Eslestirmeleri listele / ara",
          security: [{ ApiKeyAuth: [] }],
          parameters: [
            { name: "search", in: "query", required: false, schema: { type: "string" }, description: "Otel adi veya ID ile arama" },
            { name: "active", in: "query", required: false, schema: { type: "string", enum: ["true", "false"] }, description: "Aktif/pasif filtresi" },
          ],
          responses: {
            "200": {
              description: "Eslestirme listesi",
              content: {
                "application/json": {
                  schema: { type: "array", items: { $ref: "#/components/schemas/HotelMapping" } },
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
                  required: ["internalHotelId", "tatilsepetiHotelId"],
                  properties: {
                    internalHotelId: { type: "string", example: "hotel-99" },
                    tatilsepetiHotelId: { type: "string", example: "12345" },
                    hotelName: { type: "string", example: "Ornek Otel" },
                    isActive: { type: "boolean", default: true },
                  },
                },
              },
            },
          },
          responses: {
            "201": { description: "Eslestirme olusturuldu" },
            "400": { description: "Eksik parametre" },
            "401": { description: "Gecersiz API anahtari" },
            "409": { description: "internalHotelId zaten mevcut" },
          },
        },
      },
      "/api/admin/mappings/bulk": {
        post: {
          tags: ["Admin - Otel Eslestirme"],
          summary: "Toplu eslestirme ekle (upsert)",
          security: [{ ApiKeyAuth: [] }],
          requestBody: {
            required: true,
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  required: ["mappings"],
                  properties: {
                    mappings: {
                      type: "array",
                      items: {
                        type: "object",
                        required: ["internalHotelId", "tatilsepetiHotelId"],
                        properties: {
                          internalHotelId: { type: "string" },
                          tatilsepetiHotelId: { type: "string" },
                          hotelName: { type: "string" },
                          isActive: { type: "boolean", default: true },
                        },
                      },
                      example: [
                        { internalHotelId: "hotel-10", tatilsepetiHotelId: "111", hotelName: "Otel A" },
                        { internalHotelId: "hotel-11", tatilsepetiHotelId: "222", hotelName: "Otel B" },
                      ],
                    },
                  },
                },
              },
            },
          },
          responses: {
            "201": { description: "Eslestirmeler olusturuldu" },
            "400": { description: "mappings dizisi zorunlu" },
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
            { name: "id", in: "path", required: true, schema: { type: "integer" }, example: 1 },
          ],
          requestBody: {
            required: true,
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    internalHotelId: { type: "string" },
                    tatilsepetiHotelId: { type: "string" },
                    hotelName: { type: "string" },
                    isActive: { type: "boolean" },
                  },
                },
                example: { hotelName: "Guncel Otel Adi", isActive: false },
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
            { name: "id", in: "path", required: true, schema: { type: "integer" }, example: 1 },
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
