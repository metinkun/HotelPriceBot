package touristica

import (
	"testing"

	"pricebot/internal/types"
)

func TestParseRoomBlock(t *testing.T) {
	pb := parseRoomBlock("3 Gecelik Toplam Fiyat 87.000 TL 51.330 TL Rezervasyon Yap")
	if pb.Price == nil || *pb.Price != 51330 || pb.ListPrice == nil || *pb.ListPrice != 87000 || pb.Nights == nil || *pb.Nights != 3 {
		t.Fatalf("blok parse hatali: %+v", pb)
	}
	single := parseRoomBlock("2 Gecelik Toplam Fiyat 12.500 TL")
	if single.Price == nil || *single.Price != 12500 || single.ListPrice != nil {
		t.Fatalf("tek fiyat hatali: %+v", single)
	}
}

func TestNormalize(t *testing.T) {
	raw := &RawPrice{RoomBlocks: []string{
		"3 Gecelik Toplam Fiyat 87.000 TL 51.330 TL",
		"3 Gecelik Toplam Fiyat 95.000 TL 60.000 TL",
	}, HotelName: types.Str("Acapulco"), City: types.Str("Girne")}
	p := normalize(raw)
	if !p.Available || *p.Price != 51330 || *p.ListPrice != 87000 || p.DiscountRate != 41 {
		t.Fatalf("en ucuz blok secilmeli (%%41): %+v", p)
	}
	empty := normalize(&RawPrice{HotelName: types.Str("X")})
	if empty.Available || empty.HotelName == nil {
		t.Fatalf("bos blok musait olmamali: %+v", empty)
	}
}

func TestToTouristicaDate(t *testing.T) {
	if toTouristicaDate("2026-09-01") != "01.09.2026" {
		t.Error("dd.MM.yyyy bekleniyordu")
	}
}
