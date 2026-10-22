package tatildukkani

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func loadPage(t *testing.T) *HotelPage {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(read(t, "page.json"), &env); err != nil {
		t.Fatal(err)
	}
	p, err := parseHotelPage(env.Data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func loadAvailable(t *testing.T, name string) *AvailableHotel {
	t.Helper()
	// Fixture ham Socket.IO paketi: 42["available-hotel","{...}"]
	evt, payload, ok := parseEvent(read(t, name))
	if !ok || evt != "available-hotel" {
		t.Fatalf("socket paketi parse edilemedi: %s", name)
	}
	var av AvailableHotel
	if err := json.Unmarshal(payload, &av); err != nil {
		t.Fatal(err)
	}
	return &av
}

func TestParseHotelPage(t *testing.T) {
	p := loadPage(t)
	if p.HotelID != 1404 || p.Title != "Acapulco Resort Convention Spa" || p.City != "Girne" {
		t.Fatalf("sayfa cozumu yanlis: %+v", p)
	}
	if p.AirportCode != "ECN" || len(p.Criteria) == 0 {
		t.Errorf("airportCode/criteria eksik: %s %d", p.AirportCode, len(p.Criteria))
	}
	var c struct {
		HotelID int `json:"hotelId"`
	}
	if json.Unmarshal(p.Criteria, &c) != nil || c.HotelID != 1404 {
		t.Errorf("avabilityCriteria.hotelId 1404 bekleniyordu: %s", p.Criteria)
	}
}

func TestNormalizePrice(t *testing.T) {
	p := normalizePrice(loadPage(t), loadAvailable(t, "available.json"))
	if !p.Available || p.Price == nil || *p.Price != 33150 {
		t.Fatalf("33150 bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 51000 || p.DiscountRate != 35 {
		t.Errorf("liste 51000 / %%35 bekleniyordu: list=%v rate=%v", p.ListPrice, p.DiscountRate)
	}
	if p.RoomName == nil || *p.RoomName != "Acapulco villa" || p.BoardType == nil || *p.BoardType != "Yarım pansiyon" {
		t.Errorf("oda/pansiyon yanlis: %v %v", p.RoomName, p.BoardType)
	}
	if p.HotelName == nil || *p.HotelName != "Acapulco Resort Convention Spa" || p.City == nil || *p.City != "Girne" {
		t.Errorf("otel/sehir yanlis: %v %v", p.HotelName, p.City)
	}
	if p.Currency == nil || *p.Currency != "TL" {
		t.Errorf("TRY -> TL bekleniyordu: %v", p.Currency)
	}
	if p.CampaignName == nil || *p.CampaignName == "" {
		t.Errorf("roomCampaign adi bekleniyordu")
	}
}

func TestNormalizeUnavailable(t *testing.T) {
	av := &AvailableHotel{Status: "NOT_AVAILABLE", Rooms: []Room{{RoomTitle: "X", Status: "STOP_SALE", Fee: 100}}}
	p := normalizePrice(loadPage(t), av)
	if p.Available || p.Price != nil {
		t.Fatalf("musait degil bekleniyordu: %+v", p)
	}
	if p.HotelName == nil || p.City == nil {
		t.Errorf("otel adi/sehir korunmali")
	}
	if normalizePrice(nil, nil).Available {
		t.Errorf("nil girdi musait olmamali")
	}
}

func TestNormalizeOnRequestFallback(t *testing.T) {
	// Otel durumu ON_REQUEST, odalar da ON_REQUEST: site fiyati gosterir -> en ucuz on-request oda.
	p := normalizePrice(nil, loadAvailable(t, "on_request.json"))
	if !p.Available || p.Price == nil || *p.Price != 63000 {
		t.Fatalf("63000 (on-request) bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 126000 || p.DiscountRate != 50 {
		t.Errorf("liste 126000 / %%50 bekleniyordu: list=%v rate=%v", p.ListPrice, p.DiscountRate)
	}
	if p.RoomName == nil || *p.RoomName != "Kara manzaralı standart oda" || p.CampaignName != nil {
		t.Errorf("oda adi / kampanya yanlis: %v %v", p.RoomName, p.CampaignName)
	}
	// AVAILABLE oda daha pahali olsa bile ON_REQUEST'e tercih edilir; STOP_SALE sayilmaz.
	rooms := []Room{
		{RoomTitle: "A", Status: "ON_REQUEST", Fee: 100},
		{RoomTitle: "B", Status: "AVAILABLE", Fee: 300},
		{RoomTitle: "C", Status: "AVAILABLE", Fee: 200},
		{RoomTitle: "D", Status: "STOP_SALE", Fee: 50},
	}
	if best := cheapestRoom(rooms); best == nil || best.RoomTitle != "C" {
		t.Errorf("AVAILABLE en ucuz (C) bekleniyordu: %+v", best)
	}
}

func TestFlexFloat(t *testing.T) {
	var r Room
	if err := json.Unmarshal([]byte(`{"fee":33150,"realFee":"33150.00","posterPrice":"51000","discountPrice":null}`), &r); err != nil {
		t.Fatal(err)
	}
	if r.Fee != 33150 || r.RealFee != 33150 || r.PosterPrice != 51000 || r.DiscountPrice != 0 {
		t.Errorf("flexFloat yanlis: %+v", r)
	}
}

func TestChildBirthDate(t *testing.T) {
	out, _ := time.Parse("2006-01-02", "2026-12-08")
	// moment: 2026-12-08 - 5y = 2021-12-08, -1 ay = 2021-11-08, -10 gun = 2021-10-29
	if got := childBirthDate(out, 5); got != "2021-10-29" {
		t.Errorf("2021-10-29 bekleniyordu, %s", got)
	}
	// Ay sonu kirpma: 2026-03-31 - 1 ay = 2026-02-28, -10 gun = 2026-02-18 (yas 0)
	mar, _ := time.Parse("2006-01-02", "2026-03-31")
	if got := childBirthDate(mar, 0); got != "2026-02-18" {
		t.Errorf("2026-02-18 bekleniyordu, %s", got)
	}
	if got := chdParam(out, []int{5, 12}); got != "2021-10-29,2014-10-29" {
		t.Errorf("iki cocuk: %s", got)
	}
	if chdParam(out, nil) != "" {
		t.Errorf("cocuksuz bos olmali")
	}
	if adtParam(2) != "1970-01-01,1970-01-01" || adtParam(0) != "1970-01-01" {
		t.Errorf("adt parametresi yanlis: %s", adtParam(2))
	}
}

func TestSlugFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.tatildukkani.com/acapulco-resort-convention-spa":                     "acapulco-resort-convention-spa",
		"https://www.tatildukkani.com/acapulco-resort-convention-spa/?checkIn=2026-12-05": "acapulco-resort-convention-spa",
		"https://tatildukkani.com/club-simena#rooms":                                      "club-simena",
		"https://www.tatildukkani.com/":                                                   "",
	}
	for in, want := range cases {
		if got := slugFromURL(in); got != want {
			t.Errorf("%s -> %q, %q bekleniyordu", in, got, want)
		}
	}
}

func TestSioEventRoundTrip(t *testing.T) {
	b, err := sioEvent("available-hotel", map[string]any{"uid": "UID_x", "socketToken": "STK_y"})
	if err != nil {
		t.Fatal(err)
	}
	evt, payload, ok := parseEvent(b)
	if !ok || evt != "available-hotel" {
		t.Fatalf("paket geri cozulemedi: %s", b)
	}
	var m map[string]string
	if json.Unmarshal(payload, &m) != nil || m["uid"] != "UID_x" {
		t.Errorf("payload yanlis: %s", payload)
	}
}
