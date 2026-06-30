import { PrismaClient } from "@prisma/client";
import { getProvider, getAllProviders } from "../providers";
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

/**
 * Bir otelin TUM provider'larindan (etstur + tatilsepeti...) paralel fiyat ceker.
 * providerNames verilirse sadece onlar; bos ise otelin eslesen tum provider'lari.
 */
export async function getMultiProviderPrices(
  query: PriceQuery,
  providerNames?: string[]
) {
  const where: any = {
    internalHotelId: query.hotelId,
    isActive: true,
  };
  if (providerNames?.length) {
    where.provider = { in: providerNames };
  }

  const mappings = await prisma.hotelMapping.findMany({ where });

  if (mappings.length === 0) {
    throw new Error(`Otel eslestirmesi bulunamadi: ${query.hotelId}`);
  }

  const settled = await Promise.allSettled(
    mappings.map(async (mapping) => {
      const provider = getProvider(mapping.provider);
      const start = Date.now();
      const result = await provider.getPrice(
        mapping as unknown as ProviderMapping,
        query
      );
      return {
        provider: mapping.provider,
        ...result,
        durationMs: Date.now() - start,
      };
    })
  );

  const results: any[] = [];
  const errors: Array<{ provider: string; error: string }> = [];
  settled.forEach((r, i) => {
    if (r.status === "fulfilled") {
      results.push(r.value);
    } else {
      errors.push({
        provider: mappings[i].provider,
        error: r.reason?.message || "Bilinmeyen hata",
      });
    }
  });

  return {
    hotelId: query.hotelId,
    checkIn: query.checkIn,
    checkOut: query.checkOut,
    results,
    errors: errors.length ? errors : undefined,
  };
}
