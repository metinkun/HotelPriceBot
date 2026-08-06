import {
  BaseProvider,
  ProviderCapabilities,
  ProviderMapping,
  ProviderMappingInput,
  ProviderPriceResult,
  ResolvedHotel,
} from "../base";
import { PriceQuery } from "../../types";
import { fetchRoomPrice, resolveHotelFromUrl } from "./api";
import { normalizeTatilbudur } from "./normalizer";

export class TatilbudurProvider extends BaseProvider {
  readonly name = "tatilbudur";
  readonly capabilities: ProviderCapabilities = {
    supportsPackages: false,
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
