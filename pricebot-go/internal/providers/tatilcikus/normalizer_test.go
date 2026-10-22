package tatilcikus

import (
	"encoding/json"
	"os"
	"testing"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func loadDetail(t *testing.T) *HotelDetail {
	t.Helper()
	d, err := parseDetail(read(t, "detail_trimmed.json"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestParseDetail(t *testing.T) {
	d := loadDetail(t)
	h := d.Hotel
	if h.ID != 5 || h.URL != "acapulco-resort-hotel-spa" || h.PriceOperator != opAkgunler || h.supplierID() != "ACAPULCO" {
		t.Fatalf("otel detayi beklenmedik: %+v", h)
	}
	if len(d.Rooms) != 14 || len(d.Concepts) != 2 {
		t.Errorf("14 oda / 2 pansiyon bekleniyordu: %d / %d", len(d.Rooms), len(d.Concepts))
	}
	if it := findItem(d.Rooms, opAkgunler, "38"); it == nil || it.Name != "Villa Oda" {
		t.Errorf("Akgunler oda 38 -> Villa Oda bekleniyordu: %+v", it)
	}
	if it := findItem(d.Concepts, opGezinomi, "24"); it == nil || it.Name != "Yarım Pansiyon" {
		t.Errorf("Gezinomi concept 24 -> Yarim Pansiyon bekleniyordu: %+v", it)
	}
}

func TestNormalizeAkgunler(t *testing.T) {
	d := loadDetail(t)
	var resp AkgunlerResponse
	if err := json.Unmarshal(read(t, "akgunler.json"), &resp); err != nil {
		t.Fatal(err)
	}
	offers := normalizeAkgunler(d, &resp, nil)
	if len(offers) != 12 {
		t.Fatalf("12 teklif bekleniyordu, %d", len(offers))
	}
	p := normalizePrice(offers, d.Hotel.Name, "Girne")
	if !p.Available || p.Price == nil || *p.Price != 28050 {
		t.Fatalf("28050 bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 51000 || p.DiscountRate != 45 {
		t.Errorf("liste 51000 / %%45 bekleniyordu: list=%v rate=%v", p.ListPrice, p.DiscountRate)
	}
	if p.RoomName == nil || *p.RoomName != "Villa Oda" || p.BoardType == nil || *p.BoardType != "Yarım Pansiyon" {
		t.Errorf("Villa Oda / Yarim Pansiyon bekleniyordu: %v %v", p.RoomName, p.BoardType)
	}
	if p.Currency == nil || *p.Currency != "TL" || p.City == nil || *p.City != "Girne" {
		t.Errorf("TL / Girne bekleniyordu: %v %v", p.Currency, p.City)
	}
	// Release=true olan Deniz Manzarali odalar STOP_SALE olmali
	stops := 0
	for _, o := range offers {
		if o.RoomName == "Deniz Manzaralı Oda" && o.Status == statusStopSale {
			stops++
		}
	}
	if stops != 2 {
		t.Errorf("2 stop-sale (Deniz Manzarali) bekleniyordu, %d", stops)
	}
}

func TestNormalizeAkgunlerChild(t *testing.T) {
	d := loadDetail(t)
	var resp AkgunlerResponse
	if err := json.Unmarshal(read(t, "akgunler_child.json"), &resp); err != nil {
		t.Fatal(err)
	}
	p := normalizePrice(normalizeAkgunler(d, &resp, nil), d.Hotel.Name, "")
	if !p.Available || p.Price == nil || *p.Price <= 0 {
		t.Fatalf("cocuklu sorguda fiyat bekleniyordu: %+v", p)
	}
}

func TestNetCommissionPrice(t *testing.T) {
	// Sitedeki Mt formulu: GrossPrice - SUM parseInt(GrossAmount*(w+EB)/100)
	h := &Hotel{IsActiveCustomerNetPriceCommission: true, CustomerNetPriceCommission: 10}
	r := &AkgunlerRoom{GrossPrice: 51000, CommissionPercentEB: 45, DailyPrices: []AkgunlerDaily{
		{GrossAmount: 17000, Date: "2026-12-05T00:00:00"}, {GrossAmount: 17000, Date: "2026-12-06T00:00:00"}, {GrossAmount: 17000, Date: "2026-12-07T00:00:00"},
	}}
	price, rate := netCommissionPrice(h, r, nil)
	if price != 51000-3*9350 || rate != 55 {
		t.Errorf("22950 / 55 bekleniyordu: %v / %v", price, rate)
	}
	// tarih araligi override: 2 gun icin -5
	ov := []CustomerCommission{{CommissionRate: -5, DateOfStart: "2026-12-06T00:00:00", DateOfEnd: "2026-12-07T00:00:00"}}
	price, rate = netCommissionPrice(h, r, ov)
	want := 51000.0 - (9350 + 6800 + 6800)
	if price != want || rate != 40 {
		t.Errorf("%v / 40 bekleniyordu: %v / %v", want, price, rate)
	}
	// EB 0 -> Mt null -> GrossPrice
	r.CommissionPercentEB = 0
	if price, _ = netCommissionPrice(h, r, nil); price != 51000 {
		t.Errorf("EB=0'da GrossPrice bekleniyordu: %v", price)
	}
}

func TestNormalizeCoral(t *testing.T) {
	d := &HotelDetail{
		Hotel: Hotel{ID: 61, Name: "Abant Aden Boutique Hotel & Spa", PriceOperator: opCoral},
		Rooms: []Item{
			{ID: 1, Name: "Aden Suit", IsShown: true, OpenForSale: true, CoralIDs: []string{"74389"}},
			{ID: 2, Name: "Dublex Suit", IsShown: true, OpenForSale: true, CoralIDs: []string{"74573"}},
		},
		Concepts: []Item{{ID: 9, Name: "Oda Kahvaltı", IsShown: true, OpenForSale: true, CoralIDs: []string{"18"}}},
	}
	var resp CoralResponse
	if err := json.Unmarshal(read(t, "coral.json"), &resp); err != nil {
		t.Fatal(err)
	}
	offers := normalizeCoral(d, &resp)
	if len(offers) != 2 {
		t.Fatalf("2 teklif bekleniyordu, %d", len(offers))
	}
	p := normalizePrice(offers, d.Hotel.Name, "Bolu")
	// saleStatus=2 -> ON_REQUEST: fiyat verilir ama Available=false
	if p.Available || p.Price == nil || *p.Price != 23797.5 || p.ListPrice == nil || *p.ListPrice != 25050 || p.DiscountRate != 5 {
		t.Fatalf("on-request 23797.5 / 25050 / %%5 bekleniyordu: %+v", p)
	}
	if p.RoomName == nil || *p.RoomName != "Aden Suit" || p.BoardType == nil || *p.BoardType != "Oda Kahvaltı" || *p.Currency != "TL" {
		t.Errorf("Aden Suit / Oda Kahvalti / TL bekleniyordu: %v %v %v", p.RoomName, p.BoardType, p.Currency)
	}
	// gosterimde olmayan oda ele alinmamali
	d.Rooms[0].IsShown = false
	if got := normalizeCoral(d, &resp); len(got) != 1 {
		t.Errorf("gizli oda filtrelenmeli, %d", len(got))
	}
}

func TestNormalizeGezinomi(t *testing.T) {
	d := loadDetail(t)
	var resp GezinomiResponse
	if err := json.Unmarshal(read(t, "gezinomi.json"), &resp); err != nil {
		t.Fatal(err)
	}
	offers := normalizeGezinomi(d, &resp)
	if len(offers) != 12 {
		t.Fatalf("12 teklif bekleniyordu, %d", len(offers))
	}
	p := normalizePrice(offers, d.Hotel.Name, "")
	if !p.Available || *p.Price != 31620 || *p.ListPrice != 51000 || p.DiscountRate != 38 {
		t.Fatalf("31620 / 51000 / %%38 bekleniyordu: %+v", p)
	}
	if *p.RoomName != "Villa Oda" || *p.BoardType != "Yarım Pansiyon" {
		t.Errorf("Villa Oda / Yarim Pansiyon bekleniyordu: %v %v", *p.RoomName, *p.BoardType)
	}
	if gezinomiStatus([]string{"Kota"}) != statusOnRequest || gezinomiStatus([]string{"Stop", "Kota"}) != statusStopSale || gezinomiStatus(nil) != statusAvailable {
		t.Error("gezinomiStatus beklenen durumlari uretmedi")
	}
}

func TestLeadOfferEmpty(t *testing.T) {
	p := normalizePrice(nil, "X", "")
	if p.Available || p.Price != nil {
		t.Errorf("bos listede unavailable bekleniyordu: %+v", p)
	}
	p = normalizePrice([]offer{{RoomName: "A", Price: 100, Status: statusStopSale}}, "X", "")
	if p.Available || p.Price != nil {
		t.Errorf("sadece stop-sale varsa unavailable bekleniyordu: %+v", p)
	}
}

func TestDatesAndChildren(t *testing.T) {
	if d, _ := toSiteDate("2026-12-05"); d != "05.12.2026" {
		t.Errorf("05.12.2026 bekleniyordu, %s", d)
	}
	if _, err := toSiteDate("05.12.2026"); err == nil {
		t.Error("TR formati reddedilmeli")
	}
	b, err := childBirthDates("2026-12-05", []int{6, 0})
	if err != nil || len(b) != 2 || b[0] != "05.06.2020" || b[1] != "05.06.2026" {
		t.Errorf("05.06.2020 / 05.06.2026 bekleniyordu: %v %v", b, err)
	}
	req, err := buildRequest("ACAPULCO", "2026-12-05", "2026-12-08", 2, []int{6})
	if err != nil || req.StartDate != "05.12.2026" || req.EndDate != "08.12.2026" || req.AdultCount != 2 || len(req.ChildrenBirths) != 1 {
		t.Errorf("istek govdesi beklenmedik: %+v %v", req, err)
	}
}

func TestSlugFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.tatilcikus.com/acapulco-resort-hotel-spa":                              "acapulco-resort-hotel-spa",
		"https://tatilcikus.com/acapulco-resort-hotel-spa/?start=05.12.2026&end=08.12.2026": "acapulco-resort-hotel-spa",
		"https://www.tatilcikus.com/karinna-hotel-uludag#oda":                               "karinna-hotel-uludag",
	}
	for u, want := range cases {
		if got, err := slugFromURL(u); err != nil || got != want {
			t.Errorf("%s -> %q (%v), %q bekleniyordu", u, got, err, want)
		}
	}
	for _, u := range []string{"https://www.jollytur.com/x", "https://www.tatilcikus.com/", "https://www.tatilcikus.com/k/kampanya", "https://www.tatilcikus.com/favicon.ico"} {
		if _, err := slugFromURL(u); err == nil {
			t.Errorf("%s reddedilmeliydi", u)
		}
	}
}

func TestPromoteOlderChildren(t *testing.T) {
	h := &Hotel{FirstChildAge: 11}
	a, c, ch := promoteOlderChildren(h, 2, []int{6, 12})
	if !ch || a != 3 || len(c) != 1 || c[0] != 6 {
		t.Errorf("3 yetiskin + [6] bekleniyordu: %d %v %v", a, c, ch)
	}
	if _, _, ch := promoteOlderChildren(h, 2, []int{6, 11}); ch {
		t.Error("sinir yasi (11) cocuk kalmali")
	}
	if _, _, ch := promoteOlderChildren(&Hotel{}, 2, []int{15}); ch {
		t.Error("firstChildAge 0 iken degisiklik olmamali")
	}
}
