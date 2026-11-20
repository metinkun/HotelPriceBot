package setur

import (
	"os"
	"strings"
	"testing"
	"time"

	"pricebot/internal/provider"
)

func TestParseTrimmedPage(t *testing.T) {
	b, err := os.ReadFile("testdata/detail_trimmed.html")
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.ReplaceAll(string(b), `\"`, `"`)
	lp := parseLowest(raw)
	if lp == nil || lp.Price == nil || *lp.Price <= 0 {
		t.Fatalf("lowestPrice cozulemedi")
	}
	d := &HotelData{LowestPrice: lp, BoardType: provider.FirstMatch(reBoard, raw), HotelName: provider.FirstMatch(reMetaTitle, raw), City: provider.FirstMatch(reCity, raw), NightCount: intPtr(provider.FirstMatch(reNights, raw))}
	p := normalize(d)
	if !p.Available || p.Price == nil || *p.Price != *lp.Price {
		t.Fatalf("normalize hatali: %+v", p)
	}
	if p.HotelName == nil || !strings.Contains(*p.HotelName, "Acapulco") {
		t.Errorf("otel adi bekleniyordu: %v", p.HotelName)
	}
	if p.City == nil || *p.City != "Girne" {
		t.Errorf("Girne bekleniyordu: %v", p.City)
	}
	if p.MinStayNights == nil || *p.MinStayNights != 3 {
		t.Errorf("3 gece bekleniyordu: %v", p.MinStayNights)
	}
}

func TestBuildRoomParam(t *testing.T) {
	if got := buildRoomParam(2, nil); got != "2" {
		t.Errorf("2 bekleniyordu, %s", got)
	}
	got := buildRoomParam(2, []int{1})
	if !strings.HasPrefix(got, "2_") || strings.Contains(got, ",") {
		t.Errorf("2_<dogumTarihi> bekleniyordu, %s", got)
	}
	y := time.Now().Year() - 1
	if !strings.Contains(got, string(rune('0'+(y/1000)%10))) {
		t.Errorf("dogum yili bekleniyordu: %s", got)
	}
}

func TestSlugFromURL(t *testing.T) {
	if s := slugFromURL("https://www.setur.com.tr/acapulco-resort?in=2026-01-01"); s != "acapulco-resort" {
		t.Errorf("slug hatali: %s", s)
	}
}

func TestDecode(t *testing.T) {
	got := decode(`Acapulco \\u0026 Casino\\t Hotel\\`)
	if got == nil || *got != "Acapulco & Casino Hotel" {
		t.Errorf("decode hatali: %v", got)
	}
}

func TestParseRooms(t *testing.T) {
	b, err := os.ReadFile("testdata/initial_data_rooms.txt")
	if err != nil {
		t.Fatal(err)
	}
	rooms := parseRooms(string(b))
	if len(rooms) != 3 {
		t.Fatalf("3 oda bekleniyordu: %d", len(rooms))
	}
	if rooms[0].RoomName != "Salamis Park Oda" || rooms[0].BoardType != "Ultra Her Şey Dahil" || *rooms[0].Price != 36000 || *rooms[0].ListPrice != 60000 || rooms[0].DiscountRate != 40 || !rooms[0].Available {
		t.Fatalf("ilk oda beklenen degil: %+v", rooms[0])
	}
	// AskSellPlan -> on_request ve musait olanlardan sonra siralanir
	last := rooms[len(rooms)-1]
	if last.Status != "on_request" || last.Available {
		t.Fatalf("on_request bekleniyordu: %+v", last)
	}
}

func TestPackageRoomsIncludeFlightsAndTransfers(t *testing.T) {
	b, err := os.ReadFile("testdata/package_initial.txt")
	if err != nil {
		t.Fatal(err)
	}
	d, err := parsePackageData(string(b))
	if err != nil {
		t.Fatal(err)
	}
	flights, transfers, note := pkgExtras(d)
	if flights != 14278 || transfers != 4800 {
		t.Fatalf("ucus %v transfer %v beklenen 14278/4800 (%s)", flights, transfers, note)
	}
	rooms := normalizePackageRooms(d)
	if len(rooms) < 2 || rooms[0].RoomName != "SALAMIS PARK ODA" {
		t.Fatalf("odalar: %+v", rooms)
	}
	b0 := rooms[0].Boards[0]
	if *b0.Price != 41078 || *b0.ListPrice != 59078 || *b0.BoardType != "Ultra Herşey Dahil" || *b0.Cancellation != "FREE_CANCELLATION" {
		t.Fatalf("Salamis Park paket: %+v", b0)
	}
}
