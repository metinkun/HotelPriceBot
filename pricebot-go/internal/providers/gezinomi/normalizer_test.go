package gezinomi

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"pricebot/internal/types"
)

func load(t *testing.T, name string) *PriceResponse {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var r PriceResponse
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func TestInvalidResponse(t *testing.T) {
	r := load(t, "invalid.json")
	p := normalizePrice(r, types.Str("X"))
	if p.Available || p.Price != nil {
		t.Fatalf("Invalid yanit musait olmamali: %+v", p)
	}
	if p.HotelName == nil || *p.HotelName != "X" {
		t.Errorf("otel adi mapping'den gelmeli")
	}
	b, _ := os.ReadFile("testdata/invalid.json")
	if !isInvalid(b) {
		t.Errorf("isInvalid true olmali")
	}
}

// Gercek yanit (Acapulco, 5-8 Aralik 2026, 2 yetiskin): en ucuz Villa / Yarim Pansiyon 31.620 TL.
func TestNormalizeRealPrice(t *testing.T) {
	r := load(t, "room_prices.json")
	b, _ := os.ReadFile("testdata/room_prices.json")
	if isInvalid(b) {
		t.Fatalf("basarili yanit Invalid sayilmamali")
	}
	p := normalizePrice(r, nil)
	if !p.Available || p.Price == nil || *p.Price != 31620 {
		t.Fatalf("en ucuz fiyat 31620 bekleniyor: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 51000 || p.DiscountRate != 38 {
		t.Errorf("liste fiyati/indirim hatali: list=%v rate=%v", p.ListPrice, p.DiscountRate)
	}
	if types.Deref(p.RoomName) != "Acapulco Villa Odası" || types.Deref(p.BoardType) != "Yarım Pansiyon" {
		t.Errorf("oda/pansiyon hatali: %q %q", types.Deref(p.RoomName), types.Deref(p.BoardType))
	}
	if types.Deref(p.Currency) != "TL" {
		t.Errorf("para birimi TL olmali")
	}
	// Otel-only yanitta paket alanlari 0 -> paket listesi bos
	if rooms := normalizePackages(r, types.Int(3)); len(rooms) != 0 {
		t.Errorf("otel-only yanitta paket olmamali: %+v", rooms)
	}
}

// Gercek paket yaniti (SAW -> ECN gidis-donus): oda bazinda flightPackageTotal* alanlari.
func TestNormalizeRealPackage(t *testing.T) {
	r := load(t, "package_prices.json")
	rooms := normalizePackages(r, types.Int(3))
	// Fixture'daki 3 roomDetails kaydi 2 odaya gruplanir (1485 x2 pansiyon, 1483)
	if len(rooms) != 2 {
		t.Fatalf("2 oda bekleniyor: %d", len(rooms))
	}
	first := rooms[0]
	if first.RoomID != "1485" || first.RoomName != "Acapulco Villa Odası" || len(first.Boards) != 2 {
		t.Fatalf("ilk oda hatali: %+v", first)
	}
	b0 := first.Boards[0]
	if types.Deref(b0.BoardType) != "Yarım Pansiyon" || b0.Price == nil || *b0.Price != 41166.09 || b0.ListPrice == nil || *b0.ListPrice != 60546.09 || b0.DiscountRate != 38 {
		t.Errorf("ilk pansiyon hatali: %+v", b0)
	}
	if first.NightCount == nil || *first.NightCount != 3 {
		t.Errorf("gece sayisi 3 olmali")
	}
	// Ayni yanittan otel-only fiyat da okunabilir
	if p := normalizePrice(r, nil); !p.Available || *p.Price != 31620 {
		t.Errorf("paket yanitinda otel fiyati 31620 olmali: %+v", p)
	}
}

func TestAbsentSkipped(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	id := 7
	r := &PriceResponse{Status: "Success", Data: &PriceData{RoomDetails: []RoomDetail{{RoomID: &id, RoomName: "Standart", ReservationDetails: []ReservationDetail{
		{Absent: true, ConceptName: "Her Şey Dahil", TotalPrice: f(60000), TotalDiscountedPrice: f(10000), FlightPackageTotalDiscountedPrice: f(20000)},
		{ConceptName: "Yarım Pansiyon", TotalPrice: f(50000), TotalDiscountedPrice: f(40000), DiscountRate: f(20), FlightPackageTotalPrice: f(80000), FlightPackageTotalDiscountedPrice: f(70000)},
	}}}}}
	p := normalizePrice(r, nil)
	if !p.Available || *p.Price != 40000 || *p.BoardType != "Yarım Pansiyon" {
		t.Fatalf("absent teklif atlanmali: %+v", p)
	}
	rooms := normalizePackages(r, types.Int(3))
	if len(rooms) != 1 || len(rooms[0].Boards) != 1 || *rooms[0].Boards[0].Price != 70000 {
		t.Fatalf("paket normalize hatali: %+v", rooms)
	}
}

func TestBuildPersons(t *testing.T) {
	ps := buildPersons(2, []int{5, 17})
	if len(ps) != 4 || !ps[2].IsChild || ps[2].Age != 5 || ps[0].IsChild || ps[3].IsChild {
		t.Fatalf("kisi listesi hatali: %+v", ps)
	}
}

func TestBuildBody(t *testing.T) {
	var hotelOnly, pkg map[string]any
	if err := json.Unmarshal(buildBody(932, "2026-12-05", "2026-12-08", 2, nil, nil), &hotelOnly); err != nil {
		t.Fatal(err)
	}
	if hotelOnly["checkInDate"] != "2026-12-05T00:00:00" || hotelOnly["searchTypeId"].(float64) != 1 {
		t.Errorf("otel govdesi hatali: %v", hotelOnly)
	}
	fs := hotelOnly["flightSearch"].(map[string]any)
	if len(fs["flights"].([]any)) != 0 || fs["passengers"].(map[string]any)["adult"].(float64) != 0 {
		t.Errorf("otel govdesinde ucus olmamali: %v", fs)
	}
	if err := json.Unmarshal(buildBody(932, "2026-12-05", "2026-12-08", 2, []int{5}, &flightOpts{Origin: "SAW", Destination: "ECN"}), &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg["searchTypeId"].(float64) != 2 {
		t.Errorf("paket searchTypeId 2 olmali")
	}
	fs = pkg["flightSearch"].(map[string]any)
	legs := fs["flights"].([]any)
	if len(legs) != 2 || legs[0].(map[string]any)["originCode"] != "SAW" || legs[1].(map[string]any)["originCode"] != "ECN" || legs[0].(map[string]any)["flightDate"] != "2026-12-05T03:00:00.000Z" {
		t.Errorf("ucus bacaklari hatali: %v", legs)
	}
	if pax := fs["passengers"].(map[string]any); pax["adult"].(float64) != 2 || pax["child"].(float64) != 1 {
		t.Errorf("yolcu sayilari hatali: %v", pax)
	}
}

func TestParseDetailPage(t *testing.T) {
	b, err := os.ReadFile("testdata/detail_snippet.html")
	if err != nil {
		t.Fatal(err)
	}
	r, err := parseDetailPage(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if r.ProviderHotelID != "932" || !strings.HasPrefix(r.HotelName, "Acapulco Resort Convention") {
		t.Errorf("otel id/adi hatali: %+v", r)
	}
	if r.HotelLocationID != "14361" || r.ParentID != "14360" {
		t.Errorf("hotelLocationId/parentId hatali: %+v", r)
	}
}

func TestAirportsFixture(t *testing.T) {
	b, _ := os.ReadFile("testdata/airports.json")
	var out airportsResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data) != 1 || out.Data[0].AirportCode != "ECN" || out.Data[0].IsCity {
		t.Fatalf("havalimani hatali: %+v", out.Data)
	}
}
