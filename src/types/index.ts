export interface PriceQuery {
  hotelId: string;
  checkIn: string; // yyyy-MM-dd (ornek: 2026-06-24)
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

// Etstur arama endpoint'ine (services/api/search/v2/hotels) gonderilen govde.
export interface EtsturRoomRequest {
  adultCount: number;
  childCount: number;
  childAges: number[];
  infantCount: number;
}

export interface EtsturSearchRequest {
  checkIn: string; // ISO: 2026-06-24T00:00:00.000
  checkOut: string;
  rooms: EtsturRoomRequest[];
  url: string; // destinasyon slug: "Kemer-Otelleri"
  offset: number;
  limit: number;
  minPrice: number | null;
  maxPrice: number | null;
  filters: any[];
  loc: null;
  zoom: null;
  radiusKm: null;
  excludeHotelIds: string[];
  sort: { type: string; direction: string };
}

export interface HotelMappingInput {
  internalHotelId: string;
  etsturHotelId: string;
  destinationUrl: string; // "Kemer-Otelleri"
  hotelName?: string;
  isActive?: boolean;
}

// ---- Etstur response modeli (services/api/search/v2/hotels) ----
export interface EtsturPriceBlock {
  currency: string;
  amount: number;
  discountedPrice: number | null;
  discountRate: number;
}

export interface EtsturRoom {
  roomNames: string[];
  boardType: string[];
  boardTypeLabel: string[];
  nightCount: number;
  availability?: { type: string; nightCount: number; availableDate: string | null };
  price: EtsturPriceBlock;
  campaignHighlightedPrice?: {
    label: string;
    type: string;
    price: EtsturPriceBlock;
  } | null;
}

export interface EtsturHotel {
  hotelId: string;
  name: string;
  url: string;
  location?: { city?: string; state?: string; label?: string };
  room?: EtsturRoom | null;
}

export interface EtsturSearchResponse {
  success: boolean;
  errorCode: string | null;
  errorMessage: string | null;
  result?: {
    totalCount: number;
    offset: number;
    limit: number;
    hotels: EtsturHotel[];
  };
}

// Bizim API'mizin dondurecegi sade fiyat ozeti
export interface NormalizedPrice {
  available: boolean;
  hotelName: string | null;
  city: string | null;
  roomName: string | null;
  boardType: string | null; // ornek: "Ultra Her Sey Dahil"
  currency: string | null;
  listPrice: number | null; // amount
  price: number | null; // discountedPrice ?? amount
  discountRate: number;
  bankCampaignPrice: number | null; // campaignHighlightedPrice
  bankCampaignLabel: string | null;
  minStayNights: number | null;
}
