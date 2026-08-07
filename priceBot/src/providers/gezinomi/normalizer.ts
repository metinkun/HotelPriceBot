import { NormalizedPrice, NormalizedPackageRoom } from "../../types";
import { GezinomiPriceResponse } from "./api";

/** En ucuz (lead) teklifi ortak NormalizedPrice formatina cevirir. */
export function normalizeGezinomi(
  data: GezinomiPriceResponse,
  hotelName: string | null
): NormalizedPrice {
  const rooms = data?.data?.roomDetails ?? [];

  let best: {
    price: number;
    listPrice: number | null;
    discountRate: number;
    roomName: string | null;
    boardType: string | null;
  } | null = null;

  for (const room of rooms) {
    for (const rd of room.reservationDetails ?? []) {
      const price = rd.totalDiscountedPrice ?? rd.totalPrice;
      if (!price || price <= 0) continue;
      if (!best || price < best.price) {
        best = {
          price,
          listPrice: rd.totalPrice ?? null,
          discountRate: rd.discountRate ?? 0,
          roomName: rd.hotelRoomName ?? room.roomName ?? null,
          boardType: rd.conceptName ?? null,
        };
      }
    }
  }

  if (!best) {
    return {
      available: false,
      hotelName,
      city: null,
      roomName: null,
      boardType: null,
      currency: null,
      listPrice: null,
      price: null,
      discountRate: 0,
    };
  }

  return {
    available: true,
    hotelName,
    city: null,
    roomName: best.roomName,
    boardType: best.boardType,
    currency: "TL",
    listPrice: best.listPrice,
    price: best.price,
    discountRate: best.discountRate,
  };
}

/**
 * Paket (otel+ucak) tekliflerini NormalizedPackageRoom listesine cevirir.
 * Ayni yanitta `flightPackageTotal*` alanlari doludur.
 */
export function normalizeGezinomiPackages(
  data: GezinomiPriceResponse,
  nightCount: number | null
): NormalizedPackageRoom[] {
  const rooms = data?.data?.roomDetails ?? [];
  const byRoom = new Map<string, NormalizedPackageRoom>();

  for (const room of rooms) {
    for (const rd of room.reservationDetails ?? []) {
      const price =
        rd.flightPackageTotalDiscountedPrice || rd.flightPackageTotalPrice || 0;
      if (!price || price <= 0) continue; // paket yoksa atla

      const name = rd.hotelRoomName ?? room.roomName ?? "Oda";
      const key = String(room.roomId ?? name);
      if (!byRoom.has(key)) {
        byRoom.set(key, {
          roomId: key,
          roomName: name,
          roomSize: null,
          nightCount,
          boards: [],
        });
      }
      byRoom.get(key)!.boards.push({
        boardType: rd.conceptName ?? null,
        currency: "TL",
        listPrice: rd.flightPackageTotalPrice ?? null,
        price,
        discountRate: rd.flightPackageDiscountRate ?? 0,
        cancellation: null,
      });
    }
  }

  return Array.from(byRoom.values());
}
