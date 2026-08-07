import {
  BaseProvider,
  ProviderCapabilities,
  ProviderMapping,
  ProviderMappingInput,
  ProviderPriceResult,
  ProviderPackageResult,
  ResolvedHotel,
} from "../base";
import { PriceQuery, PackageQuery } from "../../types";
import { fetchRoomPrices, resolveHotelFromUrl } from "./api";
import { normalizeGezinomi, normalizeGezinomiPackages } from "./normalizer";

function nightsBetween(checkIn: string, checkOut: string): number | null {
  const a = new Date(checkIn).getTime();
  const b = new Date(checkOut).getTime();
  if (!Number.isFinite(a) || !Number.isFinite(b)) return null;
  return Math.round((b - a) / 86400000);
}

export class GezinomiProvider extends BaseProvider {
  readonly name = "gezinomi";
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
    const data = await fetchRoomPrices(
      mapping.providerHotelId,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.childAges ?? [],
      false
    );

    const price = normalizeGezinomi(data, mapping.hotelName);

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
    const data = await fetchRoomPrices(
      mapping.providerHotelId,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.childAges ?? [],
      true // ucak dahil
    );

    const rooms = normalizeGezinomiPackages(
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
      note:
        "Gezinomi paket fiyatlari otelin varsayilan ucus secenegiyle doner; " +
        "kalkis havalimani (airportCode) sorguya dahil edilmiyor.",
    };
  }

  validateMappingInput(_input: ProviderMappingInput): string | null {
    return null; // providerHotelId yeterli
  }

  async resolveFromUrl(url: string): Promise<ResolvedHotel> {
    if (!/gezinomi\.com/i.test(url)) {
      throw new Error("Gecersiz Gezinomi URL'i (gezinomi.com bekleniyor)");
    }
    const r = await resolveHotelFromUrl(url);
    return {
      providerHotelId: r.providerHotelId,
      hotelName: r.hotelName,
      metadata: { sourceUrl: url },
    };
  }
}
