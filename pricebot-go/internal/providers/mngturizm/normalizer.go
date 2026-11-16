package mngturizm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

// otelprice yaniti: her oda+pansiyon kombinasyonu bir <section class="otelfiyatsablon ..."> blogu.
// Ilk <div class="title"> oda adi, <div class="title hidden-xs"> pansiyon; fiyat masaustu blogunda
// (<div class="fiyat hidden-xs">) old/new span'lari; kesin satis fiyati ayrica taksit rel="33150.00/TRY/OTEL/id".
var (
	reSection     = regexp.MustCompile(`<section class="otelfiyatsablon[^"]*">`)
	reRoomTitle   = regexp.MustCompile(`<div class="title">([^<]*)</div>`)
	reBoardTitle  = regexp.MustCompile(`<div class="title hidden-xs">([^<]*)</div>`)
	reTaksitRel   = regexp.MustCompile(`class="taksit"\s+rel="([0-9.]+)/([A-Z]{3})/`)
	reOldPrice    = regexp.MustCompile(`<div class="old"><span class="a">([^<]*)</span><span class="b">([^<]*)</span>`)
	reNewPrice    = regexp.MustCompile(`<div class="new"><span class="a">([^<]*)</span><span class="b">([^<]*)</span>`)
	reNote        = regexp.MustCompile(`(?s)<div class="note">(.*?)<span`)
	reMinGece     = regexp.MustCompile(`<div class="min-gece">([^<]*)</div>`)
	reDigits      = regexp.MustCompile(`\d+`)
	reUnavailable = regexp.MustCompile(`(?i)fiyat bilgisi bulunamad`)
	// reRoomSoldOut oda blogunda "Aradiginiz tarihlerde musait degil" notu: fiyat gosterilir ama satilamaz.
	reRoomSoldOut = regexp.MustCompile(`(?i)m[üu]sait\s*de[ğg]il|sat[ıi][şs]a\s*kapal[ıi]|stop\s*sale`)
)

// Offer tek bir oda + pansiyon teklifi.
type Offer struct {
	RoomName  string
	BoardType string
	Currency  string
	Price     float64
	ListPrice float64
	Campaign  string
	MinStay   int
	Package   bool // otel+ucak paket fiyati
	SoldOut   bool // "Aradiginiz tarihlerde musait degil" (fiyat gosterilir, satis yok)
}

func trCurrency(c string) string {
	if c == "" || c == "TRY" {
		return "TL"
	}
	return c
}

// parseOffers otelprice HTML'ini teklif listesine cevirir (yanit sirasi korunur).
func parseOffers(html string) []Offer {
	idx := reSection.FindAllStringIndex(html, -1)
	offers := make([]Offer, 0, len(idx))
	for i, m := range idx {
		end := len(html)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		seg := html[m[0]:end]
		o := Offer{
			RoomName:  provider.DecodeEntities(provider.FirstMatch(reRoomTitle, seg)),
			BoardType: provider.DecodeEntities(provider.FirstMatch(reBoardTitle, seg)),
			Package:   strings.Contains(seg, `class="ucakfiyatadahil"`),
			SoldOut:   reRoomSoldOut.MatchString(provider.DecodeEntities(provider.StripTags(seg))),
		}
		if tm := reTaksitRel.FindStringSubmatch(seg); tm != nil {
			o.Price, _ = strconv.ParseFloat(tm[1], 64)
			o.Currency = trCurrency(tm[2])
		}
		if nm := reNewPrice.FindStringSubmatch(seg); nm != nil {
			if o.Price <= 0 {
				o.Price = provider.ParseTRPrice(nm[1] + nm[2])
			}
			if o.Currency == "" {
				o.Currency = "TL"
			}
		}
		if om := reOldPrice.FindStringSubmatch(seg); om != nil {
			o.ListPrice = provider.ParseTRPrice(om[1] + om[2])
		}
		if n := provider.FirstMatch(reNote, seg); n != "" {
			o.Campaign = strings.TrimRight(provider.DecodeEntities(provider.StripTags(n)), ", ")
		}
		if mg := provider.FirstMatch(reMinGece, seg); mg != "" {
			if d := reDigits.FindString(mg); d != "" {
				o.MinStay, _ = strconv.Atoi(d)
			}
		}
		if o.Price <= 0 {
			continue
		}
		if o.RoomName == "" {
			o.RoomName = "Oda"
		}
		offers = append(offers, o)
	}
	return offers
}

// isUnavailable "Sectiginiz tarihler icin oda fiyat bilgisi bulunamadi" mesaji.
func isUnavailable(html string) bool { return reUnavailable.MatchString(html) }

// roomOffers teklifleri RoomOffer listesine cevirir (otel+ucak paket satirlari haric; hepsi paketse hepsi).
func roomOffers(offers []Offer) []types.RoomOffer {
	rooms := []types.RoomOffer{}
	seen := map[string]bool{}
	hasHotelOnly := false
	for _, o := range offers {
		if !o.Package {
			hasHotelOnly = true
			break
		}
	}
	for _, o := range offers {
		if o.Package && hasHotelOnly {
			continue
		}
		key := fmt.Sprintf("%s|%s|%v", o.RoomName, o.BoardType, o.Price)
		if seen[key] {
			continue
		}
		seen[key] = true
		st := types.RoomAvailable
		if o.SoldOut {
			st = types.RoomSoldOut
		}
		ro := types.NewRoom(o.RoomName, o.BoardType, o.Price, o.ListPrice, o.Currency, st)
		ro.CampaignName = types.StrOrNil(o.Campaign)
		if o.MinStay > 0 {
			ro.MinStayNights = types.Int(o.MinStay)
		}
		rooms = append(rooms, ro)
	}
	return types.SortRooms(rooms)
}

// normalizePrice en ucuz teklifi NormalizedPrice'a cevirir; Rooms tum teklifler.
func normalizePrice(html string, hotelName, city *string) *types.NormalizedPrice {
	offers := parseOffers(html)
	if len(offers) == 0 {
		return types.Unavailable(hotelName, city)
	}
	// Lead: satilabilir (musait) tekliflerin en ucuzu; hepsi "musait degil" ise en ucuzu fiyatla ama Available=false.
	var best *Offer
	for i := range offers {
		o := &offers[i]
		if o.SoldOut {
			continue
		}
		if best == nil || o.Price < best.Price {
			best = o
		}
	}
	available := best != nil
	if best == nil {
		best = &offers[0]
		for i := range offers[1:] {
			if offers[i+1].Price < best.Price {
				best = &offers[i+1]
			}
		}
	}
	price := types.F64Pos(best.Price)
	listPrice := types.F64Pos(best.ListPrice)
	out := &types.NormalizedPrice{
		Rooms:        roomOffers(offers),
		Available:    available,
		HotelName:    hotelName,
		City:         city,
		RoomName:     types.StrOrNil(best.RoomName),
		BoardType:    types.StrOrNil(best.BoardType),
		Currency:     types.Str(best.Currency),
		ListPrice:    listPrice,
		Price:        price,
		DiscountRate: types.DiscountRateOf(listPrice, price),
		CampaignName: types.StrOrNil(best.Campaign),
	}
	if best.MinStay > 0 {
		out.MinStayNights = types.Int(best.MinStay)
	}
	if !available {
		out.Note = types.Str("Musait degil; sitede gosterilen fiyat")
	}
	return out
}

// normalizePackages teklifleri oda bazinda gruplar (her pansiyon bir board).
func normalizePackages(html string, nightCount *int) []types.NormalizedPackageRoom {
	rooms := []types.NormalizedPackageRoom{}
	seen := map[string]bool{}
	for _, o := range parseOffers(html) {
		key := fmt.Sprintf("%s|%s|%v", o.RoomName, o.BoardType, o.Price)
		if seen[key] {
			continue
		}
		seen[key] = true
		price := types.F64Pos(o.Price)
		listPrice := types.F64Pos(o.ListPrice)
		board := types.NormalizedPackageBoard{
			BoardType: types.StrOrNil(o.BoardType), Currency: types.Str(o.Currency),
			ListPrice: listPrice, Price: price, DiscountRate: types.DiscountRateOf(listPrice, price),
		}
		placed := false
		for r := range rooms {
			if rooms[r].RoomName == o.RoomName {
				rooms[r].Boards = append(rooms[r].Boards, board)
				placed = true
				break
			}
		}
		if !placed {
			rooms = append(rooms, types.NormalizedPackageRoom{
				RoomID: strconv.Itoa(len(rooms)), RoomName: o.RoomName, NightCount: nightCount,
				Boards: []types.NormalizedPackageBoard{board},
			})
		}
	}
	return rooms
}
