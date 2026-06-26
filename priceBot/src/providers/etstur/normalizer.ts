import { NormalizedPrice, NormalizedPackageRoom } from "../../types";
import { EtsturHotel, EtsturPackageRoom } from "../../types/etstur";

export function normalizeHotel(hotel: EtsturHotel): NormalizedPrice {
  const room = hotel.room;
  const price = room?.price;

  return {
    available: !!price,
    hotelName: hotel.name ?? null,
    city: hotel.location?.city ?? null,
    roomName: room?.roomNames?.[0] ?? null,
    boardType: room?.boardTypeLabel?.[0] ?? null,
    currency: price?.currency ?? null,
    listPrice: price?.amount ?? null,
    price: price ? price.discountedPrice ?? price.amount : null,
    discountRate: price?.discountRate ?? 0,
    bankCampaignPrice: room?.campaignHighlightedPrice?.price?.amount ?? null,
    bankCampaignLabel: room?.campaignHighlightedPrice?.label ?? null,
    minStayNights: room?.availability?.nightCount ?? null,
  };
}

export function normalizePackageRoom(
  room: EtsturPackageRoom
): NormalizedPackageRoom {
  return {
    roomId: room.roomId,
    roomName: room.roomName,
    roomSize: room.roomSize ?? null,
    nightCount: room.nightCount ?? null,
    boards: (room.subBoards ?? []).map((sb) => ({
      boardType: sb.boardType?.label ?? null,
      currency: sb.price?.currency ?? null,
      listPrice: sb.price?.amount ?? null,
      price: sb.price ? sb.price.discountedPrice ?? sb.price.amount : null,
      discountRate: sb.price?.discountRate ?? 0,
      cancellation: sb.cancellation ?? null,
    })),
  };
}
