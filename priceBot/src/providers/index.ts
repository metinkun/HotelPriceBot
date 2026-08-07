import { HotelProvider } from "./base";
import { EtsturProvider } from "./etstur";
import { TatilSepetiProvider } from "./tatilsepeti";
import { JollyProvider } from "./jolly";
import { TatilbudurProvider } from "./tatilbudur";
import { TouristicaProvider } from "./touristica";
import { SeturProvider } from "./setur";
import { GezinomiProvider } from "./gezinomi";

const providers = new Map<string, HotelProvider>();

function register(provider: HotelProvider) {
  providers.set(provider.name, provider);
}

// Provider'lari kaydet
register(new EtsturProvider());
register(new TatilSepetiProvider());
register(new JollyProvider());
register(new TatilbudurProvider());
register(new TouristicaProvider());
register(new SeturProvider());
register(new GezinomiProvider());

export function getProvider(name: string): HotelProvider {
  const provider = providers.get(name);
  if (!provider) {
    throw new Error(
      `Bilinmeyen provider: ${name}. Desteklenen: ${Array.from(providers.keys()).join(", ")}`
    );
  }
  return provider;
}

export function getAllProviderNames(): string[] {
  return Array.from(providers.keys());
}

export function getAllProviders(): HotelProvider[] {
  return Array.from(providers.values());
}
