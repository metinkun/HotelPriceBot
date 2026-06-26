import { Router, Request, Response } from "express";
import { getPackagePrices, getMultiProviderPackagePrices } from "../services/packageService";

const router = Router();

function parseChildAges(childAges: any): number[] {
  if (!childAges) return [];
  return String(childAges)
    .split(",")
    .map((a) => Number(a.trim()))
    .filter((n) => !Number.isNaN(n));
}

// GET /api/packages?provider=etstur&hotelId=hotel-cy-1&airportCode=IST&checkIn=...&checkOut=...&adults=2
// GET /api/packages?provider=etstur,tatilbudur&hotelId=hotel-cy-1&...   (multi-provider)
// GET /api/packages?hotelId=hotel-cy-1&...                              (tum provider'lar)
router.get("/", async (req: Request, res: Response) => {
  try {
    const { provider, hotelId, airportCode, checkIn, checkOut, adults, children, childAges } =
      req.query;

    if (!hotelId || !airportCode || !checkIn || !checkOut || !adults) {
      res.status(400).json({
        error:
          "Eksik parametre. hotelId, airportCode, checkIn, checkOut, adults zorunlu.",
      });
      return;
    }

    const query = {
      hotelId: String(hotelId),
      airportCode: String(airportCode),
      checkIn: String(checkIn),
      checkOut: String(checkOut),
      adults: Number(adults),
      children: Number(children || 0),
      childAges: parseChildAges(childAges),
    };

    // Tek provider ise dogrudan tek sonuc don
    const providerStr = provider ? String(provider) : "";
    const providerList = providerStr
      .split(",")
      .map((p) => p.trim())
      .filter(Boolean);

    if (providerList.length === 1) {
      const result = await getPackagePrices(providerList[0], query);
      res.json(result);
      return;
    }

    // Birden fazla provider veya hepsi
    const result = await getMultiProviderPackagePrices(
      providerList.length > 0 ? providerList : undefined,
      query
    );
    res.json(result);
  } catch (err: any) {
    if (err.message?.includes("bulunamadi")) {
      res.status(404).json({ error: err.message });
    } else if (
      err.message?.includes("Bilinmeyen provider") ||
      err.message?.includes("desteklemiyor")
    ) {
      res.status(400).json({ error: err.message });
    } else {
      res.status(500).json({ error: err.message });
    }
  }
});

export default router;
