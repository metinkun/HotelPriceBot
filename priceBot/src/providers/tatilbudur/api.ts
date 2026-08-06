import axios from "axios";
import qs from "qs";
import { getNextProxy } from "../../services/proxyService";

const BASE = "https://www.tatilbudur.com";
const CALC_URL = `${BASE}/hotel/calculate-room-price`;
const CALC_PACKAGE_URL = `${BASE}/hotel/calculate-package-room-price`;

const UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36";

// Tam HTML sayfa cekerken tarayici-gezinme header'lari (403'u asan set).
const PAGE_HEADERS = {
  accept:
    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
  "accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
  "user-agent": UA,
  "sec-ch-ua": '"Chromium";v="150", "Google Chrome";v="150", "Not;A=Brand";v="8"',
  "sec-ch-ua-mobile": "?0",
  "sec-ch-ua-platform": '"macOS"',
  "sec-fetch-dest": "document",
  "sec-fetch-mode": "navigate",
  "sec-fetch-site": "none",
  "upgrade-insecure-requests": "1",
};

export interface TatilbudurRoomPriceResponse {
  view?: string;
  alternativeHtml?: string;
  [key: string]: any;
}

/** yyyy-MM-dd -> "d.M.yyyy" (sifirsiz, checkInDate/checkOutDate icin) */
function toDotDate(iso: string): string {
  const [y, m, d] = iso.slice(0, 10).split("-");
  return `${Number(d)}.${Number(m)}.${y}`;
}

/** yyyy-MM-dd -> "dd/MM/yyyy" (daterange-1 icin) */
function toSlashDate(iso: string): string {
  const [y, m, d] = iso.slice(0, 10).split("-");
  return `${d}/${m}/${y}`;
}

function cookieHeaderFrom(setCookie: string[] | undefined): string {
  if (!setCookie?.length) return "";
  return setCookie.map((c) => c.split(";")[0]).join("; ");
}

export interface TokenSession {
  token: string;
  cookie: string;
  html: string;
}

/** Detay sayfasini ceker; CSRF _token + oturum cookie'lerini (Akamai dahil) doner. */
export async function fetchTokenAndCookies(url: string): Promise<TokenSession> {
  const response = await axios.get<string>(url, {
    headers: PAGE_HEADERS,
    timeout: 25000,
    proxy: getNextProxy(),
    responseType: "text",
    transformResponse: (d) => d,
  });
  const html = response.data;
  const token =
    html.match(/name="_token"\s+value="([^"]+)"/)?.[1] ||
    html.match(/csrf-token"\s+content="([^"]+)"/)?.[1];
  if (!token) {
    throw new Error("Tatilbudur sayfasinda _token bulunamadi");
  }
  return {
    token,
    cookie: cookieHeaderFrom(response.headers["set-cookie"] as string[]),
    html,
  };
}

/**
 * Bir otel + tarih + kisi icin oda fiyat yanitini (view HTML) ceker.
 * Akis: detay sayfasindan taze token+cookie -> calculate-room-price POST.
 */
/** Iki fiyat cagrisinin da kullandigi ortak form govdesi (#allHotelTabForm). */
function buildBaseForm(
  token: string,
  providerHotelId: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  children: number
): string {
  const quickPersonCount =
    `${adults} Yetişkin` + (children > 0 ? ` ${children} Çocuk` : "");

  return qs.stringify({
    _token: token,
    productType: "hotel",
    hotelId: providerHotelId,
    selectedRoom: "",
    selectedPricingId: "",
    selectedMealType: "",
    hotelOldId: "",
    productLoc: "",
    productTypeId: "",
    code: "",
    alertRoom: "0",
    actionPricingId: "",
    currencyId: "",
    "actions[]": "0",
    selectedActionCategory: "",
    hidePrice: "0",
    googleRemarketingCategory: "",
    autoPost: "0",
    isFlightPackage: "0",
    adult: String(adults),
    child: String(children),
    isCyprusPackageManual: "0",
    priceConfig: "",
    loyaltyPoint: "0",
    "daterange-1": `${toSlashDate(checkIn)} - ${toSlashDate(checkOut)}`,
    checkInDate: toDotDate(checkIn),
    checkOutDate: toDotDate(checkOut),
    quickPersonCount,
    type: "",
  });
}

function priceHeaders(token: string, detailUrl: string, cookie: string) {
  return {
    "user-agent": UA,
    "x-requested-with": "XMLHttpRequest",
    "x-csrf-token": token,
    "content-type": "application/x-www-form-urlencoded; charset=UTF-8",
    accept: "*/*",
    origin: BASE,
    referer: detailUrl,
    cookie,
  };
}

export async function fetchRoomPrice(
  detailUrl: string,
  providerHotelId: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  children: number
): Promise<TatilbudurRoomPriceResponse> {
  const { token, cookie } = await fetchTokenAndCookies(detailUrl);
  const body = buildBaseForm(
    token,
    providerHotelId,
    checkIn,
    checkOut,
    adults,
    children
  );

  const response = await axios.post<TatilbudurRoomPriceResponse>(CALC_URL, body, {
    headers: priceHeaders(token, detailUrl, cookie),
    timeout: 30000,
    proxy: getNextProxy(),
  });

  return response.data;
}

/**
 * Paket (Otel + Ucak + Transfer) fiyatlari.
 * Oda fiyati ile ayni form + `to` (varis havalimani, sayfa JS'inden) ve
 * `from` (kalkis havalimani, kullanicidan) parametreleri.
 */
export async function fetchPackagePrice(
  detailUrl: string,
  providerHotelId: string,
  departureCode: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  children: number
): Promise<{ data: TatilbudurRoomPriceResponse; destinationCode: string | null }> {
  const { token, cookie, html } = await fetchTokenAndCookies(detailUrl);

  // Varis havalimani kodu sayfa JS'inde: formData += "&to=ECN&from="+...
  const destinationCode = html.match(/&to=([A-Z]{3})&from=/)?.[1] ?? null;
  if (!destinationCode) {
    throw new Error(
      "Tatilbudur varis havalimani kodu bulunamadi (bu otel icin paket sunulmuyor olabilir)"
    );
  }

  const body =
    buildBaseForm(token, providerHotelId, checkIn, checkOut, adults, children) +
    `&to=${destinationCode}&from=${encodeURIComponent(departureCode.toUpperCase())}`;

  const response = await axios.post<TatilbudurRoomPriceResponse>(
    CALC_PACKAGE_URL,
    body,
    {
      headers: priceHeaders(token, detailUrl, cookie),
      timeout: 45000,
      proxy: getNextProxy(),
    }
  );

  return { data: response.data, destinationCode };
}

// ---- URL'den otel verisi cozme ----

export interface TatilbudurResolved {
  providerHotelId: string;
  hotelName: string | null;
}

function decodeEntities(s: string): string {
  return s
    .replace(/&amp;/g, "&")
    .replace(/&#231;/g, "ç")
    .replace(/&nbsp;/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

/** Detay sayfasindan hotelId (providerHotelId) ve otel adini cikarir. */
export async function resolveHotelFromUrl(
  url: string
): Promise<TatilbudurResolved> {
  const { html } = await fetchTokenAndCookies(url);

  const idMatch = html.match(/hotelId=(\d+)/);
  if (!idMatch) {
    throw new Error(
      "Tatilbudur sayfasinda hotelId bulunamadi (otel detay URL'i olmali)"
    );
  }
  const nameMatch = html.match(
    /<h1[^>]*hotel-name[^>]*>\s*([^<]{2,80})/
  );

  return {
    providerHotelId: idMatch[1],
    hotelName: nameMatch ? decodeEntities(nameMatch[1]) : null,
  };
}
