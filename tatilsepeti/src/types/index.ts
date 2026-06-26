export interface PriceQuery {
  hotelId: string;
  checkIn: string; // dd.MM.yyyy
  checkOut: string; // dd.MM.yyyy
  adults: number;
  children: number;
}

export interface BulkPriceQuery {
  hotelIds: string[];
  checkIn: string;
  checkOut: string;
  adults: number;
  children: number;
}

export interface TatilSepetiPriceRequest {
  AdultCount: number;
  ChildCount: number;
  CampaignType: number;
  CampaignId: string;
  checkinDate: string;
  checkoutDate: string;
  HotelIds: string;
}

export interface HotelMappingInput {
  internalHotelId: string;
  tatilsepetiHotelId: string;
  hotelName?: string;
  isActive?: boolean;
}
