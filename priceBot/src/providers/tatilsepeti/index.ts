import {
  BaseProvider,
  ProviderCapabilities,
  ProviderMapping,
  ProviderMappingInput,
  ProviderPriceResult,
  ProviderPackageResult,
} from "../base";
import { PriceQuery, BulkPriceQuery, PackageQuery } from "../../types";
import { fetchPricesFromTatilSepeti, fetchFlyingPackages } from "./api";
import { normalizeTatilSepetiResponse, parsePackageRoomList } from "./normalizer";

function calculateNights(checkIn: string, checkOut: string): number {
  const d1 = new Date(checkIn);
  const d2 = new Date(checkOut);
  return Math.round((d2.getTime() - d1.getTime()) / (1000 * 60 * 60 * 24));
}

export class TatilSepetiProvider extends BaseProvider {
  readonly name = "tatilsepeti";
  readonly capabilities: ProviderCapabilities = {
    supportsPackages: true,
    supportsBulkPrices: true,
    requiresCookie: true,
    requiresPuppeteer: false,
  };

  async getPrice(mapping: ProviderMapping, query: PriceQuery): Promise<ProviderPriceResult> {
    const data = await fetchPricesFromTatilSepeti(
      [mapping.providerHotelId],
      query.checkIn,
      query.checkOut,
      query.adults,
      query.children
    );

    return {
      internalHotelId: mapping.internalHotelId,
      providerHotelId: mapping.providerHotelId,
      hotelName: mapping.hotelName,
      found: !!data,
      price: normalizeTatilSepetiResponse(data, mapping.providerHotelId),
    };
  }

  /** TatilSepeti comma-separated HotelIds ile native bulk destekler. */
  async getBulkPrices(mappings: ProviderMapping[], query: BulkPriceQuery): Promise<ProviderPriceResult[]> {
    const hotelIds = mappings.map((m) => m.providerHotelId);

    const data = await fetchPricesFromTatilSepeti(
      hotelIds,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.children
    );

    return mappings.map((m) => ({
      internalHotelId: m.internalHotelId,
      providerHotelId: m.providerHotelId,
      hotelName: m.hotelName,
      found: !!data,
      price: normalizeTatilSepetiResponse(data, m.providerHotelId),
    }));
  }

  async getPackagePrices(mapping: ProviderMapping, query: PackageQuery): Promise<ProviderPackageResult> {
    const destinationCode = (mapping.metadata as any)?.destinationCode;
    if (!destinationCode) {
      throw new Error(
        "Mapping metadata'da destinationCode eksik (ornek: \"ecnc\")"
      );
    }

    const nightCount = calculateNights(query.checkIn, query.checkOut);

    const raw = await fetchFlyingPackages(
      mapping.providerHotelId,
      query.airportCode,
      destinationCode,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.childAges ?? []
    );

    const rooms = parsePackageRoomList(raw.roomList, nightCount);

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

  validateMappingInput(_input: ProviderMappingInput): string | null {
    // destinationCode sadece paket sorgusu icin gerekli,
    // fiyat sorgusu icin metadata gerekmez
    return null;
  }
}
