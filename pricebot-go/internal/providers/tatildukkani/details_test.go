package tatildukkani

// Statik detay ve isimle arama fixture testleri (Acapulco 1404, Ekim 2026; kirpilmis gercek yanitlar, yorumlar cikarildi).

import (
	"encoding/json"
	"strings"
	"testing"
)

func loadData(t *testing.T, name string) json.RawMessage {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(read(t, name), &env); err != nil {
		t.Fatal(err)
	}
	return env.Data
}

func TestBuildDetails(t *testing.T) {
	data := loadData(t, "page_static.json")
	out, err := buildDetails(data, "https://www.tatildukkani.com/acapulco-resort-convention-spa")
	if err != nil {
		t.Fatal(err)
	}
	if out.Provider != "tatildukkani" || out.ProviderHotelID != "1404" || out.Name != "Acapulco Resort Convention Spa" {
		t.Fatalf("kimlik: %+v", out)
	}
	if out.Stars == nil || *out.Stars != 5 || out.City != "Girne" || out.Country != "Kıbrıs" || out.Region != "" || out.Address != "Çatalköy Mevkii-Girne" {
		t.Errorf("yildiz/konum: %v %q %q %q %q", out.Stars, out.City, out.Country, out.Region, out.Address)
	}
	if out.Location == nil || out.Location.Lat < 35.32 || out.Location.Lat > 35.34 || out.Location.Lng < 33.42 || out.Location.Lng > 33.43 {
		t.Errorf("koordinat: %+v", out.Location)
	}
	if out.Rating == nil || *out.Rating != 8.6 || out.RatingCount == nil || *out.RatingCount != 40 {
		t.Errorf("puan 8.6 / 40 yorum bekleniyordu: %v %v", out.Rating, out.RatingCount)
	}
	if !strings.HasPrefix(out.Description, "Acapulco Resort & Convention & SPA") || strings.Contains(out.Description, "<") {
		t.Errorf("aciklama: %.80q", out.Description)
	}

	if len(out.Images) != 7 || !strings.HasSuffix(out.Images[0].URL, "17923921216ad5bbb9c8d5f8.62253711.webp") || out.Images[0].Caption != "" {
		t.Fatalf("gorseller (kapak once, 7 adet): %d %+v", len(out.Images), out.Images[0])
	}
	if out.Images[1].Caption != "Genel" || !strings.HasSuffix(out.Images[1].URL, "6ad5bbbb553be8.28379179.webp") || out.Images[6].Caption != "Oda" {
		t.Errorf("gorsel sirasi/etiketi: %+v %+v", out.Images[1], out.Images[6])
	}

	titles := []string{}
	for _, f := range out.Facilities {
		titles = append(titles, f.Title)
	}
	if strings.Join(titles, "|") != "Genel Özellikler|Havuz & Plaj|Konum Bilgisi|Spa" {
		t.Fatalf("ozellik gruplari: %v", titles)
	}
	g := out.Facilities[0].Items
	if len(g) != 14 || g[0] != "Aquapark" || g[1] != "Araç Kiralama (Ücretli)" {
		t.Errorf("Genel Ozellikler: %v", g)
	}
	joined := strings.Join(g, "|")
	for _, want := range []string{"Açılış Yılı: 1982", "Oda Sayısı: 826 Room", "Doktor (Ücretli)"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%q bekleniyordu: %v", want, g)
		}
	}
	if items := strings.Join(out.Facilities[1].Items, "|"); !strings.Contains(items, "Plaj: Kum") || strings.Contains(items, "Bilgisi") {
		t.Errorf("Havuz & Plaj: %v", out.Facilities[1].Items)
	}
	if items := strings.Join(out.Facilities[3].Items, "|"); !strings.Contains(items, "Hamam (Ücretli)") {
		t.Errorf("Spa: %v", out.Facilities[3].Items)
	}

	if len(out.Rooms) != 0 || out.Rooms == nil || out.FAQ == nil {
		t.Errorf("odalar bos dizi, FAQ bos dizi olmali: %#v %#v", out.Rooms, out.FAQ)
	}
	if len(out.Boards) != 1 || out.Boards[0] != "Tam Pansiyon Plus" {
		t.Errorf("pansiyon: %v", out.Boards)
	}
	p := out.Policies
	if p == nil || p.CheckIn != "15:00" || p.CheckOut != "11:00" || !strings.Contains(p.PetPolicy, "Evcil hayvan kabul edilmemektedir") || len(p.Other) < 5 {
		t.Errorf("politikalar: %+v", p)
	}

	st := []string{}
	kinds := map[string]string{}
	for _, s := range out.Sections {
		st = append(st, s.Title)
		kinds[s.Title] = s.Kind
	}
	want := "Genel Özellikler|Genel Bilgi|Genel Uyarılar|Havuz & Plaj|Havuz Plaj Bilgisi|Konum Bilgisi|Konum Bilgisi (Açıklama)|Spa|Spa Bilgisi|Etkinlikler|Misafir Değerlendirmeleri"
	if strings.Join(st, "|") != want {
		t.Errorf("bolumler:\n got  %v\n want %v", st, want)
	}
	if kinds["Genel Bilgi"] != "text" || kinds["Genel Özellikler"] != "list" || kinds["Misafir Değerlendirmeleri"] != "pairs" {
		t.Errorf("bolum turleri: %v", kinds)
	}
	for _, s := range out.Sections {
		if s.Title == "Etkinlikler" && strings.Join(s.Items, "|") != "2026-10-31 — Alya|2026-11-14 — Mert Demir|2027-07-17 — Alya" {
			t.Errorf("etkinlikler: %v", s.Items)
		}
		if s.Title == "Misafir Değerlendirmeleri" && (len(s.Pairs) != 8 || s.Pairs[0].Value != "4.3 / 5" || s.Pairs[1].Value != "40") {
			t.Errorf("degerlendirme: %+v", s.Pairs)
		}
	}

	a, _ := json.Marshal(out)
	out2, _ := buildDetails(data, "https://www.tatildukkani.com/acapulco-resort-convention-spa")
	b, _ := json.Marshal(out2)
	if string(a) != string(b) {
		t.Error("buildDetails deterministik degil")
	}

	// Otel olmayan sayfa
	if _, err := buildDetails(json.RawMessage(`{"type":"HOTEL_DESTINATION","typeId":5,"result":{}}`), "x"); err == nil {
		t.Error("destinasyon sayfasi hata vermeli")
	}
}

func TestParseAutocomplete(t *testing.T) {
	got, err := parseAutocomplete(loadData(t, "autocomplete.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 { // slug'siz kayit atilir
		t.Fatalf("4 aday bekleniyordu: %+v", got)
	}
	c := got[0]
	if c.Name != "Merit Royal Diamond Hotel & Spa" || c.URL != "https://www.tatildukkani.com/merit-royal-diamond-hotel-spa" || c.City != "Girne" || c.ProviderHotelID != "1433" {
		t.Errorf("aday: %+v", c)
	}
	if got[1].Name != "Merit Royal Gardens Hotel" || got[1].City != "Gazimağusa" {
		t.Errorf("ad bosluklari kirpilmali / sehir son destinasyon: %+v", got[1])
	}
	if got[3].URL != "https://www.tatildukkani.com/merit-royal-premium-hotel" {
		t.Errorf("premium url: %+v", got[3])
	}
}
