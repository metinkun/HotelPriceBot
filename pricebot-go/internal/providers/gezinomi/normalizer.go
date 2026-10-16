package gezinomi

import (
	"strconv"

	"pricebot/internal/types"
)

func orZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// normalizePrice en ucuz (lead) teklifi ortak NormalizedPrice formatina cevirir.
func normalizePrice(data *PriceResponse, hotelName *string) *types.NormalizedPrice {
	var best *types.NormalizedPrice
	if data != nil && data.Data != nil {
		for _, room := range data.Data.RoomDetails {
			for _, rd := range room.ReservationDetails {
				price := rd.TotalDiscountedPrice
				if price == nil {
					price = rd.TotalPrice
				}
				if price == nil || *price <= 0 {
					continue
				}
				if best == nil || *price < *best.Price {
					name := rd.HotelRoomName
					if name == "" {
						name = room.RoomName
					}
					best = &types.NormalizedPrice{
						Available: true, HotelName: hotelName, Currency: types.Str("TL"),
						RoomName: types.StrOrNil(name), BoardType: types.StrOrNil(rd.ConceptName),
						ListPrice: rd.TotalPrice, Price: types.F64(*price), DiscountRate: orZero(rd.DiscountRate),
					}
				}
			}
		}
	}
	if best == nil {
		return types.Unavailable(hotelName, nil)
	}
	return best
}

// normalizePackages flightPackageTotal* alanlarindan paket tekliflerini oda bazinda gruplar.
func normalizePackages(data *PriceResponse, nightCount *int) []types.NormalizedPackageRoom {
	order := []string{}
	byRoom := map[string]*types.NormalizedPackageRoom{}
	if data != nil && data.Data != nil {
		for _, room := range data.Data.RoomDetails {
			for _, rd := range room.ReservationDetails {
				price := orZero(rd.FlightPackageTotalDiscountedPrice)
				if price == 0 {
					price = orZero(rd.FlightPackageTotalPrice)
				}
				if price <= 0 {
					continue
				}
				name := rd.HotelRoomName
				if name == "" {
					name = room.RoomName
				}
				if name == "" {
					name = "Oda"
				}
				key := name
				if room.RoomID != nil {
					key = strconv.Itoa(*room.RoomID)
				}
				if _, ok := byRoom[key]; !ok {
					byRoom[key] = &types.NormalizedPackageRoom{RoomID: key, RoomName: name, NightCount: nightCount, Boards: []types.NormalizedPackageBoard{}}
					order = append(order, key)
				}
				byRoom[key].Boards = append(byRoom[key].Boards, types.NormalizedPackageBoard{
					BoardType: types.StrOrNil(rd.ConceptName), Currency: types.Str("TL"),
					ListPrice: rd.FlightPackageTotalPrice, Price: types.F64(price), DiscountRate: orZero(rd.FlightPackageDiscountRate),
				})
			}
		}
	}
	out := make([]types.NormalizedPackageRoom, 0, len(order))
	for _, k := range order {
		out = append(out, *byRoom[k])
	}
	return out
}
