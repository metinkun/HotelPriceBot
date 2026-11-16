package enuygun

import (
	"strconv"
	"strings"

	"pricebot/internal/types"
)

// offerAmount teklifin TL toplam fiyati: tryPrice (TRY, yuvarlanmis) oncelikli, yoksa price.total.
func offerAmount(o *Offer) (float64, string) {
	if v, err := strconv.ParseFloat(strings.TrimSpace(o.TryPrice.Total.Amount), 64); err == nil && v > 0 {
		cur := o.TryPrice.Total.Currency
		if cur == "" {
			cur = "TRY"
		}
		return v, cur
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(o.Price.Total.Amount), 64); err == nil && v > 0 {
		return v, o.Price.Total.Currency
	}
	return 0, ""
}

func trCurrency(c string) *string {
	if c == "TRY" || c == "" {
		return types.Str("TL")
	}
	return types.Str(c)
}

// findHotel yanitta istenen otel kaydini bulur (id eslesmezse ilk oteli doner).
func findHotel(sr *SearchResponse, hotelID int) *Hotel {
	if sr == nil || len(sr.Hotels) == 0 {
		return nil
	}
	for i := range sr.Hotels {
		if sr.Hotels[i].ID == hotelID {
			return &sr.Hotels[i]
		}
	}
	return &sr.Hotels[0]
}

type pick struct {
	room   *Room
	offer  *Offer
	amount float64
	cur    string
}

// cheapestOffer satilabilir, gizli olmayan en ucuz teklifi secer (lead fiyat).
func cheapestOffer(h *Hotel) *pick {
	var best *pick
	for ri := range h.Rooms {
		r := &h.Rooms[ri]
		if r.HiddenState.Hidden {
			continue
		}
		for oi := range r.Offers {
			o := &r.Offers[oi]
			if o.HiddenState.Hidden || !o.Saleable {
				continue
			}
			amt, cur := offerAmount(o)
			if amt <= 0 {
				continue
			}
			if best == nil || amt < best.amount {
				best = &pick{room: r, offer: o, amount: amt, cur: cur}
			}
		}
	}
	return best
}

func campaignOf(o *Offer) *string {
	for _, t := range o.DiscountTags {
		if t.DiscountString != "" && t.Tag != "provider" {
			return types.Str(strings.TrimSpace(t.Label + " " + t.DiscountString))
		}
	}
	return nil
}

// allRooms otelin gizli olmayan tum oda+pansiyon teklifleri (satilamayanlar sold_out).
func allRooms(h *Hotel) []types.RoomOffer {
	rooms := []types.RoomOffer{}
	for ri := range h.Rooms {
		r := &h.Rooms[ri]
		if r.HiddenState.Hidden {
			continue
		}
		for oi := range r.Offers {
			o := &r.Offers[oi]
			if o.HiddenState.Hidden {
				continue
			}
			amt, cur := offerAmount(o)
			if amt <= 0 {
				continue
			}
			st := types.RoomAvailable
			if !o.Saleable {
				st = types.RoomSoldOut
			}
			ro := types.NewRoom(r.Type.Name, o.Concept.Description, amt, o.DiscountFrom, cur, st)
			ro.CampaignName = campaignOf(o)
			rooms = append(rooms, ro)
		}
	}
	return types.SortRooms(rooms)
}

// normalizePrice detail-search yanitini ortak NormalizedPrice formatina cevirir.
func normalizePrice(sr *SearchResponse, hotelID int) *types.NormalizedPrice {
	h := findHotel(sr, hotelID)
	if h == nil {
		return types.Unavailable(nil, nil)
	}
	hotelName := types.StrOrNil(h.Data.Name)
	city := types.StrOrNil(h.Data.Address.City.Name)
	rooms := allRooms(h)
	p := cheapestOffer(h)
	if p == nil {
		u := types.Unavailable(hotelName, city)
		u.Rooms = rooms
		return u
	}
	price := types.F64(p.amount)
	var listPrice *float64
	if p.offer.DiscountFrom > p.amount {
		listPrice = types.F64(p.offer.DiscountFrom)
	}
	campaign := campaignOf(p.offer)
	return &types.NormalizedPrice{
		Rooms:        rooms,
		Available:    true,
		HotelName:    hotelName,
		City:         city,
		RoomName:     types.StrOrNil(strings.TrimSpace(p.room.Type.Name)),
		BoardType:    types.StrOrNil(strings.TrimSpace(p.offer.Concept.Description)),
		Currency:     trCurrency(p.cur),
		ListPrice:    listPrice,
		Price:        price,
		DiscountRate: types.DiscountRateOf(listPrice, price),
		CampaignName: campaign,
	}
}
