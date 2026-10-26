package setur

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestParseDetailPage(t *testing.T) {
	b, err := os.ReadFile("testdata/detail_rsc.html")
	if err != nil {
		t.Fatal(err)
	}
	d, err := parseDetailPage(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != 492 || d.Name != "Acapulco Resort Convention Spa Hotel & Casino" {
		t.Fatalf("id/ad yanlis: %d %q", d.ID, d.Name)
	}
	if !strings.HasPrefix(d.GeneralConditions, "<p") || strings.HasPrefix(d.GeneralConditions, "$") {
		t.Errorf("generalConditions $ref cozulmedi: %.40q", d.GeneralConditions)
	}
	for _, c := range d.SeasonFeatures[0].Categories {
		if strings.HasPrefix(c.Description, "$") {
			t.Errorf("kategori aciklamasi $ref cozulmedi: %s -> %s", c.Name, c.Description)
		}
	}
	out := normalizeDetails(d, "492", "https://www.setur.com.tr/acapulco-resort-convention-spa-hotel-casino")
	if out.Provider != "setur" || out.ProviderHotelID != "492" || out.City != "Girne" || out.Country != "Kıbrıs" || out.Region != "Girne Merkez" {
		t.Errorf("kimlik/konum alanlari yanlis: %+v", out)
	}
	if out.Stars == nil || *out.Stars != 5 {
		t.Errorf("5 yildiz bekleniyordu: %v", out.Stars)
	}
	if out.Location == nil || out.Location.Lat < 35 || out.Location.Lng < 33 {
		t.Errorf("konum yanlis: %+v", out.Location)
	}
	if len(out.Images) != 3 || !strings.HasPrefix(out.Images[0].URL, "https://img-setur.sm.mncdn.com/images/hotels/492/") || out.Images[0].Caption != "Genel" {
		t.Errorf("gorseller yanlis: %+v", out.Images)
	}
	if len(out.Rooms) != 4 || out.Rooms[0].Name != "Standart Oda - Deniz Manzaralı" || out.Rooms[0].Size == nil || *out.Rooms[0].Size != 30 || len(out.Rooms[0].Images) != 2 {
		t.Errorf("odalar yanlis: %+v", out.Rooms)
	}
	if out.Rooms[1].Name != "Standart Oda - Kara Manzaralı" {
		t.Errorf("oda adindaki cift bosluk sadelesmeli: %q", out.Rooms[1].Name)
	}
	if len(out.Facilities) != 7 || out.Facilities[0].Title != "Havuz & Plaj" || out.Facilities[0].Items[0] != "Aquapark" {
		t.Errorf("ozellik gruplari yanlis: %+v", out.Facilities)
	}
	if out.Policies == nil || out.Policies.CheckIn != "15:00" || out.Policies.CheckOut != "11:00" || !strings.Contains(out.Policies.PetPolicy, "evcil hayvan kabul edilmiyor") {
		t.Errorf("politikalar yanlis: %+v", out.Policies)
	}
	if !strings.Contains(out.Description, "Akdeniz’in eşsiz doğası") || strings.Contains(out.Description, "<") {
		t.Errorf("aciklama HTML temizlenmemis/eksik: %.80q", out.Description)
	}
	titles := []string{}
	for _, s := range out.Sections {
		titles = append(titles, s.Title)
	}
	want := []string{"Genel Bilgiler", "Yiyecek & İçecek", "Spor & Eğlence", "Spa & Sağlık", "Çocuk Hizmetleri", "Balayı", "Konum", "Ödüller ve Sertifikalar", "Evcil Hayvan Kabul Şartları", "Öne Çıkan Özellikler", "Mesafeler", "Etkinlikler"}
	if strings.Join(titles, "|") != strings.Join(want, "|") {
		t.Errorf("section basliklari yanlis:\n got %v\nwant %v", titles, want)
	}
	for _, s := range out.Sections {
		if strings.Contains(s.Text, "<") || strings.Contains(s.Text, "&nbsp;") {
			t.Errorf("section %s HTML iceriyor: %.60q", s.Title, s.Text)
		}
	}
	// Deterministiklik: ayni girdi -> ayni JSON
	a, _ := json.Marshal(normalizeDetails(d, "492", "x"))
	b2, _ := json.Marshal(normalizeDetails(d, "492", "x"))
	if string(a) != string(b2) {
		t.Errorf("normalizeDetails deterministik degil")
	}
	if out.FAQ == nil || out.Images == nil || out.Facilities == nil || out.Rooms == nil || out.Sections == nil {
		t.Errorf("dizi alanlari nil olmamali")
	}
}

func TestParseSearchHits(t *testing.T) {
	b, err := os.ReadFile("testdata/search_hotel.json")
	if err != nil {
		t.Fatal(err)
	}
	cands, err := parseSearchHits(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 9 { // 10 kayit, biri HotelChain
		t.Fatalf("9 otel bekleniyordu, %d: %+v", len(cands), cands)
	}
	c := cands[0]
	if c.Name != "Merit Royal Diamond Hotel & Spa" || c.URL != "https://www.setur.com.tr/merit-royal-diamond-hotel-spa" || c.City != "Girne, Kıbrıs" || c.ProviderHotelID != "169030" {
		t.Errorf("ilk aday yanlis: %+v", c)
	}
	for _, c := range cands {
		if strings.Contains(c.Name, "Otelleri") {
			t.Errorf("zincir/sehir sonucu filtrelenmeli: %+v", c)
		}
		if slugFromURL(c.URL) == "" {
			t.Errorf("URL ResolveFromURL ile uyumsuz: %s", c.URL)
		}
	}
}

func TestHTMLHelpers(t *testing.T) {
	got := htmlToText(`<p class="x">Bir&nbsp;iki<br>üç</p><script>bad()</script><ul><li>a</li><li>b &amp; c</li></ul>`)
	if got != "Bir iki\nüç\na\nb & c" {
		t.Errorf("htmlToText: %q", got)
	}
	sec := htmlSection("Baslik", `<p>Giris</p><ul><li>a</li><li>b</li></ul>`)
	if sec == nil || sec.Kind != "list" || len(sec.Items) != 2 || sec.Text != "Giris" {
		t.Errorf("htmlSection: %+v", sec)
	}
	if htmlSection("Bos", "<br>") != nil {
		t.Errorf("bos icerik nil olmali")
	}
	if cleanImageURL("//cdn.x/a.jpg?v=123") != "https://cdn.x/a.jpg" {
		t.Errorf("cleanImageURL")
	}
}
