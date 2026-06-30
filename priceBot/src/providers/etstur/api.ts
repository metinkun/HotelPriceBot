import axios from "axios";
import { config } from "../../config";
import { getNextProxy } from "../../services/proxyService";
import {
  EtsturSearchRequest,
  EtsturSearchResponse,
  EtsturHotel,
  EtsturRoomSearchResponse,
  EtsturPackageResponse,
} from "../../types/etstur";

const ETSTUR_SEARCH_URL =
  "https://www.etstur.com/services/api/search/v2/hotels";
const ETSTUR_ROOM_URL = "https://www.etstur.com/services/api/room";
const ETSTUR_PACKAGE_URL =
  "https://www.etstur.com/services/api/room/package";

const PAGE_LIMIT = 100;
const MAX_PAGES = 20;

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
  checkOut: string;
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

/** "2026-06-24" -> "2026-06-24" (paket /room icin saatsiz) */
export function toRoomDate(input: string): string {
  return toEtsturDate(input).slice(0, 10);
}

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

export async function searchEtstur(
  body: EtsturSearchRequest
): Promise<EtsturSearchResponse> {
  const response = await axios.post<EtsturSearchResponse>(
    ETSTUR_SEARCH_URL,
    body,
    {
      headers: {
        ...DEFAULT_HEADERS,
        ...(config.providers.etstur.cookie
          ? { cookie: config.providers.etstur.cookie }
          : {}),
      },
      timeout: 20000,
      proxy: getNextProxy(),
    }
  );
  return response.data;
}

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

// ---- Paket akisi ----

function cookieHeaderFrom(setCookie: string[] | undefined): string {
  if (!setCookie?.length) return "";
  return setCookie.map((c) => c.split(";")[0]).join("; ");
}

export async function fetchRoomSearch(
  etsturHotelId: string,
  occ: DateOccupancy
): Promise<{ roomSearchId: string; cookie: string }> {
  const body = {
    hotelId: etsturHotelId,
    checkIn: toRoomDate(occ.checkIn),
    checkOut: toRoomDate(occ.checkOut),
    room: {
      adultCount: occ.adults,
      childCount: occ.children,
      childAges: occ.childAges,
      infantCount: 0,
    },
  };

  const response = await axios.post<EtsturRoomSearchResponse>(
    ETSTUR_ROOM_URL,
    body,
    { headers: { ...DEFAULT_HEADERS }, timeout: 20000, proxy: getNextProxy() }
  );

  const data = response.data;
  if (!data.success || !data.result?.roomSearchId) {
    throw new Error(
      data.errorMessage || "Oda aramasi basarisiz (roomSearchId yok)"
    );
  }

  return {
    roomSearchId: data.result.roomSearchId,
    cookie: cookieHeaderFrom(response.headers["set-cookie"] as string[]),
  };
}

export async function fetchPackage(
  roomSearchId: string,
  airportCode: string,
  cookie: string
): Promise<EtsturPackageResponse> {
  const response = await axios.post<EtsturPackageResponse>(
    ETSTUR_PACKAGE_URL,
    { roomSearchId, airportCode },
    {
      headers: { ...DEFAULT_HEADERS, ...(cookie ? { cookie } : {}) },
      timeout: 20000,
      proxy: getNextProxy(),
    }
  );
  return response.data;
}

// ---- URL'den otel verisi cozme ----

export interface EtsturResolved {
  providerHotelId: string;
  hotelName: string | null;
  destinationUrl: string | null;
  slug: string | null;
}

/**
 * Etstur otel detay sayfasini cekip __NEXT_DATA__ icinden
 * providerHotelId (data.hotelId), ad ve destinationUrl (breadcrumb) cikarir.
 */
export async function resolveHotelFromUrl(url: string): Promise<EtsturResolved> {
  // Tam HTML sayfa: tarayici-gezinme header'lari (XHR/origin YOK).
  const response = await axios.get<string>(url, {
    headers: {
      accept:
        "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
      "accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
      "user-agent":
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
      "sec-fetch-dest": "document",
      "sec-fetch-mode": "navigate",
      "sec-fetch-site": "none",
      "upgrade-insecure-requests": "1",
    },
    timeout: 20000,
    proxy: getNextProxy(),
    responseType: "text",
    transformResponse: (d) => d,
  });

  const html = response.data;
  const i = html.indexOf("__NEXT_DATA__");
  if (i === -1) throw new Error("Etstur sayfasinda __NEXT_DATA__ bulunamadi (URL gecersiz olabilir)");
  const j = html.indexOf(">", i) + 1;
  const k = html.indexOf("</script>", j);
  let data: any;
  try {
    data = JSON.parse(html.slice(j, k));
  } catch {
    throw new Error("Etstur __NEXT_DATA__ parse edilemedi");
  }

  const d = data?.props?.pageProps?.data;
  const hotelId = d?.hotelId;
  if (!hotelId) {
    throw new Error("Etstur sayfasinda hotelId bulunamadi (otel detay URL'i olmali)");
  }

  const breadcrumbs = d?.location?.locationBreadCrumbs;
  const destinationUrl = Array.isArray(breadcrumbs) && breadcrumbs.length
    ? breadcrumbs[0]?.url ?? null
    : null;

  return {
    providerHotelId: String(hotelId),
    hotelName: d?.name ?? null,
    destinationUrl,
    slug: d?.url ?? null,
  };
}
