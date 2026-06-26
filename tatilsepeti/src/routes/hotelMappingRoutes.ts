import { Router, Request, Response } from "express";
import { PrismaClient } from "@prisma/client";
import { adminAuth } from "../middleware/auth";
import { HotelMappingInput } from "../types";

const router = Router();
const prisma = new PrismaClient();

// Tum admin route'lari API key ile korunuyor
router.use(adminAuth);

// GET /api/admin/mappings?search=otel&active=true
router.get("/", async (req: Request, res: Response) => {
  try {
    const { search, active } = req.query;

    const where: any = {};
    if (active !== undefined) {
      where.isActive = active === "true";
    }
    if (search) {
      where.OR = [
        { hotelName: { contains: String(search), mode: "insensitive" } },
        { internalHotelId: { contains: String(search) } },
        { tatilsepetiHotelId: { contains: String(search) } },
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
    const { internalHotelId, tatilsepetiHotelId, hotelName, isActive } =
      req.body as HotelMappingInput;

    if (!internalHotelId || !tatilsepetiHotelId) {
      res.status(400).json({
        error: "internalHotelId ve tatilsepetiHotelId zorunlu.",
      });
      return;
    }

    const mapping = await prisma.hotelMapping.create({
      data: {
        internalHotelId,
        tatilsepetiHotelId,
        hotelName: hotelName || null,
        isActive: isActive ?? true,
      },
    });

    res.status(201).json(mapping);
  } catch (err: any) {
    if (err.code === "P2002") {
      res.status(409).json({ error: "Bu internalHotelId zaten mevcut." });
      return;
    }
    res.status(500).json({ error: err.message });
  }
});

// POST /api/admin/mappings/bulk
router.post("/bulk", async (req: Request, res: Response) => {
  try {
    const { mappings } = req.body as { mappings: HotelMappingInput[] };

    if (!mappings?.length) {
      res.status(400).json({ error: "mappings dizisi zorunlu." });
      return;
    }

    const results = await prisma.$transaction(
      mappings.map((m) =>
        prisma.hotelMapping.upsert({
          where: { internalHotelId: m.internalHotelId },
          create: {
            internalHotelId: m.internalHotelId,
            tatilsepetiHotelId: m.tatilsepetiHotelId,
            hotelName: m.hotelName || null,
            isActive: m.isActive ?? true,
          },
          update: {
            tatilsepetiHotelId: m.tatilsepetiHotelId,
            hotelName: m.hotelName || null,
            isActive: m.isActive ?? true,
          },
        })
      )
    );

    res.status(201).json({ count: results.length, mappings: results });
  } catch (err: any) {
    res.status(500).json({ error: err.message });
  }
});

// PUT /api/admin/mappings/:id
router.put("/:id", async (req: Request, res: Response) => {
  try {
    const id = parseInt(req.params.id as string, 10);
    const { internalHotelId, tatilsepetiHotelId, hotelName, isActive } =
      req.body as Partial<HotelMappingInput>;

    const mapping = await prisma.hotelMapping.update({
      where: { id },
      data: {
        ...(internalHotelId !== undefined && { internalHotelId }),
        ...(tatilsepetiHotelId !== undefined && { tatilsepetiHotelId }),
        ...(hotelName !== undefined && { hotelName }),
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
