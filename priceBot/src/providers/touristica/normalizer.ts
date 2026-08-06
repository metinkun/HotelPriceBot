import { NormalizedPrice } from "../../types";
import { TouristicaRawPrice } from "./api";

/** "51.330" -> 51330 */
function parseTL(s: string): number | null {
  const n = Number(s.replace(/\./g, "").replace(",", "."));
  return Number.isFinite(n) && n > 0 ? n : null;
}

/**
 * Oda blogu ornegi:
 *   "3 Gecelik Toplam Fiyat 87.000 TL 51.330 TL Rezervasyon Yap"
 * Ilk fiyat liste (uzeri cizili), ikinci fiyat satis fiyatidir.
 * Tek fiyat varsa indirim yoktur.
 */
function parseRoomBlock(block: string): {
  listPrice: number | null;
  price: number | null;
  nights: number | null;
} {
  const prices = Array.from(
    block.matchAll(/([0-9]{1,3}(?:\.[0-9]{3})+|[0-9]{4,})\s*(?:TL|₺)/g)
  ).map((m) => parseTL(m[1]));
  const nightsM = block.match(/(\d+)\s*Gecelik/i);
  const valid = prices.filter((p): p is number => p != null);

  if (valid.length === 0) return { listPrice: null, price: null, nights: null };
  const nights = nightsM ? Number(nightsM[1]) : null;

  if (valid.length === 1) {
    return { listPrice: null, price: valid[0], nights };
  }
  // Iki fiyat: buyuk olan liste, kucuk olan satis
  const price = Math.min(...valid);
  const listPrice = Math.max(...valid);
  return { listPrice: listPrice > price ? listPrice : null, price, nights };
}

/** Touristica ham fiyat bloklarini ortak NormalizedPrice'a cevirir (en ucuz teklif). */
export function normalizeTouristica(raw: TouristicaRawPrice): NormalizedPrice {
  const parsed = raw.roomBlocks
    .map(parseRoomBlock)
    .filter((p) => p.price != null);

  if (parsed.length === 0) {
    return {
      available: false,
      hotelName: raw.hotelName,
      city: raw.city,
      roomName: null,
      boardType: null,
      currency: null,
      listPrice: null,
      price: null,
      discountRate: 0,
    };
  }

  // En ucuz teklifi lead fiyat olarak al
  const best = parsed.reduce((a, b) => (b.price! < a.price! ? b : a));
  const discountRate =
    best.listPrice && best.price && best.listPrice > best.price
      ? Math.round((1 - best.price / best.listPrice) * 100)
      : 0;

  return {
    available: true,
    hotelName: raw.hotelName,
    city: raw.city,
    roomName: null, // Touristica blogu oda adini ayri vermiyor
    boardType: null,
    currency: "TL",
    listPrice: best.listPrice,
    price: best.price,
    discountRate,
    minStayNights: best.nights ?? null,
  };
}
