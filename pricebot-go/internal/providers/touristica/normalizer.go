package touristica

import (
	"regexp"
	"strconv"

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

// normalize ham fiyat bloklarini ortak NormalizedPrice'a cevirir (en ucuz teklif).
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
	if best == nil {
		return types.Unavailable(raw.HotelName, raw.City)
	}
	return &types.NormalizedPrice{
		Available: true, HotelName: raw.HotelName, City: raw.City,
		Currency: types.Str("TL"), ListPrice: best.ListPrice, Price: best.Price,
		DiscountRate: types.DiscountRateOf(best.ListPrice, best.Price), MinStayNights: best.Nights,
	}
}
