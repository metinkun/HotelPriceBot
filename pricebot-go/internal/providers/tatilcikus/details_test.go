package tatilcikus

// Statik detay ve isimle arama fixture testleri (Acapulco, Ekim 2026; kirpilmis gercek yanitlar).

import (
	"encoding/json"
	"strings"
	"testing"
)

func loadLookups(t *testing.T) *lookups {
	t.Helper()
	tags, err := parseTags(read(t, "tags_trimmed.json"))
	if err != nil {
		t.Fatal(err)
	}
	groups, err := parseTagGroups(read(t, "taggroups.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &lookups{
		Tags: tags, Groups: groups,
		Cities: map[int]string{83: "Girne"}, Districts: map[int]string{985: "Çatalköy"}, Countries: map[int]string{53: "Kıbrıs"},
	}
}

func TestBuildDetails(t *testing.T) {
	d, err := parseDetail(read(t, "detail_static.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := buildDetails(d, loadLookups(t), "https://www.tatilcikus.com/acapulco-resort-hotel-spa")

	if out.Provider != "tatilcikus" || out.ProviderHotelID != "5" || out.Name != "Acapulco Resort Hotel & Spa" {
		t.Fatalf("kimlik alanlari: %+v", out)
	}
	if out.City != "Girne" || out.Region != "Çatalköy" || out.Country != "Kıbrıs" {
		t.Errorf("konum adlari: %q %q %q", out.City, out.Region, out.Country)
	}
	if out.Rating == nil || *out.Rating != 8.9 {
		t.Errorf("puan 8.9 bekleniyordu: %v", out.Rating)
	}
	if out.Location == nil || out.Location.Lat != 35.330273 || out.Location.Lng != 33.4209 {
		t.Errorf("koordinat googleMapsUrl'den cozulmeli: %+v", out.Location)
	}
	if !strings.HasPrefix(out.Description, "Kıbrıs’ın en güzel şehirlerinden") || strings.Contains(out.Description, "<p>") || strings.Contains(out.Description, "&nbsp;") {
		t.Errorf("aciklama HTML'siz olmali: %.80q", out.Description)
	}

	// Gorseller: 12 gosterimde (1 gizli atildi), grup+id sirali, tam URL, .jpg
	if len(out.Images) != 12 {
		t.Fatalf("12 gorsel bekleniyordu: %d", len(out.Images))
	}
	want := "https://www.tatilcikus.com/images/hotels/acapulco_resort_hotel_spa-5/gallery/optimized/acapulco-resort-hotel-spa-5-20221227-043828-7431147.jpg"
	if out.Images[0].URL != want || out.Images[0].Caption != "Genel" {
		t.Errorf("ilk gorsel: %+v", out.Images[0])
	}
	if out.Images[2].Caption != "Odalar" || out.Images[11].Caption != "Aktivite" {
		t.Errorf("gorsel grup etiketleri: %q %q", out.Images[2].Caption, out.Images[11].Caption)
	}

	// Ozellikler: yalnizca "Genel" grubu, gizli tag'ler (Yemekler Acik Bufe, Acik Restoran, Bar) atilir, ucretli eki
	if len(out.Facilities) != 1 || out.Facilities[0].Title != "Genel" {
		t.Fatalf("1 ozellik grubu (Genel) bekleniyordu: %+v", out.Facilities)
	}
	items := out.Facilities[0].Items
	if len(items) != 5 || items[0] != "Araba Kiralama Servisi (Ücretli)" || items[4] != "Emanet Kasa" {
		t.Errorf("Genel ozellikleri beklenmedik: %v", items)
	}

	// Odalar: 2 gosterimde; Villa Oda m2/kapasite/ozellik/gorsel
	if len(out.Rooms) != 2 || out.Rooms[0].Name != "Villa Oda" || out.Rooms[1].Name != "Standart Oda" {
		t.Fatalf("odalar: %+v", out.Rooms)
	}
	r := out.Rooms[0]
	if r.Size == nil || *r.Size != 30 {
		t.Errorf("Villa Oda 30 m2 bekleniyordu: %v", r.Size)
	}
	if r.Capacity != "3 yetişkin + 1 çocuk" {
		t.Errorf("kapasite: %q", r.Capacity)
	}
	if len(r.Features) != 3 || r.Features[0] != "Mini Bar (Ücretli)" { // Langirt tag'i gizli (isShown=false)
		t.Errorf("oda ozellikleri: %v", r.Features)
	}
	if len(r.Images) != 2 || !strings.HasSuffix(r.Images[0], ".jpg") {
		t.Errorf("oda gorselleri: %v", r.Images)
	}
	if len(out.Rooms[1].Images) != 0 || out.Rooms[1].Images == nil {
		t.Errorf("gorselsiz oda icin bos dizi bekleniyordu: %#v", out.Rooms[1].Images)
	}

	if len(out.Boards) != 2 || out.Boards[0] != "Tam Pansiyon Plus" {
		t.Errorf("pansiyonlar: %v", out.Boards)
	}
	if out.Policies == nil || !strings.Contains(out.Policies.ChildPolicy, "Pam Club") || len(out.Policies.Other) != 3 {
		t.Errorf("politikalar: %+v", out.Policies)
	}

	// Bolumler: sitenin basliklari, sirasi sabit
	titles := []string{}
	kinds := map[string]string{}
	for _, s := range out.Sections {
		titles = append(titles, s.Title)
		kinds[s.Title] = s.Kind
	}
	wantTitles := []string{"Öne Çıkanlar", "Acapulco Resort Hotel & Spa Hakkında", "Plaj & Havuz", "Spa", "Aktivite", "Balayı", "Çocuk", "Çocuk Bilgileri", "Balayı Tesisi", "Çağrı Merkezi Bilgilendirmesi", "Pansiyon: Tam Pansiyon Plus", "Pansiyon: Yarım Pansiyon", "Tripadvisor", "Video"}
	if strings.Join(titles, "|") != strings.Join(wantTitles, "|") {
		t.Errorf("bolum basliklari:\n got  %v\n want %v", titles, wantTitles)
	}
	if kinds["Plaj & Havuz"] != "list" || kinds["Acapulco Resort Hotel & Spa Hakkında"] != "text" || kinds["Pansiyon: Tam Pansiyon Plus"] != "html" || kinds["Tripadvisor"] != "pairs" {
		t.Errorf("bolum turleri: %v", kinds)
	}
	for _, s := range out.Sections {
		if s.Kind == "html" && (strings.Contains(s.HTML, "class=") || strings.Contains(s.HTML, "&nbsp;") || !strings.Contains(s.HTML, "<table>")) {
			t.Errorf("html bolumu temiz degil: %.200q", s.HTML)
		}
	}
	if out.FAQ == nil || len(out.FAQ) != 0 {
		t.Errorf("FAQ bos dizi olmali: %#v", out.FAQ)
	}

	// Determinizm: ayni girdi -> ayni JSON
	a, _ := json.Marshal(out)
	b, _ := json.Marshal(buildDetails(d, loadLookups(t), "https://www.tatilcikus.com/acapulco-resort-hotel-spa"))
	if string(a) != string(b) {
		t.Error("buildDetails deterministik degil")
	}
}

func TestMatchHotels(t *testing.T) {
	list, err := parseSummary(read(t, "summary_trimmed.json"))
	if err != nil {
		t.Fatal(err)
	}
	countries := map[int]string{53: "Kıbrıs", 219: "Türkiye"}
	cases := map[string][]string{
		"Merit Royal Premium": {"Merit Royal Premium Hotel & Spa"},
		"elexus":              {"Elexus Hotel Resort & Spa"},
		"ACAPULCO":            {"Acapulco Resort Hotel & Spa"},
		"Cratos Premium":      {"Cratos Premium Hotel"},
		"Salamis Bay Conti":   {"Salamis Bay Conti Hotel"},
		"merıt royal":         {"Merit Royal Diamond Hotel & Spa", "Merit Royal Gardens Hotel", "Merit Royal Hotel & Spa", "Merit Royal Premium Hotel & Spa"},
		"lefkoşa":             {"Merit Lefkoşa Hotel"},
		"yok boyle":           {},
	}
	for q, want := range cases {
		got := matchHotels(list, q, countries)
		names := []string{}
		for _, c := range got {
			names = append(names, c.Name)
		}
		if strings.Join(names, "|") != strings.Join(want, "|") {
			t.Errorf("%q: got %v want %v", q, names, want)
		}
	}
	got := matchHotels(list, "Acapulco", countries)
	if got[0].URL != "https://www.tatilcikus.com/acapulco-resort-hotel-spa" || got[0].ProviderHotelID != "5" || got[0].City != "Kıbrıs" {
		t.Errorf("aday alanlari: %+v", got[0])
	}
}

func TestFoldTR(t *testing.T) {
	if foldTR("Merit Royal & Spa, ÇİĞDEM Işık") != "merit royal spa cigdem isik" {
		t.Errorf("foldTR: %q", foldTR("Merit Royal & Spa, ÇİĞDEM Işık"))
	}
}

func TestHTMLHelpers(t *testing.T) {
	raw := `<p>Giriş <strong>15:00</strong>&nbsp;</p><ul><li>Bir</li><li>İki &amp; üç</li></ul><!-- yorum --><script>x()</script>`
	if got := htmlText(raw); got != "Giriş 15:00\n- Bir\n- İki & üç" {
		t.Errorf("htmlText: %q", got)
	}
	if got := htmlListItems(raw); len(got) != 2 || got[1] != "İki & üç" {
		t.Errorf("htmlListItems: %v", got)
	}
	s, ok := sectionFromHTML("X", `<ul><li>a</li><li>b</li></ul>`)
	if !ok || s.Kind != "list" || len(s.Items) != 2 {
		t.Errorf("list bolumu: %+v", s)
	}
	s, _ = sectionFromHTML("T", `<figure class="table"><table><tbody><tr><td rowspan="2">A</td><td>B</td></tr></tbody></table></figure>`)
	if s.Kind != "html" || s.HTML != `<figure><table><tbody><tr><td rowspan="2">A</td><td>B</td></tr></tbody></table></figure>` || s.Text != "A | B" {
		t.Errorf("html bolumu: %+v", s)
	}
	if _, ok := sectionFromHTML("E", "<p>&nbsp;</p>"); ok {
		t.Error("bos icerik bolum olmamali")
	}
}
