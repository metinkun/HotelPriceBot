package etstur

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func loadDetailFixture(t *testing.T) (*detailData, []roomStatic) {
	t.Helper()
	b, err := os.ReadFile("testdata/detail_api.json")
	if err != nil {
		t.Fatal(err)
	}
	var dr detailResponse
	if err := json.Unmarshal(b, &dr); err != nil || dr.Result == nil {
		t.Fatalf("detail fixture parse: %v", err)
	}
	rb, err := os.ReadFile("testdata/room_multi.json")
	if err != nil {
		t.Fatal(err)
	}
	var rm roomMultiResponse
	if err := json.Unmarshal(rb, &rm); err != nil || rm.Result == nil {
		t.Fatalf("room fixture parse: %v", err)
	}
	var rooms []roomStatic
	for _, g := range rm.Result.RoomGroups {
		rooms = append(rooms, g.Rooms...)
	}
	return dr.Result, rooms
}

func TestNormalizeDetails(t *testing.T) {
	d, rooms := loadDetailFixture(t)
	out := normalizeDetails(d, rooms, "61b893df0b29", "https://www.etstur.com/Acapulco-Resort-Convention-Spa")

	if out.Name != "Acapulco Resort Convention Spa" {
		t.Errorf("name: %q", out.Name)
	}
	if out.City != "Girne" || out.Country != "Kıbrıs" || out.Region != "Girne Merkez" {
		t.Errorf("location: %q %q %q", out.City, out.Region, out.Country)
	}
	if out.Rating == nil || *out.Rating != 7.6 || out.RatingCount == nil || *out.RatingCount != 3862 {
		t.Errorf("rating: %v %v", out.Rating, out.RatingCount)
	}
	if out.Location == nil || out.Location.Lat < 35 || out.Location.Lat > 36 {
		t.Errorf("geo: %+v", out.Location)
	}
	if out.Contact == nil || out.Contact.Email != "reservations@acapulco.com.tr" {
		t.Errorf("contact: %+v", out.Contact)
	}
	if len(out.Images) != 3 || !strings.HasPrefix(out.Images[0].URL, "https://images.etstur.com/") {
		t.Errorf("images: %d %+v", len(out.Images), out.Images)
	}
	if len(out.Facilities) < 3 || out.Facilities[0].Title != "Genel Özellikler" {
		t.Errorf("facilities: %+v", out.Facilities)
	}
	if len(out.FAQ) != 2 || out.FAQ[0].Question == "" {
		t.Errorf("faq: %+v", out.FAQ)
	}
	if out.Policies == nil || out.Policies.CheckIn != "15:00" || out.Policies.CheckOut != "11:00" {
		t.Errorf("policies: %+v", out.Policies)
	}
	if !strings.HasPrefix(out.Description, "Girne’nin en temiz") || strings.Contains(out.Description, "&") || strings.Contains(out.Description, "<") {
		t.Errorf("description temiz degil: %q", out.Description[:80])
	}
	titles := map[string]bool{}
	for _, s := range out.Sections {
		titles[s.Title] = true
	}
	for _, want := range []string{"Genel Bilgiler", "Temalar", "Önemli Bilgiler", "Konum / Önemli Noktalar", "Misafir Değerlendirmesi"} {
		if !titles[want] {
			t.Errorf("section eksik: %s (%v)", want, titles)
		}
	}
	// Odalar: pansiyon eki atilir, ayni oda birlesir, ada gore sirali
	if len(out.Rooms) != 2 || out.Rooms[0].Name != "Acapulco Villa (Bungalow)" || out.Rooms[1].Name != "Otel Odası Kara Manzaralı" {
		t.Fatalf("rooms: %+v", out.Rooms)
	}
	r := out.Rooms[0]
	if r.Size == nil || *r.Size != 30 || r.Capacity != "2 yetişkin" || len(r.Features) == 0 || len(r.Images) == 0 {
		t.Errorf("room0: %+v", r)
	}
	if strings.Contains(r.Description, "&lt;") || !strings.Contains(r.Description, "m²") {
		t.Errorf("room description cift escape cozulmedi: %q", r.Description)
	}
	if len(out.Boards) == 0 || out.Boards[len(out.Boards)-1] != "Yarım Pansiyon" {
		t.Errorf("boards: %v", out.Boards)
	}
}

func TestNormalizeDetailsDeterministic(t *testing.T) {
	d, rooms := loadDetailFixture(t)
	a := normalizeDetails(d, rooms, "x", "u")
	b := normalizeDetails(d, rooms, "x", "u")
	a.FetchedAt, b.FetchedAt = time.Time{}, time.Time{}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Error("iki normalize cagrisi farkli JSON uretti")
	}
}

func TestCleanHTML(t *testing.T) {
	got := cleanHTML("&lt;p&gt;Villa 30 m&lt;sup&gt;2&lt;/sup&gt; b&amp;uuml;y&amp;uuml;k.&lt;/p&gt;\r\n&lt;p&gt;Ikinci&lt;/p&gt;")
	if got != "Villa 30 m² büyük.\nIkinci" {
		t.Errorf("cleanHTML: %q", got)
	}
	if got := cleanHTML("<p>Girne&rsquo;nin\nen&nbsp;temiz</p>"); got != "Girne’nin en temiz" {
		t.Errorf("cleanHTML satir/entity: %q", got)
	}
}

func TestHotelCandidates(t *testing.T) {
	b, err := os.ReadFile("testdata/autocomplete.json")
	if err != nil {
		t.Fatal(err)
	}
	var sr suggestResponse
	if err := json.Unmarshal(b, &sr); err != nil {
		t.Fatal(err)
	}
	c := hotelCandidates(sr.Result)
	if len(c) != 3 {
		t.Fatalf("aday sayisi: %d", len(c))
	}
	if c[1].Name != "Merit Royal Premium Hotel & Spa" || c[1].URL != "https://www.etstur.com/Merit-Royal-Premium-Hotel-Spa" || c[1].City != "Alsancak, Girne" {
		t.Errorf("aday: %+v", c[1])
	}
}

func TestStaticStayWindow(t *testing.T) {
	ci, co := staticStayWindow(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))
	if ci != "2026-12-01" || co != "2026-12-04" {
		t.Errorf("%s %s", ci, co)
	}
	if s := slugFromURL("https://www.etstur.com/Acapulco-Resort-Convention-Spa?x=1"); s != "Acapulco-Resort-Convention-Spa" {
		t.Errorf("slug: %s", s)
	}
}
