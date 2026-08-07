import axios from "axios";
import { getNextProxy } from "../../services/proxyService";

const BASE = "https://www.setur.com.tr";
const SEARCH_URL = `${BASE}/api/services/v4/SearchesService/searchesUrl`;

const UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36";

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

/** Setur detay sayfasi tarihli sorguda yavas render ediyor (~20-40sn). */
const PAGE_TIMEOUT = 90000;

export interface SeturLowestPrice {
  price?: number;
  currencyCode?: string;
  originalPrice?: number;
  originalDiscountedPrice?: number;
  discountRatio?: number;
  roomInformation?: string;
}

export interface SeturHotelData {
  lowestPrice: SeturLowestPrice | null;
  /** "2 Yetişkin 1 Çocuk 3 Gece" — sunucunun anladigi kisi bilgisi */
  roomInformation: string | null;
  guestAdultCount: number | null;
  guestChildCount: number | null;
  boardType: string | null;
  hotelName: string | null;
  city: string | null;
  country: string | null;
  nightCount: number | null;
  hasPackage: boolean;
}

/** Yastan yaklasik dogum tarihi uretir (yyyy-MM-dd). */
export function birthDateFromAge(age: number, today = new Date()): string {
  const y = today.getFullYear() - Math.max(0, Math.floor(age));
  const m = String(today.getMonth() + 1).padStart(2, "0");
  const d = String(today.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/**
 * Oda/kisi parametresi.
 * Format: `<yetiskin>[_<cocukDogumTarihi>...]`  ornek: `2` veya `2_2025-11-11`
 * NOT: virgul YENI ODA demektir (`room=2,2` = iki oda) — cocuk icin kullanilmaz.
 * Setur cocugu yas yerine DOGUM TARIHI ile tasidigi icin yas -> dogum tarihine
 * cevrilir (yaklasik; sitenin yas secici davranisiyla ayni sonucu verir).
 */
export function buildRoomParam(adults: number, childAges: number[]): string {
  const parts: string[] = [String(adults)];
  for (const age of childAges) {
    parts.push(birthDateFromAge(age));
  }
  return parts.join("_");
}

/**
 * Otel detay sayfasini tarih/kisi ile ceker ve gomulu (RSC) JSON'dan
 * fiyat alanlarini ayiklar. Cookie/oturum gerekmez.
 */
export async function fetchHotelPage(
  slug: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  childAges: number[]
): Promise<SeturHotelData> {
  const url =
    `${BASE}/${slug}?in=${checkIn}&out=${checkOut}` +
    `&room=${buildRoomParam(adults, childAges)}`;

  const response = await axios.get<string>(url, {
    headers: PAGE_HEADERS,
    timeout: PAGE_TIMEOUT,
    proxy: getNextProxy(),
    responseType: "text",
    transformResponse: (d) => d,
  });

  // RSC payload'inda JSON escape'li (\" ) gomulu geliyor
  const raw = response.data.replace(/\\"/g, '"');

  const lpMatch = raw.match(/"lowestPrice":\{[^}]*\}/);
  let lowestPrice: SeturLowestPrice | null = null;
  if (lpMatch) {
    try {
      lowestPrice = JSON.parse(lpMatch[0].replace(/^"lowestPrice":/, ""));
    } catch {
      lowestPrice = null;
    }
  }

  const pick = (re: RegExp): string | null => {
    const m = raw.match(re);
    return m ? m[1] : null;
  };

  const nights = pick(/"nightCount":(\d+)/);

  // Otel adi: metaTitle en guvenilir anchor ("name" React internal'lerini yakaliyor)
  const metaTitle = pick(/"metaTitle":"([^"]{3,120})"/);

  // Sunucunun anladigi kisi bilgisi (dogrulama icin): "2 Yetişkin 1 Çocuk 3 Gece"
  const roomInformation = lowestPrice?.roomInformation ?? null;
  const guestAdults = pick(/"guests":\{"adultCount":(\d+)/);
  const guestChildren = pick(/"guests":\{"adultCount":\d+,"childCount":(\d+)/);

  return {
    lowestPrice,
    roomInformation,
    guestAdultCount: guestAdults ? Number(guestAdults) : null,
    guestChildCount: guestChildren ? Number(guestChildren) : null,
    boardType: pick(/"boardType":"([^"]{2,40})"/),
    hotelName: metaTitle,
    city: pick(/"city":"([^"]{2,40})"/),
    country: pick(/"country":"([^"]{2,40})"/),
    nightCount: nights ? Number(nights) : null,
    hasPackage: /"hasPackage":true/.test(raw),
  };
}

/**
 * Paket (Otel + Ucak) fiyati.
 * URL: `?in=&out=&room=&isPackage=true&dep=<kalkisIATA>&depIsCity=true[&arr=<varisIATA>]`
 * Paket fiyati sayfanin `packageInitialData.data.hotel.lowestPrice` alanindadir
 * (ust seviyedeki `lowestPrice` sadece-otel fiyatidir).
 */
export async function fetchPackagePage(
  slug: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  childAges: number[],
  departureCode: string,
  arrivalCode?: string
): Promise<SeturHotelData> {
  const params = [
    `in=${checkIn}`,
    `out=${checkOut}`,
    `room=${buildRoomParam(adults, childAges)}`,
    "isPackage=true",
    `dep=${encodeURIComponent(departureCode.toUpperCase())}`,
    "depIsCity=true",
  ];
  if (arrivalCode) params.push(`arr=${encodeURIComponent(arrivalCode.toUpperCase())}`);

  const url = `${BASE}/${slug}?${params.join("&")}`;

  const response = await axios.get<string>(url, {
    headers: PAGE_HEADERS,
    timeout: PAGE_TIMEOUT,
    proxy: getNextProxy(),
    responseType: "text",
    transformResponse: (d) => d,
  });

  const raw = response.data.replace(/\\"/g, '"');

  // Paket blogu: packageInitialData -> data -> hotel
  const pkgIndex = raw.indexOf('"packageInitialData"');
  if (pkgIndex === -1) {
    return {
      lowestPrice: null,
      roomInformation: null,
      guestAdultCount: null,
      guestChildCount: null,
      boardType: null,
      hotelName: null,
      city: null,
      country: null,
      nightCount: null,
      hasPackage: false,
    };
  }
  const seg = raw.slice(pkgIndex, pkgIndex + 4000);

  const lpMatch = seg.match(/"lowestPrice":\{[^}]*\}/);
  let lowestPrice: SeturLowestPrice | null = null;
  if (lpMatch) {
    try {
      lowestPrice = JSON.parse(lpMatch[0].replace(/^"lowestPrice":/, ""));
    } catch {
      lowestPrice = null;
    }
  }

  const pick = (re: RegExp): string | null => {
    const m = seg.match(re);
    return m ? m[1] : null;
  };
  const nights = pick(/"nightCount":(\d+)/);

  return {
    lowestPrice,
    roomInformation: lowestPrice?.roomInformation ?? null,
    guestAdultCount: null,
    guestChildCount: null,
    boardType: pick(/"boardType":"([^"]{2,40})"/),
    hotelName: pick(/"metaTitle":"([^"]{3,120})"/),
    city: pick(/"city":"([^"]{2,40})"/),
    country: pick(/"country":"([^"]{2,40})"/),
    nightCount: nights ? Number(nights) : null,
    hasPackage: true,
  };
}

// ---- URL'den otel verisi cozme ----

export interface SeturResolved {
  providerHotelId: string;
  hotelName: string | null;
  slug: string;
  hasPackage: boolean;
}

/** URL'den slug cikarir: https://www.setur.com.tr/<slug> */
export function slugFromUrl(url: string): string {
  const path = url.replace(/^https?:\/\/[^/]+\//, "").split("?")[0];
  return path.replace(/\/+$/, "");
}

/**
 * Slug -> otel kimligi. Setur'un public arama API'si (auth yok):
 * POST searchesUrl {"args":[{"s":"<slug>"}]} -> {id, text, type, package}
 */
export async function resolveHotelFromUrl(
  url: string
): Promise<SeturResolved> {
  const slug = slugFromUrl(url);
  if (!slug) throw new Error("Setur URL'inden slug cozulemedi");

  const response = await axios.post<any>(
    SEARCH_URL,
    { args: [{ s: slug }] },
    {
      headers: {
        "user-agent": UA,
        "content-type": "application/json",
        accept: "application/json",
        origin: BASE,
        referer: `${BASE}/${slug}`,
      },
      timeout: 25000,
      proxy: getNextProxy(),
    }
  );

  const d = response.data;
  if (!d || !d.id || d.type !== "Hotel") {
    throw new Error(
      "Setur otel bulunamadi (otel detay sayfasi URL'i olmali)"
    );
  }

  return {
    providerHotelId: String(d.id),
    hotelName: d.text ? String(d.text).trim() : null,
    slug: d.url ? String(d.url) : slug,
    hasPackage: !!d.package,
  };
}
