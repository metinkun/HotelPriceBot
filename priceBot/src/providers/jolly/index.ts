import {
  BaseProvider,
  ProviderCapabilities,
  ProviderMapping,
  ProviderMappingInput,
  ProviderPriceResult,
  ResolvedHotel,
} from "../base";
import { PriceQuery } from "../../types";
import { fetchHotelReservation, resolveHotelFromUrl } from "./api";
import { normalizeJolly } from "./normalizer";

export class JollyProvider extends BaseProvider {
  readonly name = "jolly";
  readonly capabilities: ProviderCapabilities = {
    supportsPackages: false,
    supportsBulkPrices: false,
    requiresCookie: false,
    requiresPuppeteer: false,
  };

  async getPrice(mapping: ProviderMapping, query: PriceQuery): Promise<ProviderPriceResult> {
    const hotelType = (mapping.metadata as any)?.hotelType;
    if (!hotelType) {
      throw new Error("Mapping metadata'da hotelType eksik (ornek: \"Cyprus\", \"Domestic\")");
    }

    const data = await fetchHotelReservation(
      mapping.providerHotelId,
      hotelType,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.childAges ?? []
    );

    const price = normalizeJolly(data);

    return {
      internalHotelId: mapping.internalHotelId,
      providerHotelId: mapping.providerHotelId,
      hotelName: mapping.hotelName,
      found: price.available,
      price,
    };
  }

  validateMappingInput(input: ProviderMappingInput): string | null {
    if (!input.metadata?.hotelType) {
      return "metadata.hotelType zorunlu (ornek: \"Cyprus\", \"Domestic\")";
    }
    return null;
  }

  requiredMetadataFields(): string[] {
    return ["hotelType"];
  }

  async resolveFromUrl(url: string): Promise<ResolvedHotel> {
    if (!/jollytur\.com/i.test(url)) {
      throw new Error("Gecersiz Jolly URL'i (jollytur.com bekleniyor)");
    }
    const r = await resolveHotelFromUrl(url);
    if (!r.hotelType) {
      throw new Error("Jolly hotelType cozulemedi (otel detay sayfasi URL'i olmali)");
    }
    return {
      providerHotelId: r.providerHotelId,
      hotelName: r.hotelName,
      metadata: {
        hotelType: r.hotelType,
        sourceUrl: url,
      },
    };
  }
}
