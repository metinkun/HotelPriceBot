package coraltatil

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestNormalizePrice(t *testing.T) {
	b, err := os.ReadFile("testdata/price.json")
	if err != nil {
		t.Fatal(err)
	}
	var d PriceData
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	p := normalizePrice(&d)
	if !p.Available || p.Price == nil || *p.Price != 34020 || *p.ListPrice != 63000 || p.DiscountRate != 46 {
		t.Fatalf("lead beklenen degil: %+v", p)
	}
	if *p.RoomName != "Salamis Garden Oda" || *p.BoardType != "Ultra Her Şey Dahil" || *p.Currency != "TL" || *p.HotelName != "Salamis Bay Conti" || *p.City != "Magusa" {
		t.Errorf("alanlar: %v %v %v %v %v", *p.RoomName, *p.BoardType, *p.Currency, *p.HotelName, *p.City)
	}
	if len(p.Rooms) != 4 || p.Rooms[0].RoomName != "Salamis Garden Oda" || p.Rooms[1].RoomName != "Junior Villa Oda" {
		t.Fatalf("oda sirasi: %+v", p.Rooms)
	}
	if p.Rooms[2].Status != "on_request" || p.Rooms[3].Status != "sold_out" || *p.Rooms[3].Price != 32400 {
		t.Errorf("durumlar: %+v %+v", p.Rooms[2], p.Rooms[3])
	}
}

func TestBuildCriteriaAndSlug(t *testing.T) {
	c := buildCriteria("3787", "salamis-bay-conti", "Salamis Bay Conti", "2026-12-15", 3, 2, []int{5})
	b, _ := json.Marshal(c)
	s := string(b)
	for _, want := range []string{`"beginDates":["2026-12-15","2026-12-15"]`, `"nights":[{"value":3}]`, `"id":"3787-7"`, `"passengerType":1`, `"age":5`, `"adultPassengerCount":2`, `"childOrInfantCount":1`} {
		if !strings.Contains(s, want) {
			t.Errorf("kriterde eksik: %s\n%s", want, s)
		}
	}
	for in, want := range map[string]string{
		"https://www.coraltatil.com/salamis-bay-conti/?sidp=abc&p=2":  "salamis-bay-conti",
		"https://coraltatil.com/Acapulco-Resort-Convention-Spa-Hotel": "acapulco-resort-convention-spa-hotel",
		"https://www.etstur.com/x":                                    "",
	} {
		if got := slugFromURL(in); got != want {
			t.Errorf("slug(%s)=%q, beklenen %q", in, got, want)
		}
	}
	if n, _ := nightsOf("2026-12-15", "2026-12-18"); n != 3 {
		t.Errorf("gece: %d", n)
	}
}
