import axios from "axios";
import { config } from "../config";
import {
  EtsturSearchRequest,
  EtsturSearchResponse,
  EtsturHotel,
  NormalizedPrice,
} from "../types";

// Etstur otel arama / fiyat endpoint'i (DevTools'tan dogrulandi, cookie gerekmez).
const ETSTUR_SEARCH_URL =
  "https://www.etstur.com/services/api/search/v2/hotels";

const PAGE_LIMIT = 100; // tek istekte cekilen otel sayisi (test edildi: 100 calisiyor)
const MAX_PAGES = 20; // guvenlik siniri (max 2000 otel taranir)

const DEFAULT_HEADERS = {
  accept: "application/json, text/plain, */*",
  "accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
  "content-type": "application/json",
  origin: "https://www.etstur.com",
  referer: "https://www.etstur.com/",
  "user-agent":
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
  "x-requested-with": "XMLHttpRequest",
};

export interface DateOccupancy {
  checkIn: string; // yyyy-MM-dd
  checkOut: string; // yyyy-MM-dd
  adults: number;
  children: number;
  childAges: number[];
}

/** "2026-06-24" veya "24.06.2026" -> "2026-06-24T00:00:00.000" */
export function toEtsturDate(input: string): string {
  let y: string, m: string, d: string;
  if (/^\d{4}-\d{2}-\d{2}/.test(input)) {
    [y, m, d] = input.slice(0, 10).split("-");
  } else if (/^\d{2}\.\d{2}\.\d{4}/.test(input)) {
    [d, m, y] = input.slice(0, 10).split(".");
  } else {
    throw new Error(
      `Gecersiz tarih formati: ${input} (yyyy-MM-dd bekleniyor)`
    );
  }
  return `${y}-${m}-${d}T00:00:00.000`;
}

/** Arama endpoint'i icin istek govdesini olusturur (gercek payload yapisi). */
export function buildSearchBody(
  destinationUrl: string,
  occ: DateOccupancy,
  offset = 0,
  limit = PAGE_LIMIT
): EtsturSearchRequest {
  return {
    checkIn: toEtsturDate(occ.checkIn),
    checkOut: toEtsturDate(occ.checkOut),
    rooms: [
      {
        adultCount: occ.adults,
        childCount: occ.children,
        childAges: occ.childAges,
        infantCount: 0,
      },
    ],
    url: destinationUrl,
    offset,
    limit,
    minPrice: null,
    maxPrice: null,
    filters: [],
    loc: null,
    zoom: null,
    radiusKm: null,
    excludeHotelIds: [],
    sort: { type: "", direction: "" },
  };
}

/** Arama endpoint'ine tek istek atar. */
export async function searchEtstur(
  body: EtsturSearchRequest
): Promise<EtsturSearchResponse> {
  const response = await axios.post<EtsturSearchResponse>(
    ETSTUR_SEARCH_URL,
    body,
    {
      headers: {
        ...DEFAULT_HEADERS,
        ...(config.etsturCookie ? { cookie: config.etsturCookie } : {}),
      },
      timeout: 20000,
    }
  );
  return response.data;
}

/**
 * Bir destinasyonda, istenen hotelId'leri sayfalayarak bulur.
 * Hepsi bulununca ya da sayfa/sinir bitince durur.
 */
export async function collectHotelsByIds(
  destinationUrl: string,
  wantedIds: string[],
  occ: DateOccupancy
): Promise<Map<string, EtsturHotel>> {
  const wanted = new Set(wantedIds);
  const found = new Map<string, EtsturHotel>();
  let offset = 0;
  let total = Infinity;

  for (let page = 0; page < MAX_PAGES && found.size < wanted.size; page++) {
    if (offset >= total) break;

    const data = await searchEtstur(
      buildSearchBody(destinationUrl, occ, offset, PAGE_LIMIT)
    );
    const result = data.result;
    if (!data.success || !result) {
      throw new Error(data.errorMessage || "Etstur arama basarisiz");
    }
    total = result.totalCount;

    for (const hotel of result.hotels) {
      if (wanted.has(hotel.hotelId)) found.set(hotel.hotelId, hotel);
    }

    if (result.hotels.length < PAGE_LIMIT) break;
    offset += PAGE_LIMIT;
  }

  return found;
}

/** Bir Etstur otel nesnesini sade fiyat ozetine cevirir. */
export function normalizeHotel(hotel: EtsturHotel): NormalizedPrice {
  const room = hotel.room;
  const price = room?.price;

  return {
    available: !!price,
    hotelName: hotel.name ?? null,
    city: hotel.location?.city ?? null,
    roomName: room?.roomNames?.[0] ?? null,
    boardType: room?.boardTypeLabel?.[0] ?? null,
    currency: price?.currency ?? null,
    listPrice: price?.amount ?? null,
    price: price ? price.discountedPrice ?? price.amount : null,
    discountRate: price?.discountRate ?? 0,
    bankCampaignPrice: room?.campaignHighlightedPrice?.price?.amount ?? null,
    bankCampaignLabel: room?.campaignHighlightedPrice?.label ?? null,
    minStayNights: room?.availability?.nightCount ?? null,
  };
}
