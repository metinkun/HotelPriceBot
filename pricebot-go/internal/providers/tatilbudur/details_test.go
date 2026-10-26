package tatilbudur

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"pricebot/internal/provider"
)

func TestParseDetails(t *testing.T) {
	b, err := os.ReadFile("testdata/detail_page.html")
	if err != nil {
		t.Fatal(err)
	}
	out, err := parseDetails(string(b), "6713", "https://www.tatilbudur.com/acapulco-resort-hotel-casino")
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Acapulco Resort Hotel & Casino" {
		t.Errorf("name: %q", out.Name)
	}
	if out.Stars == nil || *out.Stars != 5 {
		t.Errorf("stars: %v", out.Stars)
	}
	if out.Rating == nil || *out.Rating != 7.7 || out.RatingCount == nil || *out.RatingCount != 407 {
		t.Errorf("rating: %v %v", out.Rating, out.RatingCount)
	}
	if out.City != "Girne" || out.Region != "Çatalköy" || out.Country != "Kuzey Kıbrıs" || out.Address == "" {
		t.Errorf("konum: %q %q %q %q", out.City, out.Region, out.Country, out.Address)
	}
	if out.Location == nil || out.Location.Lat < 35 || out.Contact == nil || out.Contact.Phone == "" {
		t.Errorf("geo/contact: %+v %+v", out.Location, out.Contact)
	}
	if len(out.Images) != 3 || out.Images[0].Caption != "Genel" || !strings.HasPrefix(out.Images[0].URL, "https://productcdn.tatilbudur.com/") {
		t.Errorf("images: %+v", out.Images)
	}
	ft := map[string][]string{}
	for _, g := range out.Facilities {
		ft[g.Title] = g.Items
	}
	if len(ft["Öne Çıkan Özellikler"]) != 6 || !contains(ft["Genel Özellikler / Tesis Hizmetleri"], "Mini Bar (Ücretli)") || !contains(ft["Genel Özellikler"], "İnternet (Ücretsiz)") {
		t.Errorf("facilities: %+v", ft)
	}
	if out.Policies == nil || out.Policies.CheckIn != "14:00" || out.Policies.CheckOut != "12:00" {
		t.Errorf("policies: %+v", out.Policies)
	}
	titles := map[string]string{}
	for _, s := range out.Sections {
		titles[s.Title] = s.Kind
	}
	for want, kind := range map[string]string{"Misafir Değerlendirmesi": "pairs", "Konsept Özellikleri": "text", "Konum Bilgileri": "pairs", "Etkinlikler": "list", "KKTC Giriş Kuralları": "text"} {
		if titles[want] != kind {
			t.Errorf("section %s: kind=%q (var: %v)", want, titles[want], titles)
		}
	}
	for _, s := range out.Sections {
		if s.Title == "Konum Bilgileri" && (len(s.Pairs) < 3 || s.Pairs[0].Key != "Mevkii" || s.Pairs[0].Value != "Çatalköy") {
			t.Errorf("konum pairs: %+v", s.Pairs)
		}
	}
	js, _ := json.Marshal(out)
	if strings.Contains(string(js), "&amp;") || strings.Contains(string(js), "&nbsp;") {
		t.Error("cozulmemis entity var")
	}
}

func TestParseRoomsAndModal(t *testing.T) {
	b, err := os.ReadFile("testdata/room_price.json")
	if err != nil {
		t.Fatal(err)
	}
	view, err := parseView(b)
	if err != nil {
		t.Fatal(err)
	}
	rooms, boards, ids := parseRooms(view.View)
	if len(rooms) != 6 {
		t.Fatalf("oda sayisi: %d", len(rooms))
	}
	if ids["Acapulco Villa"] != "d76b9c9fdc7611e8b4e642010a050a0b" {
		t.Errorf("roomTypeId: %v", ids)
	}
	if strings.Join(boards, ",") != "Tam Pansiyon Plus,Yarım Pansiyon" {
		t.Errorf("boards: %v", boards)
	}
	var villa *provider.RoomInfo
	for i := range rooms {
		if rooms[i].Name == "Acapulco Villa" {
			villa = &rooms[i]
		}
	}
	if villa == nil || len(villa.Images) != 1 || len(villa.Features) == 0 {
		t.Fatalf("villa karti: %+v", villa)
	}
	mb, err := os.ReadFile("testdata/room_modal.html")
	if err != nil {
		t.Fatal(err)
	}
	applyRoomModal(villa, string(mb))
	if len(villa.Features) != 7 || !contains(villa.Features, "Mini Bar (Ücretli)") || !contains(villa.Features, "Duşakabin") {
		t.Errorf("modal ozellikleri: %v", villa.Features)
	}
	if len(villa.Images) != 3 || !strings.HasSuffix(villa.Images[0], "_6713_1113675.jpg") {
		t.Errorf("modal gorselleri: %v", villa.Images)
	}
	// JSON hata yaniti odayi bozmaz
	before := len(villa.Features)
	applyRoomModal(villa, `{"success":false,"error":"An error occurred"}`)
	if len(villa.Features) != before {
		t.Error("hata yaniti ozellikleri degistirdi")
	}
}

func TestHotelCandidates(t *testing.T) {
	b, err := os.ReadFile("testdata/hotel_search.json")
	if err != nil {
		t.Fatal(err)
	}
	var hits []searchHit
	if err := json.Unmarshal(b, &hits); err != nil {
		t.Fatal(err)
	}
	c := hotelCandidates(hits)
	if len(c) != 4 {
		t.Fatalf("aday sayisi: %d", len(c))
	}
	if c[0].Name != "Merit Royal Premium Hotel & Casino & Spa" || c[0].URL != "https://www.tatilbudur.com/merit-royal-premium-hotel-casino-spa" || c[0].ProviderHotelID != "6733" || c[0].City != "Girne" {
		t.Errorf("aday: %+v", c[0])
	}
}

func TestMergeCookies(t *testing.T) {
	got := mergeCookies("a=1; ak_bmsc=old; b=2", "ak_bmsc=new; c=3")
	if got != "a=1; ak_bmsc=new; b=2; c=3" {
		t.Errorf("mergeCookies: %q", got)
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
