package api

import (
	"testing"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/types"
)

func future(days int) string { return time.Now().AddDate(0, 0, days).Format("2006-01-02") }

func TestValidatePriceQuery(t *testing.T) {
	ok := types.PriceQuery{HotelID: "h", CheckIn: future(10), CheckOut: future(13), Adults: 2, Children: 1, ChildAges: []int{5}}
	if err := validatePriceQuery(&ok); err != nil {
		t.Fatalf("gecerli sorgu reddedildi: %v", err)
	}
	cases := map[string]types.PriceQuery{
		"gecmis":        {HotelID: "h", CheckIn: "2020-01-01", CheckOut: future(1), Adults: 2},
		"ters tarih":    {HotelID: "h", CheckIn: future(5), CheckOut: future(3), Adults: 2},
		"format":        {HotelID: "h", CheckIn: "05.11.2026", CheckOut: future(3), Adults: 2},
		"adults 0":      {HotelID: "h", CheckIn: future(1), CheckOut: future(3), Adults: 0},
		"yas sayisi":    {HotelID: "h", CheckIn: future(1), CheckOut: future(3), Adults: 2, Children: 2, ChildAges: []int{5}},
		"yas araligi":   {HotelID: "h", CheckIn: future(1), CheckOut: future(3), Adults: 2, Children: 1, ChildAges: []int{25}},
		"hotelId bos":   {CheckIn: future(1), CheckOut: future(3), Adults: 2},
		"cok uzun":      {HotelID: "h", CheckIn: future(1), CheckOut: future(40), Adults: 2},
	}
	for name, q := range cases {
		err := validatePriceQuery(&q)
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", name)
			continue
		}
		if apperr.Status(err) != 400 {
			t.Errorf("%s: 400 bekleniyordu, %d", name, apperr.Status(err))
		}
	}
}

func TestValidatePackageQuery(t *testing.T) {
	q := types.PackageQuery{HotelID: "h", AirportCode: "saw", CheckIn: future(10), CheckOut: future(13), Adults: 2}
	if err := validatePackageQuery(&q); err != nil || q.AirportCode != "SAW" {
		t.Fatalf("IATA buyuk harfe cevrilmeli: %v %s", err, q.AirportCode)
	}
	q.AirportCode = "ISTANBUL"
	if err := validatePackageQuery(&q); err == nil {
		t.Error("gecersiz IATA reddedilmeli")
	}
}
