import { PrismaClient } from "@prisma/client";
import { getProvider } from "../providers";
import { ProviderMapping } from "../providers/base";
import { PriceQuery, BulkPriceQuery } from "../types";

const prisma = new PrismaClient();

export async function getPrice(providerName: string, query: PriceQuery) {
  const provider = getProvider(providerName);

  const mapping = await prisma.hotelMapping.findUnique({
    where: {
      provider_internalHotelId: {
        provider: providerName,
        internalHotelId: query.hotelId,
      },
    },
  });

  if (!mapping) {
    throw new Error(`Otel eslestirmesi bulunamadi: ${query.hotelId} (${providerName})`);
  }
  if (!mapping.isActive) {
    throw new Error(`Otel eslestirmesi pasif durumda: ${query.hotelId}`);
  }

  const start = Date.now();
  const result = await provider.getPrice(mapping as unknown as ProviderMapping, query);
  result.durationMs = Date.now() - start;
  return result;
}

export async function getBulkPrices(providerName: string, query: BulkPriceQuery) {
  const provider = getProvider(providerName);

  const mappings = await prisma.hotelMapping.findMany({
    where: {
      provider: providerName,
      internalHotelId: { in: query.hotelIds },
      isActive: true,
    },
  });

  if (mappings.length === 0) {
    throw new Error("Hicbir otel eslestirmesi bulunamadi");
  }

  const start = Date.now();
  const results = await provider.getBulkPrices(
    mappings as unknown as ProviderMapping[],
    query
  );
  const durationMs = Date.now() - start;

  return {
    provider: providerName,
    durationMs,
    results,
    notFound: query.hotelIds.filter(
      (id) => !mappings.find((m) => m.internalHotelId === id)
    ),
  };
}
