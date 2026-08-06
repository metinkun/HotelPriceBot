import {
  BaseProvider,
  ProviderCapabilities,
  ProviderMapping,
  ProviderMappingInput,
  ProviderPriceResult,
  ResolvedHotel,
} from "../base";
import { PriceQuery, PackageQuery, NormalizedPackageRoom } from "../../types";
import { fetchRoomPrices, fetchPackagePrices, resolveHotelFromUrl } from "./api";
import { normalizeTouristica } from "./normalizer";
import { ProviderPackageResult } from "../base";

function nightsBetween(checkIn: string, checkOut: string): number | null {
  const a = new Date(checkIn).getTime();
  const b = new Date(checkOut).getTime();
  if (!Number.isFinite(a) || !Number.isFinite(b)) return null;
  return Math.round((b - a) / 86400000);
}

export class TouristicaProvider extends BaseProvider {
  readonly name = "touristica";
  readonly capabilities: ProviderCapabilities = {
    supportsPackages: true,
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

  async getPackagePrices(
    mapping: ProviderMapping,
    query: PackageQuery
  ): Promise<ProviderPackageResult> {
    const sourceUrl = (mapping.metadata as any)?.sourceUrl;
    if (!sourceUrl) {
      throw new Error("Mapping metadata'da sourceUrl eksik (otel detay URL'i)");
    }

    const { cards, hotelName } = await fetchPackagePrices(
      sourceUrl,
      query.checkIn,
      query.checkOut,
      query.adults,
      query.children
    );

    const nightCount = nightsBetween(query.checkIn, query.checkOut);

    // Ayni odanin farkli pansiyonlarini grupla
    const byRoom = new Map<string, NormalizedPackageRoom>();
    for (const c of cards) {
      if (!c.price || !Number.isFinite(c.price)) continue;
      const name = c.roomName || "Oda";
      if (!byRoom.has(name)) {
        byRoom.set(name, {
          roomId: name,
          roomName: name,
          roomSize: null,
          nightCount,
          boards: [],
        });
      }
      byRoom.get(name)!.boards.push({
        boardType: c.boardType,
        currency: "TL",
        listPrice: null, // Touristica paket kartinda liste fiyati yok
        price: c.price,
        discountRate: 0,
        cancellation: null,
      });
    }

    const rooms = Array.from(byRoom.values());

    return {
      internalHotelId: mapping.internalHotelId,
      providerHotelId: mapping.providerHotelId,
      hotelName: mapping.hotelName ?? hotelName,
      airportCode: query.airportCode,
      checkIn: query.checkIn,
      checkOut: query.checkOut,
      available: rooms.length > 0,
      roomCount: rooms.length,
      rooms,
      note:
        "Touristica otel sayfasinda kalkis havalimani secimi yoktur; " +
        "paket fiyatlari sitenin varsayilan kalkisi ile dondurulur (airportCode dikkate alinmaz).",
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
