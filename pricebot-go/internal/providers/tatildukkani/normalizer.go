package tatildukkani

import (
	"strings"

	"pricebot/internal/types"
)

// roomTier odanin satis durumu: 0 = AVAILABLE (aninda satis), 1 = ON_REQUEST (site fiyati gosterir,
// "talep" butonu), -1 = satilamaz (STOP_SALE / NO_ROOMS "Dolu", fiyatsiz veya bilinmeyen durum).
func roomTier(r Room) int {
	if r.Fee <= 0 {
		return -1
	}
	switch strings.ToUpper(strings.TrimSpace(r.Status)) {
	case "AVAILABLE":
		return 0
	case "ON_REQUEST":
		return 1
	}
	return -1
}

// cheapestRoom once AVAILABLE odalarin en ucuzu; hic yoksa ON_REQUEST odalarin en ucuzu; yoksa nil.
func cheapestRoom(rooms []Room) *Room {
	var best *Room
	bestTier := 2
	for i := range rooms {
		r := &rooms[i]
		t := roomTier(*r)
		if t < 0 {
			continue
		}
		if t < bestTier || (t == bestTier && r.Fee < best.Fee) {
			best, bestTier = r, t
		}
	}
	return best
}

func trCurrency(c string) *string {
	if c == "" || strings.EqualFold(c, "TRY") || strings.EqualFold(c, "TL") {
		return types.Str("TL")
	}
	return types.Str(c)
}

// roomOffers tum odalar: AVAILABLE -> available, ON_REQUEST -> on_request, digerleri (STOP_SALE, NO_ROOMS) sold_out.
func roomOffers(list []Room) []types.RoomOffer {
	rooms := []types.RoomOffer{}
	for _, r := range list {
		if r.Fee <= 0 {
			continue
		}
		st := types.RoomSoldOut
		switch roomTier(r) {
		case 0:
			st = types.RoomAvailable
		case 1:
			st = types.RoomOnRequest
		}
		ro := types.NewRoom(r.RoomTitle, r.ConceptTitle, float64(r.Fee), float64(r.PosterPrice), r.Currency, st)
		if len(r.RoomCampaign) > 0 && r.RoomCampaign[0].Name != "" {
			ro.CampaignName = types.Str(r.RoomCampaign[0].Name)
		}
		rooms = append(rooms, ro)
	}
	return types.SortRooms(rooms)
}

// normalizePrice oda listesini lead (en ucuz) teklif olarak NormalizedPrice'a cevirir; Rooms tum odalar.
// Fiyatlar konaklama toplamidir (gecelik degil).
func normalizePrice(page *HotelPage, av *AvailableHotel) *types.NormalizedPrice {
	var hotelName, city *string
	if page != nil {
		hotelName, city = types.StrOrNil(page.Title), types.StrOrNil(page.City)
	}
	if av == nil {
		return types.Unavailable(hotelName, city)
	}
	best := cheapestRoom(av.Rooms)
	if best == nil {
		u := types.Unavailable(hotelName, city)
		u.Rooms = roomOffers(av.Rooms)
		return u
	}
	price := types.F64Pos(float64(best.Fee))
	var listPrice *float64
	if lp := float64(best.PosterPrice); lp > *price {
		listPrice = types.F64(lp)
	}
	var campaign *string
	if len(best.RoomCampaign) > 0 && best.RoomCampaign[0].Name != "" {
		campaign = types.Str(best.RoomCampaign[0].Name)
	}
	return &types.NormalizedPrice{
		Rooms:        roomOffers(av.Rooms),
		Available:    true,
		HotelName:    hotelName,
		City:         city,
		RoomName:     types.StrOrNil(strings.TrimSpace(best.RoomTitle)),
		BoardType:    types.StrOrNil(strings.TrimSpace(best.ConceptTitle)),
		Currency:     trCurrency(best.Currency),
		ListPrice:    listPrice,
		Price:        price,
		DiscountRate: types.DiscountRateOf(listPrice, price),
		CampaignName: campaign,
	}
}
