import { PriceQuery, BulkPriceQuery, PackageQuery, NormalizedPrice, NormalizedPackageRoom } from "../types";

// ---- Provider result types ----

export interface ProviderPriceResult {
  internalHotelId: string;
  providerHotelId: string;
  hotelName: string | null;
  found: boolean;
  price: NormalizedPrice | null;
  durationMs?: number;
  error?: string;
}

export interface ProviderPackageResult {
  internalHotelId: string;
  providerHotelId: string;
  hotelName: string | null;
  airportCode: string;
  checkIn: string;
  checkOut: string;
  available: boolean;
  roomCount: number;
  rooms: NormalizedPackageRoom[];
  durationMs?: number;
}

export interface ProviderMapping {
  id: number;
  provider: string;
  internalHotelId: string;
  providerHotelId: string;
  hotelName: string | null;
  metadata: Record<string, any> | null;
  isActive: boolean;
}

export interface ProviderMappingInput {
  internalHotelId: string;
  providerHotelId: string;
  hotelName?: string;
  isActive?: boolean;
  metadata?: Record<string, any>;
}

export interface ProviderCapabilities {
  supportsPackages: boolean;
  supportsBulkPrices: boolean;
  requiresCookie: boolean;
  requiresPuppeteer: boolean;
}

// ---- Provider interface ----

export interface HotelProvider {
  readonly name: string;
  readonly capabilities: ProviderCapabilities;

  getPrice(mapping: ProviderMapping, query: PriceQuery): Promise<ProviderPriceResult>;
  getBulkPrices(mappings: ProviderMapping[], query: BulkPriceQuery): Promise<ProviderPriceResult[]>;
  getPackagePrices?(mapping: ProviderMapping, query: PackageQuery): Promise<ProviderPackageResult>;

  /** Mapping olusturulurken provider-spesifik validasyon. Hata mesaji doner, gecerliyse null. */
  validateMappingInput(input: ProviderMappingInput): string | null;

  /** Bu provider icin zorunlu metadata alanlari. */
  requiredMetadataFields(): string[];
}

// ---- Base abstract class (default bulk fallback) ----

export abstract class BaseProvider implements HotelProvider {
  abstract readonly name: string;
  abstract readonly capabilities: ProviderCapabilities;

  abstract getPrice(mapping: ProviderMapping, query: PriceQuery): Promise<ProviderPriceResult>;

  /** Default: tek tek sorgular. Provider native bulk destekliyorsa override edebilir. */
  async getBulkPrices(mappings: ProviderMapping[], query: BulkPriceQuery): Promise<ProviderPriceResult[]> {
    const results: ProviderPriceResult[] = [];
    for (const mapping of mappings) {
      try {
        const result = await this.getPrice(mapping, {
          hotelId: mapping.internalHotelId,
          checkIn: query.checkIn,
          checkOut: query.checkOut,
          adults: query.adults,
          children: query.children,
          childAges: query.childAges,
        });
        results.push(result);
      } catch (err: any) {
        results.push({
          internalHotelId: mapping.internalHotelId,
          providerHotelId: mapping.providerHotelId,
          hotelName: mapping.hotelName,
          found: false,
          price: null,
          error: err.message,
        });
      }
    }
    return results;
  }

  abstract validateMappingInput(input: ProviderMappingInput): string | null;

  requiredMetadataFields(): string[] {
    return [];
  }
}
