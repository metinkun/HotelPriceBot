import { PrismaClient } from "@prisma/client";
import {
  fetchRoomSearch,
  fetchPackage,
  normalizePackageRoom,
  DateOccupancy,
} from "./etsturService";
import { PackageQuery } from "../types";

const prisma = new PrismaClient();

/**
 * Otel + Ucak + Transfer paket fiyatlarini doner.
 *
 * Akis: internalHotelId -> etsturHotelId (mapping) -> /room (roomSearchId + oturum)
 * -> /room/package (airportCode) -> normalize.
 */
export async function getPackagePrices(query: PackageQuery) {
  const mapping = await prisma.hotelMapping.findUnique({
    where: { internalHotelId: query.hotelId },
  });

  if (!mapping) {
    throw new Error(`Otel eslestirmesi bulunamadi: ${query.hotelId}`);
  }
  if (!mapping.isActive) {
    throw new Error(`Otel eslestirmesi pasif durumda: ${query.hotelId}`);
  }

  const occ: DateOccupancy = {
    checkIn: query.checkIn,
    checkOut: query.checkOut,
    adults: query.adults,
    children: query.children,
    childAges: query.childAges ?? [],
  };

  // 1) Oda aramasi -> roomSearchId + oturum cookie'si
  const { roomSearchId, cookie } = await fetchRoomSearch(
    mapping.etsturHotelId,
    occ
  );

  // 2) Paket fiyatlari (ayni oturum)
  const pkg = await fetchPackage(roomSearchId, query.airportCode, cookie);
  const rooms = pkg.result?.rooms ?? [];

  return {
    internalHotelId: mapping.internalHotelId,
    etsturHotelId: mapping.etsturHotelId,
    hotelName: mapping.hotelName,
    airportCode: query.airportCode,
    checkIn: query.checkIn,
    checkOut: query.checkOut,
    available: rooms.length > 0,
    roomCount: rooms.length,
    rooms: rooms.map(normalizePackageRoom),
  };
}
