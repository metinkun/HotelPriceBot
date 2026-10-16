package tatilbudur

import (
	"regexp"
	"strconv"
	"strings"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

func decode(s string) string {
	return provider.DecodeEntities(strings.ReplaceAll(s, "-", " "))
}

var reButtons = regexp.MustCompile(`<a[^>]*v2-reservation-button[^>]*>`)

// attr acilis etiketindeki attribute degerini doner (buyuk/kucuk harf duyarsiz).
func attr(block, name string) string {
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(name) + `="([^"]*)"`)
	return provider.FirstMatch(re, block)
}

func num(s string) float64 {
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return n
}

func trCurrency(c string) *string {
	if c == "TRY" {
		return types.Str("TL")
	}
	return types.StrOrNil(c)
}

// normalizePrice fiyati ilk `v2-reservation-button` <a>'sinin temiz attribute'larindan alir.
func normalizePrice(data *RoomPriceResponse) *types.NormalizedPrice {
	view := ""
	if data != nil {
		view = data.View
	}
	i := strings.Index(view, "v2-reservation-button")
	if i == -1 {
		return types.Unavailable(nil, nil)
	}
	end := strings.Index(view[i:], ">")
	if end == -1 {
		return types.Unavailable(nil, nil)
	}
	block := view[i : i+end+1]

	price := types.F64Pos(num(attr(block, "amount")))
	listPrice := types.F64Pos(num(attr(block, "poster_price")))
	out := &types.NormalizedPrice{
		Available:    price != nil,
		Currency:     trCurrency(attr(block, "currency")),
		ListPrice:    listPrice,
		Price:        price,
		DiscountRate: num(attr(block, "discountRate")),
	}
	if rn := attr(block, "roomName"); rn != "" {
		out.RoomName = types.Str(decode(rn))
	}
	if b := attr(block, "conceptName"); b != "" {
		out.BoardType = types.Str(decode(b))
	}
	return out
}

// normalizePackages packageType="P" kartlarini oda bazinda gruplar.
func normalizePackages(data *RoomPriceResponse) []types.NormalizedPackageRoom {
	if data == nil || data.View == "" {
		return []types.NormalizedPackageRoom{}
	}
	order := []string{}
	byRoom := map[string]*types.NormalizedPackageRoom{}

	for _, block := range reButtons.FindAllString(data.View, -1) {
		if attr(block, "packageType") != "P" {
			continue
		}
		amount := num(attr(block, "amount"))
		if amount <= 0 {
			continue
		}
		roomID := attr(block, "roomId")
		if roomID == "" {
			roomID = attr(block, "pricingId")
		}
		roomName := "Oda"
		if rn := attr(block, "roomName"); rn != "" {
			roomName = decode(rn)
		}
		key := roomID
		if key == "" {
			key = roomName
		}
		if _, ok := byRoom[key]; !ok {
			var nights *int
			if n := attr(block, "night"); n != "" {
				if v, err := strconv.Atoi(n); err == nil {
					nights = &v
				}
			}
			byRoom[key] = &types.NormalizedPackageRoom{RoomID: key, RoomName: roomName, NightCount: nights, Boards: []types.NormalizedPackageBoard{}}
			order = append(order, key)
		}
		room := byRoom[key]
		var boardType *string
		if b := attr(block, "conceptName"); b != "" {
			boardType = types.Str(decode(b))
		}
		dup := false
		for _, b := range room.Boards {
			if types.Deref(b.BoardType) == types.Deref(boardType) && b.Price != nil && *b.Price == amount {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		room.Boards = append(room.Boards, types.NormalizedPackageBoard{
			BoardType: boardType, Currency: trCurrency(attr(block, "currency")),
			ListPrice: types.F64Pos(num(attr(block, "poster_price"))), Price: types.F64(amount),
			DiscountRate: num(attr(block, "discountRate")),
		})
	}
	out := make([]types.NormalizedPackageRoom, 0, len(order))
	for _, k := range order {
		out = append(out, *byRoom[k])
	}
	return out
}
