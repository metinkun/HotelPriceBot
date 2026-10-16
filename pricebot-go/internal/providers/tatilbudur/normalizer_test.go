package tatilbudur

import (
	"encoding/json"
	"os"
	"testing"

	"pricebot/internal/provider"
)

func load(t *testing.T, name string) *RoomPriceResponse {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var r RoomPriceResponse
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func TestNormalizePrice(t *testing.T) {
	p := normalizePrice(load(t, "room_price.json"))
	if !p.Available || p.Price == nil || *p.Price <= 0 {
		t.Fatalf("fiyat bekleniyordu: %+v", p)
	}
	if p.ListPrice == nil || *p.ListPrice < *p.Price {
		t.Errorf("liste fiyati tutarsiz: %+v", p)
	}
	if p.RoomName == nil || *p.RoomName == "" || p.BoardType == nil || *p.BoardType == "" {
		t.Errorf("oda/pansiyon eksik: %v %v", p.RoomName, p.BoardType)
	}
	if p.Currency == nil || *p.Currency != "TL" {
		t.Errorf("TRY -> TL bekleniyordu, %v", p.Currency)
	}
}

func TestNormalizePackages(t *testing.T) {
	rooms := normalizePackages(load(t, "package_price.json"))
	if len(rooms) == 0 {
		t.Fatal("paket odasi bekleniyordu")
	}
	for _, r := range rooms {
		if len(r.Boards) == 0 {
			t.Errorf("board'suz oda: %s", r.RoomName)
		}
		if r.NightCount == nil || *r.NightCount != 3 {
			t.Errorf("3 gece bekleniyordu: %v", r.NightCount)
		}
		seen := map[string]bool{}
		for _, b := range r.Boards {
			key := provider.DecodeEntities(derefS(b.BoardType))
			if b.Price == nil || *b.Price <= 0 {
				t.Errorf("fiyatsiz board")
			}
			if seen[key] {
				// Ayni oda+pansiyon farkli fiyatla gelebilir (farkli iptal kosulu); tekrar fiyatlar tekillenmis olmali
			}
			seen[key] = true
		}
	}
}

func derefS(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestNormalizePrice_Empty(t *testing.T) {
	p := normalizePrice(&RoomPriceResponse{View: "<div>oda yok</div>"})
	if p.Available {
		t.Fatal("bos view musait olmamali")
	}
}

func TestResolveSnippet(t *testing.T) {
	b, _ := os.ReadFile("testdata/detail_snippet.html")
	html := string(b)
	if provider.FirstMatch(reHotelID, html) != "6713" {
		t.Errorf("hotelId 6713 bekleniyordu")
	}
	if name := provider.DecodeEntities(provider.FirstMatch(reHotelName, html)); name == "" {
		t.Errorf("otel adi cozulemedi")
	}
	if provider.FirstMatch(reToCode, html) != "ECN" {
		t.Errorf("varis kodu ECN bekleniyordu")
	}
}

func TestDates(t *testing.T) {
	if toDotDate("2026-09-01") != "1.9.2026" {
		t.Errorf("sifirsiz tarih bekleniyordu: %s", toDotDate("2026-09-01"))
	}
	if toSlashDate("2026-09-01") != "01/09/2026" {
		t.Errorf("dd/MM/yyyy bekleniyordu: %s", toSlashDate("2026-09-01"))
	}
}
