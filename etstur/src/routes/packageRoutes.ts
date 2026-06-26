import { Router, Request, Response } from "express";
import { getPackagePrices } from "../services/packageService";

const router = Router();

// GET /api/packages?hotelId=hotel-1&airportCode=IST&checkIn=2026-07-07&checkOut=2026-07-18&adults=2&children=1&childAges=0
router.get("/", async (req: Request, res: Response) => {
  try {
    const { hotelId, airportCode, checkIn, checkOut, adults, children, childAges } =
      req.query;

    if (!hotelId || !airportCode || !checkIn || !checkOut || !adults) {
      res.status(400).json({
        error:
          "Eksik parametre. hotelId, airportCode, checkIn, checkOut, adults zorunlu.",
      });
      return;
    }

    const result = await getPackagePrices({
      hotelId: String(hotelId),
      airportCode: String(airportCode),
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

export default router;
