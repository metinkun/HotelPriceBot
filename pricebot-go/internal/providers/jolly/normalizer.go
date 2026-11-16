package jolly

import (
	"fmt"
	"regexp"
	"strings"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

var (
	reRoomTitle    = regexp.MustCompile(`room-title"[^>]*>\s*([^<]{2,60})<`)
	reRoomTitleAll = regexp.MustCompile(`room-title"[^>]*>\s*([^<]{2,60})`)
	reConcept      = regexp.MustCompile(`room-concept[^>]*data-concept="([^"]*)"`)
	reOldPrice     = regexp.MustCompile(`old-price"[^>]*>\s*([0-9.,]+)\s*TL`)
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

// segment oda kartindan okunan tek teklif (room-title ile baslayan HTML parcasi).
type segment struct {
	Index     int
	RoomName  string
	BoardType *string
	Price     *float64
	ListPrice *float64
}

// parseSegments HTML'i room-title basliklarina gore boler; fiyatsiz parcalar atlanir, ayni oda|pansiyon|fiyat tekillestirilir.
func parseSegments(html string) []segment {
	out := []segment{}
	if html == "" {
		return out
	}
	idx := reRoomTitleAll.FindAllStringSubmatchIndex(html, -1)
	seen := map[string]bool{}
	for i, m := range idx {
		end := m[0] + 6000
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		if end > len(html) {
			end = len(html)
		}
		seg := html[m[0]:end]
		var price, listPrice *float64
		if pm := reCurPrice.FindStringSubmatch(seg); pm != nil {
			price = parseTR(pm[1] + pm[2])
		}
		if om := reOldPrice.FindStringSubmatch(seg); om != nil {
			listPrice = parseTR(om[1])
		}
		if price == nil {
			continue
		}
		var boardType *string
		if bm := reBoard.FindStringSubmatch(provider.DecodeEntities(provider.StripTags(seg))); bm != nil {
			boardType = types.Str(bm[1])
		}
		roomName := provider.DecodeEntities(html[m[2]:m[3]])
		if roomName == "" {
			roomName = "Oda"
		}
		key := fmt.Sprintf("%s|%s|%v", roomName, types.Deref(boardType), *price)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, segment{Index: i, RoomName: roomName, BoardType: boardType, Price: price, ListPrice: listPrice})
	}
	return out
}

// roomOffers otel yanitindaki tum oda kartlari (site satilamayan odalari listelemez; hepsi available).
func roomOffers(html string) []types.RoomOffer {
	rooms := []types.RoomOffer{}
	for _, s := range parseSegments(html) {
		lp := 0.0
		if s.ListPrice != nil {
			lp = *s.ListPrice
		}
		rooms = append(rooms, types.NewRoom(s.RoomName, types.Deref(s.BoardType), *s.Price, lp, "TL", types.RoomAvailable))
	}
	return types.SortRooms(rooms)
}

// normalizePrice Jolly yanitini ortak NormalizedPrice formatina cevirir (lead/en ucuz teklif; Rooms tum kartlar).
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
		Rooms:        roomOffers(data.HTML),
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
	rooms := []types.NormalizedPackageRoom{}
	for _, s := range parseSegments(data.HTML) {
		i, roomName, boardType, price, listPrice := s.Index, s.RoomName, s.BoardType, s.Price, s.ListPrice
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
