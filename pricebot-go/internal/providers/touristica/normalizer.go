package touristica

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

func typesF64Pos(v float64) *float64 { return types.F64Pos(v) }

var (
	rePrices = regexp.MustCompile(`([0-9]{1,3}(?:\.[0-9]{3})+|[0-9]{4,})\s*(?:TL|₺)`)
	reNights = regexp.MustCompile(`(?i)(\d+)\s*Gecelik`)
)

type parsedBlock struct {
	ListPrice *float64
	Price     *float64
	Nights    *int
}

// parseRoomBlock "3 Gecelik Toplam Fiyat 87.000 TL 51.330 TL Rezervasyon Yap" -> liste/satis.
func parseRoomBlock(block string) parsedBlock {
	var valid []float64
	for _, m := range rePrices.FindAllStringSubmatch(block, -1) {
		if v := provider.ParseTRPrice(m[1]); v > 0 {
			valid = append(valid, v)
		}
	}
	var nights *int
	if m := reNights.FindStringSubmatch(block); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			nights = &n
		}
	}
	if len(valid) == 0 {
		return parsedBlock{}
	}
	if len(valid) == 1 {
		return parsedBlock{Price: types.F64(valid[0]), Nights: nights}
	}
	minV, maxV := valid[0], valid[0]
	for _, v := range valid[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	out := parsedBlock{Price: types.F64(minV), Nights: nights}
	if maxV > minV {
		out.ListPrice = types.F64(maxV)
	}
	return out
}

var reCardSoldOut = regexp.MustCompile(`(?i)m[üu]sait\s*de[ğg]il|sat[ıi][şs]a\s*kapal[ıi]|t[üu]kendi|doludur|stop\s*sale`)

// roomOffers oda kartlarini tekliflere cevirir: karttaki en dusuk TL satis, en yuksek liste fiyatidir.
// "Otel + Ucak" paket metni iceren kartlar (paket sekmesi) atlanir; "musait degil" notu olanlar sold_out.
func roomOffers(cards []RoomCard) []types.RoomOffer {
	rooms := []types.RoomOffer{}
	seen := map[string]bool{}
	for _, c := range cards {
		txt := c.Text
		if i := strings.Index(txt, "Otel + Uçak"); i >= 0 {
			txt = txt[:i]
		}
		pb := parseRoomBlock(txt)
		if pb.Price == nil {
			continue
		}
		key := fmt.Sprintf("%s|%s|%v", c.RoomName, c.BoardType, *pb.Price)
		if seen[key] {
			continue
		}
		seen[key] = true
		st := types.RoomAvailable
		if reCardSoldOut.MatchString(c.Text) {
			st = types.RoomSoldOut
		}
		lp := 0.0
		if pb.ListPrice != nil {
			lp = *pb.ListPrice
		}
		ro := types.NewRoom(c.RoomName, c.BoardType, *pb.Price, lp, "TL", st)
		ro.MinStayNights = pb.Nights
		rooms = append(rooms, ro)
	}
	return types.SortRooms(rooms)
}

// normalize ham fiyat bloklarini ortak NormalizedPrice'a cevirir (en ucuz teklif; Rooms oda kartlari).
func normalize(raw *RawPrice) *types.NormalizedPrice {
	var best *parsedBlock
	for _, b := range raw.RoomBlocks {
		pb := parseRoomBlock(b)
		if pb.Price == nil {
			continue
		}
		if best == nil || *pb.Price < *best.Price {
			cp := pb
			best = &cp
		}
	}
	rooms := roomOffers(raw.RoomCards)
	if best == nil {
		u := types.Unavailable(raw.HotelName, raw.City)
		u.Rooms = rooms
		return u
	}
	out := &types.NormalizedPrice{
		Rooms:     rooms,
		Available: true, HotelName: raw.HotelName, City: raw.City,
		Currency: types.Str("TL"), ListPrice: best.ListPrice, Price: best.Price,
		DiscountRate: types.DiscountRateOf(best.ListPrice, best.Price), MinStayNights: best.Nights,
	}
	// Oda adi/pansiyon: lead fiyatla eslesen musait kart
	for _, r := range rooms {
		if r.Available && r.Price != nil && *r.Price == *best.Price {
			out.RoomName, out.BoardType = types.StrOrNil(r.RoomName), types.StrOrNil(r.BoardType)
			break
		}
	}
	return out
}
