// ---- Shared query types (standardized: yyyy-MM-dd) ----

export interface PriceQuery {
  hotelId: string;
  checkIn: string; // yyyy-MM-dd
  checkOut: string; // yyyy-MM-dd
  adults: number;
  children: number;
  childAges?: number[];
}

export interface BulkPriceQuery {
  hotelIds: string[];
  checkIn: string;
  checkOut: string;
  adults: number;
  children: number;
  childAges?: number[];
}

export interface PackageQuery {
  hotelId: string;
  airportCode: string; // IATA kodu, ornek: IST
  checkIn: string; // yyyy-MM-dd
  checkOut: string; // yyyy-MM-dd
  adults: number;
  children: number;
  childAges?: number[];
}

// ---- Normalized output types (tum provider'lar bu formatta doner) ----

export interface NormalizedPrice {
  available: boolean;
  hotelName: string | null;
  city: string | null;
  roomName: string | null;
  boardType: string | null;
  currency: string | null;
  listPrice: number | null;
  price: number | null;
  discountRate: number;
  discountPrice?: number;
  campaignName?: string;
  bankCampaignPrice?: number | null;
  bankCampaignLabel?: string | null;
  minStayNights?: number | null;
}

export interface NormalizedPackageBoard {
  boardType: string | null;
  currency: string | null;
  listPrice: number | null;
  price: number | null;
  discountRate: number;
  cancellation: string | null;
}

export interface NormalizedPackageRoom {
  roomId: string;
  roomName: string;
  roomSize: number | null;
  nightCount: number | null;
  boards: NormalizedPackageBoard[];
}
