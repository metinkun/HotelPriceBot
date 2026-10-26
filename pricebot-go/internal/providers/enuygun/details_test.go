package enuygun

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestNormalizeDetails(t *testing.T) {
	b, err := os.ReadFile("testdata/api_detail_full.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := parseAPIDetailFull(b)
	if err != nil {
		t.Fatal(err)
	}
	out := normalizeDetails(d, "580512", "https://www.enuygun.com/otel/detay/acapulco-resort-convention-spa-580512/")
	if out.Provider != "enuygun" || out.ProviderHotelID != "580512" || out.Name != "Acapulco Resort Convention & Spa" {
		t.Fatalf("kimlik yanlis: %+v", out)
	}
	if out.City != "Girne" || out.Region != "Girne" || out.Country != "Kıbrıs" || out.Address != "Girne/Kuzey Kıbrıs Türk Cumhuriyeti" {
		t.Errorf("konum alanlari yanlis: %s / %s / %s / %s", out.City, out.Region, out.Country, out.Address)
	}
	if out.Stars == nil || *out.Stars != 5 || out.Rating == nil || *out.Rating != 7.24 || out.RatingCount == nil || *out.RatingCount != 39 {
		t.Errorf("yildiz/puan yanlis: %v %v %v", out.Stars, out.Rating, out.RatingCount)
	}
	if out.Location == nil || out.Location.Lat != 35.330433 || out.Contact == nil || out.Contact.Phone != "00903928244110" {
		t.Errorf("konum/iletisim yanlis: %+v %+v", out.Location, out.Contact)
	}
	if len(out.Images) != 3 || !strings.HasPrefix(out.Images[0].URL, "https://cdn3.enuygun.com/media/lib/") || out.Images[0].Caption != "Genel" {
		t.Errorf("gorseller yanlis: %+v", out.Images)
	}
	if len(out.Facilities) != 3 || out.Facilities[0].Title != "Yeme & İçme" || out.Facilities[0].Items[1] != "Bar (Ücretli)" {
		t.Errorf("ozellik gruplari yanlis: %+v", out.Facilities)
	}
	if len(out.Rooms) != 3 || out.Rooms[0].Name != "Otel Standard Kara Manzaralı Oda" || out.Rooms[0].Size == nil || *out.Rooms[0].Size != 35 || len(out.Rooms[0].Features) != 9 || len(out.Rooms[0].Images) != 2 {
		t.Errorf("odalar yanlis: %+v", out.Rooms)
	}
	if strings.Join(out.Boards, "|") != "Her Şey Dahil|Ultra Her Şey Dahil|Yarım Pansiyon" {
		t.Errorf("pansiyonlar yanlis: %v", out.Boards)
	}
	if out.Policies == nil || out.Policies.CheckIn != "15:00 - 23:59" || out.Policies.CheckOut != "11:00" || len(out.Policies.Other) != 1 || !strings.Contains(out.Policies.Other[0], "18") {
		t.Errorf("politikalar yanlis: %+v", out.Policies)
	}
	// 9 SSS; fiyat verisinden uretilen 2 tanesi atlanir
	if len(out.FAQ) != 7 {
		t.Errorf("7 SSS bekleniyordu: %d", len(out.FAQ))
	}
	for _, qa := range out.FAQ {
		if strings.Contains(qa.Answer, "Son güncelleme") || strings.Contains(qa.Answer, "Verilerimize göre") {
			t.Errorf("gunluk degisen SSS atlanmali: %s", qa.Question)
		}
	}
	if !strings.HasPrefix(out.Description, "Girne’de, Akdeniz’in") || strings.Contains(out.Description, "<p>") {
		t.Errorf("aciklama yanlis: %.60q", out.Description)
	}
	titles := []string{}
	for _, s := range out.Sections {
		titles = append(titles, s.Title)
		if strings.Contains(s.Text, "<") || strings.Contains(strings.Join(s.Items, ""), "<") {
			t.Errorf("section %s HTML iceriyor", s.Title)
		}
	}
	want := "Otel Hakkında|Önemli Bilgiler|Bilmeniz Gerekenler|Otel Politikaları|Konaklama Konseptleri|Popüler Özellikler|Otel Etiketleri|Yakın Çevre|Etkinlikler"
	if strings.Join(titles, "|") != want {
		t.Errorf("section basliklari yanlis:\n got %v", titles)
	}
	for _, s := range out.Sections {
		switch s.Title {
		case "Otel Politikaları":
			if s.Kind != "list" || len(s.Items) < 6 {
				t.Errorf("politikalar liste olmali: %+v", s)
			}
		case "Yakın Çevre":
			if s.Kind != "pairs" || len(s.Pairs) != 1 || s.Pairs[0].Key != "Ercan Havaalanı" || s.Pairs[0].Value != "21.7 km" {
				t.Errorf("yakin cevre yanlis: %+v", s)
			}
		case "Konaklama Konseptleri":
			if s.Kind != "pairs" || len(s.Pairs) != 3 || !strings.Contains(s.Pairs[2].Value, "Yarım Pansiyon konaklamalarda") {
				t.Errorf("konseptler yanlis: %+v", s)
			}
		}
	}
	a, _ := json.Marshal(normalizeDetails(d, "580512", "x"))
	b2, _ := json.Marshal(normalizeDetails(d, "580512", "x"))
	if string(a) != string(b2) {
		t.Errorf("normalizeDetails deterministik degil")
	}
}

func TestParseAutocomplete(t *testing.T) {
	b, err := os.ReadFile("testdata/autocomplete_core.json")
	if err != nil {
		t.Fatal(err)
	}
	cands, err := parseCoreAutocomplete(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 4 {
		t.Fatalf("4 otel bekleniyordu: %+v", cands)
	}
	c := cands[0]
	if c.Name != "Merit Royal Premium Hotel & Spa" || c.URL != "https://www.enuygun.com/otel/detay/merit-royal-premium-hotel-all-inclusive-32796096/" || c.City != "Girne, Kıbrıs" || c.ProviderHotelID != "796492" {
		t.Errorf("ilk aday yanlis: %+v", c)
	}
	for _, c := range cands {
		if slugFromURL(c.URL) == "" {
			t.Errorf("URL ResolveFromURL ile uyumsuz: %s", c.URL)
		}
	}

	b, err = os.ReadFile("testdata/autocomplete_legacy.json")
	if err != nil {
		t.Fatal(err)
	}
	cands, err = parseLegacyAutocomplete(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 3 || cands[0].Name != "Merit Royal Gardens Hotel" || cands[0].URL != "https://www.enuygun.com/otel/detay/merit-royal-gardens-hotel-580522/" || cands[0].ProviderHotelID != "580522" || cands[0].City != "İskele, Kıbrıs" {
		t.Errorf("eski uc adaylari yanlis: %+v", cands)
	}
}
