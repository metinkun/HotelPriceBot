package enuygun

import (
	"encoding/json"
	"os"
	"testing"
)

func load(t *testing.T, name string) *SearchResponse {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var r SearchResponse
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func TestNormalizePrice(t *testing.T) {
	p := normalizePrice(load(t, "detail_search.json"), 580512)
	if !p.Available || p.Price == nil || *p.Price != 33150 {
		t.Fatalf("33150 bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 51000 || p.DiscountRate != 35 {
		t.Errorf("liste 51000 / %%35 bekleniyordu: list=%v rate=%v", p.ListPrice, p.DiscountRate)
	}
	if p.HotelName == nil || *p.HotelName != "Acapulco Resort Convention & Spa" {
		t.Errorf("otel adi yanlis: %v", p.HotelName)
	}
	if p.City == nil || *p.City != "Girne" {
		t.Errorf("sehir Girne bekleniyordu: %v", p.City)
	}
	if p.RoomName == nil || *p.RoomName != "Acapulco Villa" || p.BoardType == nil || *p.BoardType != "Yarım Pansiyon" {
		t.Errorf("oda/pansiyon yanlis: %v %v", p.RoomName, p.BoardType)
	}
	if p.Currency == nil || *p.Currency != "TL" {
		t.Errorf("TRY -> TL bekleniyordu: %v", p.Currency)
	}
	if p.CampaignName != nil {
		t.Errorf("provider indirimi kampanya sayilmamali: %v", *p.CampaignName)
	}
}

func TestNormalizePriceChild(t *testing.T) {
	sr := load(t, "detail_search_child.json")
	p := normalizePrice(sr, 580512)
	if !p.Available || p.Price == nil || *p.Price != 45581.25 {
		t.Fatalf("45581.25 bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 70125 {
		t.Errorf("liste 70125 bekleniyordu: %v", p.ListPrice)
	}
	if o := sr.Hotels[0].Rooms[0].Offers[0]; o.ChildrenCount != 1 || o.AdultCount != 2 {
		t.Errorf("cocuk sayisi yanitta 1 olmali: %+v", o)
	}
}

func TestNormalizeEmpty(t *testing.T) {
	p := normalizePrice(load(t, "empty.json"), 580512)
	if p.Available || p.Price != nil {
		t.Fatalf("musait degil bekleniyordu: %+v", p)
	}
}

func TestCheapestSkipsUnsaleable(t *testing.T) {
	sr := load(t, "detail_search.json")
	h := &sr.Hotels[0]
	// En ucuz teklifi satilamaz yap: bir sonraki en ucuz secilmeli.
	for ri := range h.Rooms {
		for oi := range h.Rooms[ri].Offers {
			if h.Rooms[ri].Offers[oi].TryPrice.Total.Amount == "33150" {
				h.Rooms[ri].Offers[oi].Saleable = false
			}
		}
	}
	p := normalizePrice(sr, 580512)
	if p.Price == nil || *p.Price != 34125 {
		t.Fatalf("34125 bekleniyordu: %+v", p)
	}
}

func TestParseAPIDetail(t *testing.T) {
	b, err := os.ReadFile("testdata/api_detail.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := parseAPIDetail(b)
	if err != nil {
		t.Fatal(err)
	}
	if r.ProviderHotelID != "580531" || r.HotelName != "Club Simena" || r.Slug != "club-simena" || r.Region != "Girne" {
		t.Fatalf("cozumleme yanlis: %+v", r)
	}
	if r.IsDomestic || !r.IsCyprus {
		t.Errorf("Kibris oteli bekleniyordu: %+v", r)
	}
}

func TestSlugFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.enuygun.com/otel/detay/acapulco-resort-convention-spa-580512/":         "acapulco-resort-convention-spa-580512",
		"https://www.enuygun.com/otel/detay/club-simena/?checkin=2026-12-05&rooms=2":        "club-simena",
		"https://www.enuygun.com/otel/concorde-luxury-resort-578301/":                       "concorde-luxury-resort-578301",
		"https://www.enuygun.com/otel/bolge/kibris-45/":                                     "",
		"https://www.enuygun.com/otel/":                                                     "",
		"https://www.enuygun.com/en/hotels/detail/acapulco-resort-convention-spa-580512/":   "acapulco-resort-convention-spa-580512",
		"https://www.enuygun.com/otel/detay/Acapulco-Resort-Convention-Spa-580512#yorumlar": "acapulco-resort-convention-spa-580512",
	}
	for u, want := range cases {
		if got := slugFromURL(u); got != want {
			t.Errorf("%s: %q bekleniyordu, %q", u, want, got)
		}
	}
	if idFromSlug("acapulco-resort-convention-spa-580512") != 580512 || idFromSlug("club-simena") != 0 {
		t.Errorf("idFromSlug yanlis")
	}
}

func TestToEnuygunDate(t *testing.T) {
	if d, _ := toEnuygunDate("2026-12-05"); d != "05.12.2026" {
		t.Errorf("05.12.2026 bekleniyordu, %s", d)
	}
	if _, err := toEnuygunDate("05.12.2026"); err == nil {
		t.Errorf("TR formati hata vermeli")
	}
}

func TestChildAgesParam(t *testing.T) {
	if got := childAgesParam(nil); got == nil || len(got) != 0 {
		t.Errorf("bos dizi bekleniyordu (nil degil)")
	}
	if got := childAgesParam([]int{5, -1}); len(got) != 2 || got[1] != 0 {
		t.Errorf("[5 0] bekleniyordu, %v", got)
	}
}
