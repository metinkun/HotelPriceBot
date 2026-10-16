package tatilsepeti

import (
	"testing"
)

const packageHTML = `
<div id="roomName_flight_25">
  <h3>Standart Oda</h3>
  <ul><li><span>28 m²</span></li></ul>
  <div class="x__price-div">
    <div class="x__pension-types"><span>Her Şey Dahil</span></div>
    <div class="x__multiple"><a onclick="hotelDetail.getInstallment(event, { RoomTypeId: '25', FlightPocketPrice: '92558', Nights: 5 }, '160000')">Sec</a></div>
    <div class="x__pension-types"><span>Yarım Pansiyon</span></div>
    <div class="x__multiple"><a onclick="hotelDetail.getInstallment(event, { RoomTypeId: '25', FlightPocketPrice: '80000' })">Sec</a></div>
  </div>
  <div class="free-cancel">Ücretsiz iptal</div>
</div>
<div id="roomName_flight_30">
  <h3>Suit</h3>
  <div class="x__price-div">
    <div class="x__pension-types"><span>Her Şey Dahil</span></div>
    <div class="x__multiple"><a onclick="hotelDetail.getInstallment(event, { RoomTypeId: '30', FlightPocketPrice: '120000' })">Sec</a></div>
  </div>
</div>`

func TestParsePackageRoomList(t *testing.T) {
	rooms := parsePackageRoomList(packageHTML, 5)
	if len(rooms) != 2 {
		t.Fatalf("2 oda bekleniyordu, %d", len(rooms))
	}
	std := rooms[0]
	if std.RoomID != "25" || std.RoomName != "Standart Oda" || std.RoomSize == nil || *std.RoomSize != 28 {
		t.Fatalf("standart oda hatali: %+v", std)
	}
	if len(std.Boards) != 2 || *std.Boards[0].Price != 92558 || *std.Boards[0].BoardType != "Her Şey Dahil" {
		t.Fatalf("board hatali: %+v", std.Boards)
	}
	if std.Boards[0].Cancellation == nil || *std.Boards[0].Cancellation != "FREE_CANCELLATION" {
		t.Errorf("ucretsiz iptal bekleniyordu")
	}
	if rooms[1].Boards[0].Cancellation != nil {
		t.Errorf("suit icin iptal bilgisi olmamali")
	}
	if *std.NightCount != 5 {
		t.Errorf("5 gece bekleniyordu")
	}
}

func TestParseGetInstallmentData(t *testing.T) {
	d := parseGetInstallmentData(`getInstallment(event, { RoomTypeId: '25', FlightPocketPrice: "92558", Nights: 5, Flag: true }, '1')`)
	if d["RoomTypeId"] != "25" || d["FlightPocketPrice"] != "92558" || d["Nights"] != "5" || d["Flag"] != "true" {
		t.Fatalf("kv parse hatali: %v", d)
	}
}

func TestNormalizeResponse(t *testing.T) {
	data := []any{
		map[string]any{"HotelId": float64(123), "HotelName": "Kaya", "Price": float64(89400), "DiscountRate": float64(20), "Currency": "TL"},
		map[string]any{"HotelId": float64(999), "Price": float64(1)},
	}
	p := normalizeResponse(data, "123")
	if !p.Available || *p.Price != 89400 || *p.HotelName != "Kaya" {
		t.Fatalf("normalize hatali: %+v", p)
	}
	// listPrice yok, discountRate var -> hesaplanir: 89400 / 0.8 = 111750
	if p.ListPrice == nil || *p.ListPrice != 111750 {
		t.Errorf("hesaplanan liste fiyati 111750 bekleniyordu: %v", p.ListPrice)
	}
	if normalizeResponse(data, "555").Available {
		t.Error("eslesmeyen otel musait olmamali")
	}
	if normalizeResponse(nil, "1").Available {
		t.Error("nil veri musait olmamali")
	}
}

func TestBuildPackageSearchString(t *testing.T) {
	got := buildPackageSearchString(2, []int{5, 8}, "2026-07-26", "2026-07-31")
	if got != "oda:2,5,8;tarih:26.07.2026,31.07.2026;click:true" {
		t.Errorf("beklenmeyen: %s", got)
	}
}
