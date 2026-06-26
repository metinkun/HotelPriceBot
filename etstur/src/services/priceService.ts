import { PrismaClient } from "@prisma/client";
import {
  collectHotelsByIds,
  normalizeHotel,
  DateOccupancy,
} from "./etsturService";
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

  const occ: DateOccupancy = {
    checkIn: query.checkIn,
    checkOut: query.checkOut,
    adults: query.adults,
    children: query.children,
    childAges: query.childAges ?? [],
  };

  const found = await collectHotelsByIds(
    mapping.destinationUrl,
    [mapping.etsturHotelId],
    occ
  );
  const hotel = found.get(mapping.etsturHotelId);

  return {
    internalHotelId: mapping.internalHotelId,
    etsturHotelId: mapping.etsturHotelId,
    hotelName: mapping.hotelName,
    found: !!hotel,
    price: hotel ? normalizeHotel(hotel) : null,
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

  const occ: DateOccupancy = {
    checkIn: query.checkIn,
    checkOut: query.checkOut,
    adults: query.adults,
    children: query.children,
    childAges: query.childAges ?? [],
  };

  // Ayni destinasyondaki otelleri tek aramada bulmak icin grupla.
  const byDestination = new Map<string, typeof mappings>();
  for (const m of mappings) {
    const arr = byDestination.get(m.destinationUrl) ?? [];
    arr.push(m);
    byDestination.set(m.destinationUrl, arr);
  }

  const results: any[] = [];

  await Promise.all(
    Array.from(byDestination.entries()).map(async ([destinationUrl, group]) => {
      try {
        const found = await collectHotelsByIds(
          destinationUrl,
          group.map((m) => m.etsturHotelId),
          occ
        );
        for (const m of group) {
          const hotel = found.get(m.etsturHotelId);
          results.push({
            internalHotelId: m.internalHotelId,
            etsturHotelId: m.etsturHotelId,
            hotelName: m.hotelName,
            found: !!hotel,
            price: hotel ? normalizeHotel(hotel) : null,
          });
        }
      } catch (err: any) {
        for (const m of group) {
          results.push({
            internalHotelId: m.internalHotelId,
            etsturHotelId: m.etsturHotelId,
            hotelName: m.hotelName,
            found: false,
            error: err.message,
            price: null,
          });
        }
      }
    })
  );

  return {
    results,
    notFound: query.hotelIds.filter(
      (id) => !mappings.find((m) => m.internalHotelId === id)
    ),
  };
}
