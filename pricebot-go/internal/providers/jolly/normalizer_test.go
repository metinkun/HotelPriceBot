package jolly

import (
	"encoding/json"
	"os"
	"testing"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

func load(t *testing.T, name string) *ReservationResponse {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var r ReservationResponse
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func TestNormalizePrice(t *testing.T) {
	p := normalizePrice(load(t, "reservation.json"))
	if !p.Available || p.Price == nil || *p.Price != 33150 {
		t.Fatalf("33150 bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 51000 || p.DiscountRate != 35 {
		t.Errorf("liste 51000 / %%35 bekleniyordu: list=%v rate=%v", p.ListPrice, p.DiscountRate)
	}
	if p.HotelName == nil || *p.HotelName == "" || p.City == nil || *p.City != "Girne" {
		t.Errorf("otel adi/sehir eksik: %v %v", p.HotelName, p.City)
	}
	if p.RoomName == nil || *p.RoomName == "" || p.BoardType == nil || *p.BoardType == "" {
		t.Errorf("oda/pansiyon eksik: %v %v", p.RoomName, p.BoardType)
	}
	if p.Currency == nil || *p.Currency != "TL" {
		t.Errorf("TRY -> TL bekleniyordu")
	}
}

func TestNormalizePackages(t *testing.T) {
	rooms := normalizePackages(load(t, "package.json"), types.Int(3))
	if len(rooms) == 0 {
		t.Fatal("paket odasi bekleniyordu")
	}
	boards := 0
	for _, r := range rooms {
		if r.RoomName == "" {
			t.Errorf("oda adi bos")
		}
		for _, b := range r.Boards {
			boards++
			if b.Price == nil || *b.Price <= 0 {
				t.Errorf("fiyatsiz board: %+v", b)
			}
			if b.ListPrice != nil && *b.ListPrice < *b.Price {
				t.Errorf("liste < satis: %+v", b)
			}
			if b.BoardType == nil {
				t.Errorf("pansiyon cozulemedi: %s", r.RoomName)
			}
		}
	}
	if boards < 3 {
		t.Errorf("en az 3 pansiyon secenegi bekleniyordu, %d", boards)
	}
}

func TestResolveSnippet(t *testing.T) {
	b, _ := os.ReadFile("testdata/detail_snippet.html")
	html := string(b)
	if provider.FirstMatch(reSPID, html) != "594195" || provider.FirstMatch(reSPType, html) != "Cyprus" {
		t.Fatalf("SearchParameters cozulemedi: %q", html)
	}
}

func TestBuildRoomsParam(t *testing.T) {
	if got := buildRoomsParam(2, []int{5, 1, 0}); got != "2-5-1-1" {
		t.Errorf("2-5-1-1 bekleniyordu, %s", got)
	}
	if got := buildRoomsParam(3, nil); got != "3" {
		t.Errorf("3 bekleniyordu, %s", got)
	}
}

func TestToJollyDate(t *testing.T) {
	if d, _ := toJollyDate("2026-08-31"); d != "2026.08.31" {
		t.Errorf("2026.08.31 bekleniyordu, %s", d)
	}
	if d, _ := toJollyDate("31.08.2026"); d != "2026.08.31" {
		t.Errorf("2026.08.31 bekleniyordu, %s", d)
	}
}
