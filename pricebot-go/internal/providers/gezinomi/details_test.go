package gezinomi

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestParseHotelDetail(t *testing.T) {
	b, err := os.ReadFile("testdata/detail_rsc.html")
	if err != nil {
		t.Fatal(err)
	}
	d, err := parseHotelDetail(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if d.HotelID != 932 || d.HotelName != "Acapulco Resort Convention Spa Hotel" {
		t.Fatalf("id/ad yanlis: %d %q", d.HotelID, d.HotelName)
	}
	for _, ds := range d.Descriptions {
		if ds.Text != nil && strings.HasPrefix(*ds.Text, "$") {
			t.Errorf("aciklama $ref cozulmedi: %s", ds.TypeName)
		}
	}
	out := normalizeDetails(d, "932", "https://www.gezinomi.com/otel/kibris/girne/acapulco-resort-convention-spa-hotel")
	if out.Provider != "gezinomi" || out.ProviderHotelID != "932" || out.Region != "Kıbrıs" || out.City != "Girne" || out.Country != "Kıbrıs" {
		t.Errorf("kimlik/konum alanlari yanlis: %+v", out)
	}
	if out.Stars == nil || *out.Stars != 5 {
		t.Errorf("5 yildiz bekleniyordu: %v", out.Stars)
	}
	if out.Rating == nil || *out.Rating != 8.53 || out.RatingCount == nil || *out.RatingCount != 143 {
		t.Errorf("puan yanlis: %v %v", out.Rating, out.RatingCount)
	}
	if out.Location == nil || out.Location.Lat != 35.33198 || out.Location.Lng != 33.421569 {
		t.Errorf("konum yanlis: %+v", out.Location)
	}
	// 4 gorsel, biri kampanya gorseli (atlanir)
	if len(out.Images) != 3 || out.Images[0].URL != imgBase+"acapulco-resort-convention-spa-hotel-932--1-21.04.2026174608-b0.jpg" || out.Images[0].Caption != "Genel" {
		t.Errorf("gorseller yanlis: %+v", out.Images)
	}
	if len(out.Facilities) < 2 || out.Facilities[0].Title != "Konum Bilgileri" || out.Facilities[0].Items[0] != "Hava Alanına Uzaklığı: 30 km" || out.Facilities[0].Items[1] != "Şehir Merkezine Uzaklığı: 8 km (Girne)" {
		t.Errorf("ozellik gruplari yanlis: %+v", out.Facilities)
	}
	if len(out.Rooms) != 3 || out.Rooms[0].Name != "Kara Manzaralı Otel Odası" || out.Rooms[0].Size == nil || *out.Rooms[0].Size != 26 || out.Rooms[0].Capacity != "4 kişi (en fazla 3 yetişkin)" || len(out.Rooms[0].Images) != 2 {
		t.Errorf("odalar yanlis: %+v", out.Rooms)
	}
	if out.Policies == nil || out.Policies.CheckIn != "15:00" || out.Policies.CheckOut != "11:00" || out.Policies.Cancellation != "Giriş Gününden önce 7 Güne Kadar İptal Hakkı" || !strings.Contains(out.Policies.PetPolicy, "evcil hayvan kabul edilmemektedir") || out.Policies.ChildPolicy == "" {
		t.Errorf("politikalar yanlis: %+v", out.Policies)
	}
	if !strings.HasPrefix(out.Description, "Girne’de denize sıfır") {
		t.Errorf("aciklama yanlis: %.60q", out.Description)
	}
	titles := []string{}
	for _, s := range out.Sections {
		titles = append(titles, s.Title)
		if strings.Contains(s.Text, "<") || strings.Contains(strings.Join(s.Items, ""), "<") {
			t.Errorf("section %s HTML iceriyor", s.Title)
		}
	}
	joined := strings.Join(titles, "|")
	for _, w := range []string{"Tesis Hakkında Genel Bilgiler", "Yeme - İçme", "Bebek/Çocuk", "Evcil Hayvan Politikası", "Öne Çıkan Özellikler", "Kategoriler", "Etkinlikler"} {
		if !strings.Contains(joined, w) {
			t.Errorf("section %q eksik: %v", w, titles)
		}
	}
	if strings.Contains(joined, "A'la Carte") || strings.Contains(joined, "Kampanyalar") {
		t.Errorf("bos aciklama / gunluk degisen kampanya section olmamali: %v", titles)
	}
	if out.Policies.ChildPolicy != "Ücretsiz çocuk yaşları dönemsel değişkenlik gösterebilmektedir. 1. çocuk 0-11 yaş, 2. çocuk 0-3 yaş." {
		t.Errorf("cocuk politikasi yanlis: %q", out.Policies.ChildPolicy)
	}
	a, _ := json.Marshal(normalizeDetails(d, "932", "x"))
	b2, _ := json.Marshal(normalizeDetails(d, "932", "x"))
	if string(a) != string(b2) {
		t.Errorf("normalizeDetails deterministik degil")
	}
	if out.FAQ == nil || out.Images == nil || out.Facilities == nil || out.Rooms == nil || out.Sections == nil {
		t.Errorf("dizi alanlari nil olmamali")
	}
}

func TestPictureURL(t *testing.T) {
	if got := pictureURL(gzPicture{Name: "kampanyagorselleri-04-14.10.2026171649.jpg"}); got != imgBase+"kampanyagorselleri-04-14.10.2026171649.jpg" {
		t.Errorf("uzantili ad: %s", got)
	}
	ext := "https://cdn.x/a.png?x=1"
	if got := pictureURL(gzPicture{Name: "n", ExternalImageURL: &ext}); got != "https://cdn.x/a.png" {
		t.Errorf("harici URL: %s", got)
	}
}

func TestParseAutoComplete(t *testing.T) {
	b, err := os.ReadFile("testdata/autocomplete.json")
	if err != nil {
		t.Fatal(err)
	}
	cands, err := parseAutoComplete(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) < 4 {
		t.Fatalf("en az 4 otel bekleniyordu: %+v", cands)
	}
	c := cands[0]
	if c.Name != "Merit Royal Diamond Hotel & Spa" || c.URL != "https://www.gezinomi.com/otel/kibris/girne/merit-royal-diamond-hotel-spa" || c.City != "Girne" {
		t.Errorf("ilk aday yanlis: %+v", c)
	}
	for _, c := range cands {
		if strings.Contains(c.Name, "Otelleri") || !reDomain.MatchString(c.URL) {
			t.Errorf("otel disi sonuc/gecersiz URL: %+v", c)
		}
	}
	if _, err := parseAutoComplete([]byte(`{"data":null,"status":"Invalid","message":"Sorgu uzunluğu en az 3 karakterden oluşmalı"}`)); err != nil {
		t.Errorf("kisa sorgu hata vermemeli: %v", err)
	}
	if _, err := parseAutoComplete([]byte(`{"data":null,"status":"Invalid","message":"x"}`)); err == nil {
		t.Errorf("Invalid yanit hata vermeli")
	}
}
