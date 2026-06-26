import axios from "axios";
import qs from "qs";
import { config } from "../config";
import { TatilSepetiPriceRequest } from "../types";

const TATILSEPETI_URL =
  "https://www.tatilsepeti.com/hotel/GetHotelListPrice/";

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
    checkinDate: `${checkIn} `,
    checkoutDate: checkOut,
    HotelIds: hotelIds.join(","),
  };

  const response = await axios.post(
    TATILSEPETI_URL,
    qs.stringify(body),
    {
      headers: {
        ...DEFAULT_HEADERS,
        referer: "https://www.tatilsepeti.com/",
        cookie: config.tatilsepetiCookie,
      },
      timeout: 15000,
    }
  );

  return response.data;
}
