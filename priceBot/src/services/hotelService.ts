import { randomUUID } from "crypto";
import { PrismaClient } from "@prisma/client";
import { getProvider, getAllProviderNames } from "../providers";

const prisma = new PrismaClient();

/** Otomatik internalHotelId uretir: htl_<8 hex> */
function generateHotelId(): string {
  return "htl_" + randomUUID().replace(/-/g, "").slice(0, 8);
}

export interface AddHotelInput {
  urls: Record<string, string>; // { etstur: "...", tatilsepeti: "..." }
  internalHotelId?: string; // opsiyonel; verilmezse otomatik uretilir
  hotelName?: string; // opsiyonel ortak ad; verilmezse cozulen ad kullanilir
}

interface ProviderResolveResult {
  provider: string;
  ok: boolean;
  providerHotelId?: string;
  hotelName?: string | null;
  metadata?: Record<string, any>;
  error?: string;
}

/** Verilen URL'leri cozer ve tek internalHotelId altinda mapping olusturur/gunceller. */
export async function addHotelFromUrls(input: AddHotelInput) {
  const providerNames = Object.keys(input.urls || {}).filter(
    (k) => input.urls[k]?.trim()
  );

  if (providerNames.length === 0) {
    throw new Error(
      `En az bir site URL'i gerekli. Desteklenen provider'lar: ${getAllProviderNames().join(", ")}`
    );
  }

  // Provider adlari gecerli mi?
  for (const name of providerNames) {
    const provider = getProvider(name); // bilinmiyorsa firlatir
    if (!provider.resolveFromUrl) {
      throw new Error(`Provider '${name}' URL cozumlemeyi desteklemiyor`);
    }
  }

  const internalHotelId = input.internalHotelId?.trim() || generateHotelId();

  // Tum URL'leri paralel coz
  const resolved = await Promise.all(
    providerNames.map(async (name): Promise<ProviderResolveResult> => {
      try {
        const provider = getProvider(name);
        const r = await provider.resolveFromUrl!(input.urls[name].trim());
        return {
          provider: name,
          ok: true,
          providerHotelId: r.providerHotelId,
          hotelName: r.hotelName,
          metadata: r.metadata,
        };
      } catch (err: any) {
        return { provider: name, ok: false, error: err.message };
      }
    })
  );

  const successful = resolved.filter((r) => r.ok);
  if (successful.length === 0) {
    const err: any = new Error("Hicbir URL cozumlenemedi");
    err.details = resolved;
    throw err;
  }

  // Her basarili provider icin upsert
  for (const r of successful) {
    await prisma.hotelMapping.upsert({
      where: {
        provider_internalHotelId: {
          provider: r.provider,
          internalHotelId,
        },
      },
      create: {
        provider: r.provider,
        internalHotelId,
        providerHotelId: r.providerHotelId!,
        hotelName: input.hotelName || r.hotelName || null,
        metadata: (r.metadata ?? undefined) as any,
        isActive: true,
      },
      update: {
        providerHotelId: r.providerHotelId!,
        hotelName: input.hotelName || r.hotelName || null,
        metadata: (r.metadata ?? undefined) as any,
        isActive: true,
      },
    });
  }

  const hotel = await getHotel(internalHotelId);
  return {
    ...hotel,
    errors: resolved
      .filter((r) => !r.ok)
      .map((r) => ({ provider: r.provider, error: r.error })),
  };
}

/** Bir internalHotelId'nin tum provider eslestirmelerini gruplu doner. */
export async function getHotel(internalHotelId: string) {
  const mappings = await prisma.hotelMapping.findMany({
    where: { internalHotelId },
    orderBy: { provider: "asc" },
  });

  if (mappings.length === 0) {
    throw new Error(`Otel bulunamadi: ${internalHotelId}`);
  }

  return {
    internalHotelId,
    hotelName: mappings.find((m) => m.hotelName)?.hotelName ?? null,
    providers: mappings.map((m) => ({
      provider: m.provider,
      providerHotelId: m.providerHotelId,
      hotelName: m.hotelName,
      sourceUrl: (m.metadata as any)?.sourceUrl ?? null,
      metadata: m.metadata,
      isActive: m.isActive,
    })),
  };
}

/** Sistemdeki tum otelleri (internalHotelId bazinda gruplu) listeler. */
export async function listHotels(opts: { search?: string; active?: boolean }) {
  const where: any = {};
  if (opts.active !== undefined) where.isActive = opts.active;
  if (opts.search) {
    where.OR = [
      { hotelName: { contains: opts.search, mode: "insensitive" } },
      { internalHotelId: { contains: opts.search } },
      { providerHotelId: { contains: opts.search } },
    ];
  }

  const mappings = await prisma.hotelMapping.findMany({
    where,
    orderBy: { createdAt: "desc" },
  });

  const byHotel = new Map<string, typeof mappings>();
  for (const m of mappings) {
    const arr = byHotel.get(m.internalHotelId) ?? [];
    arr.push(m);
    byHotel.set(m.internalHotelId, arr);
  }

  return Array.from(byHotel.entries()).map(([internalHotelId, group]) => ({
    internalHotelId,
    hotelName: group.find((m) => m.hotelName)?.hotelName ?? null,
    providers: group.map((m) => ({
      provider: m.provider,
      providerHotelId: m.providerHotelId,
      sourceUrl: (m.metadata as any)?.sourceUrl ?? null,
      isActive: m.isActive,
    })),
  }));
}

/** Otelin URL(ler)ini gunceller: verilen provider'lari yeniden cozer ve upsert eder. */
export async function updateHotel(
  internalHotelId: string,
  input: { urls?: Record<string, string>; hotelName?: string }
) {
  const existing = await prisma.hotelMapping.findMany({
    where: { internalHotelId },
  });
  if (existing.length === 0) {
    throw new Error(`Otel bulunamadi: ${internalHotelId}`);
  }

  if (input.urls && Object.keys(input.urls).length > 0) {
    await addHotelFromUrls({
      urls: input.urls,
      internalHotelId,
      hotelName: input.hotelName,
    });
  } else if (input.hotelName !== undefined) {
    await prisma.hotelMapping.updateMany({
      where: { internalHotelId },
      data: { hotelName: input.hotelName },
    });
  }

  return getHotel(internalHotelId);
}

/** Otelin TUM provider eslestirmelerini siler. */
export async function deleteHotel(internalHotelId: string) {
  const result = await prisma.hotelMapping.deleteMany({
    where: { internalHotelId },
  });
  if (result.count === 0) {
    throw new Error(`Otel bulunamadi: ${internalHotelId}`);
  }
  return { internalHotelId, deletedMappings: result.count };
}
