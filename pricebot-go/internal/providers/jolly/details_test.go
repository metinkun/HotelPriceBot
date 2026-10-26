package jolly

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseDetails(t *testing.T) {
	b, err := os.ReadFile("testdata/detail_page.html")
	if err != nil {
		t.Fatal(err)
	}
	out, err := parseDetails(string(b), "594195", "https://www.jollytur.com/acapulco-resort-convention-spa-hotel")
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Acapulco Resort Convention Spa Hotel" {
		t.Errorf("name: %q", out.Name)
	}
	if out.Country != "Kıbrıs" || out.City != "Girne" || out.Region != "Çatalköy" {
		t.Errorf("breadcrumb konum: %q %q %q", out.Country, out.City, out.Region)
	}
	if out.Address != "Çatalköy Kyrenia Mersin 10 - Turkey" || out.Location == nil || out.Location.Lat < 35 {
		t.Errorf("adres/geo: %q %+v", out.Address, out.Location)
	}
	if len(out.Images) != 3 || out.Images[0].Caption != "Genel" || !strings.HasPrefix(out.Images[0].URL, "https://cdng.jollytur.com/files/cms/media/hotel/") {
		t.Errorf("images: %+v", out.Images)
	}
	if !strings.HasPrefix(out.Description, "Girne’nin en temiz sahillerinden") || strings.Contains(out.Description, "\nen") {
		t.Errorf("description: %q", out.Description[:60])
	}
	if len(out.Facilities) == 0 || out.Facilities[0].Title != "Otel Özellikleri" || len(out.Facilities[0].Items) != 31 {
		t.Errorf("facilities: %+v", out.Facilities)
	}
	if !contains(out.Facilities[0].Items, "Çocuk Bakıcısı (Ücretli)") || !contains(out.Facilities[0].Items, "Otopark") {
		t.Errorf("facility etiketleri: %v", out.Facilities[0].Items)
	}
	if strings.Join(out.Boards, ",") != "Tam Pansiyon Plus,Yarım Pansiyon" {
		t.Errorf("boards: %v", out.Boards)
	}
	if out.Policies == nil || out.Policies.CheckIn != "15:00" || out.Policies.CheckOut != "11:00" || out.Policies.PetPolicy != "Evcil Hayvan Kabul Edilmez" || len(out.Policies.Other) == 0 {
		t.Errorf("policies: %+v", out.Policies)
	}
	titles := map[string]string{}
	for _, s := range out.Sections {
		titles[s.Title] = s.Kind
	}
	// Fixture'da tek sezon sekmesi var -> basliklarda sezon on eki yok
	for want, kind := range map[string]string{"Sezonlar": "pairs", "Otel Özellikleri": "list", "Otel Konsepti": "pairs", "Önemli Bilgiler": "text", "Giriş / Çıkış": "pairs", "Konum Bilgisi": "text", "Genel Bilgiler · Toplantı Salonları": "text", "Etkinlikler": "list"} {
		if titles[want] != kind {
			t.Errorf("section %s: kind=%q (var: %v)", want, titles[want], titles)
		}
	}
	for _, s := range out.Sections {
		if s.Title == "Otel Konsepti" && (len(s.Pairs) != 7 || s.Pairs[0].Key != "Tam Pansiyon Plus – Ana Restoran – Sabah Kahvaltısı" || s.Pairs[0].Value != "07:00-10:00") {
			t.Errorf("konsept pairs: %+v", s.Pairs)
		}
		if s.Title == "Konum Bilgisi" && !contains(s.Items, "Havalimanı: Ercan: 28 km") {
			t.Errorf("konum items: %v", s.Items)
		}
	}
	js, _ := json.Marshal(out)
	if strings.Contains(string(js), "&#x") || strings.Contains(string(js), "&amp;") {
		t.Error("cozulmemis entity var")
	}
}

func TestParseRooms(t *testing.T) {
	b, err := os.ReadFile("testdata/reservation.json")
	if err != nil {
		t.Fatal(err)
	}
	var rr ReservationResponse
	if err := json.Unmarshal(b, &rr); err != nil {
		t.Fatal(err)
	}
	rooms := parseRooms(rr.HTML)
	if len(rooms) != 3 {
		t.Fatalf("oda sayisi: %d", len(rooms))
	}
	if rooms[0].Name != "Acapulco Villa" || rooms[1].Name != "Deniz Manzaralı Otel Odası" || rooms[2].Name != "Kara Manzaralı Otel Odası" {
		t.Errorf("oda adlari/sira: %v %v %v", rooms[0].Name, rooms[1].Name, rooms[2].Name)
	}
	r := rooms[0]
	if r.Size == nil || *r.Size != 30 || !strings.HasPrefix(r.Description, "Oda genişliği 30-32 m²") {
		t.Errorf("villa: size=%v desc=%q", r.Size, r.Description)
	}
	if !contains(r.Features, "Telefon (Ücretli)") || !contains(r.Features, "Split Klima") || len(r.Images) == 0 {
		t.Errorf("villa ozellik/gorsel: %v %v", r.Features, r.Images)
	}
	if rooms[1].Size == nil || *rooms[1].Size != 27 {
		t.Errorf("deniz manzarali size: %v", rooms[1].Size)
	}
}

func TestHotelCandidates(t *testing.T) {
	b, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Fatal(err)
	}
	var hits []searchHit
	if err := json.Unmarshal(b, &hits); err != nil {
		t.Fatal(err)
	}
	c := hotelCandidates(hits, map[string]bool{})
	if len(c) != 3 {
		t.Fatalf("aday sayisi: %d", len(c))
	}
	if c[0].Name != "Merit Royal & Spa" || c[0].URL != "https://www.jollytur.com/merit-royal" || c[0].ProviderHotelID != "594810" || c[0].City != "Alsancak, Girne, Kıbrıs" {
		t.Errorf("aday: %+v", c[0])
	}
	// "Sonuc Bulunamadi" satiri (id=0) atlanir
	if got := hotelCandidates([]searchHit{{CategoryName: "Sonuç Bulunamadı"}}, map[string]bool{}); len(got) != 0 {
		t.Errorf("bos sonuc atlanmadi: %+v", got)
	}
}

func TestStaticStayWindow(t *testing.T) {
	ci, co := staticStayWindow(time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC))
	if ci != "2027-02-01" || co != "2027-02-04" {
		t.Errorf("%s %s", ci, co)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
