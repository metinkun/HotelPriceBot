import axios from "axios";
import { getNextProxy } from "../../services/proxyService";

const SITE = "https://www.gezinomi.com";
const API = "https://apigezinomi.gezinomi.com/api";
const ROOM_PRICES_URL = `${API}/Hotel/GetHotelRoomPrices`;
const AIRPORTS_URL = `${API}/Hotel/GetHotelAirports`;

const UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36";

const API_HEADERS = {
  accept: "*/*",
  "accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8",
  "content-type": "application/json",
  origin: SITE,
  referer: `${SITE}/`,
  "user-agent": UA,
};

const PAGE_HEADERS = {
  accept:
    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
  "accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8",
  "user-agent": UA,
  "sec-fetch-dest": "document",
  "sec-fetch-mode": "navigate",
  "sec-fetch-site": "none",
  "upgrade-insecure-requests": "1",
};

/** yyyy-MM-dd -> yyyy-MM-ddT00:00:00 */
function toApiDate(iso: string): string {
  return `${iso.slice(0, 10)}T00:00:00`;
}

// ---- Yanit tipleri (sadece kullandigimiz alanlar) ----

export interface GezinomiPriceEntry {
  discountedPrice?: number;
  notDiscountedPrice?: number;
  originalPrice?: number;
  conceptName?: string;
}

export interface GezinomiReservationDetail {
  conceptName?: string;
  hotelRoomName?: string;
  totalPrice?: number;
  totalDiscountedPrice?: number;
  discountRate?: number;
  flightPackageTotalPrice?: number;
  flightPackageTotalDiscountedPrice?: number;
  flightPackageDiscountRate?: number;
  roomCapacity?: string;
  prices?: GezinomiPriceEntry[];
}

export interface GezinomiRoomDetail {
  roomId?: number;
  roomName?: string;
  reservationDetails?: GezinomiReservationDetail[];
}

export interface GezinomiPriceResponse {
  data?: {
    roomDetails?: GezinomiRoomDetail[];
    transferDetailList?: any[];
    flights?: any[];
    searchId?: string;
  } | null;
  status?: string;
  message?: string;
}

/**
 * Kisi listesi olusturur. Gezinomi her misafiri ayri obje olarak istiyor:
 * yetiskin -> {isChild:false, age:0}, cocuk -> {isChild:true, age:<yas>}
 */
function buildPersons(adults: number, childAges: number[]) {
  const persons: any[] = [];
  for (let i = 0; i < adults; i++) {
    persons.push({ isChild: false, isMen: false, age: 0, roomOrderId: 0 });
  }
  for (const age of childAges) {
    persons.push({ isChild: true, isMen: false, age, roomOrderId: 0 });
  }
  return persons;
}

/**
 * Oda fiyatlarini ceker. `withFlight` true ise ayni yanitta paket
 * (flightPackage*) alanlari da dolar. Auth/cookie gerekmez.
 */
export async function fetchRoomPrices(
  providerHotelId: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  childAges: number[],
  withFlight = false
): Promise<GezinomiPriceResponse> {
  const body = {
    checkInDate: toApiDate(checkIn),
    checkOutDate: toApiDate(checkOut),
    hotelId: Number(providerHotelId),
    apiId: 0,
    searchTypeId: 1,
    apiProviderName: "",
    roomDetails: [
      { roomId: 1, persons: buildPersons(adults, childAges) },
    ],
    flightSearch: {
      flights: [],
      passengers: withFlight
        ? { adult: adults, child: childAges.length, infant: 0 }
        : { adult: 0, child: 0, infant: 0 },
      isMultiTrip: false,
      directFlightsOnly: false,
      cabinCodeType: "",
    },
  };

  const response = await axios.post<GezinomiPriceResponse>(
    ROOM_PRICES_URL,
    body,
    { headers: API_HEADERS, timeout: 40000, proxy: getNextProxy() }
  );

  return response.data;
}

/** Otelin varis havalimanlarini doner (paket icin bilgi amacli). */
export async function fetchHotelAirports(
  providerHotelId: string,
  hotelLocationId?: number,
  parentId?: number
): Promise<Array<{ airportName: string; airportCode: string }>> {
  const response = await axios.post<any>(
    AIRPORTS_URL,
    {
      hotelId: Number(providerHotelId),
      hotelLocationId: hotelLocationId ?? 0,
      parentId: parentId ?? 0,
    },
    { headers: API_HEADERS, timeout: 25000, proxy: getNextProxy() }
  );
  return Array.isArray(response.data?.data) ? response.data.data : [];
}

// ---- URL'den otel verisi cozme ----

export interface GezinomiResolved {
  providerHotelId: string;
  hotelName: string | null;
}

/**
 * Otel detay sayfasindan hotelId ve adi cikarir.
 * Sayfa Next.js RSC; veri escape'li JSON olarak gomulu geliyor.
 */
export async function resolveHotelFromUrl(
  url: string
): Promise<GezinomiResolved> {
  const response = await axios.get<string>(url, {
    headers: PAGE_HEADERS,
    timeout: 30000,
    proxy: getNextProxy(),
    responseType: "text",
    transformResponse: (d) => d,
  });
  const raw = response.data.replace(/\\"/g, '"');

  const idMatch = raw.match(/"hotelId":(\d+)/);
  if (!idMatch) {
    throw new Error(
      "Gezinomi sayfasinda hotelId bulunamadi (otel detay URL'i olmali)"
    );
  }
  const nameMatch =
    raw.match(/"hotelName":"([^"]{3,90})"/) ||
    raw.match(/"productName":"([^"]{3,90})"/);

  return {
    providerHotelId: idMatch[1],
    hotelName: nameMatch ? nameMatch[1].trim() : null,
  };
}
