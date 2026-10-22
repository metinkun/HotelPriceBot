package prontotour

import (
	"encoding/json"
	"os"
	"testing"

	"pricebot/internal/types"
)

func loadDetail(t *testing.T, name string) *HotelDetailResponse {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var r HotelDetailResponse
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func loadBundle(t *testing.T, name string) *BundleResponse {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var r BundleResponse
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func TestNormalizePrice(t *testing.T) {
	p := normalizePrice(loadDetail(t, "hotel_detail.json"))
	if !p.Available || p.Price == nil || *p.Price != 30345 {
		t.Fatalf("30345 bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 51000 || p.DiscountRate != 41 {
		t.Errorf("liste 51000 / %%41 bekleniyordu: list=%v rate=%v", p.ListPrice, p.DiscountRate)
	}
	if p.HotelName == nil || *p.HotelName != "Acapulco Resort Hotel" {
		t.Errorf("otel adi eksik: %v", p.HotelName)
	}
	if p.RoomName == nil || *p.RoomName != "Villa Oda" || p.BoardType == nil || *p.BoardType != "Yarım Pansiyon" {
		t.Errorf("oda/pansiyon beklenen degil: %v %v", p.RoomName, p.BoardType)
	}
	if p.Currency == nil || *p.Currency != "TL" {
		t.Errorf("TL bekleniyordu")
	}
}

func TestNormalizePriceSkipsNoAllotment(t *testing.T) {
	d := loadDetail(t, "hotel_detail.json")
	// En ucuz odalarin kontenjanini kapat: sonraki satilabilir oda secilmeli.
	for i := range d.HotelRoomDetailsList {
		if d.HotelRoomDetailsList[i].RoomName == "Villa Oda" {
			d.HotelRoomDetailsList[i].Allotment = false
		}
	}
	p := normalizePrice(d)
	if p.RoomName == nil || *p.RoomName != "Kara manzaralı Standart" || p.Price == nil || *p.Price != 31238 {
		t.Fatalf("Kara manzarali Standart 31238 bekleniyordu: %+v", p)
	}
}

func TestNormalizePriceEmpty(t *testing.T) {
	p := normalizePrice(&HotelDetailResponse{HotelListResponse: &HotelSummary{Name: "X"}})
	if p.Available || p.Price != nil {
		t.Fatalf("musait degil bekleniyordu: %+v", p)
	}
}

func TestNormalizePackages(t *testing.T) {
	b := loadBundle(t, "bundle_search.json")
	d := loadDetail(t, "hotel_detail_package.json")
	rooms := normalizePackages(b, d, types.Int(3))
	if len(rooms) != 3 {
		t.Fatalf("3 oda (kontenjanli) bekleniyordu, %d: %+v", len(rooms), rooms)
	}
	first := rooms[0]
	if first.RoomName != "Villa Oda" || len(first.Boards) != 2 {
		t.Fatalf("ilk oda Villa Oda / 2 pansiyon bekleniyordu: %+v", first)
	}
	// Taban paket: 61341 liste / 41597 satis; en ucuz odanin farki 0.
	yp := first.Boards[0]
	if yp.Price == nil || *yp.Price != 41597 || yp.ListPrice == nil || *yp.ListPrice != 61341 || yp.DiscountRate != 32 {
		t.Errorf("Villa Oda YP 41597/61341 (%%32) bekleniyordu: %+v", yp)
	}
	// Kara manzarali Standart YP farki 1545 / 919.79 -> 42516.79 / 62886
	var kara *types.NormalizedPackageBoard
	for _, r := range rooms {
		if r.RoomName == "Kara manzaralı Standart" {
			kara = &r.Boards[0]
		}
	}
	if kara == nil || kara.Price == nil || *kara.Price != 42516.79 || kara.ListPrice == nil || *kara.ListPrice != 62886 {
		t.Errorf("Kara manzarali Standart 42516.79/62886 bekleniyordu: %+v", kara)
	}
	for _, r := range rooms {
		if r.NightCount == nil || *r.NightCount != 3 {
			t.Errorf("gece sayisi 3 bekleniyordu: %+v", r)
		}
		for _, bd := range r.Boards {
			if bd.BoardType == nil || *bd.BoardType == "" {
				t.Errorf("pansiyon bos: %+v", bd)
			}
			if bd.ListPrice != nil && *bd.ListPrice < *bd.Price {
				t.Errorf("liste < satis: %+v", bd)
			}
		}
	}
}

func TestNormalizePackagesWithoutDetail(t *testing.T) {
	b := loadBundle(t, "bundle_search.json")
	rooms := normalizePackages(b, nil, types.Int(3))
	if len(rooms) != 1 || rooms[0].RoomID != "lowest" {
		t.Fatalf("tek 'lowest' satir bekleniyordu: %+v", rooms)
	}
	bd := rooms[0].Boards[0]
	if bd.Price == nil || *bd.Price != 41597 || bd.BoardType == nil || *bd.BoardType != "Yarım Pansiyon" {
		t.Errorf("41597 / Yarim Pansiyon bekleniyordu: %+v", bd)
	}
	if normalizePackages(&BundleResponse{}, nil, nil) == nil || len(normalizePackages(&BundleResponse{}, nil, nil)) != 0 {
		t.Errorf("bos yanit icin bos liste bekleniyordu")
	}
}

func TestFlightSummary(t *testing.T) {
	s := flightSummary(loadBundle(t, "bundle_search.json"))
	if s == "" || !contains(s, "SAW→ECN") || !contains(s, "ECN→SAW") || !contains(s, "PC1912") {
		t.Fatalf("ucus ozeti beklenen degil: %q", s)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestBuildTravelers(t *testing.T) {
	tr := buildTravelers(2, 3, []int{5, 0, 16})
	if len(tr) != 5 {
		t.Fatalf("5 yolcu bekleniyordu: %+v", tr)
	}
	if tr[0].Type != travelerAdult || tr[0].Age != "35" {
		t.Errorf("yetiskin 35 bekleniyordu: %+v", tr[0])
	}
	if tr[2].Type != travelerChild || tr[2].Age != 5 {
		t.Errorf("cocuk 5 bekleniyordu: %+v", tr[2])
	}
	if tr[3].Type != travelerInfant {
		t.Errorf("0 yas bebek bekleniyordu: %+v", tr[3])
	}
	if tr[4].Type != travelerAdult {
		t.Errorf("16 yas yetiskin bekleniyordu: %+v", tr[4])
	}
	// Yas verilmeyen cocuk varsayilan yas alir; sifir yetiskin 1'e cekilir.
	tr = buildTravelers(0, 1, nil)
	if len(tr) != 2 || tr[1].Type != travelerChild || tr[1].Age != defaultChildAge {
		t.Errorf("1 yetiskin + varsayilan yasli cocuk bekleniyordu: %+v", tr)
	}
	raw, _ := json.Marshal(buildTravelers(1, 1, []int{7}))
	if string(raw) != `[{"Type":0,"Age":"35"},{"Type":1,"Age":7}]` {
		t.Errorf("JSON formati site ile ayni olmali: %s", raw)
	}
}

func TestParseDetailURL(t *testing.T) {
	p, err := parseDetailURL("https://www.prontotour.com/Acapulco-Resort-Hotel-h3234-8?CheckIn=2026-12-05&CheckOut=2026-12-08&Adult=2")
	if err != nil || p.Code != "3234" || p.ServiceType != "8" || p.Slug != "Acapulco-Resort-Hotel" {
		t.Fatalf("3234/8/Acapulco-Resort-Hotel bekleniyordu: %+v %v", p, err)
	}
	if nameFromSlug(p.Slug) != "Acapulco Resort Hotel" {
		t.Errorf("slug'dan ad: %q", nameFromSlug(p.Slug))
	}
	if p, err := parseDetailURL("https://www.prontotour.com/Merit-Park-Hotel-h35-8"); err != nil || p.Code != "35" {
		t.Errorf("sorgusuz URL cozulmeli: %+v %v", p, err)
	}
	if _, err := parseDetailURL("https://www.prontotour.com/Kibris-Otelleri-H26059"); err == nil {
		t.Errorf("liste URL'i hata vermeli")
	}
}

func TestMatchAirport(t *testing.T) {
	items := []autoCompleteItem{
		{Name: "İstanbul", Type: "H", IdDest: 28228},
		{Name: "Istanbul Airport", Type: "F", IdDest: 23709, AirPortCode: "IST"},
		{Name: "Sabiha Gökçen International Airport", Type: "F", IdDest: 25078, AirPortCode: "SAW"},
	}
	a := matchAirport(items, "saw")
	if a == nil || a.ID != 25078 || a.Code != "SAW" {
		t.Fatalf("SAW 25078 bekleniyordu: %+v", a)
	}
	if matchAirport(items, "ADB") != nil {
		t.Errorf("ADB eslesmemeli")
	}
}

func TestNormDate(t *testing.T) {
	if d, err := normDate("2026-12-05T00:00:00"); err != nil || d != "2026-12-05" {
		t.Errorf("2026-12-05 bekleniyordu: %q %v", d, err)
	}
	if _, err := normDate("05.12.2026"); err == nil {
		t.Errorf("TR formati hata vermeli")
	}
}
