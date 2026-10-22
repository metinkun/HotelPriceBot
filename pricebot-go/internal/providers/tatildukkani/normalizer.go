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

// normalizePrice socket yanitini lead (en ucuz) teklif olarak NormalizedPrice'a cevirir.
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
		return types.Unavailable(hotelName, city)
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
