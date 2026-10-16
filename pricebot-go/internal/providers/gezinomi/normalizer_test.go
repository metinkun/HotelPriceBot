package gezinomi

import (
	"encoding/json"
	"os"
	"testing"

	"pricebot/internal/types"
)

func TestInvalidResponse(t *testing.T) {
	b, _ := os.ReadFile("testdata/invalid.json")
	var r PriceResponse
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	p := normalizePrice(&r, types.Str("X"))
	if p.Available || p.Price != nil {
		t.Fatalf("Invalid yanit musait olmamali: %+v", p)
	}
	if p.HotelName == nil || *p.HotelName != "X" {
		t.Errorf("otel adi mapping'den gelmeli")
	}
}

func TestNormalizeSynthetic(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	id := 7
	r := &PriceResponse{Status: "Success"}
	r.Data = &struct {
		RoomDetails []RoomDetail `json:"roomDetails"`
		SearchID    string       `json:"searchId"`
	}{RoomDetails: []RoomDetail{{RoomID: &id, RoomName: "Standart", ReservationDetails: []ReservationDetail{
		{ConceptName: "Her Şey Dahil", TotalPrice: f(60000), TotalDiscountedPrice: f(45000), DiscountRate: f(25), FlightPackageTotalPrice: f(80000), FlightPackageTotalDiscountedPrice: f(70000)},
		{ConceptName: "Yarım Pansiyon", TotalPrice: f(50000), TotalDiscountedPrice: f(40000), DiscountRate: f(20)},
	}}}}
	p := normalizePrice(r, nil)
	if !p.Available || *p.Price != 40000 || *p.BoardType != "Yarım Pansiyon" {
		t.Fatalf("en ucuz teklif secilmeli: %+v", p)
	}
	rooms := normalizePackages(r, types.Int(3))
	if len(rooms) != 1 || len(rooms[0].Boards) != 1 || *rooms[0].Boards[0].Price != 70000 {
		t.Fatalf("paket normalize hatali: %+v", rooms)
	}
}

func TestBuildPersons(t *testing.T) {
	ps := buildPersons(2, []int{5})
	if len(ps) != 3 || ps[2].IsChild != true || ps[2].Age != 5 || ps[0].IsChild {
		t.Fatalf("kisi listesi hatali: %+v", ps)
	}
}
