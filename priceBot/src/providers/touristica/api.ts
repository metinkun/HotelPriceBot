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
/**
 * Otel sayfasini acar, cerezleri kabul eder, tarih/kisi doldurup
 * "Oda Ara" tetikler ve fiyatlarin yuklenmesini bekler.
 * Hem oda hem paket akisi bu adimlari paylasir.
 */
async function openAndSearch(
  page: any,
  detailUrl: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  children: number
): Promise<void> {
  await page.goto(detailUrl, { waitUntil: "networkidle2", timeout: 60000 });
  await acceptCookies(page);
  await new Promise((r) => setTimeout(r, 1500));

  const ci = toTouristicaDate(checkIn);
  const co = toTouristicaDate(checkOut);

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
      const adultEl = document.querySelector("#ddlAdultCount") as any;
      if (adultEl) {
        adultEl.value = String(ad);
        adultEl.dispatchEvent(new Event("change", { bubbles: true }));
      }
      const childEl = document.querySelector("#ddlChildCount") as any;
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

  // Fiyatlarin yuklenmesini bekle (max ~20sn)
  for (let i = 0; i < 20; i++) {
    await new Promise((r) => setTimeout(r, 1000));
    const ready = await page.evaluate(() =>
      /Gecelik Toplam Fiyat|Otel \+ Uçak/i.test(document.body?.innerText || "")
    );
    if (ready) break;
  }
}

/** Paket bloklari gorunene kadar bekler; gerekirse "Otel + Ucak" sekmesini tetikler. */
async function waitForPackages(page: any, maxSeconds = 25): Promise<boolean> {
  for (let i = 0; i < maxSeconds; i++) {
    const has = await page.evaluate(() =>
      /Otel \+ Uçak \+ Transfer Dahil\s*[\d.,]+\s*TL/i.test(
        document.body?.innerText || ""
      )
    );
    if (has) return true;

    // Birkac denemede bir paket sekmesini tetikle (lazy yukleniyor olabilir)
    if (i === 3 || i === 10) {
      await page.evaluate(() => {
        const tab = Array.from(
          document.querySelectorAll("a,button,li")
        ).find(
          (e: any) =>
            /otel\s*\+\s*uçak/i.test((e.textContent || "").trim()) &&
            (e.textContent || "").length < 60
        );
        if (tab) (tab as any).click();
      });
    }
    await new Promise((r) => setTimeout(r, 1000));
  }
  return false;
}

export interface TouristicaPackageCard {
  roomName: string | null;
  boardType: string | null;
  price: number | null;
}

/**
 * Paket (Otel + Ucak + Transfer) kartlarini doner.
 * NOT: Touristica otel sayfasinda kalkis havalimani secici YOKTUR;
 * paket fiyatlari sitenin varsayilan kalkisi ile gelir.
 */
export async function fetchPackagePrices(
  detailUrl: string,
  checkIn: string,
  checkOut: string,
  adults: number,
  children: number
): Promise<{ cards: TouristicaPackageCard[]; hotelName: string | null }> {
  const page = await newPage();
  try {
    await openAndSearch(page, detailUrl, checkIn, checkOut, adults, children);
    await waitForPackages(page);

    const data = await page.evaluate(() => {
      const BOARDS = [
        "Ultra Her Şey Dahil",
        "Her Şey Dahil",
        "Tam Pansiyon Plus",
        "Tam Pansiyon",
        "Yarım Pansiyon",
        "Oda Kahvaltı",
        "Sadece Oda",
      ];
      const seen = new Set<string>();
      const cards: any[] = [];

      document.querySelectorAll("*").forEach((el: any) => {
        const t = (el.innerText || "").replace(/\s+/g, " ").trim();
        if (!/Otel \+ Uçak \+ Transfer Dahil/i.test(t)) return;
        if (t.length > 400) return;

        const priceM = t.match(/Otel \+ Uçak \+ Transfer Dahil\s*([\d.,]+)\s*TL/i);
        if (!priceM) return;

        // Oda adi: kart icindeki h4.accommodation-type basligi (en guvenilir)
        let roomName: string | null = null;
        let cardText = t;
        let p: any = el;
        for (let i = 0; i < 10 && p; i++) {
          p = p.parentElement;
          if (!p) break;
          const head = p.querySelector(
            'h4.accommodation-type, h2,h3,h4,[class*="room-name"],[class*="room-title"]'
          );
          if (head) {
            const ht = (head.textContent || "").replace(/\s+/g, " ").trim();
            if (ht && ht.length < 70) {
              roomName = ht;
              cardText = (p.innerText || "").replace(/\s+/g, " ").trim();
              break;
            }
          }
        }

        // Pansiyon: kart metninden (oda adinda gecmiyorsa)
        let boardType: string | null = null;
        for (const b of BOARDS) {
          if (cardText.includes(b)) {
            boardType = b;
            break;
          }
        }
        if (roomName && boardType && roomName.includes(boardType)) {
          roomName = roomName.split(boardType)[0].trim();
        }

        const key = `${roomName}|${boardType}|${priceM[1]}`;
        if (seen.has(key)) return;
        seen.add(key);
        cards.push({ roomName, boardType, priceText: priceM[1] });
      });

      const h1 = document.querySelector("h1");
      return {
        cards,
        hotelName: h1 ? (h1.textContent || "").trim() : null,
      };
    });

    return {
      cards: data.cards.map((c: any) => ({
        roomName: c.roomName,
        boardType: c.boardType,
        price: Number(String(c.priceText).replace(/\./g, "").replace(",", ".")),
      })),
      hotelName: data.hotelName,
    };
  } finally {
    await page.close();
  }
}

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
