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
  fetchHotelReservation,
  resolveHotelFromUrl,
  lookupAirport,
  PACKAGE_TYPE_FLIGHT_TRANSFER,
} from "./api";
import { normalizeJolly, normalizeJollyPackages } from "./normalizer";
import { ProviderPackageResult } from "../base";

function nightsBetween(checkIn: string, checkOut: string): number | null {
  const a = new Date(checkIn).getTime();
  const b = new Date(checkOut).getTime();
  if (!Number.isFinite(a) || !Number.isFinite(b)) return null;
  return Math.round((b - a) / 86400000);
}

export class JollyProvider extends BaseProvider {
  readonly name = "jolly";
  readonly capabilities: ProviderCapabilities = {
    supportsPackages: true,
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

  async getPackagePrices(
    mapping: ProviderMapping,
    query: PackageQuery
  ): Promise<ProviderPackageResult> {
    const hotelType = (mapping.metadata as any)?.hotelType;
    if (!hotelType) {
      throw new Error("Mapping metadata'da hotelType eksik");
    }

    // IATA kodu -> Jolly ic havalimani id'si
    const origin = await lookupAirport(query.airportCode);

    const data = await fetchHotelReservation(
      mapping.providerHotelId,
      hotelType,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.childAges ?? [],
      { origin, packageSearchType: PACKAGE_TYPE_FLIGHT_TRANSFER }
    );

    const rooms = normalizeJollyPackages(
      data,
      nightsBetween(query.checkIn, query.checkOut)
    );

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
