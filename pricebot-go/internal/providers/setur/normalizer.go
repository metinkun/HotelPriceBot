package setur

import (
	"regexp"
	"strings"

	"pricebot/internal/types"
)

var (
	reU0026   = regexp.MustCompile(`\\+u0026`)
	reCtrl    = regexp.MustCompile(`\\+[tnr]`)
	reSpaces  = regexp.MustCompile(`\s+`)
	reTrailBS = regexp.MustCompile(`\\+$`)
)

func decode(s string) *string {
	if s == "" {
		return nil
	}
	s = reU0026.ReplaceAllString(s, "&")
	s = reCtrl.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = reSpaces.ReplaceAllString(s, " ")
	s = reTrailBS.ReplaceAllString(s, "")
	return types.Str(strings.TrimSpace(s))
}

// normalize Setur sayfa verisini ortak NormalizedPrice formatina cevirir.
func normalize(d *HotelData) *types.NormalizedPrice {
	out := &types.NormalizedPrice{
		HotelName:     decode(d.HotelName),
		City:          types.StrOrNil(d.City),
		BoardType:     types.StrOrNil(d.BoardType),
		MinStayNights: d.NightCount,
	}
	if lp := d.LowestPrice; lp != nil {
		out.Price = lp.Price
		out.ListPrice = lp.OriginalPrice
		out.Currency = types.StrOrNil(lp.CurrencyCode)
		if lp.DiscountRatio != nil {
			out.DiscountRate = *lp.DiscountRatio
		}
		out.Available = lp.Price != nil && *lp.Price > 0
	}
	return out
}
