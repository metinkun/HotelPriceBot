import { NormalizedPrice } from "../../types";
import { JollyReservationResponse } from "./api";

function decode(s: string): string {
  return s
    .replace(/&#x2B;/g, "+")
    .replace(/&#231;/g, "ç")
    .replace(/&#246;/g, "ö")
    .replace(/&#252;/g, "ü")
    .replace(/&amp;/g, "&")
    .replace(/&nbsp;/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

/** html'deki ilk (en ucuz) odanin adi ve pansiyon tipini cikarir. */
function firstRoom(html: string): { roomName: string | null; boardType: string | null } {
  if (!html) return { roomName: null, boardType: null };
  const nameM = html.match(/room-title"[^>]*>\s*([^<]{2,60})</);
  const boardM = html.match(/room-concept[^>]*data-concept="([^"]*)"/);
  return {
    roomName: nameM ? decode(nameM[1]) : null,
    boardType: boardM ? decode(boardM[1].replace(/-/g, " ")) : null,
  };
}

/** Jolly yanitini ortak NormalizedPrice formatina cevirir (lead/en ucuz teklif). */
export function normalizeJolly(data: JollyReservationResponse): NormalizedPrice {
  const a = data.analyticsDataModel || {};
  const listPrice = a.totalPrice ?? null;
  const price = a.discountedPrice ?? a.totalPrice ?? null;
  const discountRate =
    listPrice && price && listPrice > price
      ? Math.round((1 - price / listPrice) * 100)
      : 0;

  const { roomName, boardType } = firstRoom(data.html || "");
  const available = !!data.isAvailableHotel && price != null && price > 0;

  return {
    available,
    hotelName: a.productName ?? null,
    city: a.cityName ?? null,
    roomName,
    boardType,
    currency: a.currency === "TRY" ? "TL" : a.currency ?? null,
    listPrice,
    price,
    discountRate,
  };
}
