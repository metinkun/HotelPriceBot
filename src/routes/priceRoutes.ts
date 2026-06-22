import { Router, Request, Response } from "express";
import { getPrice, getBulkPrices } from "../services/priceService";

const router = Router();

// GET /api/prices?hotelId=hotel-1&checkIn=2026-06-24&checkOut=2026-06-25&adults=2&children=0
router.get("/", async (req: Request, res: Response) => {
  try {
    const { hotelId, checkIn, checkOut, adults, children, childAges } =
      req.query;

    if (!hotelId || !checkIn || !checkOut || !adults) {
      res.status(400).json({
        error: "Eksik parametre. hotelId, checkIn, checkOut, adults zorunlu.",
      });
      return;
    }

    const result = await getPrice({
      hotelId: String(hotelId),
      checkIn: String(checkIn),
      checkOut: String(checkOut),
      adults: Number(adults),
      children: Number(children || 0),
      childAges: childAges
        ? String(childAges)
            .split(",")
            .map((a) => Number(a.trim()))
            .filter((n) => !Number.isNaN(n))
        : [],
    });

    res.json(result);
  } catch (err: any) {
    const status = err.message?.includes("bulunamadi") ? 404 : 500;
    res.status(status).json({ error: err.message });
  }
});

// POST /api/prices/bulk
router.post("/bulk", async (req: Request, res: Response) => {
  try {
    const { hotelIds, checkIn, checkOut, adults, children, childAges } =
      req.body;

    if (!hotelIds?.length || !checkIn || !checkOut || !adults) {
      res.status(400).json({
        error:
          "Eksik parametre. hotelIds (array), checkIn, checkOut, adults zorunlu.",
      });
      return;
    }

    const result = await getBulkPrices({
      hotelIds,
      checkIn,
      checkOut,
      adults: Number(adults),
      children: Number(children || 0),
      childAges: Array.isArray(childAges) ? childAges.map(Number) : [],
    });

    res.json(result);
  } catch (err: any) {
    res.status(500).json({ error: err.message });
  }
});

export default router;
