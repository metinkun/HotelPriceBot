import { NormalizedPrice, NormalizedPackageRoom } from "../../types";
import { JollyReservationResponse } from "./api";

/** HTML entity'lerini cozer (&#x131; gibi hex/ondalik kodlar dahil). */
function decode(s: string): string {
  return s
    .replace(/&#x([0-9a-fA-F]+);/g, (_, hex) =>
      String.fromCodePoint(parseInt(hex, 16))
    )
    .replace(/&#(\d+);/g, (_, dec) => String.fromCodePoint(Number(dec)))
    .replace(/&amp;/g, "&")
    .replace(/&nbsp;/g, " ")
    .replace(/&quot;/g, '"')
    .replace(/\s+/g, " ")
    .trim();
}

/** html'deki ilk (en ucuz) odanin adi ve pansiyon tipini cikarir. */
function firstRoom(html: string): { roomName: string | null; boardType: string | null } {
  if (!html) return { roomName: null, boardType: null };
  const nameM = html.match(/room-title"[^>]*>\s*([^<]{2,60})</);
  const boardM = html.match(/room-concept[^>]*data-concept="([^"]*)"/);
  return {
    roomName: nameM ? decode(nameM[1]) : null,
    boardType: boardM ? decode(boardM[1].replace(/-/g, " ")) : null,
  };
}

/** Jolly yanitini ortak NormalizedPrice formatina cevirir (lead/en ucuz teklif). */
export function normalizeJolly(data: JollyReservationResponse): NormalizedPrice {
  const a = data.analyticsDataModel || {};
  const listPrice = a.totalPrice ?? null;
  const price = a.discountedPrice ?? a.totalPrice ?? null;
  const discountRate =
    listPrice && price && listPrice > price
      ? Math.round((1 - price / listPrice) * 100)
      : 0;

  const { roomName, boardType } = firstRoom(data.html || "");
  const available = !!data.isAvailableHotel && price != null && price > 0;

  return {
    available,
    hotelName: a.productName ?? null,
    city: a.cityName ?? null,
    roomName,
    boardType,
    currency: a.currency === "TRY" ? "TL" : a.currency ?? null,
    listPrice,
    price,
    discountRate,
  };
}

/** "104.546,04" -> 104546.04 (TR formati) */
function parseTrPrice(s: string): number | null {
  const n = Number(s.replace(/\./g, "").replace(",", "."));
  return Number.isFinite(n) && n > 0 ? n : null;
}

/**
 * Paket yanitindaki (packageSearchType=0) oda kartlarini
 * NormalizedPackageRoom listesine cevirir.
 * Kart yapisi: room-title -> pansiyon -> old-price / current-price
 */
export function normalizeJollyPackages(
  data: JollyReservationResponse,
  nightCount: number | null
): NormalizedPackageRoom[] {
  const html = data?.html || "";
  if (!html) return [];

  // Oda kartlarini room-title'a gore bol
  const starts: number[] = [];
  const re = /room-title"[^>]*>\s*([^<]{2,60})/g;
  let m: RegExpExecArray | null;
  const names: string[] = [];
  while ((m = re.exec(html)) !== null) {
    starts.push(m.index);
    names.push(decode(m[1]));
  }

  const rooms: NormalizedPackageRoom[] = [];
  const seen = new Set<string>();

  for (let i = 0; i < starts.length; i++) {
    const seg = html.slice(starts[i], starts[i + 1] ?? starts[i] + 6000);

    const listM = seg.match(/old-price"[^>]*>\s*([0-9.,]+)\s*TL/);
    // current-price ikiye bolunmus olabilir: <b>63.656<small>,04 TL</small></b>
    const curM = seg.match(
      /current-price"[^>]*>\s*<b>\s*([0-9.]+)\s*<small>\s*([,0-9]*)/
    );
    const price = curM
      ? parseTrPrice(curM[1] + (curM[2] || ""))
      : null;
    const listPrice = listM ? parseTrPrice(listM[1]) : null;
    if (price == null) continue;

    // Pansiyon adi entity'li gelebiliyor (Yar&#x131;m Pansiyon) -> once coz
    const segText = decode(seg.replace(/<[^>]+>/g, " "));
    const boardM = segText.match(
      /(Ultra Her Şey Dahil|Her Şey Dahil|Yarım Pansiyon|Tam Pansiyon Plus|Tam Pansiyon|Oda Kahvaltı|Sadece Oda)/
    );
    const boardType = boardM ? boardM[1] : null;
    const roomName = names[i] || "Oda";

    const key = `${roomName}|${boardType}|${price}`;
    if (seen.has(key)) continue;
    seen.add(key);

    const discountRate =
      listPrice && listPrice > price
        ? Math.round((1 - price / listPrice) * 100)
        : 0;

    const existing = rooms.find((r) => r.roomName === roomName);
    const board = {
      boardType,
      currency: "TL",
      listPrice,
      price,
      discountRate,
      cancellation: null,
    };
    if (existing) existing.boards.push(board);
    else
      rooms.push({
        roomId: String(i),
        roomName,
        roomSize: null,
        nightCount,
        boards: [board],
      });
  }

  return rooms;
}
