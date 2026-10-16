package jolly

import (
	"fmt"
	"regexp"
	"strings"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

var (
	reRoomTitle = regexp.MustCompile(`room-title"[^>]*>\s*([^<]{2,60})<`)
	reRoomTitleAll = regexp.MustCompile(`room-title"[^>]*>\s*([^<]{2,60})`)
	reConcept   = regexp.MustCompile(`room-concept[^>]*data-concept="([^"]*)"`)
	reOldPrice  = regexp.MustCompile(`old-price"[^>]*>\s*([0-9.,]+)\s*TL`)
	// current-price ikiye bolunmus olabilir: <b>63.656<small>,04 TL</small></b>
	reCurPrice = regexp.MustCompile(`current-price"[^>]*>\s*<b>\s*([0-9.]+)\s*<small>\s*([,0-9]*)`)
	reBoard    = regexp.MustCompile(`(Ultra Her Şey Dahil|Her Şey Dahil|Yarım Pansiyon|Tam Pansiyon Plus|Tam Pansiyon|Oda Kahvaltı|Sadece Oda)`)
)

func firstRoom(html string) (roomName, boardType *string) {
	if html == "" {
		return nil, nil
	}
	if m := reRoomTitle.FindStringSubmatch(html); m != nil {
		roomName = types.Str(provider.DecodeEntities(m[1]))
	}
	if m := reConcept.FindStringSubmatch(html); m != nil {
		boardType = types.Str(provider.DecodeEntities(strings.ReplaceAll(m[1], "-", " ")))
	}
	return
}

func trCurrency(c string) *string {
	if c == "TRY" {
		return types.Str("TL")
	}
	return types.StrOrNil(c)
}

// normalizePrice Jolly yanitini ortak NormalizedPrice formatina cevirir (lead/en ucuz teklif).
func normalizePrice(data *ReservationResponse) *types.NormalizedPrice {
	a := data.AnalyticsDataModel
	if a == nil {
		a = &Analytics{}
	}
	listPrice := a.TotalPrice
	price := a.DiscountedPrice
	if price == nil {
		price = a.TotalPrice
	}
	roomName, boardType := firstRoom(data.HTML)
	available := data.IsAvailableHotel && price != nil && *price > 0

	return &types.NormalizedPrice{
		Available:    available,
		HotelName:    types.StrOrNil(a.ProductName),
		City:         types.StrOrNil(a.CityName),
		RoomName:     roomName,
		BoardType:    boardType,
		Currency:     trCurrency(a.Currency),
		ListPrice:    listPrice,
		Price:        price,
		DiscountRate: types.DiscountRateOf(listPrice, price),
	}
}

func parseTR(s string) *float64 {
	return types.F64Pos(provider.ParseTRPrice(s))
}

// normalizePackages paket yanitindaki (packageSearchType=0) oda kartlarini NormalizedPackageRoom listesine cevirir.
func normalizePackages(data *ReservationResponse, nightCount *int) []types.NormalizedPackageRoom {
	html := data.HTML
	if html == "" {
		return []types.NormalizedPackageRoom{}
	}
	idx := reRoomTitleAll.FindAllStringSubmatchIndex(html, -1)
	starts := make([]int, 0, len(idx))
	names := make([]string, 0, len(idx))
	for _, m := range idx {
		starts = append(starts, m[0])
		names = append(names, provider.DecodeEntities(html[m[2]:m[3]]))
	}

	rooms := []types.NormalizedPackageRoom{}
	seen := map[string]bool{}

	for i := range starts {
		end := starts[i] + 6000
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if end > len(html) {
			end = len(html)
		}
		seg := html[starts[i]:end]

		var price, listPrice *float64
		if m := reCurPrice.FindStringSubmatch(seg); m != nil {
			price = parseTR(m[1] + m[2])
		}
		if m := reOldPrice.FindStringSubmatch(seg); m != nil {
			listPrice = parseTR(m[1])
		}
		if price == nil {
			continue
		}

		segText := provider.DecodeEntities(provider.StripTags(seg))
		var boardType *string
		if m := reBoard.FindStringSubmatch(segText); m != nil {
			boardType = types.Str(m[1])
		}
		roomName := names[i]
		if roomName == "" {
			roomName = "Oda"
		}
		key := fmt.Sprintf("%s|%s|%v", roomName, types.Deref(boardType), *price)
		if seen[key] {
			continue
		}
		seen[key] = true

		board := types.NormalizedPackageBoard{
			BoardType: boardType, Currency: types.Str("TL"), ListPrice: listPrice, Price: price,
			DiscountRate: types.DiscountRateOf(listPrice, price),
		}
		placed := false
		for r := range rooms {
			if rooms[r].RoomName == roomName {
				rooms[r].Boards = append(rooms[r].Boards, board)
				placed = true
				break
			}
		}
		if !placed {
			rooms = append(rooms, types.NormalizedPackageRoom{
				RoomID: fmt.Sprint(i), RoomName: roomName, NightCount: nightCount,
				Boards: []types.NormalizedPackageBoard{board},
			})
		}
	}
	return rooms
}
