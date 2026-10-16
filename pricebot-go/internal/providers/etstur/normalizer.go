package etstur

import "pricebot/internal/types"

func normalizeHotel(h *Hotel) *types.NormalizedPrice {
	room := h.Room
	var price *PriceBlock
	if room != nil {
		price = room.Price
	}
	out := &types.NormalizedPrice{
		Available: price != nil,
		HotelName: types.StrOrNil(h.Name),
	}
	if h.Location != nil {
		out.City = types.StrOrNil(h.Location.City)
	}
	if room != nil {
		if len(room.RoomNames) > 0 {
			out.RoomName = types.Str(room.RoomNames[0])
		}
		if len(room.BoardTypeLabel) > 0 {
			out.BoardType = types.Str(room.BoardTypeLabel[0])
		}
		if room.CampaignHighlightedPrice != nil {
			if room.CampaignHighlightedPrice.Price != nil {
				out.BankCampaignPrice = types.F64(room.CampaignHighlightedPrice.Price.Amount)
			}
			out.BankCampaignLabel = types.StrOrNil(room.CampaignHighlightedPrice.Label)
		}
		if room.Availability != nil {
			out.MinStayNights = types.Int(room.Availability.NightCount)
		}
	}
	if price != nil {
		out.Currency = types.StrOrNil(price.Currency)
		out.ListPrice = types.F64(price.Amount)
		if price.DiscountedPrice != nil {
			out.Price = types.F64(*price.DiscountedPrice)
		} else {
			out.Price = types.F64(price.Amount)
		}
		out.DiscountRate = price.DiscountRate
	}
	return out
}

func normalizePackageRoom(r PackageRoom) types.NormalizedPackageRoom {
	boards := make([]types.NormalizedPackageBoard, 0, len(r.SubBoards))
	for _, sb := range r.SubBoards {
		b := types.NormalizedPackageBoard{Cancellation: sb.Cancellation}
		if sb.BoardType != nil {
			b.BoardType = types.StrOrNil(sb.BoardType.Label)
		}
		if sb.Price != nil {
			b.Currency = types.StrOrNil(sb.Price.Currency)
			b.ListPrice = types.F64(sb.Price.Amount)
			if sb.Price.DiscountedPrice != nil {
				b.Price = types.F64(*sb.Price.DiscountedPrice)
			} else {
				b.Price = types.F64(sb.Price.Amount)
			}
			b.DiscountRate = sb.Price.DiscountRate
		}
		boards = append(boards, b)
	}
	return types.NormalizedPackageRoom{
		RoomID: r.RoomID, RoomName: r.RoomName, RoomSize: r.RoomSize, NightCount: r.NightCount, Boards: boards,
	}
}
