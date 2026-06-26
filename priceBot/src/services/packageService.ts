import { PrismaClient } from "@prisma/client";
import { getProvider, getAllProviders } from "../providers";
import { ProviderMapping, ProviderPackageResult } from "../providers/base";
import { PackageQuery } from "../types";

const prisma = new PrismaClient();

export async function getPackagePrices(providerName: string, query: PackageQuery) {
  const provider = getProvider(providerName);

  if (!provider.capabilities.supportsPackages || !provider.getPackagePrices) {
    throw new Error(`Provider '${providerName}' paket sorgusunu desteklemiyor`);
  }

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
  const result = await provider.getPackagePrices(mapping as unknown as ProviderMapping, query);
  result.durationMs = Date.now() - start;
  return result;
}

/**
 * Birden fazla provider'dan paket fiyatlarini paralel sorgular.
 * providerNames bos veya undefined ise paket destekleyen tum provider'lar sorgulanir.
 */
export async function getMultiProviderPackagePrices(
  providerNames: string[] | undefined,
  query: PackageQuery
) {
  // Hangi provider'lar sorgulanacak?
  const targetProviders = providerNames?.length
    ? providerNames.map((n) => getProvider(n))
    : getAllProviders();

  // Paket desteklemeyen provider'lari ayir
  const notSupported = targetProviders
    .filter((p) => !p.capabilities.supportsPackages || !p.getPackagePrices)
    .map((p) => p.name);

  // Paket destekleyen provider'lari filtrele
  const packageProviders = targetProviders.filter(
    (p) => p.capabilities.supportsPackages && p.getPackagePrices
  );

  if (packageProviders.length === 0) {
    return {
      hotelId: query.hotelId,
      results: [],
      notSupported: notSupported.length > 0 ? notSupported : undefined,
    };
  }

  // Her provider icin mapping'i bul
  const mappings = await prisma.hotelMapping.findMany({
    where: {
      provider: { in: packageProviders.map((p) => p.name) },
      internalHotelId: query.hotelId,
      isActive: true,
    },
  });

  // Paralel sorgula
  const results = await Promise.allSettled(
    mappings.map(async (mapping) => {
      const provider = getProvider(mapping.provider);
      if (!provider.getPackagePrices) return null;
      const start = Date.now();
      const result = await provider.getPackagePrices(
        mapping as unknown as ProviderMapping,
        query
      );
      return { provider: mapping.provider, ...result, durationMs: Date.now() - start };
    })
  );

  const fulfilled: Array<{ provider: string } & ProviderPackageResult> = [];
  const errors: Array<{ provider: string; error: string }> = [];

  for (const r of results) {
    if (r.status === "fulfilled" && r.value) {
      fulfilled.push(r.value);
    } else if (r.status === "rejected") {
      errors.push({
        provider: "unknown",
        error: r.reason?.message || "Bilinmeyen hata",
      });
    }
  }

  // Mapping'i olmayan provider'lar
  const queriedProviders = new Set(mappings.map((m) => m.provider));
  const notMapped = packageProviders
    .filter((p) => !queriedProviders.has(p.name))
    .map((p) => p.name);

  return {
    hotelId: query.hotelId,
    results: fulfilled,
    errors: errors.length > 0 ? errors : undefined,
    notMapped: notMapped.length > 0 ? notMapped : undefined,
    notSupported: notSupported.length > 0 ? notSupported : undefined,
  };
}
