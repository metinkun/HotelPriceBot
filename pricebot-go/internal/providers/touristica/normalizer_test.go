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

func TestRoomOffers(t *testing.T) {
	raw := &RawPrice{
		RoomBlocks: []string{"3 Gecelik Toplam Fiyat 87.000 TL 51.330 TL Rezervasyon Yap", "3 Gecelik Toplam Fiyat 95.000 TL 60.000 TL Rezervasyon Yap"},
		RoomCards: []RoomCard{
			{RoomName: "Bungalow Oda", BoardType: "Tam Pansiyon Plus", Text: "Bungalow Oda Tam Pansiyon Plus Oda Kapasitesi 3 Gecelik Toplam Fiyat 95.000 TL 60.000 TL Rezervasyon Yap"},
			{RoomName: "Standart Oda", BoardType: "Yarım Pansiyon", Text: "Standart Oda Yarım Pansiyon 3 Gecelik Toplam Fiyat 87.000 TL 51.330 TL Rezervasyon Yap"},
			{RoomName: "Suit Oda", BoardType: "Yarım Pansiyon", Text: "Suit Oda Yarım Pansiyon 3 Gecelik Toplam Fiyat 120.000 TL 70.000 TL Seçtiğiniz tarihlerde müsait değil"},
		},
	}
	p := normalize(raw)
	if len(p.Rooms) != 3 || *p.Price != 51330 {
		t.Fatalf("beklenmeyen: %+v", p)
	}
	if p.Rooms[0].RoomName != "Standart Oda" || p.Rooms[0].BoardType != "Yarım Pansiyon" || *p.Rooms[0].ListPrice != 87000 || !p.Rooms[0].Available {
		t.Errorf("ilk teklif: %+v", p.Rooms[0])
	}
	if p.Rooms[2].Status != "sold_out" || *p.Rooms[2].Price != 70000 {
		t.Errorf("satis disi: %+v", p.Rooms[2])
	}
	if p.RoomName == nil || *p.RoomName != "Standart Oda" || p.BoardType == nil || *p.BoardType != "Yarım Pansiyon" {
		t.Errorf("lead oda/pansiyon: %v %v", p.RoomName, p.BoardType)
	}
}
