import {
  BaseProvider,
  ProviderCapabilities,
  ProviderMapping,
  ProviderMappingInput,
  ProviderPriceResult,
  ResolvedHotel,
} from "../base";
import { PriceQuery } from "../../types";
import { fetchRoomPrices, resolveHotelFromUrl } from "./api";
import { normalizeTouristica } from "./normalizer";

export class TouristicaProvider extends BaseProvider {
  readonly name = "touristica";
  readonly capabilities: ProviderCapabilities = {
    supportsPackages: false, // paket verisi var ama ayri akis; ileride eklenebilir
    supportsBulkPrices: false,
    requiresCookie: false,
    requiresPuppeteer: true, // Cloudflare challenge -> headless tarayici sart
  };

  async getPrice(
    mapping: ProviderMapping,
    query: PriceQuery
  ): Promise<ProviderPriceResult> {
    const sourceUrl = (mapping.metadata as any)?.sourceUrl;
    if (!sourceUrl) {
      throw new Error("Mapping metadata'da sourceUrl eksik (otel detay URL'i)");
    }

    const raw = await fetchRoomPrices(
      sourceUrl,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.children
    );

    const price = normalizeTouristica(raw);
    if (!price.hotelName) price.hotelName = mapping.hotelName;

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
    if (!/touristica\.com\.tr/i.test(url)) {
      throw new Error(
        "Gecersiz Touristica URL'i (touristica.com.tr bekleniyor)"
      );
    }
    const r = await resolveHotelFromUrl(url);
    return {
      providerHotelId: r.providerHotelId,
      hotelName: r.hotelName,
      metadata: { sourceUrl: url, slug: r.slug, city: r.city },
    };
  }
}
