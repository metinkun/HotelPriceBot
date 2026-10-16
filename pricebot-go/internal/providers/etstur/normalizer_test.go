package etstur

import (
	"encoding/json"
	"os"
	"testing"
)

func loadSearch(t *testing.T) *searchResponse {
	t.Helper()
	b, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Fatal(err)
	}
	var r searchResponse
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func TestNormalizeHotel_Acapulco(t *testing.T) {
	r := loadSearch(t)
	if !r.Success || r.Result == nil || len(r.Result.Hotels) == 0 {
		t.Fatal("fixture bos")
	}
	var h *Hotel
	for i := range r.Result.Hotels {
		if r.Result.Hotels[i].HotelID == "61b893df0b29" {
			h = &r.Result.Hotels[i]
		}
	}
	if h == nil {
		t.Fatal("Acapulco fixture'da yok")
	}
	p := normalizeHotel(h)
	if !p.Available || p.Price == nil || *p.Price <= 0 {
		t.Fatalf("fiyat bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice < *p.Price {
		t.Fatalf("liste fiyati satis fiyatindan kucuk olamaz: %+v", p)
	}
	if p.DiscountRate <= 0 {
		t.Errorf("indirim orani bekleniyordu")
	}
	if p.City == nil || *p.City != "Girne" {
		t.Errorf("city Girne bekleniyordu, %v", p.City)
	}
	if p.BoardType == nil || *p.BoardType == "" {
		t.Errorf("boardType bos")
	}
	if p.Currency == nil || *p.Currency != "TL" {
		t.Errorf("currency TL bekleniyordu")
	}
}

func TestNormalizeHotel_NoRoom(t *testing.T) {
	p := normalizeHotel(&Hotel{HotelID: "x", Name: "Test"})
	if p.Available || p.Price != nil {
		t.Fatalf("odasiz otel musait olmamali: %+v", p)
	}
}

func TestToEtsturDate(t *testing.T) {
	for in, want := range map[string]string{"2026-06-24": "2026-06-24T00:00:00.000", "24.06.2026": "2026-06-24T00:00:00.000"} {
		got, err := ToEtsturDate(in)
		if err != nil || got != want {
			t.Errorf("%s -> %s (%v), beklenen %s", in, got, err, want)
		}
	}
	if _, err := ToEtsturDate("06/24/2026"); err == nil {
		t.Error("gecersiz format hata vermeli")
	}
}

func TestBuildSearchBody(t *testing.T) {
	b, err := buildSearchBody("Kibris-Otelleri", Occupancy{CheckIn: "2026-11-05", CheckOut: "2026-11-08", Adults: 2, ChildAges: []int{5}}, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if b.URL != "Kibris-Otelleri" || len(b.Rooms) != 1 || b.Rooms[0].AdultCount != 2 || len(b.Rooms[0].ChildAges) != 1 {
		t.Fatalf("beklenmeyen govde: %+v", b)
	}
	raw, _ := json.Marshal(b)
	for _, must := range []string{`"minPrice":null`, `"filters":[]`, `"excludeHotelIds":[]`, `"sort":{"type":"","direction":""}`} {
		if !contains(string(raw), must) {
			t.Errorf("JSON'da %s bekleniyordu: %s", must, raw)
		}
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestPackageMultiFormat(t *testing.T) {
	b, err := os.ReadFile("testdata/package_multi.json")
	if err != nil {
		t.Fatal(err)
	}
	var pr packageResponse
	if err := json.Unmarshal(b, &pr); err != nil {
		t.Fatal(err)
	}
	rooms := pr.allRooms()
	if len(rooms) != 3 {
		t.Fatalf("roomGroups altindan 3 oda bekleniyordu, %d", len(rooms))
	}
	n := normalizePackageRoom(rooms[0])
	if n.RoomID != "6530" || n.RoomName == "" || n.RoomSize == nil || *n.RoomSize != 30 || n.NightCount == nil || *n.NightCount != 3 {
		t.Fatalf("oda alanlari hatali: %+v", n)
	}
	if len(n.Boards) != 1 || n.Boards[0].Price == nil || *n.Boards[0].Price != 44376.2 || *n.Boards[0].ListPrice != 51000 || n.Boards[0].DiscountRate != 40 {
		t.Fatalf("board fiyati hatali: %+v", n.Boards)
	}
	if n.Boards[0].BoardType == nil || *n.Boards[0].BoardType != "Yarım Pansiyon" || n.Boards[0].Cancellation == nil || *n.Boards[0].Cancellation != "FREE_CANCELLATION" {
		t.Fatalf("pansiyon/iptal hatali: %+v", n.Boards[0])
	}
}
