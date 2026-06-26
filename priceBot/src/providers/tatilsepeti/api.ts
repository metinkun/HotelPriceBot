import axios, { AxiosProxyConfig } from "axios";
import qs from "qs";
import { config } from "../../config";
import { TatilSepetiPriceRequest } from "../../types/tatilsepeti";

function getProxyConfig(): AxiosProxyConfig | false {
  const { host, port, username, password } = config.proxy;
  if (!host || !port) return false;
  return { host, port, auth: { username, password }, protocol: "http" };
}

const TATILSEPETI_URL =
  "https://www.tatilsepeti.com/hotel/GetHotelListPrice/";
const TATILSEPETI_PACKAGE_URL =
  "https://www.tatilsepeti.com/Hotel/GetFlyingPackages";

const DEFAULT_HEADERS = {
  accept: "application/json, text/javascript, */*; q=0.01",
  "accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
  "cache-control": "no-cache",
  "content-type": "application/x-www-form-urlencoded; charset=UTF-8",
  origin: "https://www.tatilsepeti.com",
  pragma: "no-cache",
  "user-agent":
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
  "x-requested-with": "XMLHttpRequest",
};

/** yyyy-MM-dd -> dd.MM.yyyy */
function toTatilSepetiDate(isoDate: string): string {
  const [y, m, d] = isoDate.split("-");
  return `${d}.${m}.${y}`;
}

export async function fetchPricesFromTatilSepeti(
  hotelIds: string[],
  checkIn: string,
  checkOut: string,
  adults: number,
  children: number
): Promise<any> {
  const body: TatilSepetiPriceRequest = {
    AdultCount: adults,
    ChildCount: children,
    CampaignType: 0,
    CampaignId: "",
    checkinDate: `${toTatilSepetiDate(checkIn)} `,
    checkoutDate: toTatilSepetiDate(checkOut),
    HotelIds: hotelIds.join(","),
  };

  const response = await axios.post(
    TATILSEPETI_URL,
    qs.stringify(body),
    {
      headers: {
        ...DEFAULT_HEADERS,
        referer: "https://www.tatilsepeti.com/",
        cookie: config.providers.tatilsepeti.cookie,
      },
      timeout: 15000,
      proxy: getProxyConfig(),
    }
  );

  return response.data;
}

// ---- Paket (Ucak + Otel + Transfer) ----

export interface FlyingPackagesRawResponse {
  roomList: string;
  flightListResult: string;
  transferList: string;
  IsFlyingPackageOnline: boolean;
  CityCode: number;
  [key: string]: any;
}

/**
 * TatilSepeti'nin Search parametresi formatini olusturur.
 * Ornek: oda:2;tarih:26.07.2026,31.07.2026;click:true
 * Cocuklu: oda:2,5,8;tarih:... (2 yetiskin, 5 ve 8 yasinda cocuk)
 */
function buildPackageSearchString(
  adults: number,
  childAges: number[],
  checkIn: string,
  checkOut: string
): string {
  let odaPart = `oda:${adults}`;
  if (childAges.length > 0) {
    odaPart += "," + childAges.join(",");
  }
  return `${odaPart};tarih:${toTatilSepetiDate(checkIn)},${toTatilSepetiDate(checkOut)};click:true`;
}

export async function fetchFlyingPackages(
  hotelId: string,
  departureCode: string,
  destinationCode: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  childAges: number[]
): Promise<FlyingPackagesRawResponse> {
  const searchStr = buildPackageSearchString(adults, childAges, checkIn, checkOut);

  const body = qs.stringify({
    Search: searchStr,
    Id: hotelId,
    IsFlightPocket: "true",
    depatureCode: departureCode.toLowerCase(),
    destinationCode: destinationCode.toLowerCase(),
  });

  const headers: Record<string, string> = {
    ...DEFAULT_HEADERS,
    referer: "https://www.tatilsepeti.com/",
  };

  if (config.providers.tatilsepeti.cookie) {
    headers.cookie = config.providers.tatilsepeti.cookie;
  }
  if (config.providers.tatilsepeti.verificationToken) {
    headers.verificationtoken = config.providers.tatilsepeti.verificationToken;
  }

  const response = await axios.post<FlyingPackagesRawResponse>(
    TATILSEPETI_PACKAGE_URL,
    body,
    { headers, timeout: 30000, proxy: getProxyConfig() }
  );

  return response.data;
}
