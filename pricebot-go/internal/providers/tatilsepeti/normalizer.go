package tatilsepeti

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

// pick ilk bulunan anahtarin degerini doner (TS'teki a ?? b ?? c zinciri).
func pick(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func asString(v any) *string {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return types.Str(x)
	default:
		return types.Str(fmt.Sprint(x))
	}
}

func asNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n, err == nil
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func idOf(m map[string]any) string {
	v := pick(m, "HotelId", "hotelId")
	if v == nil {
		return ""
	}
	if f, ok := v.(float64); ok {
		return strconv.FormatInt(int64(f), 10)
	}
	return fmt.Sprint(v)
}

// normalizeResponse GetHotelListPrice'in degisken yapili yanitini NormalizedPrice'a cevirir.
func normalizeResponse(data any, providerHotelID string) *types.NormalizedPrice {
	empty := types.Unavailable(nil, nil)
	if data == nil {
		return empty
	}
	var hotel map[string]any
	switch d := data.(type) {
	case []any:
		for _, it := range d {
			if m, ok := it.(map[string]any); ok && idOf(m) == providerHotelID {
				hotel = m
				break
			}
		}
	case map[string]any:
		hotel = d
	}
	if hotel == nil {
		return empty
	}

	priceV := pick(hotel, "Price", "price")
	listV := pick(hotel, "ListPrice", "listPrice", "OldPrice")
	rateV := pick(hotel, "DiscountRate", "discountRate")
	discV := pick(hotel, "DiscountPrice", "discountPrice")

	price, hasPrice := asNumber(priceV)
	rate, _ := asNumber(rateV)
	disc, _ := asNumber(discV)

	// GetHotelListPrice: Price = indirimsiz (liste) fiyat, DiscountPrice = DiscountRate uygulanmis
	// odenecek fiyat (orn. 67200 / %15 / 57120). Normalize: Price = odenecek, ListPrice = liste.
	var listPrice *float64
	if l, ok := asNumber(listV); ok && l != 0 {
		listPrice = types.F64(l)
	} else if hasPrice && price > 0 && disc > 0 && disc < price {
		listPrice = types.F64(price)
		price = disc
	} else if hasPrice && price != 0 && rate > 0 {
		listPrice = types.F64(math.Round(price / (1 - rate/100)))
	}

	out := &types.NormalizedPrice{
		Available:    hasPrice && price > 0,
		HotelName:    asString(pick(hotel, "HotelName", "hotelName")),
		City:         asString(pick(hotel, "City", "city", "AreaName")),
		RoomName:     asString(pick(hotel, "RoomName", "roomName", "RoomType")),
		BoardType:    asString(pick(hotel, "BoardType", "boardType", "Accommodation", "AccommodationType")),
		Currency:     types.Str("TL"),
		ListPrice:    listPrice,
		DiscountRate: rate,
		CampaignName: asString(pick(hotel, "CampaignName")),
	}
	if c := asString(pick(hotel, "Currency", "currency")); c != nil {
		out.Currency = c
	}
	if hasPrice && price != 0 {
		out.Price = types.F64(price)
	}
	if disc != 0 {
		out.DiscountPrice = types.F64(disc)
	}
	return out
}

// ---- Paket HTML parse ----

var (
	reInstallObj = regexp.MustCompile(`\{([\s\S]+)\}`)
	reKV         = regexp.MustCompile(`(\w+)\s*:\s*(?:'([^']*)'|"([^"]*)"|([+-]?\d+(?:\.\d+)?)|(\w+))`)
	reRoomSize   = regexp.MustCompile(`(?i)(\d+)\s*m`)
)

// parseGetInstallmentData onclick="hotelDetail.getInstallment(event, { RoomTypeId: '25', FlightPocketPrice: '92558', ... })"
func parseGetInstallmentData(onclick string) map[string]string {
	m := reInstallObj.FindStringSubmatch(onclick)
	if m == nil {
		return nil
	}
	data := map[string]string{}
	for _, kv := range reKV.FindAllStringSubmatch(m[1], -1) {
		val := ""
		for _, g := range kv[2:] {
			if g != "" {
				val = g
				break
			}
		}
		data[kv[1]] = val
	}
	if len(data) == 0 {
		return nil
	}
	return data
}

// parsePackageRoomList GetFlyingPackages roomList HTML'ini odalara/pansiyonlara ayristirir.
func parsePackageRoomList(html string, nightCount int) []types.NormalizedPackageRoom {
	if strings.TrimSpace(html) == "" {
		return []types.NormalizedPackageRoom{}
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return []types.NormalizedPackageRoom{}
	}

	order := []string{}
	roomMap := map[string]*types.NormalizedPackageRoom{}

	doc.Find(`[id^="roomName_flight_"]`).Each(func(_ int, card *goquery.Selection) {
		idAttr, _ := card.Attr("id")
		cardRoomID := strings.TrimPrefix(idAttr, "roomName_flight_")
		roomName := strings.TrimSpace(card.Find("h3").First().Text())

		var roomSize *int
		card.Find("li span").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if m := reRoomSize.FindStringSubmatch(s.Text()); m != nil {
				n, _ := strconv.Atoi(m[1])
				roomSize = &n
				return false
			}
			return true
		})

		hasCancellation := card.Find(`[class*="free-cancel"], [class*="cancelation"]`).Length() > 0

		card.Find(`[class*="pension-types"]:not([class*="__badge"])`).Each(func(_ int, pension *goquery.Selection) {
			boardType := strings.TrimSpace(pension.Find("span").First().Text())
			if boardType == "" {
				boardType = "Standard"
			}
			multiple := pension.NextAllFiltered(`[class*="multiple"]`).First()
			if multiple.Length() == 0 {
				return
			}
			onclickEl := multiple.Find(`[onclick*='getInstallment']`).First()
			if onclickEl.Length() == 0 {
				return
			}
			onclick, _ := onclickEl.Attr("onclick")
			data := parseGetInstallmentData(onclick)
			if data == nil || data["FlightPocketPrice"] == "" {
				return
			}
			roomTypeID := data["RoomTypeId"]
			if roomTypeID == "" {
				roomTypeID = cardRoomID
			}
			// FlightPocketPrice TR formatinda gelir: '65533,58' (virgul ondalik) veya '92558'.
			total := provider.ParseTRPrice(data["FlightPocketPrice"])
			if total <= 0 {
				return
			}

			var cancel *string
			if hasCancellation {
				cancel = types.Str("FREE_CANCELLATION")
			}
			board := types.NormalizedPackageBoard{
				BoardType: types.Str(boardType), Currency: types.Str("TL"),
				Price: types.F64(total), Cancellation: cancel,
			}
			if ex, ok := roomMap[roomTypeID]; ok {
				ex.Boards = append(ex.Boards, board)
				return
			}
			name := roomName
			if name == "" {
				name = "Room " + roomTypeID
			}
			nc := nightCount
			roomMap[roomTypeID] = &types.NormalizedPackageRoom{
				RoomID: roomTypeID, RoomName: name, RoomSize: roomSize, NightCount: &nc,
				Boards: []types.NormalizedPackageBoard{board},
			}
			order = append(order, roomTypeID)
		})
	})

	// Fallback: getInstallment bulunamazsa gorunen fiyatlardan parse et
	if len(roomMap) == 0 {
		doc.Find(`[id^="roomName_flight_"]`).Each(func(_ int, card *goquery.Selection) {
			idAttr, _ := card.Attr("id")
			cardRoomID := strings.TrimPrefix(idAttr, "roomName_flight_")
			roomName := strings.TrimSpace(card.Find("h3").First().Text())
			boards := []types.NormalizedPackageBoard{}

			card.Find(`[class*="discount-price"]`).Each(func(_ int, pe *goquery.Selection) {
				price := math.Round(provider.ParseTRPrice(pe.Text()))
				if price <= 0 {
					return
				}
				section := pe.Closest(`[class*="price"]`)
				boardType := strings.TrimSpace(section.Find(`[class*="pension"] span`).First().Text())
				if boardType == "" {
					boardType = "Standard"
				}
				boards = append(boards, types.NormalizedPackageBoard{
					BoardType: types.Str(boardType), Currency: types.Str("TL"), Price: types.F64(price),
				})
			})
			if len(boards) > 0 {
				name := roomName
				if name == "" {
					name = "Room " + cardRoomID
				}
				nc := nightCount
				roomMap[cardRoomID] = &types.NormalizedPackageRoom{RoomID: cardRoomID, RoomName: name, NightCount: &nc, Boards: boards}
				order = append(order, cardRoomID)
			}
		})
	}

	out := make([]types.NormalizedPackageRoom, 0, len(order))
	for _, id := range order {
		out = append(out, *roomMap[id])
	}
	return out
}
