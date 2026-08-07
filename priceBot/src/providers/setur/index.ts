import {
  BaseProvider,
  ProviderCapabilities,
  ProviderMapping,
  ProviderMappingInput,
  ProviderPriceResult,
  ResolvedHotel,
} from "../base";
import { PriceQuery, PackageQuery } from "../../types";
import {
  fetchHotelPage,
  fetchPackagePage,
  resolveHotelFromUrl,
  slugFromUrl,
} from "./api";
import { normalizeSetur } from "./normalizer";
import { ProviderPackageResult } from "../base";

export class SeturProvider extends BaseProvider {
  readonly name = "setur";
  readonly capabilities: ProviderCapabilities = {
    supportsPackages: true,
    supportsBulkPrices: false,
    requiresCookie: false,
    requiresPuppeteer: false,
  };

  async getPrice(
    mapping: ProviderMapping,
    query: PriceQuery
  ): Promise<ProviderPriceResult> {
    const slug =
      (mapping.metadata as any)?.slug ||
      slugFromUrl((mapping.metadata as any)?.sourceUrl || "");
    if (!slug) {
      throw new Error("Mapping metadata'da slug/sourceUrl eksik");
    }

    const data = await fetchHotelPage(
      slug,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.childAges ?? []
    );

    const price = normalizeSetur(data);
    if (!price.hotelName) price.hotelName = mapping.hotelName;

    return {
      internalHotelId: mapping.internalHotelId,
      providerHotelId: mapping.providerHotelId,
      hotelName: mapping.hotelName,
      found: price.available,
      price,
    };
  }

  async getPackagePrices(
    mapping: ProviderMapping,
    query: PackageQuery
  ): Promise<ProviderPackageResult> {
    const meta = (mapping.metadata as any) || {};
    const slug = meta.slug || slugFromUrl(meta.sourceUrl || "");
    if (!slug) throw new Error("Mapping metadata'da slug/sourceUrl eksik");

    const data = await fetchPackagePage(
      slug,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.childAges ?? [],
      query.airportCode,
      meta.arrivalCode // opsiyonel: otelin varis havalimani (ECN gibi)
    );

    const p = normalizeSetur(data);
    const rooms = p.available
      ? [
          {
            roomId: "lowest",
            roomName: "En uygun paket",
            roomSize: null,
            nightCount: data.nightCount,
            boards: [
              {
                boardType: p.boardType,
                currency: p.currency,
                listPrice: p.listPrice,
                price: p.price,
                discountRate: p.discountRate,
                cancellation: null,
              },
            ],
          },
        ]
      : [];

    return {
      internalHotelId: mapping.internalHotelId,
      providerHotelId: mapping.providerHotelId,
      hotelName: mapping.hotelName ?? p.hotelName,
      airportCode: query.airportCode,
      checkIn: query.checkIn,
      checkOut: query.checkOut,
      available: rooms.length > 0,
      roomCount: rooms.length,
      rooms,
      note: "Setur paket sayfasi yalnizca en uygun (lowest) paket fiyatini veriyor; oda bazli liste sunmuyor.",
    };
  }

  validateMappingInput(input: ProviderMappingInput): string | null {
    if (!input.metadata?.slug && !input.metadata?.sourceUrl) {
      return "metadata.slug veya metadata.sourceUrl zorunlu";
    }
    return null;
  }

  requiredMetadataFields(): string[] {
    return ["slug"];
  }

  async resolveFromUrl(url: string): Promise<ResolvedHotel> {
    if (!/setur\.com\.tr/i.test(url)) {
      throw new Error("Gecersiz Setur URL'i (setur.com.tr bekleniyor)");
    }
    const r = await resolveHotelFromUrl(url);
    return {
      providerHotelId: r.providerHotelId,
      hotelName: r.hotelName,
      metadata: { slug: r.slug, sourceUrl: url, hasPackage: r.hasPackage },
    };
  }
}
