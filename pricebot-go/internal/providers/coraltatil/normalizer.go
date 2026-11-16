package coraltatil

import (
	"strings"

	"pricebot/internal/types"
)

// roomStatus availability 1 = satista; stopSale true -> satis disi; diger degerler "sorulacak" kabul edilir.
func roomStatus(p product) string {
	switch {
	case p.StopSale:
		return types.RoomSoldOut
	case p.Availability == 1:
		return types.RoomAvailable
	}
	return types.RoomOnRequest
}

// roomOffers urunleri oda tekliflerine cevirir (oda/pansiyon adlari sozluklerden).
func roomOffers(d *PriceData) []types.RoomOffer {
	rooms := []types.RoomOffer{}
	if d == nil {
		return rooms
	}
	seen := map[string]bool{}
	for _, p := range d.Products {
		if p.Amount == nil || *p.Amount <= 0 {
			continue
		}
		name := strings.TrimSpace(d.Rooms[p.RoomKey])
		if name == "" {
			name = "Oda " + p.RoomKey
		}
		board := strings.TrimSpace(d.Meals[p.MealKey])
		key := name + "|" + board + "|" + ftoa(*p.Amount)
		if seen[key] {
			continue
		}
		seen[key] = true
		list := 0.0
		if p.OldAmount != nil {
			list = *p.OldAmount
		}
		ro := types.NewRoom(name, board, *p.Amount, list, p.Currency, roomStatus(p))
		if p.DiscountPercent > 0 {
			ro.DiscountRate = p.DiscountPercent
		}
		rooms = append(rooms, ro)
	}
	return types.SortRooms(rooms)
}

func ftoa(f float64) string { return strings.TrimRight(strings.TrimRight(formatFloat(f), "0"), ".") }

// normalizePrice lead = en ucuz musait teklif; hic musait yoksa sorulacak olanlarin en ucuzu (Available=false).
func normalizePrice(d *PriceData) *types.NormalizedPrice {
	var hotelName, city *string
	if d != nil && d.Hotel != nil {
		hotelName = types.StrOrNil(strings.TrimSpace(d.Hotel.Name))
		city = types.StrOrNil(strings.TrimSpace(d.Hotel.Place))
	}
	rooms := roomOffers(d)
	var lead *types.RoomOffer
	for i := range rooms {
		if rooms[i].Available {
			lead = &rooms[i]
			break
		}
	}
	available := lead != nil
	if lead == nil {
		for i := range rooms {
			if rooms[i].Status == types.RoomOnRequest {
				lead = &rooms[i]
				break
			}
		}
	}
	if lead == nil {
		u := types.Unavailable(hotelName, city)
		u.Rooms = rooms
		return u
	}
	out := &types.NormalizedPrice{
		Rooms: rooms, Available: available, HotelName: hotelName, City: city,
		RoomName: types.StrOrNil(lead.RoomName), BoardType: types.StrOrNil(lead.BoardType),
		Currency: lead.Currency, ListPrice: lead.ListPrice, Price: lead.Price, DiscountRate: lead.DiscountRate,
	}
	if !available {
		out.Note = types.Str("Anlik musaitlik yok; site 'sorulacak' gosteriyor")
	}
	return out
}
