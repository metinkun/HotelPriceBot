package tatilsepeti

// Gercek yanitlardan kirpilmis fixture'lar (Ekim 2026, Kaya Palazzo Resort / 7741, 05-08.12.2026, 2 yetiskin).

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNormalizeResponseFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/hotel_list_price.json")
	if err != nil {
		t.Fatal(err)
	}
	var data any
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	p := normalizeResponse(data, "7741")
	if !p.Available || p.Price == nil || *p.Price != 57120 {
		t.Fatalf("odenecek fiyat 57120 bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice != 67200 || p.DiscountRate != 15 {
		t.Errorf("liste 67200 / %%15 bekleniyordu: %v %v", p.ListPrice, p.DiscountRate)
	}
	if p.DiscountPrice == nil || *p.DiscountPrice != 57120 {
		t.Errorf("discountPrice 57120 bekleniyordu: %v", p.DiscountPrice)
	}
	if p.BoardType == nil || *p.BoardType != "Oda Kahvaltı" || p.Currency == nil || *p.Currency != "TL" {
		t.Errorf("pansiyon/para birimi: %v %v", p.BoardType, p.Currency)
	}
	if p.CampaignName == nil || *p.CampaignName == "" {
		t.Error("kampanya adi bekleniyordu")
	}
	// Price 0 / HasPrice 0 (orn. 4 yetiskin tek oda) -> musait degil
	zero := []any{map[string]any{"HotelId": float64(7741), "Price": float64(0), "DiscountPrice": float64(0)}}
	if normalizeResponse(zero, "7741").Available {
		t.Error("Price 0 musait sayilmamali")
	}
}

func TestParsePackageRoomListFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/flying_packages_roomlist.html")
	if err != nil {
		t.Fatal(err)
	}
	rooms := parsePackageRoomList(string(b), 3)
	if len(rooms) != 2 {
		t.Fatalf("2 oda bekleniyordu, %d", len(rooms))
	}
	r := rooms[0]
	if r.RoomID != "46" || r.RoomName != "Kara Manzaralı Deluxe Oda" {
		t.Fatalf("oda hatali: %+v", r)
	}
	if len(r.Boards) != 2 {
		t.Fatalf("2 pansiyon bekleniyordu: %+v", r.Boards)
	}
	if *r.Boards[0].BoardType != "Oda Kahvaltı" || *r.Boards[0].Price != 65533.58 {
		t.Errorf("OK fiyati 65533.58 bekleniyordu: %+v", r.Boards[0])
	}
	if *r.Boards[1].BoardType != "Tam Pansiyon Plus" || *r.Boards[1].Price != 78133.58 {
		t.Errorf("TP+ fiyati 78133.58 bekleniyordu: %+v", r.Boards[1])
	}
	if r.Boards[0].Cancellation == nil || *r.Boards[0].Cancellation != "FREE_CANCELLATION" {
		t.Error("ucretsiz iptal bekleniyordu")
	}
	if rooms[1].RoomID != "45" || *rooms[1].Boards[0].Price != 82633.58 {
		t.Errorf("ikinci oda hatali: %+v", rooms[1])
	}
	if *r.NightCount != 3 {
		t.Error("3 gece bekleniyordu")
	}
}
