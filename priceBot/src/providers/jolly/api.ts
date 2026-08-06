import axios from "axios";
import qs from "qs";
import { getNextProxy } from "../../services/proxyService";

// Jolly otel oda/fiyat endpoint'i (cookie gerekmez, form-urlencoded).
const JOLLY_RESERVATION_URL =
  "https://www.jollytur.com/hotel/GetReservationCompletePartial";
// Havalimani/destinasyon autocomplete (originId cozmek icin).
const JOLLY_SEARCH_URL = "https://www.jollytur.com/Shared/Search";

/** packageSearchType: 0 = Ucak + Transfer Dahil, 1 = Ucak Dahil (transfersiz) */
export const PACKAGE_TYPE_FLIGHT_TRANSFER = "0";

const UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36";

const XHR_HEADERS = {
  accept: "application/json",
  "accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
  "content-type": "application/x-www-form-urlencoded; charset=UTF-8",
  origin: "https://www.jollytur.com",
  "user-agent": UA,
  "x-requested-with": "XMLHttpRequest",
};

// Tam HTML sayfa cekerken tarayici-gezinme header'lari (XHR isaretleri YOK).
const PAGE_HEADERS = {
  accept:
    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
  "accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
  "user-agent": UA,
  "sec-fetch-dest": "document",
  "sec-fetch-mode": "navigate",
  "sec-fetch-site": "none",
  "upgrade-insecure-requests": "1",
};

// analyticsDataModel (temiz lead-fiyat + kimlik verisi)
export interface JollyAnalytics {
  productId?: string;
  productName?: string;
  cityName?: string;
  countryName?: string;
  currency?: string;
  totalPrice?: number;
  discountedPrice?: number;
  exchangedTotalPrice?: number;
  exchangedDiscountedPrice?: number;
}

export interface JollyReservationResponse {
  html: string;
  analyticsDataModel?: JollyAnalytics;
  isAvailableHotel?: boolean;
  [key: string]: any;
}

/** "2026-08-31" (veya "31.08.2026") -> "2026.08.31" */
export function toJollyDate(input: string): string {
  if (/^\d{4}-\d{2}-\d{2}/.test(input)) return input.slice(0, 10).replace(/-/g, ".");
  if (/^\d{2}\.\d{2}\.\d{4}/.test(input)) {
    const [d, m, y] = input.slice(0, 10).split(".");
    return `${y}.${m}.${d}`;
  }
  throw new Error(`Gecersiz tarih formati: ${input} (yyyy-MM-dd bekleniyor)`);
}

/**
 * Oda/kisi parametresi (tek oda).
 * Format: "<yetiskin>[-<cocukYas>...]" — oda ici tire ile, odalar virgulle ayrilir.
 * Ornek: "2" (2 yetiskin), "2-5" (2 yetiskin + 5 yas cocuk), "2-5-8".
 * Cocuk yas degeri: 0-1 yas -> 1 (Jolly "0-2" kovasi), 2+ -> yasin kendisi.
 */
export function buildRoomsParam(adults: number, childAges: number[]): string {
  const ageVals = childAges.map((a) => (a < 2 ? 1 : a));
  return [adults, ...ageVals].join("-");
}

export interface JollyOrigin {
  id: number;
  name: string;
}

/**
 * IATA kodundan (SAW, IST, ESB...) Jolly'nin ic havalimani id'sini bulur.
 * destinationType === 7 -> Airport
 */
export async function lookupAirport(iata: string): Promise<JollyOrigin> {
  const response = await axios.get<any[]>(JOLLY_SEARCH_URL, {
    params: { key: iata.toUpperCase(), type: "FlightPlanner" },
    headers: { "user-agent": UA, "x-requested-with": "XMLHttpRequest" },
    timeout: 20000,
    proxy: getNextProxy(),
  });

  const list = Array.isArray(response.data) ? response.data : [];
  const airport = list.find(
    (x) => x?.destinationType === 7 && x?.id > 0
  );
  if (!airport) {
    throw new Error(
      `Jolly'de '${iata}' havalimani bulunamadi (IATA kodu kontrol edilmeli)`
    );
  }
  return { id: airport.id, name: String(airport.value ?? iata) };
}

/** Bir otel + tarih + kisi icin oda/fiyat yanitini ceker. */
export async function fetchHotelReservation(
  providerHotelId: string,
  hotelType: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  childAges: number[],
  pkg?: { origin: JollyOrigin; packageSearchType: string }
): Promise<JollyReservationResponse> {
  const body = qs.stringify({
    id: providerHotelId,
    startDate: toJollyDate(checkIn),
    endDate: toJollyDate(checkOut),
    rooms: buildRoomsParam(adults, childAges),
    originId: pkg ? String(pkg.origin.id) : "",
    originType: pkg ? "Airport" : "Zone",
    originName: pkg ? pkg.origin.name : "",
    hotelType,
    searchType: "Product",
    customerTrackId: "00000000-0000-0000-0000-000000000000",
    packageSearchType: pkg ? pkg.packageSearchType : "",
    trivagoReferenceId: "",
    neredeKalReferenceId: "",
    hadsKey: "",
    utmSource: "",
  });

  const response = await axios.post<JollyReservationResponse>(
    JOLLY_RESERVATION_URL,
    body,
    {
      headers: { ...XHR_HEADERS, referer: "https://www.jollytur.com/" },
      timeout: 30000,
      proxy: getNextProxy(),
    }
  );

  return response.data;
}

// ---- URL'den otel verisi cozme ----

export interface JollyResolved {
  providerHotelId: string;
  hotelName: string | null;
  hotelType: string | null;
}

function decodeEntities(s: string): string {
  return s
    .replace(/&#x2B;/g, "+")
    .replace(/&#x27;/g, "'")
    .replace(/&amp;/g, "&")
    .replace(/&nbsp;/g, " ")
    .trim();
}

/**
 * Jolly otel detay sayfasindan SearchParameters.Id (providerHotelId),
 * HotelType ve adi cikarir.
 */
export async function resolveHotelFromUrl(url: string): Promise<JollyResolved> {
  const response = await axios.get<string>(url, {
    headers: PAGE_HEADERS,
    timeout: 20000,
    proxy: getNextProxy(),
    responseType: "text",
    transformResponse: (d) => d,
  });
  const html = response.data;

  const idMatch = html.match(
    /name="SearchParameters\.Id"[^>]*value="([^"]*)"/
  );
  if (!idMatch || !idMatch[1]) {
    throw new Error(
      "Jolly sayfasinda SearchParameters.Id bulunamadi (otel detay URL'i olmali)"
    );
  }
  const typeMatch = html.match(
    /name="SearchParameters\.HotelType"[^>]*value="([^"]*)"/
  );
  const nameMatch = html.match(
    /name="SearchParameters\.HotelName"[^>]*value="([^"]*)"/
  );

  return {
    providerHotelId: idMatch[1],
    hotelType: typeMatch ? typeMatch[1] : null,
    hotelName: nameMatch ? decodeEntities(nameMatch[1]) : null,
  };
}
