import {
  BaseProvider,
  ProviderCapabilities,
  ProviderMapping,
  ProviderMappingInput,
  ProviderPriceResult,
  ResolvedHotel,
} from "../base";
import { PriceQuery, PackageQuery } from "../../types";
import { fetchRoomPrice, fetchPackagePrice, resolveHotelFromUrl } from "./api";
import { normalizeTatilbudur, normalizeTatilbudurPackages } from "./normalizer";
import { ProviderPackageResult } from "../base";

export class TatilbudurProvider extends BaseProvider {
  readonly name = "tatilbudur";
  readonly capabilities: ProviderCapabilities = {
    supportsPackages: true,
    supportsBulkPrices: false,
    requiresCookie: false,
    requiresPuppeteer: false,
  };

  async getPrice(mapping: ProviderMapping, query: PriceQuery): Promise<ProviderPriceResult> {
    const sourceUrl = (mapping.metadata as any)?.sourceUrl;
    if (!sourceUrl) {
      throw new Error("Mapping metadata'da sourceUrl eksik (otel detay URL'i)");
    }

    const data = await fetchRoomPrice(
      sourceUrl,
      mapping.providerHotelId,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.children
    );

    const price = normalizeTatilbudur(data);
    price.hotelName = mapping.hotelName; // otel adini mapping'den doldur

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
    const sourceUrl = (mapping.metadata as any)?.sourceUrl;
    if (!sourceUrl) {
      throw new Error("Mapping metadata'da sourceUrl eksik (otel detay URL'i)");
    }

    const { data } = await fetchPackagePrice(
      sourceUrl,
      mapping.providerHotelId,
      query.airportCode,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.children
    );

    const rooms = normalizeTatilbudurPackages(data);

    return {
      internalHotelId: mapping.internalHotelId,
      providerHotelId: mapping.providerHotelId,
      hotelName: mapping.hotelName,
      airportCode: query.airportCode,
      checkIn: query.checkIn,
      checkOut: query.checkOut,
      available: rooms.length > 0,
      roomCount: rooms.length,
      rooms,
    };
  }

  validateMappingInput(input: ProviderMappingInput): string | null {
    if (!input.metadata?.sourceUrl) {
      return "metadata.sourceUrl zorunlu (otel detay URL'i)";
    }
    return null;
  }

  requiredMetadataFields(): string[] {
    return ["sourceUrl"];
  }

  async resolveFromUrl(url: string): Promise<ResolvedHotel> {
    if (!/tatilbudur\.com/i.test(url)) {
      throw new Error("Gecersiz Tatilbudur URL'i (tatilbudur.com bekleniyor)");
    }
    const r = await resolveHotelFromUrl(url);
    return {
      providerHotelId: r.providerHotelId,
      hotelName: r.hotelName,
      metadata: { sourceUrl: url },
    };
  }
}
