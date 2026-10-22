package mngturizm

import (
	"os"
	"testing"

	"pricebot/internal/types"
)

func load(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNormalizePrice(t *testing.T) {
	p := normalizePrice(load(t, "room_price.html"), types.Str("Acapulco"), types.Str("Kıbrıs Girne"))
	if !p.Available || p.Price == nil || *p.Price != 33150 {
		t.Fatalf("33150 bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 51000 || p.DiscountRate != 35 {
		t.Errorf("liste 51000 / %%35 bekleniyordu: list=%v rate=%v", p.ListPrice, p.DiscountRate)
	}
	if types.Deref(p.RoomName) != "Bungalow Oda" || types.Deref(p.BoardType) != "Yarım Pansiyon" {
		t.Errorf("oda/pansiyon yanlis: %v %v", p.RoomName, p.BoardType)
	}
	if types.Deref(p.Currency) != "TL" {
		t.Errorf("TRY -> TL bekleniyordu: %v", p.Currency)
	}
	if types.Deref(p.CampaignName) != "%35 Erken Rezervasyon İndirimi ve Vade Farksız 3 Taksit İmkanı" {
		t.Errorf("kampanya adi yanlis: %q", types.Deref(p.CampaignName))
	}
	if types.Deref(p.HotelName) != "Acapulco" || types.Deref(p.City) != "Kıbrıs Girne" {
		t.Errorf("otel adi/sehir aktarilmadi: %v %v", p.HotelName, p.City)
	}
	if offers := parseOffers(load(t, "room_price.html")); len(offers) != 3 || offers[2].Price != 39975 || offers[2].BoardType != "Tam Pansiyon Plus" {
		t.Errorf("3 teklif (sonuncusu TP+ 39975) bekleniyordu: %+v", offers)
	}
}

func TestNormalizePriceChild(t *testing.T) {
	p := normalizePrice(load(t, "room_price_child.html"), nil, nil)
	if !p.Available || p.Price == nil || *p.Price != 45581.25 || p.ListPrice == nil || *p.ListPrice != 70125 {
		t.Fatalf("45581.25 / 70125 bekleniyordu: %+v", p)
	}
}

func TestUnavailable(t *testing.T) {
	html := load(t, "unavailable.html")
	if !isUnavailable(html) {
		t.Error("musait degil mesaji taninmadi")
	}
	if p := normalizePrice(html, nil, nil); p.Available || p.Price != nil {
		t.Errorf("Available=false bekleniyordu: %+v", p)
	}
	if len(parseOffers(html)) != 0 {
		t.Error("teklif beklenmiyordu")
	}
}

func TestNormalizePackages(t *testing.T) {
	html := load(t, "package_price.html")
	rooms := normalizePackages(html, types.Int(3))
	if len(rooms) != 2 {
		t.Fatalf("2 oda bekleniyordu: %+v", rooms)
	}
	b := rooms[0].Boards[0]
	if rooms[0].RoomName != "Bungalow Oda" || b.Price == nil || *b.Price != 46683.58 || types.Deref(b.BoardType) != "Yarım Pansiyon" {
		t.Errorf("Bungalow YP 46683.58 bekleniyordu: %+v %+v", rooms[0], b)
	}
	if b.ListPrice != nil || b.DiscountRate != 0 {
		t.Errorf("paket fiyatinda liste fiyati beklenmiyordu: %+v", b)
	}
	if rooms[1].RoomName != "Standart Otel Odası" || *rooms[1].Boards[0].Price != 47658.58 {
		t.Errorf("Standart 47658.58 bekleniyordu: %+v", rooms[1])
	}
	if !parseOffers(html)[0].Package {
		t.Error("paket isareti (ucakfiyatadahil) taninmadi")
	}
}

func TestParseFlightSearchAndSelect(t *testing.T) {
	fs := parseFlightSearch(load(t, "flight_search.html"))
	if fs.MinToplam == "" || fs.Pax == "" {
		t.Fatalf("mintoplam/pax cozulemedi: %+v", fs)
	}
	if len(fs.Outbound) != 2 || len(fs.Return) != 2 {
		t.Fatalf("2 gidis / 2 donus bekleniyordu: %d/%d", len(fs.Outbound), len(fs.Return))
	}
	if fs.Outbound[1].Company != "VF" || fs.Outbound[1].Provider != "AJET" || fs.Outbound[1].Surcharge != 67.7 || fs.Outbound[1].Depart != "09:45" {
		t.Errorf("AJET ucusu yanlis cozuldu: %+v", fs.Outbound[1])
	}
	sel, err := selectFlights(fs)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Outbound.Company != "PC" || sel.Outbound.Surcharge != 0 || sel.Outbound.Depart != "05:45" || sel.Outbound.Fiyat == "" || sel.Outbound.Book == "" {
		t.Errorf("en ucuz gidis PC 05:45 bekleniyordu: %+v", sel.Outbound)
	}
	if sel.Return.Direction != "return-flight" || sel.Return.Book == "" {
		t.Errorf("donus secilemedi: %+v", sel.Return)
	}
	if _, err := selectFlights(&FlightSearch{}); err == nil {
		t.Error("ucus yokken hata bekleniyordu")
	}
}

func TestParseDetailPage(t *testing.T) {
	r := parseDetailPage(load(t, "detail_snippet.html"))
	if r.ProviderHotelID != "13791" || r.ToID != "air-ECN" {
		t.Fatalf("id/toid yanlis: %+v", r)
	}
	if r.HotelName != "Acapulco Resort Hotel & Convention & SPA" || r.City != "Kıbrıs Girne" {
		t.Errorf("ad/sehir yanlis: %q %q", r.HotelName, r.City)
	}
	if !r.HasPriceForm || !r.HasFlightTab {
		t.Errorf("fiyat formu / ucak sekmesi taninmadi: %+v", r)
	}
	if r2 := parseDetailPage(`<title>X Hotel Fiyatları - MNG Turizm</title><div data-id="99">`); r2.ProviderHotelID != "99" || r2.HotelName != "X Hotel" || r2.HasPriceForm {
		t.Errorf("fallback cozumleme yanlis: %+v", r2)
	}
}

func TestChildParamsAndDate(t *testing.T) {
	ages, n := childParams([]int{5, 0, 20})
	if ages != [4]string{"5", "0", "17", "-1"} || n != "3" {
		t.Errorf("5,0,17,-1 / 3 bekleniyordu: %v %s", ages, n)
	}
	if ages, n := childParams(nil); ages != [4]string{"-1", "-1", "-1", "-1"} || n != "0" {
		t.Errorf("bos cocuk: %v %s", ages, n)
	}
	if d, _ := toMNGDate("2026-12-05"); d != "05.12.2026" {
		t.Errorf("05.12.2026 bekleniyordu, %s", d)
	}
	if d, _ := toMNGDate("05.12.2026"); d != "05.12.2026" {
		t.Errorf("05.12.2026 bekleniyordu, %s", d)
	}
	if _, err := toMNGDate("12/05/2026"); err == nil {
		t.Error("hatali tarih icin hata bekleniyordu")
	}
}
