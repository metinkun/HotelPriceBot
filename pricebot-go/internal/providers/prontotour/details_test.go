package prontotour

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func loadDetailFull(t *testing.T, name string) *HotelDetailFull {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var r HotelDetailFull
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func TestNormalizeDetails(t *testing.T) {
	d := normalizeDetails(loadDetailFull(t, "hotel_detail_static.json"))
	if d.Name != "Acapulco Resort Hotel" {
		t.Fatalf("ad: %q", d.Name)
	}
	if d.City != "Girne" || d.Country != "Kuzey Kıbrıs" {
		t.Errorf("sehir/ulke: %q / %q", d.City, d.Country)
	}
	if d.Location == nil || d.Location.Lat != 35.33031 || d.Location.Lng != 33.420868 {
		t.Errorf("konum: %+v", d.Location)
	}
	if len(d.Images) != 7 || d.Images[0].URL != "https://cdn.crtbooking.com/hotels/acapulco-resort-hotel/J1X9CI_455172418.jpg" {
		t.Errorf("gorseller: %d %+v", len(d.Images), d.Images)
	}
	if !strings.Contains(d.Description, "Girne’nin en temiz sahillerinden") || strings.Contains(d.Description, "<") {
		t.Errorf("aciklama: %q", d.Description)
	}
	// Genel Bilgiler / Konum Bilgileri ozellik grubu olmaz; diger 7 kategori grup olur.
	titles := []string{}
	for _, g := range d.Facilities {
		titles = append(titles, g.Title)
	}
	want := []string{"Temalar", "Spa & Fitness", "Havuz & Plaj", "Genel Özellikler", "Bebek & Çocuk", "Balayı Çiftleri", "Aktiviteler"}
	if !reflect.DeepEqual(titles, want) {
		t.Errorf("ozellik gruplari: %v", titles)
	}
	for _, g := range d.Facilities {
		if g.Title == "Spa & Fitness" {
			if len(g.Items) != 9 || g.Items[0] != "Jakuzi (Ücretli)" || g.Items[1] != "Spa Bilgisi" || g.Items[3] != "Buhar Odası" {
				t.Errorf("spa ozellikleri: %v", g.Items)
			}
		}
	}
	if d.Policies == nil || d.Policies.CheckIn != "14:00" || d.Policies.CheckOut != "12:00" {
		t.Errorf("politikalar: %+v", d.Policies)
	}
	if len(d.Rooms) != 3 || d.Rooms[0].Name != "Deniz Manzaralı Standart" || d.Rooms[2].Name != "Villa Oda" {
		t.Fatalf("odalar: %+v", d.Rooms)
	}
	villa := d.Rooms[2]
	if len(villa.Features) == 0 || villa.Features[0] != "Banyo (Duşakabin)" {
		t.Errorf("villa ozellikleri: %v", villa.Features)
	}
	for _, f := range villa.Features {
		if f == "Kahvaltı (Ücretli)" {
			t.Errorf("pansiyona bagli ozellik odaya yazilmamali: %v", villa.Features)
		}
	}
	if !reflect.DeepEqual(d.Boards, []string{"Tam Pansiyon Plus", "Yarım Pansiyon"}) {
		t.Errorf("pansiyonlar: %v", d.Boards)
	}
	secTitles := []string{}
	for _, s := range d.Sections {
		secTitles = append(secTitles, s.Title+"/"+s.Kind)
	}
	wantSec := []string{"Otel Hakkında/text", "Tesis Kuralları/pairs", "Konum Bilgileri/pairs", "Mesafeler/pairs"}
	if !reflect.DeepEqual(secTitles, wantSec) {
		t.Errorf("bolumler: %v", secTitles)
	}
	for _, s := range d.Sections {
		if s.Title == "Konum Bilgileri" {
			for _, kv := range s.Pairs {
				if strings.Contains(strings.ToUpper(kv.Key), "LATITUDE") {
					t.Errorf("koordinat konum ciftlerinde olmamali: %v", s.Pairs)
				}
			}
			if s.Pairs[0].Key != "Havalimanına Mesafe" || s.Pairs[0].Value != "28 KM" {
				t.Errorf("konum ciftleri: %v", s.Pairs)
			}
		}
	}
	if d.FAQ == nil || d.Rooms == nil || d.Images == nil {
		t.Error("dizi alanlari nil olmamali")
	}
}

func TestNormalizeDetailsDeterministic(t *testing.T) {
	a, _ := json.Marshal(normalizeDetails(loadDetailFull(t, "hotel_detail_static.json")))
	b, _ := json.Marshal(normalizeDetails(loadDetailFull(t, "hotel_detail_static.json")))
	if string(a) != string(b) {
		t.Fatal("iki calistirma farkli JSON uretti")
	}
}

func TestFilterHotelsAndURL(t *testing.T) {
	b, err := os.ReadFile("testdata/autocomplete_hotels.json")
	if err != nil {
		t.Fatal(err)
	}
	var items []autoCompleteHotel
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatal(err)
	}
	hs := filterHotels(items)
	if len(hs) != 4 || hs[3].Code != "39" || hs[3].Parent1 != "Girne" {
		t.Fatalf("otel filtresi: %+v", hs)
	}
	u := detailURL(hs[3].Name, hs[3].Code, "8")
	if u != "https://www.prontotour.com/Merit-Royal-Premium-Hotel-Casino-h39-8" {
		t.Errorf("url: %s", u)
	}
	if _, err := parseDetailURL(u); err != nil {
		t.Errorf("uretilen URL ResolveFromURL tarafindan kabul edilmeli: %v", err)
	}
	if detailURL("Acapulco Resort Hotel", "3234", "8") != "https://www.prontotour.com/Acapulco-Resort-Hotel-h3234-8" {
		t.Errorf("acapulco url: %s", detailURL("Acapulco Resort Hotel", "3234", "8"))
	}
}

func TestCleanImageURL(t *testing.T) {
	if got := cleanImageURL("https://cdn.x.com/a.jpg?v=12345&w=300"); got != "https://cdn.x.com/a.jpg?w=300" {
		t.Errorf("cache-buster temizlenmeli: %s", got)
	}
	if got := cleanImageURL("https://cdn.x.com/a.jpg?t=1"); got != "https://cdn.x.com/a.jpg" {
		t.Errorf("bos sorgu atilmali: %s", got)
	}
}
