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

// normalizePrice en ucuz teklifi NormalizedPrice'a cevirir.
func normalizePrice(html string, hotelName, city *string) *types.NormalizedPrice {
	offers := parseOffers(html)
	if len(offers) == 0 {
		return types.Unavailable(hotelName, city)
	}
	best := offers[0]
	for _, o := range offers[1:] {
		if o.Price < best.Price {
			best = o
		}
	}
	price := types.F64Pos(best.Price)
	listPrice := types.F64Pos(best.ListPrice)
	out := &types.NormalizedPrice{
		Available:    true,
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
