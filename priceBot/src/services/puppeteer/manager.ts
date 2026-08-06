import { config } from "../../config";

let browser: any = null;

/** Puppeteer kendi Chromium'unu indirmediyse sistemdeki Chrome'a dus. */
const FALLBACK_CHROME_PATHS = [
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  "/usr/bin/google-chrome",
  "/usr/bin/chromium-browser",
  "/usr/bin/chromium",
];

function resolveExecutablePath(): string | undefined {
  if (config.puppeteer.executablePath) return config.puppeteer.executablePath;
  // Lazy require: fs sadece burada gerekli
  const fs = require("fs");
  for (const p of FALLBACK_CHROME_PATHS) {
    try {
      if (fs.existsSync(p)) return p;
    } catch {
      /* yoksay */
    }
  }
  return undefined; // puppeteer kendi indirdigi Chromium'u kullanir
}

export async function getBrowser(): Promise<any> {
  if (browser && browser.isConnected()) {
    return browser;
  }

  // Lazy import: puppeteer sadece gerektiginde yuklenir
  const puppeteer = await import("puppeteer");
  browser = await puppeteer.default.launch({
    headless: config.puppeteer.headless,
    executablePath: resolveExecutablePath(),
    args: [
      "--no-sandbox",
      "--disable-setuid-sandbox",
      "--disable-blink-features=AutomationControlled",
      "--disable-dev-shm-usage",
    ],
  });

  return browser;
}

export async function closeBrowser(): Promise<void> {
  if (browser) {
    await browser.close();
    browser = null;
  }
}
