package tatilsitesi

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"pricebot/internal/types"
)

func loadJSON(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

func loadRows(t *testing.T, name string) []HotelRow {
	var r hotelSearchResponse
	loadJSON(t, name, &r)
	return r.D
}

func TestNormalizePrice(t *testing.T) {
	p := normalizePrice(loadRows(t, "hotel_rooms.json"))
	if !p.Available || p.Price == nil || *p.Price != 35700 {
		t.Fatalf("35700 bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 51000 || p.DiscountRate != 30 {
		t.Errorf("liste 51000 / %%30 bekleniyordu: list=%v rate=%v", p.ListPrice, p.DiscountRate)
	}
	if types.Deref(p.RoomName) != "Villa Oda" || types.Deref(p.BoardType) != "Yarım Pansiyon" {
		t.Errorf("oda/pansiyon yanlis: %v %v", p.RoomName, p.BoardType)
	}
	if types.Deref(p.Currency) != "TL" {
		t.Errorf("TL bekleniyordu: %v", p.Currency)
	}
	if p.CampaignName == nil || p.MinStayNights == nil || *p.MinStayNights != 1 {
		t.Errorf("kampanya/min geceleme eksik: %v %v", p.CampaignName, p.MinStayNights)
	}
}

func TestNormalizePriceSkipsStopSale(t *testing.T) {
	rows := loadRows(t, "hotel_rooms.json")
	// En ucuz satir StopSale ise atlanmali.
	rows[0].SaleAmount = 1
	rows[0].StopSale = true
	p := normalizePrice(rows)
	if p.Price == nil || *p.Price != 36750 {
		t.Errorf("StopSale satiri atlanip 36750 bekleniyordu: %v", p.Price)
	}
	for i := range rows {
		rows[i].StopSale = true
	}
	if normalizePrice(rows).Available {
		t.Errorf("tum satirlar StopSale ise Available=false olmali")
	}
}

func TestPickFlights(t *testing.T) {
	var air AirResponse
	loadJSON(t, "air.json", &air)
	fp, err := pickFlights(&air)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Total != 10333.58 || fp.Currency != "TL" {
		t.Errorf("5528 + 4805.58 = 10333.58 TL bekleniyordu: %+v", fp)
	}
}

func TestPickFlightsContractedPegasusRule(t *testing.T) {
	air := &AirResponse{}
	air.D.Status = "Ready"
	air.D.Data = &struct {
		Recommendations []FlightOption `json:"Recommendations"`
		FlightOptions   []FlightOption `json:"FlightOptions"`
	}{FlightOptions: []FlightOption{
		{OptionFlag: "OUT", TotalFare: 1000, Currency: "TL", Segments: []FlightSegment{{OperatingAirline: "PC", Cabin: "Contracted"}}},
		{OptionFlag: "IN", TotalFare: 900, Currency: "TL", Segments: []FlightSegment{{OperatingAirline: "VF", Cabin: "ECO"}}},
		{OptionFlag: "IN", TotalFare: 1200, Currency: "TL", Segments: []FlightSegment{{OperatingAirline: "PC", Cabin: "ECO"}}},
	}}
	fp, err := pickFlights(air)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Total != 2200 {
		t.Errorf("anlasmali Pegasus gidis -> donus de Pegasus (1000+1200) bekleniyordu: %v", fp.Total)
	}
	if _, err := pickFlights(&AirResponse{}); err == nil {
		t.Errorf("Data yoksa hata bekleniyordu")
	}
}

func TestPickTransfer(t *testing.T) {
	var tr TransferResponse
	loadJSON(t, "transfer.json", &tr)
	total, desc := pickTransfer(&tr)
	if total != 5000 || desc == "" {
		t.Errorf("2500 + 2500 = 5000 bekleniyordu: %v %q", total, desc)
	}
	tr.D.RoundTripTransferList = []Transfer{{Description: "Gidis-Donus", SaleAmount: 4000}}
	if total, desc = pickTransfer(&tr); total != 4000 || desc != "Gidis-Donus" {
		t.Errorf("daha ucuz gidis-donus secilmeliydi: %v %q", total, desc)
	}
	if total, _ = pickTransfer(&TransferResponse{}); total != 0 {
		t.Errorf("transfer yoksa 0 bekleniyordu: %v", total)
	}
}

func TestNormalizePackages(t *testing.T) {
	rows := loadRows(t, "package_rooms.json")
	rooms := normalizePackages(rows, 10333.58, 5000, types.Int(3))
	if len(rooms) != 2 {
		t.Fatalf("2 oda (Villa, Kara Manzarali) bekleniyordu: %d", len(rooms))
	}
	first := rooms[0]
	if first.RoomName != "Villa Oda" || len(first.Boards) != 2 || first.NightCount == nil || *first.NightCount != 3 {
		t.Fatalf("ilk oda Villa Oda / 2 pansiyon bekleniyordu: %+v", first)
	}
	b := first.Boards[0]
	// 30273.60 + 10333.58 + 5000 = 45607.18
	if b.Price == nil || *b.Price != 45607.18 {
		t.Errorf("45607.18 bekleniyordu: %v", b.Price)
	}
	if b.ListPrice == nil || *b.ListPrice != 66333.58 || b.DiscountRate <= 0 {
		t.Errorf("liste 51000+15333.58 bekleniyordu: %v rate=%v", b.ListPrice, b.DiscountRate)
	}
	if types.Deref(b.BoardType) != "Yarım Pansiyon" || types.Deref(b.Currency) != "TL" {
		t.Errorf("pansiyon/para birimi yanlis: %+v", b)
	}
}

func TestParseDetailPage(t *testing.T) {
	b, err := os.ReadFile("testdata/detail_snippet.html")
	if err != nil {
		t.Fatal(err)
	}
	r := parseDetailPage(string(b))
	if r.ProviderHotelID != "ACAPULCO" || r.CityCode != "ECN" || r.CityID != "58" || r.TownID != "232" {
		t.Fatalf("sayfa degiskenleri cozulemedi: %+v", r)
	}
	if r.HotelName != "Acapulco Resort Hotel & Convention & SPA" {
		t.Errorf("otel adi yanlis: %q", r.HotelName)
	}
}

func TestToSlashDate(t *testing.T) {
	if d, _ := toSlashDate("2026-12-05"); d != "05/12/2026" {
		t.Errorf("05/12/2026 bekleniyordu, %s", d)
	}
	if _, err := toSlashDate("05.12.2026"); err == nil {
		t.Errorf("gecersiz format hata vermeli")
	}
}

func TestBuildChildParams(t *testing.T) {
	cp, err := buildChildParams(2, []int{5, 0})
	if err != nil || cp.Count != 2 || cp.Ages != [3]int{5, 0, 0} {
		t.Errorf("2 cocuk [5 0 0] bekleniyordu: %+v %v", cp, err)
	}
	if _, err := buildChildParams(1, nil); err == nil {
		t.Errorf("yas olmadan cocuk sayisi hata vermeli")
	}
	if _, err := buildChildParams(4, []int{1, 2, 3, 4}); err == nil {
		t.Errorf("4 cocuk desteklenmemeli")
	}
}

func TestNewGUID(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if g := newGUID(); !re.MatchString(g) {
		t.Errorf("uuid v4 formati bekleniyordu: %s", g)
	}
}
