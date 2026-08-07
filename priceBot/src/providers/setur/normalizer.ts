import { NormalizedPrice } from "../../types";
import { SeturHotelData } from "./api";

function decode(s: string | null): string | null {
  if (!s) return s;
  return s
    .replace(/\\+u0026/g, "&")
    // Cift kacisli (\\t, \\n) ve tekli (\t, \n) kontrol karakterleri
    .replace(/\\+[tnr]/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/\s+/g, " ")
    .replace(/\\+$/, "") // artakalan ters bolu
    .trim();
}

/** Setur sayfa verisini ortak NormalizedPrice formatina cevirir. */
export function normalizeSetur(data: SeturHotelData): NormalizedPrice {
  const lp = data.lowestPrice;
  const price = lp?.price ?? null;
  const listPrice = lp?.originalPrice ?? null;

  return {
    available: price != null && price > 0,
    hotelName: decode(data.hotelName),
    city: data.city,
    roomName: null, // Setur lowestPrice'ta oda adi vermiyor
    boardType: data.boardType,
    currency: lp?.currencyCode ?? null,
    listPrice,
    price,
    discountRate: lp?.discountRatio ?? 0,
    minStayNights: data.nightCount,
  };
}
