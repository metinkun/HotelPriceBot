package prontotour

import (
	"fmt"
	"sort"
	"strings"

	"pricebot/internal/types"
)

// effective odanin satis fiyati: indirimli varsa o, yoksa liste.
func effective(r RoomDetail) float64 {
	if r.DiscountPriceTl > 0 {
		return r.DiscountPriceTl
	}
	return r.PriceTl
}

// sellableRooms kontenjani (Allotment) olan odalar; hic yoksa tum odalar (site ile ayni davranis).
// allowZero paket (fark) modunda en ucuz odanin 0 fiyatini gecerli sayar.
func sellableRooms(rooms []RoomDetail, allowZero bool) []RoomDetail {
	ok := func(r RoomDetail) bool { return allowZero || effective(r) > 0 }
	out := make([]RoomDetail, 0, len(rooms))
	for _, r := range rooms {
		if r.Allotment && ok(r) {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		for _, r := range rooms {
			if ok(r) {
				out = append(out, r)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return effective(out[i]) < effective(out[j]) })
	return out
}

func boardOf(r RoomDetail) *string {
	return types.StrOrNil(strings.Join(strings.Fields(r.Mealy), " "))
}

// normalizePrice HotelDetail yanitini ortak formata cevirir (en ucuz satilabilir oda = lead teklif).
func normalizePrice(d *HotelDetailResponse) *types.NormalizedPrice {
	var hotelName, city *string
	var minStay *int
	if d != nil && d.HotelListResponse != nil {
		hotelName = types.StrOrNil(strings.TrimSpace(d.HotelListResponse.Name))
		city = d.HotelListResponse.City
		if d.HotelListResponse.MinimumStay > 0 {
			minStay = types.Int(d.HotelListResponse.MinimumStay)
		}
	}
	if d == nil {
		return types.Unavailable(hotelName, city)
	}
	rooms := sellableRooms(d.HotelRoomDetailsList, false)
	if len(rooms) == 0 {
		p := types.Unavailable(hotelName, city)
		p.MinStayNights = minStay
		return p
	}
	best := rooms[0]
	price := types.F64Pos(effective(best))
	var listPrice *float64
	if best.PriceTl > effective(best) {
		listPrice = types.F64(best.PriceTl)
	}
	return &types.NormalizedPrice{
		Available:     true,
		HotelName:     hotelName,
		City:          city,
		RoomName:      types.StrOrNil(strings.TrimSpace(best.RoomName)),
		BoardType:     boardOf(best),
		Currency:      types.Str("TL"),
		ListPrice:     listPrice,
		Price:         price,
		DiscountRate:  types.DiscountRateOf(listPrice, price),
		MinStayNights: minStay,
	}
}

// packageHotel BundleSearch yanitindaki (FirstHotel sorgusunda tek) otel kaydi.
func packageHotel(b *BundleResponse) *HotelSummary {
	if b == nil || len(b.HotelListResponse) == 0 {
		return nil
	}
	return &b.HotelListResponse[0]
}

// normalizePackages paket fiyatlarini uretir.
//
// BundleSearch otel kaydi PriceTL/DiscountPriceTL = en ucuz oda + secili ucuslar (+transfer) toplamidir.
// Paket modundaki HotelDetail odalari en ucuz odaya gore FARK verir; site de her odayi
// "taban paket + fark" olarak gosterir (search-config.js HotelDetail.DetailRoom). Oda listesi yoksa
// yalnizca taban paket tek satir olarak doner.
func normalizePackages(b *BundleResponse, d *HotelDetailResponse, nightCount *int) []types.NormalizedPackageRoom {
	h := packageHotel(b)
	if h == nil {
		return []types.NormalizedPackageRoom{}
	}
	baseList := h.PriceTL
	baseDisc := h.DiscountPriceTL
	if baseDisc <= 0 {
		baseDisc = baseList
	}
	if baseDisc <= 0 {
		return []types.NormalizedPackageRoom{}
	}

	var rooms []RoomDetail
	if d != nil {
		rooms = sellableRooms(d.HotelRoomDetailsList, true)
	}
	if len(rooms) == 0 {
		price := types.F64(baseDisc)
		var list *float64
		if baseList > baseDisc {
			list = types.F64(baseList)
		}
		return []types.NormalizedPackageRoom{{
			RoomID: "lowest", RoomName: "En uygun paket", NightCount: nightCount,
			Boards: []types.NormalizedPackageBoard{{
				BoardType: h.MealType, Currency: types.Str("TL"), ListPrice: list, Price: price,
				DiscountRate: types.DiscountRateOf(list, price),
			}},
		}}
	}

	out := []types.NormalizedPackageRoom{}
	seen := map[string]bool{}
	for i, r := range rooms {
		// Fark modunda en ucuz odada her iki alan 0'dir; digerlerinde indirimli fark yoksa liste farki kullanilir.
		offDisc := r.DiscountPriceTl
		if offDisc == 0 {
			offDisc = r.PriceTl
		}
		price := types.F64Pos(round2(baseDisc + offDisc))
		if price == nil {
			continue
		}
		var list *float64
		if l := round2(baseList + r.PriceTl); l > *price {
			list = types.F64(l)
		}
		name := strings.TrimSpace(r.RoomName)
		if name == "" {
			name = "Oda"
		}
		board := boardOf(r)
		key := fmt.Sprintf("%s|%s|%v", name, types.Deref(board), *price)
		if seen[key] {
			continue
		}
		seen[key] = true
		nb := types.NormalizedPackageBoard{
			BoardType: board, Currency: types.Str("TL"), ListPrice: list, Price: price,
			DiscountRate: types.DiscountRateOf(list, price),
		}
		if r.FreeCancel {
			nb.Cancellation = types.Str("Ücretsiz iptal")
		}
		placed := false
		for k := range out {
			if out[k].RoomName == name {
				out[k].Boards = append(out[k].Boards, nb)
				placed = true
				break
			}
		}
		if !placed {
			id := r.RoomCode
			if id == "" {
				id = fmt.Sprint(i)
			}
			out = append(out, types.NormalizedPackageRoom{
				RoomID: id, RoomName: name, NightCount: nightCount,
				Boards: []types.NormalizedPackageBoard{nb},
			})
		}
	}
	return out
}

// flightSummary secili gidis/donus ucuslarini kisa metin olarak ozetler (PackageResult.Note icin).
func flightSummary(b *BundleResponse) string {
	if b == nil {
		return ""
	}
	parts := []string{}
	for _, f := range b.FlightResponse {
		if !f.Selected {
			continue
		}
		day := f.DepartureDate
		if len(day) >= 10 {
			day = day[8:10] + "." + day[5:7]
		}
		s := fmt.Sprintf("%s→%s %s %s %s", f.DepartureAirportCode, f.ArrivalAirportCode, day, f.DepartureTime, f.FlightNumber)
		if c := strings.TrimSpace(f.CompanyName); c != "" {
			s += " (" + c + ")"
		}
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		return ""
	}
	return "Secili ucuslar: " + strings.Join(parts, " / ")
}
