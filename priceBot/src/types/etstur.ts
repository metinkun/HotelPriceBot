// Etstur arama endpoint'ine gonderilen govde
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

// Etstur response modeli
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

// Paket akisi: /services/api/room ve /services/api/room/package
export interface EtsturRoomSearchResponse {
  success: boolean;
  errorCode: string | null;
  errorMessage: string | null;
  result?: {
    roomSearchId: string;
    checkIn?: string;
    checkOut?: string;
    rooms?: any[];
  };
}

export interface EtsturPackageBoard {
  id: string;
  boardType?: { code: string; label: string };
  price: EtsturPriceBlock;
  cancellation?: string;
  availability?: { type: string };
}

export interface EtsturPackageRoom {
  roomId: string;
  roomName: string;
  roomSize?: number;
  nightCount?: number;
  nightlyMinPrice?: EtsturPriceBlock;
  subBoards?: EtsturPackageBoard[];
}

export interface EtsturPackageResponse {
  success: boolean;
  errorCode: string | null;
  errorMessage: string | null;
  result?: {
    roomSearchId: string;
    rooms?: EtsturPackageRoom[];
  };
}
