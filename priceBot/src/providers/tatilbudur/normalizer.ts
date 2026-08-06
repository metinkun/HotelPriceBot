import { NormalizedPrice, NormalizedPackageRoom } from "../../types";
import { TatilbudurRoomPriceResponse } from "./api";

function decode(s: string): string {
  return s
    .replace(/&amp;/g, "&")
    .replace(/&#231;/g, "ç")
    .replace(/&nbsp;/g, " ")
    .replace(/-/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

/** view HTML'indeki ilk (en uygun) rezervasyon butonundan attribute cikarir. */
function attr(block: string, name: string): string | null {
  const m = block.match(new RegExp(`${name}="([^"]*)"`, "i"));
  return m ? m[1] : null;
}

/**
 * Tatilbudur `calculate-room-price` yanitini ortak NormalizedPrice'a cevirir.
 * Fiyat, ilk `v2-reservation-button` <a>'sinin temiz attribute'larindan alinir.
 */
export function normalizeTatilbudur(
  data: TatilbudurRoomPriceResponse
): NormalizedPrice {
  const view = data?.view || "";
  const i = view.indexOf("v2-reservation-button");
  if (i === -1) {
    return {
      available: false,
      hotelName: null,
      city: null,
      roomName: null,
      boardType: null,
      currency: null,
      listPrice: null,
      price: null,
      discountRate: 0,
    };
  }
  // Buton <a ...> blogunu al (attribute'lar acilis etiketinde)
  const block = view.slice(i, view.indexOf(">", i) + 1);

  const amount = Number(attr(block, "amount"));
  const poster = Number(attr(block, "poster_price"));
  const discount = Number(attr(block, "discountRate")) || 0;
  const currencyRaw = attr(block, "currency"); // TRY
  const roomName = attr(block, "roomName");
  const board = attr(block, "conceptName");

  const price = Number.isFinite(amount) && amount > 0 ? amount : null;
  const listPrice = Number.isFinite(poster) && poster > 0 ? poster : null;

  return {
    available: price != null,
    hotelName: null, // provider mapping.hotelName ile doldurur
    city: null,
    roomName: roomName ? decode(roomName) : null,
    boardType: board ? decode(board) : null,
    currency: currencyRaw === "TRY" ? "TL" : currencyRaw,
    listPrice,
    price,
    discountRate: discount,
  };
}

/**
 * Paket yanitini (calculate-package-room-price) NormalizedPackageRoom listesine cevirir.
 * Paket kartlari `packageType="P"` ile isaretlidir; ayni oda birden fazla
 * pansiyon secenegiyle gelebilir -> oda bazinda gruplanir.
 */
export function normalizeTatilbudurPackages(
  data: TatilbudurRoomPriceResponse
): NormalizedPackageRoom[] {
  const view = data?.view || "";
  if (!view) return [];

  const buttons = view.match(/<a[^>]*v2-reservation-button[^>]*>/g) || [];
  const byRoom = new Map<string, NormalizedPackageRoom>();

  for (const block of buttons) {
    if (attr(block, "packageType") !== "P") continue; // sadece paket kartlari

    const amount = Number(attr(block, "amount"));
    if (!Number.isFinite(amount) || amount <= 0) continue;

    const poster = Number(attr(block, "poster_price"));
    const roomId = attr(block, "roomId") || attr(block, "pricingId") || "";
    const roomNameRaw = attr(block, "roomName");
    const roomName = roomNameRaw ? decode(roomNameRaw) : "Oda";
    const boardRaw = attr(block, "conceptName");
    const nights = Number(attr(block, "night"));
    const currencyRaw = attr(block, "currency");

    const key = roomId || roomName;
    if (!byRoom.has(key)) {
      byRoom.set(key, {
        roomId: roomId || key,
        roomName,
        roomSize: null,
        nightCount: Number.isFinite(nights) ? nights : null,
        boards: [],
      });
    }

    const room = byRoom.get(key)!;
    const boardType = boardRaw ? decode(boardRaw) : null;
    // Ayni oda+pansiyon tekrarlanabiliyor (mobil/masaustu kartlari) -> tekille
    if (room.boards.some((b) => b.boardType === boardType && b.price === amount)) {
      continue;
    }
    room.boards.push({
      boardType,
      currency: currencyRaw === "TRY" ? "TL" : currencyRaw,
      listPrice: Number.isFinite(poster) && poster > 0 ? poster : null,
      price: amount,
      discountRate: Number(attr(block, "discountRate")) || 0,
      cancellation: null,
    });
  }

  return Array.from(byRoom.values());
}
