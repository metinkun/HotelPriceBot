import {
  BaseProvider,
  ProviderCapabilities,
  ProviderMapping,
  ProviderMappingInput,
  ProviderPriceResult,
  ProviderPackageResult,
} from "../base";
import { PriceQuery, BulkPriceQuery, PackageQuery } from "../../types";
import {
  collectHotelsByIds,
  fetchRoomSearch,
  fetchPackage,
  DateOccupancy,
} from "./api";
import { normalizeHotel, normalizePackageRoom } from "./normalizer";

function toOccupancy(query: { checkIn: string; checkOut: string; adults: number; children: number; childAges?: number[] }): DateOccupancy {
  return {
    checkIn: query.checkIn,
    checkOut: query.checkOut,
    adults: query.adults,
    children: query.children,
    childAges: query.childAges ?? [],
  };
}

export class EtsturProvider extends BaseProvider {
  readonly name = "etstur";
  readonly capabilities: ProviderCapabilities = {
    supportsPackages: true,
    supportsBulkPrices: true,
    requiresCookie: false,
    requiresPuppeteer: false,
  };

  async getPrice(mapping: ProviderMapping, query: PriceQuery): Promise<ProviderPriceResult> {
    const destinationUrl = (mapping.metadata as any)?.destinationUrl;
    if (!destinationUrl) {
      throw new Error("Mapping metadata'da destinationUrl eksik");
    }

    const occ = toOccupancy(query);
    const found = await collectHotelsByIds(
      destinationUrl,
      [mapping.providerHotelId],
      occ
    );
    const hotel = found.get(mapping.providerHotelId);

    return {
      internalHotelId: mapping.internalHotelId,
      providerHotelId: mapping.providerHotelId,
      hotelName: mapping.hotelName,
      found: !!hotel,
      price: hotel ? normalizeHotel(hotel) : null,
    };
  }

  /** Destinasyon bazli gruplama ile optimize edilmis bulk sorgu. */
  async getBulkPrices(mappings: ProviderMapping[], query: BulkPriceQuery): Promise<ProviderPriceResult[]> {
    const occ = toOccupancy(query);

    // Ayni destinasyondaki otelleri grupla
    const byDestination = new Map<string, ProviderMapping[]>();
    for (const m of mappings) {
      const dest = (m.metadata as any)?.destinationUrl;
      if (!dest) continue;
      const arr = byDestination.get(dest) ?? [];
      arr.push(m);
      byDestination.set(dest, arr);
    }

    const results: ProviderPriceResult[] = [];

    await Promise.all(
      Array.from(byDestination.entries()).map(async ([destinationUrl, group]) => {
        try {
          const found = await collectHotelsByIds(
            destinationUrl,
            group.map((m) => m.providerHotelId),
            occ
          );
          for (const m of group) {
            const hotel = found.get(m.providerHotelId);
            results.push({
              internalHotelId: m.internalHotelId,
              providerHotelId: m.providerHotelId,
              hotelName: m.hotelName,
              found: !!hotel,
              price: hotel ? normalizeHotel(hotel) : null,
            });
          }
        } catch (err: any) {
          for (const m of group) {
            results.push({
              internalHotelId: m.internalHotelId,
              providerHotelId: m.providerHotelId,
              hotelName: m.hotelName,
              found: false,
              price: null,
              error: err.message,
            });
          }
        }
      })
    );

    return results;
  }

  async getPackagePrices(mapping: ProviderMapping, query: PackageQuery): Promise<ProviderPackageResult> {
    const occ = toOccupancy(query);

    // 1) Oda aramasi -> roomSearchId + oturum cookie'si
    const { roomSearchId, cookie } = await fetchRoomSearch(
      mapping.providerHotelId,
      occ
    );

    // 2) Paket fiyatlari (ayni oturum)
    const pkg = await fetchPackage(roomSearchId, query.airportCode, cookie);
    const rooms = pkg.result?.rooms ?? [];

    return {
      internalHotelId: mapping.internalHotelId,
      providerHotelId: mapping.providerHotelId,
      hotelName: mapping.hotelName,
      airportCode: query.airportCode,
      checkIn: query.checkIn,
      checkOut: query.checkOut,
      available: rooms.length > 0,
      roomCount: rooms.length,
      rooms: rooms.map(normalizePackageRoom),
    };
  }

  validateMappingInput(input: ProviderMappingInput): string | null {
    if (!input.metadata?.destinationUrl) {
      return "metadata.destinationUrl zorunlu (ornek: \"Kemer-Otelleri\")";
    }
    return null;
  }

  requiredMetadataFields(): string[] {
    return ["destinationUrl"];
  }
}
