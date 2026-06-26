import * as cheerio from "cheerio";
import { NormalizedPrice, NormalizedPackageRoom, NormalizedPackageBoard } from "../../types";

/**
 * TatilSepeti API'sinden gelen ham veriyi NormalizedPrice'a cevirir.
 * TatilSepeti response yapisi degisken olabilecegindan, mevcut bilinen
 * alanlari mapliyor, bilinmeyenleri null birakiyor.
 */
export function normalizeTatilSepetiResponse(
  data: any,
  providerHotelId: string
): NormalizedPrice {
  if (!data) {
    return {
      available: false,
      hotelName: null,
      city: null,
      roomName: null,
      boardType: null,
      currency: null,
      listPrice: null,
      price: null,
      discountRate: 0,
    };
  }

  let hotelData = data;
  if (Array.isArray(data)) {
    hotelData = data.find(
      (h: any) => String(h.HotelId) === providerHotelId || String(h.hotelId) === providerHotelId
    );
  }

  if (!hotelData) {
    return {
      available: false,
      hotelName: null,
      city: null,
      roomName: null,
      boardType: null,
      currency: null,
      listPrice: null,
      price: null,
      discountRate: 0,
    };
  }

  const price = hotelData.Price ?? hotelData.price ?? null;
  const listPrice = hotelData.ListPrice ?? hotelData.listPrice ?? hotelData.OldPrice ?? null;
  const discountRate = hotelData.DiscountRate ?? hotelData.discountRate ?? 0;
  const discountPrice = hotelData.DiscountPrice ?? hotelData.discountPrice ?? null;

  // listPrice yoksa ve discountRate varsa, orijinal fiyati hesapla
  let computedListPrice = listPrice ? Number(listPrice) : null;
  if (!computedListPrice && price && discountRate > 0) {
    computedListPrice = Math.round(Number(price) / (1 - Number(discountRate) / 100));
  }

  return {
    available: price !== null && price !== undefined,
    hotelName: hotelData.HotelName ?? hotelData.hotelName ?? null,
    city: hotelData.City ?? hotelData.city ?? hotelData.AreaName ?? null,
    roomName: hotelData.RoomName ?? hotelData.roomName ?? hotelData.RoomType ?? null,
    boardType: hotelData.BoardType ?? hotelData.boardType ?? hotelData.Accommodation ?? hotelData.AccommodationType ?? null,
    currency: hotelData.Currency ?? hotelData.currency ?? "TL",
    listPrice: computedListPrice,
    price: price ? Number(price) : null,
    discountRate: Number(discountRate),
    discountPrice: discountPrice ? Number(discountPrice) : undefined,
    campaignName: hotelData.CampaignName ?? undefined,
  };
}

// ---- Paket HTML parse ----

interface InstallmentData {
  [key: string]: string;
}

/**
 * getInstallment onclick handler'indan yapisal veri cikarir.
 * Ornek: hotelDetail.getInstallment(event, { RoomTypeId: '25', FlightPocketPrice: '92558', ... })
 */
function parseGetInstallmentData(onclick: string): InstallmentData | null {
  // getInstallment(event, { ... }, '160000') formatinda - } sonrasi , veya ) gelebilir
  const match = onclick.match(/\{([\s\S]+)\}/);
  if (!match) return null;

  const data: InstallmentData = {};
  const kvRegex = /(\w+)\s*:\s*(?:'([^']*)'|"([^"]*)"|([+-]?\d+(?:\.\d+)?)|(\w+))/g;
  let m: RegExpExecArray | null;
  while ((m = kvRegex.exec(match[1])) !== null) {
    data[m[1]] = m[2] ?? m[3] ?? m[4] ?? m[5] ?? "";
  }

  return Object.keys(data).length > 0 ? data : null;
}

/**
 * TatilSepeti GetFlyingPackages'dan gelen roomList HTML'ini parse eder.
 * Her oda karti icindeki getInstallment onclick handler'indan fiyat bilgisi cikarilir.
 */
export function parsePackageRoomList(
  html: string,
  nightCount: number
): NormalizedPackageRoom[] {
  if (!html) return [];

  const $ = cheerio.load(html);
  const roomMap = new Map<string, NormalizedPackageRoom>();

  // Her oda karti: div[id^="roomName_flight_"]
  $('[id^="roomName_flight_"]').each((_i, cardEl) => {
    const $card = $(cardEl);
    const idAttr = $card.attr("id") || "";
    const cardRoomId = idAttr.replace("roomName_flight_", "");
    const roomName = $card.find("h3").first().text().trim();

    // Oda boyutu (m²)
    let roomSize: number | null = null;
    $card.find("li span").each((_j, spanEl) => {
      const text = $(spanEl).text();
      const sizeMatch = text.match(/(\d+)\s*m/i);
      if (sizeMatch && !roomSize) {
        roomSize = parseInt(sizeMatch[1], 10);
      }
    });

    // Ucretsiz iptal var mi?
    const hasCancellation =
      $card.find('[class*="free-cancel"], [class*="cancelation"]').length > 0;

    // DOM yapisi: pension-types ve multiple div'leri sibling olarak siralanir.
    //   __price-div
    //     ├── __pension-types (Board 1 etiketi)
    //     ├── __multiple     (Board 1 fiyat + getInstallment)
    //     ├── __pension-types (Board 2 etiketi)
    //     └── __multiple     (Board 2 fiyat + getInstallment)
    // Her __pension-types'in next sibling'i ilgili __multiple'dir.
    $card
      .find('[class*="pension-types"]:not([class*="__badge"])')
      .each((_j, pensionEl) => {
        const $pension = $(pensionEl);
        const boardType =
          $pension.find("span").first().text().trim() || "Standard";

        // Sonraki sibling __multiple icinde getInstallment'i bul
        const $multiple = $pension.nextAll('[class*="multiple"]').first();
        if (!$multiple.length) return;

        const onclickEl = $multiple.find("[onclick*='getInstallment']").first();
        if (!onclickEl.length) return;

        const onclick = onclickEl.attr("onclick") || "";
        const data = parseGetInstallmentData(onclick);
        if (!data || !data.FlightPocketPrice) return;

        const roomTypeId = data.RoomTypeId || cardRoomId;
        const totalPrice = Number(data.FlightPocketPrice) || 0;

        const board: NormalizedPackageBoard = {
          boardType,
          currency: "TL",
          listPrice: null,
          price: totalPrice,
          discountRate: 0,
          cancellation: hasCancellation ? "FREE_CANCELLATION" : null,
        };

        const existing = roomMap.get(roomTypeId);
        if (existing) {
          existing.boards.push(board);
        } else {
          roomMap.set(roomTypeId, {
            roomId: roomTypeId,
            roomName: roomName || `Room ${roomTypeId}`,
            roomSize,
            nightCount,
            boards: [board],
          });
        }
      });
  });

  // Fallback: getInstallment bulunamazsa, display fiyatlardan parse et
  if (roomMap.size === 0) {
    $('[id^="roomName_flight_"]').each((_i, cardEl) => {
      const $card = $(cardEl);
      const idAttr = $card.attr("id") || "";
      const cardRoomId = idAttr.replace("roomName_flight_", "");
      const roomName = $card.find("h3").first().text().trim();

      const boards: NormalizedPackageBoard[] = [];

      $card.find('[class*="discount-price"]').each((_j, priceEl) => {
        const priceText = $(priceEl).text().trim();
        const price = parseTurkishPrice(priceText);
        if (price > 0) {
          const $section = $(priceEl).closest('[class*="price"]');
          const boardType =
            $section
              .find('[class*="pension"] span')
              .first()
              .text()
              .trim() || "Standard";

          boards.push({
            boardType,
            currency: "TL",
            listPrice: null,
            price,
            discountRate: 0,
            cancellation: null,
          });
        }
      });

      if (boards.length > 0) {
        roomMap.set(cardRoomId, {
          roomId: cardRoomId,
          roomName: roomName || `Room ${cardRoomId}`,
          roomSize: null,
          nightCount,
          boards,
        });
      }
    });
  }

  return Array.from(roomMap.values());
}

/**
 * Turk lokali fiyat formatini parse eder.
 * Ornek: "92.558,00 TL" -> 92558
 */
function parseTurkishPrice(text: string): number {
  // "92.558,00 TL" -> "92558.00" -> 92558
  const cleaned = text
    .replace(/[^\d.,]/g, "") // TL ve diger harfleri kaldir
    .replace(/\./g, "")      // Binlik ayiracini kaldir
    .replace(",", ".");      // Ondalik ayiracini duzelt
  return Math.round(Number(cleaned)) || 0;
}
