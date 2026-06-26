import { PrismaClient } from "@prisma/client";
import { fetchPricesFromTatilSepeti } from "./tatilSepetiService";
import { PriceQuery, BulkPriceQuery } from "../types";

const prisma = new PrismaClient();

export async function getPrice(query: PriceQuery) {
  const mapping = await prisma.hotelMapping.findUnique({
    where: { internalHotelId: query.hotelId },
  });

  if (!mapping) {
    throw new Error(`Otel eslestirmesi bulunamadi: ${query.hotelId}`);
  }

  if (!mapping.isActive) {
    throw new Error(`Otel eslestirmesi pasif durumda: ${query.hotelId}`);
  }

  const data = await fetchPricesFromTatilSepeti(
    [mapping.tatilsepetiHotelId],
    query.checkIn,
    query.checkOut,
    query.adults,
    query.children
  );

  return {
    internalHotelId: query.hotelId,
    tatilsepetiHotelId: mapping.tatilsepetiHotelId,
    hotelName: mapping.hotelName,
    prices: data,
  };
}

export async function getBulkPrices(query: BulkPriceQuery) {
  const mappings = await prisma.hotelMapping.findMany({
    where: {
      internalHotelId: { in: query.hotelIds },
      isActive: true,
    },
  });

  if (mappings.length === 0) {
    throw new Error("Hicbir otel eslestirmesi bulunamadi");
  }

  const tatilsepetiIds = mappings.map((m) => m.tatilsepetiHotelId);

  const data = await fetchPricesFromTatilSepeti(
    tatilsepetiIds,
    query.checkIn,
    query.checkOut,
    query.adults,
    query.children
  );

  const idMap = new Map(
    mappings.map((m) => [m.tatilsepetiHotelId, m])
  );

  return {
    mappings: mappings.map((m) => ({
      internalHotelId: m.internalHotelId,
      tatilsepetiHotelId: m.tatilsepetiHotelId,
      hotelName: m.hotelName,
    })),
    prices: data,
    notFound: query.hotelIds.filter(
      (id) => !mappings.find((m) => m.internalHotelId === id)
    ),
  };
}
