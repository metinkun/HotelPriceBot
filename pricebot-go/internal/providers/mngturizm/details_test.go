package mngturizm

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func loadDetailData(t *testing.T) *detailData {
	t.Helper()
	html, err := os.ReadFile("testdata/details_page.html")
	if err != nil {
		t.Fatal(err)
	}
	d := &detailData{PageHTML: string(html)}
	var cats galleryResponse[galleryCategory]
	b, _ := os.ReadFile("testdata/gallery_categories.json")
	if err := json.Unmarshal(b, &cats); err != nil {
		t.Fatal(err)
	}
	var photos galleryResponse[galleryPhoto]
	b, _ = os.ReadFile("testdata/gallery_photos.json")
	if err := json.Unmarshal(b, &photos); err != nil {
		t.Fatal(err)
	}
	d.Categories, d.Photos = cats.Sonuclar, photos.Sonuclar
	return d
}

func TestNormalizeDetails(t *testing.T) {
	d, err := normalizeDetails(loadDetailData(t))
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Acapulco Resort Hotel & Convention & SPA" {
		t.Fatalf("ad: %q", d.Name)
	}
	if d.City != "Girne" || d.Region != "Kıbrıs" {
		t.Errorf("sehir/bolge: %q / %q", d.City, d.Region)
	}
	if d.Location == nil || d.Location.Lat != 35.3302767634096 || d.Location.Lng != 33.42090671594486 {
		t.Errorf("konum: %+v", d.Location)
	}
	if d.Rating == nil || *d.Rating != 9.6 {
		t.Errorf("puan (4.8/5 -> 9.6): %v", d.Rating)
	}
	if !strings.HasPrefix(d.Description, "Acapulco Resort Hotel & Convention & SPA Catalkoy, Girne/KKTC") {
		t.Errorf("aciklama: %q", d.Description)
	}
	if len(d.Images) != 63 || d.Images[0].URL != "https://www.mngturizm.com/images/hotel/114702_t.jpg" || d.Images[0].Caption != "Genel" {
		t.Errorf("gorseller: %d %+v", len(d.Images), d.Images[:1])
	}
	last := d.Images[len(d.Images)-1]
	if last.Caption != "Odalar" || strings.Contains(last.URL, "?") {
		t.Errorf("son gorsel: %+v", last)
	}
	ftitles := []string{}
	for _, g := range d.Facilities {
		ftitles = append(ftitles, g.Title)
	}
	if !reflect.DeepEqual(ftitles, []string{"Tesis Özellikleri", "Ücretsiz Hizmetler", "Ücretli Hizmetler"}) {
		t.Errorf("ozellik gruplari: %v", ftitles)
	}
	if len(d.Facilities[0].Items) != 12 || d.Facilities[0].Items[0] != "Açık Yüzme Havuzu" || len(d.Facilities[1].Items) != 8 || len(d.Facilities[2].Items) != 13 {
		t.Errorf("ozellik sayilari: %d %d %d", len(d.Facilities[0].Items), len(d.Facilities[1].Items), len(d.Facilities[2].Items))
	}
	if d.Policies == nil || d.Policies.CheckIn != "14:00" || d.Policies.CheckOut != "12:00" || d.Policies.ChildPolicy != "0 - 11 Yaş Ücretsiz" {
		t.Errorf("politikalar: %+v", d.Policies)
	}
	if !strings.Contains(d.Policies.PetPolicy, "evcil hayvan kabul edilmemektedir") {
		t.Errorf("evcil: %q", d.Policies.PetPolicy)
	}
	if !strings.Contains(d.Policies.Cancellation, "72 saat") {
		t.Errorf("iptal: %q", d.Policies.Cancellation)
	}
	if len(d.FAQ) != 10 || d.FAQ[0].Question != "Acapulco Resort Convention Spa Hotel nerede?" || d.FAQ[0].Answer != "Tesis Girne Çatalköy mevkiindedir." {
		t.Errorf("sss: %d %+v", len(d.FAQ), d.FAQ[:1])
	}
	stitles := []string{}
	for _, s := range d.Sections {
		stitles = append(stitles, s.Title+"/"+s.Kind)
	}
	want := []string{
		"Acapulco Resort Hotel & Convention & SPA Konum/text", "Tesis Özellikleri/text", "Oda Özellikleri/text", "Standart Oda/text", "VIP Suite/text", "Villa/text",
		"Yiyecek ve İçecek/text", "Çocuk Konsepti/text", "Plaj ve Havuz/text", "Balayı/text", "Ücretsiz Hizmetler/list", "Ücretli Hizmetler/list", "Notlar/text",
		"Kampanya ve Fırsatlar/pairs", "Etkinlikler/list",
	}
	if !reflect.DeepEqual(stitles, want) {
		t.Errorf("bolumler: %v", stitles)
	}
	for _, s := range d.Sections {
		switch s.Title {
		case "Standart Oda":
			if !strings.HasPrefix(s.Text, "Halı kaplı zemin, 24 saat oda servisi") {
				t.Errorf("standart oda: %q", s.Text)
			}
		case "Kampanya ve Fırsatlar":
			if len(s.Pairs) != 4 || s.Pairs[1].Key != "Ücretsiz Transfer Kampanyası" || !strings.Contains(s.Pairs[1].Value, "14 Ekim 2026") {
				t.Errorf("kampanyalar: %+v", s.Pairs)
			}
		case "Etkinlikler":
			if len(s.Items) != 16 || !strings.HasPrefix(s.Items[0], "Alya — 31 Ekim 2026: Gala konaklama fiyatlarına dahildir.") {
				t.Errorf("etkinlikler: %d %q", len(s.Items), s.Items[0])
			}
		}
	}
	if d.Rooms == nil || len(d.Rooms) != 0 {
		t.Errorf("odalar bos dizi olmali: %v", d.Rooms)
	}
}

func TestNormalizeDetailsDeterministic(t *testing.T) {
	x, _ := normalizeDetails(loadDetailData(t))
	y, _ := normalizeDetails(loadDetailData(t))
	a, _ := json.Marshal(x)
	b, _ := json.Marshal(y)
	if string(a) != string(b) {
		t.Fatal("iki calistirma farkli JSON uretti")
	}
}

func TestNormalizeDetailsWithoutGallery(t *testing.T) {
	d := loadDetailData(t)
	d.Categories, d.Photos = nil, nil
	out, err := normalizeDetails(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Images) != 5 || out.Images[0].URL != "https://www.mngturizm.com/images/hotel/114702_b.jpg" {
		t.Errorf("sayfa gorselleri yedek olmali: %+v", out.Images)
	}
}

func TestHotelItems(t *testing.T) {
	b, err := os.ReadFile("testdata/searchjson.json")
	if err != nil {
		t.Fatal(err)
	}
	var items []searchItem
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatal(err)
	}
	items = append(items, searchItem{Label: "Girne", Category: "İlçe", CategoryID: "x"})
	hs := hotelItems(items)
	if len(hs) != 4 || hs[3].Label != "Merit Royal Premium Hotel" || hs[3].CategoryID != "WlBwa3A5SzRtZGhILzlacVZ1dm8yQT09" {
		t.Errorf("otel filtresi: %+v", hs)
	}
}

func TestAbsImage(t *testing.T) {
	if got := absImage(`\/images\/hotel\/5729_t.jpg`); got != "https://www.mngturizm.com/images/hotel/5729_t.jpg" {
		t.Errorf("abs: %s", got)
	}
	if got := absImage("/images/hotel/1.jpg?v=3"); got != "https://www.mngturizm.com/images/hotel/1.jpg" {
		t.Errorf("cache-buster: %s", got)
	}
}
