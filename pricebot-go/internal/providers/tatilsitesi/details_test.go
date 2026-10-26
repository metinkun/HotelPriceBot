package tatilsitesi

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func readJSON(t *testing.T, name string, out any) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func loadDetailData(t *testing.T) *detailData {
	t.Helper()
	var info hotelInfoResponse
	readJSON(t, "hotel_info.json", &info)
	var content, ages, boards widgetResponse
	readJSON(t, "hd_detail_content.json", &content)
	readJSON(t, "hd_child_ages.json", &ages)
	readJSON(t, "hd_accomodation.json", &boards)
	var c coordResponse
	readJSON(t, "hotel_coordinat.json", &c)
	return &detailData{Info: info.D.Info, Content: content.D, ChildAges: ages.D, Boards: boards.D, Lat: *c.D.InGoogleX, Lng: *c.D.InGoogleY}
}

func TestNormalizeDetails(t *testing.T) {
	d := normalizeDetails(loadDetailData(t))
	if d.Name != "Acapulco Resort Hotel & Convention & SPA" {
		t.Fatalf("ad: %q", d.Name)
	}
	if d.City != "Girne" || d.Region != "Kıbrıs" || d.Country != "Kıbrıs" {
		t.Errorf("konum alanlari: %q %q %q", d.City, d.Region, d.Country)
	}
	if d.Stars != nil {
		t.Errorf("yildiz 0 ise bos olmali: %v", *d.Stars)
	}
	if d.Location == nil || d.Location.Lat != 35.330304 || d.Location.Lng != 33.421408 {
		t.Errorf("koordinat: %+v", d.Location)
	}
	if len(d.Images) != 6 || d.Images[0].Caption != "Ana Fotoğraf" || !strings.HasSuffix(d.Images[0].URL, "genel-107751.webp") {
		t.Errorf("gorseller: %+v", d.Images)
	}
	// (inOrder, inID) sirali: 107751 (0), 107763/107764/107766/107769 (1), 107748 (2)
	if !strings.HasSuffix(d.Images[1].URL, "107763.webp") || !strings.HasSuffix(d.Images[5].URL, "107748.webp") {
		t.Errorf("gorsel sirasi: %+v", d.Images)
	}
	if !strings.Contains(d.Description, "KKTC’de Girne / Çatalköy") {
		t.Errorf("aciklama: %q", d.Description)
	}
	if len(d.FAQ) != 7 || d.FAQ[0].Question != "Acapulco Resort hangi konseptte hizmet veriyor?" || !strings.HasPrefix(d.FAQ[0].Answer, "Tesis Tam Pansiyon Plus konseptte") {
		t.Errorf("sss: %d %+v", len(d.FAQ), d.FAQ)
	}
	if len(d.Facilities) == 0 || d.Facilities[0].Title != "Öne Çıkanlar" || len(d.Facilities[0].Items) != 5 || d.Facilities[0].Items[0] != "Tam Pansiyon Plus Konsept" || d.Facilities[0].Items[4] != "1.500 m2 Spa Merkezi" {
		t.Errorf("one cikanlar: %+v", d.Facilities)
	}
	if !reflect.DeepEqual(d.Boards, []string{"Tam Pansiyon Plus", "Yarım Pansiyon"}) {
		t.Errorf("pansiyonlar: %v", d.Boards)
	}
	if d.Policies == nil || d.Policies.ChildPolicy != "1. Çocuk: 0-11,99 Yaş Ücretsiz; 2. Çocuk: 0-3 Yaş Ücretsiz" {
		t.Errorf("cocuk politikasi: %+v", d.Policies)
	}
	if d.Policies.PetPolicy != "Tesise evcil hayvan kabul edilmez." {
		t.Errorf("evcil: %q", d.Policies.PetPolicy)
	}
	titles := map[string]string{}
	for _, s := range d.Sections {
		titles[s.Title] = s.Kind
		if strings.Contains(s.Text, "<") || strings.Contains(s.Text, "font-family") {
			t.Errorf("%s: HTML/stil kalintisi: %q", s.Title, s.Text[:80])
		}
	}
	for _, want := range []string{"Hakkında", "Öne Çıkanlar", "Tesis Özellikleri", "Konum", "Yeme & İçme", "Çocuk", "Plaj & Havuz", "Odalar", "Balayı", "Ücretsiz Hizmetler", "Ücretli Hizmetler", "Notlar", "Pansiyon Tipleri", "Çocuk Yaş Politikası"} {
		if _, ok := titles[want]; !ok {
			t.Errorf("bolum eksik: %s (%v)", want, titles)
		}
	}
	if _, ok := titles["Ek Bilgiler"]; ok {
		t.Error("bos (<br>) alan bolum olmamali")
	}
	for _, s := range d.Sections {
		if s.Title == "Hakkında" && strings.Contains(s.Text, "Sık Sorulan Sorular") {
			t.Error("SSS Hakkinda metninden ayrilmali")
		}
		if s.Title == "Öne Çıkanlar" && !strings.HasPrefix(s.Text, "Denize 0 Konum") {
			t.Errorf("one cikanlar metni: %q", s.Text)
		}
	}
	if d.Rooms == nil || d.Images == nil {
		t.Error("dizi alanlari nil olmamali")
	}
}

func TestNormalizeDetailsDeterministic(t *testing.T) {
	a, _ := json.Marshal(normalizeDetails(loadDetailData(t)))
	b, _ := json.Marshal(normalizeDetails(loadDetailData(t)))
	if string(a) != string(b) {
		t.Fatal("iki calistirma farkli JSON uretti")
	}
}

func TestMatchHotels(t *testing.T) {
	var gl generalListResponse
	readJSON(t, "general_list.json", &gl)
	hotels := gl.D.Hotels

	got := matchHotels(hotels, "Merit Royal")
	names := []string{}
	for _, c := range got {
		names = append(names, c.Name)
	}
	if !reflect.DeepEqual(names, []string{"Merit Royal Gardens Hotel", "Merit Royal Premium Hotel", "Merit Royal Diamond Hotel & Spa", "Merit Royal Hotel"}) {
		t.Errorf("Merit Royal: %v", names)
	}
	got = matchHotels(hotels, "merıt royal premıum")
	if len(got) != 1 || got[0].ProviderHotelID != "MERITROYALPRM" || got[0].URL != "https://www.tatilsitesi.com/merit-royal-premium-hotel--" {
		t.Errorf("Turkce karakter: %+v", got)
	}
	got = matchHotels(hotels, "Acapulco Resort")
	if len(got) != 1 || got[0].ProviderHotelID != "ACAPULCO" || got[0].URL != "https://www.tatilsitesi.com/acapulco-resort-hotel--convention--spa" {
		t.Errorf("Acapulco: %+v", got)
	}
	// Son kelime eslesmezse sorgu kisaltilir.
	if got = matchHotels(hotels, "Elexus Palace"); len(got) != 1 || !strings.HasPrefix(got[0].Name, "Elexus") {
		t.Errorf("kisaltilmis sorgu: %+v", got)
	}
	if got = matchHotels(hotels, "Yok Boyle Otel"); len(got) != 0 || got == nil {
		t.Errorf("bos sonuc bos dizi olmali: %+v", got)
	}
}
