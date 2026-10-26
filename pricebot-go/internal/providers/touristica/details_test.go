package touristica

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("fixture okunamadi: %v", err)
	}
	return string(b)
}

func TestParseDetailsFixture(t *testing.T) {
	page := readFixture(t, "detail_page.html")
	rooms := readFixture(t, "odalar.html")
	d, err := parseDetails(page, rooms)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Acapulco Resort Convention Spa" {
		t.Errorf("ad: %q", d.Name)
	}
	if d.City != "Girne" || d.Country != "Kıbrıs" || !strings.Contains(d.Address, "Çatalköy") {
		t.Errorf("adres/sehir/ulke: %q / %q / %q", d.Address, d.City, d.Country)
	}
	if d.Region != "" {
		t.Errorf("bolge ulkeyle ayni oldugundan bos olmali: %q", d.Region)
	}
	if d.Rating == nil || *d.Rating != 8.1 || d.RatingCount == nil || *d.RatingCount != 621 {
		t.Errorf("puan: %v / %v", d.Rating, d.RatingCount)
	}
	if d.Stars != nil {
		t.Errorf("site yildiz gostermiyor, nil beklenir: %v", *d.Stars)
	}
	if d.Location == nil || d.Location.Lat != 35.33029 || d.Location.Lng != 33.420895 {
		t.Errorf("konum: %+v", d.Location)
	}
	if !strings.HasPrefix(d.Description, "Yeşil ve mavinin") {
		t.Errorf("aciklama Genel Tanitim'dan gelmeli: %.60q", d.Description)
	}

	// Gorseller: gizli slider, kategori adi caption; boyut segmenti yok; video yok
	if len(d.Images) != 4 {
		t.Fatalf("4 gorsel bekleniyordu, %d", len(d.Images))
	}
	want := "https://cdnturint.touristica.com.tr/otel-resimleri/v1.00/acapulco-resort-convention-spa/acapulco-resort-genel_341768.jpg"
	if d.Images[0].URL != want || d.Images[0].Caption != "Genel Görünüm" {
		t.Errorf("ilk gorsel: %+v", d.Images[0])
	}
	for _, im := range d.Images {
		if strings.Contains(im.URL, "youtube") || strings.Contains(im.URL, "x450/") || strings.Contains(im.URL, "?") {
			t.Errorf("gorsel URL temiz degil: %s", im.URL)
		}
	}

	// Ozellik gruplari: sitenin 3 grubu; "Otel Özellikleri" ust etiketlerle birlesir
	if len(d.Facilities) != 3 {
		t.Fatalf("3 ozellik grubu bekleniyordu: %d", len(d.Facilities))
	}
	if d.Facilities[0].Title != "Ücretsiz Aktiviteler" || len(d.Facilities[0].Items) != 21 {
		t.Errorf("ucretsiz aktiviteler: %s %d", d.Facilities[0].Title, len(d.Facilities[0].Items))
	}
	feat := d.Facilities[2]
	if feat.Title != "Otel Özellikleri" || !contains(feat.Items, "Kapalı Havuz") || !contains(feat.Items, "Doğa Oteli") || !contains(feat.Items, "Balayı Oteli") {
		t.Errorf("otel ozellikleri birlesmedi: %+v", feat)
	}
	if countOf(feat.Items, "Aquapark") != 1 {
		t.Errorf("tekrarli ozellik: %v", feat.Items)
	}

	// Odalar: ad bazinda tekil (ayni oda iki pansiyonla gelir), m2, kapasite, ozellik, gorsel
	if len(d.Rooms) != 2 {
		t.Fatalf("2 oda bekleniyordu: %d", len(d.Rooms))
	}
	r := d.Rooms[0]
	if r.Name != "Bungalow Oda" || r.Size == nil || *r.Size != 30 || !strings.HasPrefix(r.Capacity, "En fazla 3 Yetişkin") {
		t.Errorf("oda: %+v", r)
	}
	if len(r.Features) != 10 || r.Features[0] != "Kasa" || len(r.Images) != 4 {
		t.Errorf("oda ozellik/gorsel: %d / %d", len(r.Features), len(r.Images))
	}
	if r.Images[0] != "https://cdnturint.touristica.com.tr/otel-resimleri/v1.00/acapulco-resort-convention-spa/acapulco-resort-oda-bung_341924.jpg" {
		t.Errorf("oda gorseli boyut segmenti temizlenmedi: %s", r.Images[0])
	}
	if d.Rooms[1].Name != "Standart Otel Odası" {
		t.Errorf("ikinci oda: %q", d.Rooms[1].Name)
	}
	if !reflect.DeepEqual(d.Boards, []string{"Tam Pansiyon Plus", "Yarım Pansiyon"}) {
		t.Errorf("pansiyon tipleri: %v", d.Boards)
	}

	// Politikalar: Genel Uyarilar'dan giris/cikis saati, iptal kampanyasi
	if d.Policies == nil || d.Policies.CheckIn != "15:00" || d.Policies.CheckOut != "11:00" {
		t.Fatalf("giris/cikis: %+v", d.Policies)
	}
	if len(d.Policies.Other) < 5 || !strings.HasPrefix(d.Policies.Other[1], "Otele giriş esnasında") {
		t.Errorf("genel uyarilar: %v", d.Policies.Other)
	}

	// Bolumler: sitenin basliklari sirayla; liste/metin ayrimi; kampanya ve etkinlik pairs
	titles := []string{}
	byTitle := map[string]int{}
	for i, s := range d.Sections {
		titles = append(titles, s.Title)
		byTitle[s.Title] = i
	}
	wantTitles := []string{"Genel Tanıtım", "Konum", "Konaklama Tipi", "Havuz ve Plaj", "Animasyon ve Aktiviteler", "Çocuk", "Balayı Ayrıcalıkları",
		"Spa Merkezi Özellikleri", "Genel Uyarılar", "Tesis Kapasitesi", "Ücretsiz Aktiviteler", "Ücretli Aktiviteler", "Otel Özellikleri", "Kampanyalar", "Etkinlik Takvimi", "Yorumlar"}
	if !reflect.DeepEqual(titles, wantTitles) {
		t.Errorf("bolum basliklari: %v", titles)
	}
	if s := d.Sections[byTitle["Havuz ve Plaj"]]; s.Kind != "list" || len(s.Items) != 4 || !strings.HasPrefix(s.Items[0], "Denize sıfır") {
		t.Errorf("havuz ve plaj listesi: %+v", s)
	}
	if s := d.Sections[byTitle["Konaklama Tipi"]]; s.Kind != "text" || !strings.HasPrefix(s.Text, "Tam Pansiyon Plus\nSunset") || strings.Contains(s.Text, "<") {
		t.Errorf("konaklama tipi metni: %.80q", s.Text)
	}
	if s := d.Sections[byTitle["Kampanyalar"]]; s.Kind != "pairs" || len(s.Pairs) != 2 || s.Pairs[0].Key != "Touristica Öneriyor" || !strings.Contains(s.Pairs[0].Value, "Katılım Tarih Aralığı") {
		t.Errorf("kampanyalar: %+v", s)
	}
	if s := d.Sections[byTitle["Etkinlik Takvimi"]]; s.Kind != "pairs" || len(s.Pairs) != 2 || s.Pairs[0].Key != "31 EKİM 2026 - Alya" {
		t.Errorf("etkinlikler: %+v", s)
	}
	if s := d.Sections[byTitle["Yorumlar"]]; s.Kind != "pairs" || len(s.Pairs) == 0 || s.Pairs[0].Key != "Konum" || s.Pairs[0].Value != "8.6" {
		t.Errorf("yorum puanlari: %+v", s)
	}
	if d.FAQ == nil || len(d.FAQ) != 0 {
		t.Errorf("FAQ bos dizi olmali: %v", d.FAQ)
	}

	// Deterministik: ayni girdi ayni JSON
	a, _ := json.Marshal(d)
	d2, _ := parseDetails(page, rooms)
	b, _ := json.Marshal(d2)
	if string(a) != string(b) {
		t.Error("ayni girdi farkli cikti uretti")
	}
}

func TestParseAutocomplete(t *testing.T) {
	c := parseAutocomplete(readFixture(t, "autocomplete_merit_roy.html"))
	if len(c) != 4 {
		t.Fatalf("4 otel bekleniyordu: %+v", c)
	}
	if c[0].Name != "Merit Royal Diamond Hotel & Spa" || c[0].URL != "https://www.touristica.com.tr/merit-royal-diamond-hotel-spa" || c[0].ProviderHotelID != "merit-royal-diamond-hotel-spa" {
		t.Errorf("ilk aday: %+v", c[0])
	}
	if c[3].Name != "Merit Royal Premium Hotel" {
		t.Errorf("son aday: %+v", c[3])
	}
	for _, x := range c {
		if strings.HasSuffix(x.URL, "-otelleri") {
			t.Errorf("kategori sonucu sizdi: %+v", x)
		}
	}
	// Genis sorgu: KATEGORİLER (merit-otelleri) atlanir
	wide := strings.Replace(readFixture(t, "autocomplete_merit_roy.html"), "<h3>TÜM OTELLER</h3>",
		`<h3>KATEGORİLER</h3><ul><li><a href="javascript:" data-link='merit-otelleri' class="auto-complete-result-box-item">Merit Otelleri</a></li></ul><h3>TÜM OTELLER</h3>`, 1)
	if w := parseAutocomplete(wide); len(w) != 4 {
		t.Errorf("kategori atlanmali: %+v", w)
	}
	if e := parseAutocomplete(readFixture(t, "autocomplete_elexus.html")); len(e) == 0 || !strings.Contains(e[0].Name, "Elexus") {
		t.Errorf("elexus: %+v", e)
	}
	if e := parseAutocomplete(readFixture(t, "autocomplete_cratos.html")); len(e) == 0 || !strings.Contains(e[0].Name, "Cratos") {
		t.Errorf("cratos: %+v", e)
	}
	if e := parseAutocomplete(`<div class="auto-complete-result-box hotel"><div class="scroll-wrapper"></div></div>`); e == nil || len(e) != 0 {
		t.Errorf("bos sonuc bos dizi olmali: %v", e)
	}
}

func TestSearchAttempts(t *testing.T) {
	got := searchAttempts("Merit Royal Premium")
	want := []string{"Merit Royal Premium", "Merit Royal", "Merit"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("denemeler: %v", got)
	}
	got = searchAttempts("Kaya Artemis Şato")
	if got[1] != "Kaya Artemis Sato" {
		t.Errorf("ASCII indirgeme: %v", got)
	}
}

func TestHelpers(t *testing.T) {
	if u := normalizeImageURL("https://cdn.x/otel/870x575/a_1.jpg?v=3"); u != "https://cdn.x/otel/a_1.jpg" {
		t.Errorf("gorsel URL: %s", u)
	}
	if u := normalizeImageURL("data:image/png;base64,xx"); u != "" {
		t.Errorf("data URI atlanmali: %s", u)
	}
	lines := htmlToLines(`<li>Bir &amp; iki</li><br><li>Üç</li>`)
	if !reflect.DeepEqual(lines, []string{"• Bir & iki", "• Üç"}) {
		t.Errorf("satirlar: %q", lines)
	}
	if s := linesToSection("X", lines); s.Kind != "list" || len(s.Items) != 2 || s.Items[0] != "Bir & iki" {
		t.Errorf("liste bolumu: %+v", s)
	}
	if s := linesToSection("X", []string{"metin", "• madde"}); s.Kind != "text" || s.Text != "metin\n• madde" {
		t.Errorf("karisik bolum: %+v", s)
	}
	if timeOf("saat 15.00 'de") != "15:00" || timeOf("10.000 TL") != "" {
		t.Error("saat ayrimi")
	}
	if m := reCheckIn.FindStringSubmatch("Otele giriş esnasında oda başı 10.000 TL ön provizyon alınacaktır."); m != nil {
		t.Errorf("para tutari saat sanilmamali: %v", m)
	}
}

func contains(xs []string, s string) bool { return countOf(xs, s) > 0 }

func countOf(xs []string, s string) int {
	n := 0
	for _, x := range xs {
		if x == s {
			n++
		}
	}
	return n
}
