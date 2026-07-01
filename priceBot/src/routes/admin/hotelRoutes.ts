import { Router, Request, Response } from "express";
import { adminAuth } from "../../middleware/auth";
import {
  addHotelFromUrls,
  listHotels,
  getHotel,
  updateHotel,
  deleteHotel,
} from "../../services/hotelService";

const router = Router();
router.use(adminAuth);

// POST /api/admin/hotels
// body: { urls: { etstur: "...", tatilsepeti: "..." }, internalHotelId?, hotelName? }
router.post("/", async (req: Request, res: Response) => {
  try {
    const { urls, internalHotelId, hotelName } = req.body;
    if (!urls || typeof urls !== "object" || Object.keys(urls).length === 0) {
      res.status(400).json({
        error: "urls zorunlu. Ornek: { \"urls\": { \"etstur\": \"...\", \"tatilsepeti\": \"...\" } }",
      });
      return;
    }
    if (!hotelName || !String(hotelName).trim()) {
      res.status(400).json({ error: "hotelName zorunlu." });
      return;
    }
    const result = await addHotelFromUrls({ urls, internalHotelId, hotelName });
    res.status(201).json(result);
  } catch (err: any) {
    if (err.message?.includes("Bilinmeyen provider") || err.message?.includes("URL")) {
      res.status(400).json({ error: err.message, details: err.details });
      return;
    }
    res.status(500).json({ error: err.message, details: err.details });
  }
});

// GET /api/admin/hotels?search=...&active=true
router.get("/", async (req: Request, res: Response) => {
  try {
    const { search, active } = req.query;
    const hotels = await listHotels({
      search: search ? String(search) : undefined,
      active: active !== undefined ? active === "true" : undefined,
    });
    res.json(hotels);
  } catch (err: any) {
    res.status(500).json({ error: err.message });
  }
});

// GET /api/admin/hotels/:id
router.get("/:id", async (req: Request, res: Response) => {
  try {
    const hotel = await getHotel(req.params.id as string);
    res.json(hotel);
  } catch (err: any) {
    res.status(err.message?.includes("bulunamadi") ? 404 : 500).json({ error: err.message });
  }
});

// PUT /api/admin/hotels/:id  body: { urls?, hotelName? }
router.put("/:id", async (req: Request, res: Response) => {
  try {
    const { urls, hotelName } = req.body;
    const hotel = await updateHotel(req.params.id as string, { urls, hotelName });
    res.json(hotel);
  } catch (err: any) {
    if (err.message?.includes("bulunamadi")) {
      res.status(404).json({ error: err.message });
    } else if (err.message?.includes("URL") || err.message?.includes("provider")) {
      res.status(400).json({ error: err.message, details: err.details });
    } else {
      res.status(500).json({ error: err.message });
    }
  }
});

// DELETE /api/admin/hotels/:id
router.delete("/:id", async (req: Request, res: Response) => {
  try {
    const result = await deleteHotel(req.params.id as string);
    res.json({ message: "Otel silindi.", ...result });
  } catch (err: any) {
    res.status(err.message?.includes("bulunamadi") ? 404 : 500).json({ error: err.message });
  }
});

export default router;
