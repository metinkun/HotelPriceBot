import { getBrowser } from "../../services/puppeteer/manager";

const UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36";

/**
 * Touristica Cloudflare challenge (cf-mitigated: challenge) arkasinda.
 * cf_clearance cookie'si TLS parmak izine bagli oldugu icin axios'a
 * tasinamaz -> tum istekler headless tarayici icinden yapilir.
 */

/** yyyy-MM-dd -> dd.MM.yyyy */
export function toTouristicaDate(iso: string): string {
  const [y, m, d] = iso.slice(0, 10).split("-");
  return `${d}.${m}.${y}`;
}

export interface TouristicaRawPrice {
  /** "Sadece Otel" sekmesindeki ham metin bloklari */
  roomBlocks: string[];
  /** Otel + Ucak + Transfer paket metinleri */
  packageBlocks: string[];
  hotelName: string | null;
  city: string | null;
}

export interface TouristicaResolved {
  providerHotelId: string;
  hotelName: string | null;
  city: string | null;
  slug: string;
}

async function newPage(): Promise<any> {
  const browser = await getBrowser();
  const page = await browser.newPage();
  await page.setViewport({ width: 1440, height: 1000 });
  await page.setUserAgent(UA);
  return page;
}

/** Cerez onay banner'ini kapatir (fiyat panelini kapatiyor). */
async function acceptCookies(page: any): Promise<void> {
  try {
    await page.evaluate(() => {
      const b = Array.from(
        document.querySelectorAll("button,a")
      ).find((x: any) => /tümünü kabul/i.test(x.textContent || ""));
      if (b) (b as any).click();
    });
  } catch {
    /* banner yoksa yoksay */
  }
}

/**
 * Otel sayfasini acar, tarih/kisi doldurup "Oda Ara" tetikler ve
 * olusan oda/paket fiyat bloklarini ham metin olarak doner.
 */
export async function fetchRoomPrices(
  detailUrl: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  children: number
): Promise<TouristicaRawPrice> {
  const page = await newPage();
  try {
    await page.goto(detailUrl, {
      waitUntil: "networkidle2",
      timeout: 60000,
    });
    await acceptCookies(page);
    await new Promise((r) => setTimeout(r, 1500));

    const ci = toTouristicaDate(checkIn);
    const co = toTouristicaDate(checkOut);

    // Tarihleri yaz ve "Oda Ara" butonunu tetikle
    await page.evaluate(
      (inDate: string, outDate: string, ad: number, ch: number) => {
        const set = (sel: string, val: string) => {
          const el = document.querySelector(sel) as any;
          if (el) {
            el.value = val;
            el.setAttribute("data-value", val);
            el.dispatchEvent(new Event("change", { bubbles: true }));
          }
        };
        set("#txtCheckInDate", inDate);
        set("#txtCheckOutDate", outDate);
        const adultEl = document.querySelector(
          '[data-model-prop="AdultCount"], #ddlAdultCount'
        ) as any;
        if (adultEl) {
          adultEl.value = String(ad);
          adultEl.dispatchEvent(new Event("change", { bubbles: true }));
        }
        const childEl = document.querySelector(
          '[data-model-prop="ChildCount"], #ddlChildCount'
        ) as any;
        if (childEl) {
          childEl.value = String(ch);
          childEl.dispatchEvent(new Event("change", { bubbles: true }));
        }
        const btn = Array.from(
          document.querySelectorAll("button,a,input")
        ).find((b: any) => /oda ara/i.test(b.textContent || b.value || ""));
        if (btn) (btn as any).click();
      },
      ci,
      co,
      adults,
      children
    );

    // Fiyatlarin yuklenmesini bekle (max ~15sn)
    let loaded = false;
    for (let i = 0; i < 15; i++) {
      await new Promise((r) => setTimeout(r, 1000));
      loaded = await page.evaluate(() =>
        /Gecelik Toplam Fiyat|Fırsat Paketi/i.test(
          document.body?.innerText || ""
        )
      );
      if (loaded) break;
    }

    const data = await page.evaluate(() => {
      const roomBlocks: string[] = [];
      const packageBlocks: string[] = [];
      document
        .querySelectorAll('[class*="room"],[class*="package"]')
        .forEach((el: any) => {
          const t = (el.innerText || "").replace(/\s+/g, " ").trim();
          if (!/TL|₺/.test(t) || t.length < 25 || t.length > 300) return;
          if (/Otel \+ Uçak/i.test(t)) packageBlocks.push(t);
          else if (/Gecelik Toplam Fiyat/i.test(t)) roomBlocks.push(t);
        });
      const h1 = document.querySelector("h1");
      // Sehir: breadcrumb'in son "... Otelleri" linki (menu linklerini almamak icin
      // sadece breadcrumb kapsayicisi icinde ara)
      const crumbBox = document.querySelector(
        '[class*="breadcrumb"], [class*="bread-crumb"]'
      );
      const crumbLinks = crumbBox
        ? Array.from(crumbBox.querySelectorAll("a")).filter((a: any) =>
            /Otelleri$/.test((a.textContent || "").trim())
          )
        : [];
      const crumb = crumbLinks.length
        ? crumbLinks[crumbLinks.length - 1]
        : null;
      return {
        roomBlocks: Array.from(new Set(roomBlocks)).slice(0, 20),
        packageBlocks: Array.from(new Set(packageBlocks)).slice(0, 10),
        hotelName: h1 ? (h1.textContent || "").trim() : null,
        city: crumb ? (crumb.textContent || "").trim().replace(/\s*Otelleri$/i, "") : null,
      };
    });

    return data;
  } finally {
    await page.close();
  }
}

/** Otel detay URL'inden providerHotelId (data-hotel-id) ve adi cozer. */
export async function resolveHotelFromUrl(
  url: string
): Promise<TouristicaResolved> {
  const page = await newPage();
  try {
    await page.goto(url, { waitUntil: "domcontentloaded", timeout: 60000 });
    await new Promise((r) => setTimeout(r, 2000));

    const info = await page.evaluate(() => {
      const w: any = window as any;
      const detail = w.PageURLDetail || {};
      const h1 = document.querySelector("h1");
      // Sayfa icinde otel id'si: dlProduct / data-hotel-id / item_id
      let id: string | null = null;
      const dl = w.dataLayer;
      if (Array.isArray(dl)) {
        for (const entry of dl) {
          const items = entry?.ecommerce?.items || entry?.ecommerce?.detail?.products;
          if (Array.isArray(items) && items[0]) {
            id = String(items[0].item_id ?? items[0].id ?? "");
            if (id) break;
          }
        }
      }
      if (!id) {
        const el = document.querySelector("[data-hotel-id]");
        if (el) id = el.getAttribute("data-hotel-id");
      }
      // Sehir: breadcrumb'in son "... Otelleri" linki (menu linklerini almamak icin
      // sadece breadcrumb kapsayicisi icinde ara)
      const crumbBox = document.querySelector(
        '[class*="breadcrumb"], [class*="bread-crumb"]'
      );
      const crumbLinks = crumbBox
        ? Array.from(crumbBox.querySelectorAll("a")).filter((a: any) =>
            /Otelleri$/.test((a.textContent || "").trim())
          )
        : [];
      const crumb = crumbLinks.length
        ? crumbLinks[crumbLinks.length - 1]
        : null;
      return {
        id,
        slug: detail.URL || null,
        name: detail.URLNAME || (h1 ? (h1.textContent || "").trim() : null),
        isHotelDetail: !!detail.IsHotelDetailPage,
        city: crumb ? (crumb.textContent || "").trim().replace(/\s*Otelleri$/i, "") : null,
      };
    });

    if (!info.isHotelDetail && !info.slug) {
      throw new Error(
        "Touristica otel detay sayfasi degil (URL kontrol edilmeli)"
      );
    }

    // providerHotelId yoksa slug kullan (fiyat akisi URL uzerinden calisiyor)
    const providerHotelId = info.id || info.slug || "";
    if (!providerHotelId) {
      throw new Error("Touristica otel kimligi cozulemedi");
    }

    return {
      providerHotelId: String(providerHotelId),
      hotelName: info.name,
      city: info.city,
      slug: info.slug || url.split("/").pop() || "",
    };
  } finally {
    await page.close();
  }
}
