import { PrismaClient } from "@prisma/client";
import axios, { AxiosProxyConfig } from "axios";
import { config } from "../config";

const prisma = new PrismaClient();

// ---- Webshare API types ----

interface WebshareProxy {
  id: string;
  username: string;
  password: string;
  proxy_address: string;
  port: number;
  valid: boolean;
  last_verification: string;
  country_code: string;
  city_name: string;
}

interface WebshareListResponse {
  count: number;
  next: string | null;
  previous: string | null;
  results: WebshareProxy[];
}

// ---- In-memory round-robin state ----

interface CachedProxy {
  host: string;
  port: number;
  username: string;
  password: string;
}

let cachedProxies: CachedProxy[] = [];
let currentIndex = 0;

// ---- Sync: Webshare API -> DB -> Memory ----

export async function syncProxiesFromWebshare(): Promise<number> {
  const { apiKey } = config.webshare;
  if (!apiKey) {
    console.warn("[ProxyService] WEBSHARE_API_KEY ayarlanmamis, sync atlaniyor");
    return 0;
  }

  let allProxies: WebshareProxy[] = [];
  let page = 1;
  let hasNext = true;

  while (hasNext) {
    const url = `https://proxy.webshare.io/api/v2/proxy/list/?mode=direct&page=${page}&page_size=100`;
    const response = await axios.get<WebshareListResponse>(url, {
      headers: { Authorization: `Token ${apiKey}` },
      timeout: 15000,
    });

    allProxies = allProxies.concat(response.data.results);
    hasNext = response.data.next !== null;
    page++;
  }

  // Upsert each proxy into DB
  for (const p of allProxies) {
    await prisma.proxy.upsert({
      where: { webshareId: p.id },
      update: {
        proxyAddress: p.proxy_address,
        port: p.port,
        username: p.username,
        password: p.password,
        countryCode: p.country_code || null,
        valid: p.valid,
        lastVerification: p.last_verification
          ? new Date(p.last_verification)
          : null,
      },
      create: {
        webshareId: p.id,
        proxyAddress: p.proxy_address,
        port: p.port,
        username: p.username,
        password: p.password,
        countryCode: p.country_code || null,
        valid: p.valid,
        lastVerification: p.last_verification
          ? new Date(p.last_verification)
          : null,
      },
    });
  }

  // Remove proxies no longer in Webshare list
  const activeIds = allProxies.map((p) => p.id);
  if (activeIds.length > 0) {
    await prisma.proxy.deleteMany({
      where: { webshareId: { notIn: activeIds } },
    });
  }

  // Refresh in-memory cache
  await refreshProxyCache();

  console.log(`[ProxyService] ${allProxies.length} proxy senkronize edildi`);
  return allProxies.length;
}

// ---- In-memory cache refresh from DB ----

async function refreshProxyCache(): Promise<void> {
  const proxies = await prisma.proxy.findMany({
    where: { valid: true },
    orderBy: { id: "asc" },
  });

  cachedProxies = proxies.map((p) => ({
    host: p.proxyAddress,
    port: p.port,
    username: p.username,
    password: p.password,
  }));

  if (currentIndex >= cachedProxies.length) {
    currentIndex = 0;
  }
}

// ---- Round-robin proxy selector ----

export function getNextProxy(): AxiosProxyConfig | false {
  if (cachedProxies.length > 0) {
    const proxy = cachedProxies[currentIndex % cachedProxies.length];
    currentIndex = (currentIndex + 1) % cachedProxies.length;
    return {
      host: proxy.host,
      port: proxy.port,
      auth: { username: proxy.username, password: proxy.password },
      protocol: "http",
    };
  }

  // Fallback: static config
  const { host, port, username, password } = config.proxy;
  if (!host || !port) return false;
  return { host, port, auth: { username, password }, protocol: "http" };
}

// ---- Scheduler ----

let syncIntervalHandle: ReturnType<typeof setInterval> | null = null;

export async function startProxySync(): Promise<void> {
  // Initial sync
  try {
    await syncProxiesFromWebshare();
    console.log("[ProxyService] Baslangic senkronizasyonu tamamlandi");
  } catch (err: any) {
    console.error(
      "[ProxyService] Baslangic senkronizasyonu basarisiz:",
      err.message
    );
    // Try to load from DB (previous sync data)
    await refreshProxyCache();
  }

  // Periodic sync
  syncIntervalHandle = setInterval(async () => {
    try {
      await syncProxiesFromWebshare();
    } catch (err: any) {
      console.error("[ProxyService] Proxy senkronizasyonu basarisiz:", err.message);
    }
  }, config.webshare.syncIntervalMs);

  console.log(
    `[ProxyService] Proxy senkronizasyonu her ${config.webshare.syncIntervalMs / 1000}s ayarlandi`
  );
}

export function stopProxySync(): void {
  if (syncIntervalHandle) {
    clearInterval(syncIntervalHandle);
    syncIntervalHandle = null;
  }
}
