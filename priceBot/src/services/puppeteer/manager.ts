import { config } from "../../config";

let browser: any = null;

export async function getBrowser(): Promise<any> {
  if (browser && browser.isConnected()) {
    return browser;
  }

  // Lazy import: puppeteer sadece gerektiginde yuklenir
  const puppeteer = await import("puppeteer");
  browser = await puppeteer.default.launch({
    headless: config.puppeteer.headless,
    executablePath: config.puppeteer.executablePath || undefined,
    args: ["--no-sandbox", "--disable-setuid-sandbox"],
  });

  return browser;
}

export async function closeBrowser(): Promise<void> {
  if (browser) {
    await browser.close();
    browser = null;
  }
}
