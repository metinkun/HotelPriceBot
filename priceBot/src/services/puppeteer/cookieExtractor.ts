import { getBrowser } from "./manager";

export interface ExtractedCookies {
  cookieString: string;
  expiresAt: Date;
}

/**
 * Headless browser ile bir URL'yi ziyaret edip cookie'leri toplar.
 * Cookie gerektiren provider'lar icin kullanilabilir.
 */
export async function extractCookies(
  url: string,
  waitForSelector?: string
): Promise<ExtractedCookies> {
  const browser = await getBrowser();
  const page = await browser.newPage();

  try {
    await page.goto(url, { waitUntil: "networkidle2", timeout: 30000 });
    if (waitForSelector) {
      await page.waitForSelector(waitForSelector, { timeout: 10000 });
    }

    const cookies = await page.cookies();
    const cookieString = cookies
      .map((c: any) => `${c.name}=${c.value}`)
      .join("; ");
    const maxExpiry = Math.max(
      ...cookies.map((c: any) => c.expires || 0)
    );

    return {
      cookieString,
      expiresAt: new Date(maxExpiry * 1000),
    };
  } finally {
    await page.close();
  }
}
