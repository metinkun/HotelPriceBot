import { Router, Request, Response } from "express";
import { PrismaClient } from "@prisma/client";
import { adminAuth } from "../../middleware/auth";
import { getProvider } from "../../providers";

const router = Router();
const prisma = new PrismaClient();

// Tum admin route'lari API key ile korunuyor
router.use(adminAuth);

// GET /api/admin/mappings?provider=etstur&search=otel&active=true
router.get("/", async (req: Request, res: Response) => {
  try {
    const { provider, search, active } = req.query;

    const where: any = {};
    if (provider) {
      where.provider = String(provider);
    }
    if (active !== undefined) {
      where.isActive = active === "true";
    }
    if (search) {
      where.OR = [
        { hotelName: { contains: String(search), mode: "insensitive" } },
        { internalHotelId: { contains: String(search) } },
        { providerHotelId: { contains: String(search) } },
      ];
    }

    const mappings = await prisma.hotelMapping.findMany({
      where,
      orderBy: { createdAt: "desc" },
    });
    res.json(mappings);
  } catch (err: any) {
    res.status(500).json({ error: err.message });
  }
});

// POST /api/admin/mappings
router.post("/", async (req: Request, res: Response) => {
  try {
    const { provider, internalHotelId, providerHotelId, hotelName, metadata, isActive } =
      req.body;

    if (!provider || !internalHotelId || !providerHotelId) {
      res.status(400).json({
        error: "provider, internalHotelId ve providerHotelId zorunlu.",
      });
      return;
    }

    // Provider-spesifik validasyon
    const providerInstance = getProvider(String(provider));
    const validationError = providerInstance.validateMappingInput({
      internalHotelId,
      providerHotelId,
      hotelName,
      metadata,
      isActive,
    });
    if (validationError) {
      res.status(400).json({ error: validationError });
      return;
    }

    const mapping = await prisma.hotelMapping.create({
      data: {
        provider: String(provider),
        internalHotelId,
        providerHotelId,
        hotelName: hotelName || null,
        metadata: metadata || null,
        isActive: isActive ?? true,
      },
    });

    res.status(201).json(mapping);
  } catch (err: any) {
    if (err.code === "P2002") {
      res.status(409).json({ error: "Bu provider + internalHotelId kombinasyonu zaten mevcut." });
      return;
    }
    if (err.message?.includes("Bilinmeyen provider")) {
      res.status(400).json({ error: err.message });
      return;
    }
    res.status(500).json({ error: err.message });
  }
});

// POST /api/admin/mappings/bulk
router.post("/bulk", async (req: Request, res: Response) => {
  try {
    const { provider, mappings } = req.body;

    if (!provider || !mappings?.length) {
      res.status(400).json({ error: "provider ve mappings dizisi zorunlu." });
      return;
    }

    // Provider-spesifik validasyon
    const providerInstance = getProvider(String(provider));
    for (const m of mappings) {
      const validationError = providerInstance.validateMappingInput(m);
      if (validationError) {
        res.status(400).json({
          error: `${m.internalHotelId}: ${validationError}`,
        });
        return;
      }
    }

    const results = await prisma.$transaction(
      mappings.map((m: any) =>
        prisma.hotelMapping.upsert({
          where: {
            provider_internalHotelId: {
              provider: String(provider),
              internalHotelId: m.internalHotelId,
            },
          },
          create: {
            provider: String(provider),
            internalHotelId: m.internalHotelId,
            providerHotelId: m.providerHotelId,
            hotelName: m.hotelName || null,
            metadata: m.metadata || null,
            isActive: m.isActive ?? true,
          },
          update: {
            providerHotelId: m.providerHotelId,
            hotelName: m.hotelName || null,
            metadata: m.metadata || null,
            isActive: m.isActive ?? true,
          },
        })
      )
    );

    res.status(201).json({ count: results.length, mappings: results });
  } catch (err: any) {
    if (err.message?.includes("Bilinmeyen provider")) {
      res.status(400).json({ error: err.message });
      return;
    }
    res.status(500).json({ error: err.message });
  }
});

// PUT /api/admin/mappings/:id
router.put("/:id", async (req: Request, res: Response) => {
  try {
    const id = parseInt(req.params.id as string, 10);
    const { internalHotelId, providerHotelId, hotelName, metadata, isActive } =
      req.body;

    const mapping = await prisma.hotelMapping.update({
      where: { id },
      data: {
        ...(internalHotelId !== undefined && { internalHotelId }),
        ...(providerHotelId !== undefined && { providerHotelId }),
        ...(hotelName !== undefined && { hotelName }),
        ...(metadata !== undefined && { metadata }),
        ...(isActive !== undefined && { isActive }),
      },
    });

    res.json(mapping);
  } catch (err: any) {
    if (err.code === "P2025") {
      res.status(404).json({ error: "Eslestirme bulunamadi." });
      return;
    }
    res.status(500).json({ error: err.message });
  }
});

// DELETE /api/admin/mappings/:id
router.delete("/:id", async (req: Request, res: Response) => {
  try {
    const id = parseInt(req.params.id as string, 10);
    await prisma.hotelMapping.delete({ where: { id } });
    res.json({ message: "Eslestirme silindi." });
  } catch (err: any) {
    if (err.code === "P2025") {
      res.status(404).json({ error: "Eslestirme bulunamadi." });
      return;
    }
    res.status(500).json({ error: err.message });
  }
});

export default router;
