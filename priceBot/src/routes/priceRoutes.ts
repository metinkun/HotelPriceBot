import { Router, Request, Response } from "express";
import { getPrice, getBulkPrices, getMultiProviderPrices } from "../services/priceService";

const router = Router();

// GET /api/prices/all?hotelId=htl_x&checkIn=2026-07-07&checkOut=2026-07-10&adults=2
// Opsiyonel: &provider=etstur,tatilsepeti (verilmezse otelin tum provider'lari)
router.get("/all", async (req: Request, res: Response) => {
  try {
    const { provider, hotelId, checkIn, checkOut, adults, children, childAges } =
      req.query;

    if (!hotelId || !checkIn || !checkOut || !adults) {
      res.status(400).json({
        error: "Eksik parametre. hotelId, checkIn, checkOut, adults zorunlu.",
      });
      return;
    }

    const providerList = provider
      ? String(provider).split(",").map((p) => p.trim()).filter(Boolean)
      : undefined;

    const result = await getMultiProviderPrices(
      {
        hotelId: String(hotelId),
        checkIn: String(checkIn),
        checkOut: String(checkOut),
        adults: Number(adults),
        children: Number(children || 0),
        childAges: childAges
          ? String(childAges).split(",").map((a) => Number(a.trim())).filter((n) => !Number.isNaN(n))
          : [],
      },
      providerList
    );

    res.json(result);
  } catch (err: any) {
    if (err.message?.includes("bulunamadi")) {
      res.status(404).json({ error: err.message });
    } else {
      res.status(500).json({ error: err.message });
    }
  }
});

// GET /api/prices?provider=etstur&hotelId=hotel-1&checkIn=2026-07-07&checkOut=2026-07-10&adults=2
router.get("/", async (req: Request, res: Response) => {
  try {
    const { provider, hotelId, checkIn, checkOut, adults, children, childAges } =
      req.query;

    if (!provider || !hotelId || !checkIn || !checkOut || !adults) {
      res.status(400).json({
        error:
          "Eksik parametre. provider, hotelId, checkIn, checkOut, adults zorunlu.",
      });
      return;
    }

    const result = await getPrice(String(provider), {
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
    if (err.message?.includes("bulunamadi")) {
      res.status(404).json({ error: err.message });
    } else if (err.message?.includes("Bilinmeyen provider")) {
      res.status(400).json({ error: err.message });
    } else {
      res.status(500).json({ error: err.message });
    }
  }
});

// POST /api/prices/bulk
router.post("/bulk", async (req: Request, res: Response) => {
  try {
    const { provider, hotelIds, checkIn, checkOut, adults, children, childAges } =
      req.body;

    if (!provider || !hotelIds?.length || !checkIn || !checkOut || !adults) {
      res.status(400).json({
        error:
          "Eksik parametre. provider, hotelIds (array), checkIn, checkOut, adults zorunlu.",
      });
      return;
    }

    const result = await getBulkPrices(String(provider), {
      hotelIds,
      checkIn,
      checkOut,
      adults: Number(adults),
      children: Number(children || 0),
      childAges: childAges ?? [],
    });

    res.json(result);
  } catch (err: any) {
    if (err.message?.includes("Bilinmeyen provider")) {
      res.status(400).json({ error: err.message });
    } else {
      res.status(500).json({ error: err.message });
    }
  }
});

export default router;
