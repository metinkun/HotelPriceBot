package tatilsepeti

// Statik detay ve isimle arama fixture testleri (Acapulco 229, Ekim 2026; detay HTML kirpilmis, yorumcu adlari cikarildi).

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseDetails(t *testing.T) {
	html := string(readFixture(t, "detail_page.html"))
	out, roomIDs, err := parseDetails(html, "https://www.tatilsepeti.com/acapulco-resort-convention-spa-hotel")
	if err != nil {
		t.Fatal(err)
	}
	if out.Provider != "tatilsepeti" || out.ProviderHotelID != "229" || out.Name != "Acapulco Resort Convention Spa Hotel" {
		t.Fatalf("kimlik: %+v", out)
	}
	if out.Country != "Kıbrıs" || out.City != "Girne" || out.Region != "Çatalköy" || out.Address != "Kıbrıs-Girne-Çatalköy" {
		t.Errorf("konum: %q %q %q %q", out.Country, out.City, out.Region, out.Address)
	}
	if out.Rating == nil || *out.Rating != 8.4 || out.RatingCount == nil || *out.RatingCount != 1786 {
		t.Errorf("puan 8.4 / 1786: %v %v", out.Rating, out.RatingCount)
	}
	if !strings.HasPrefix(out.Description, "Kıbrıs, Girne bölgesinde bulunan") || strings.Contains(out.Description, "<") {
		t.Errorf("aciklama: %.80q", out.Description)
	}
	if len(out.Images) < 10 || !strings.HasPrefix(out.Images[0].URL, "https://cdn.tatilsepeti.com/Files/Images/Tesis/00229/1050X700/") || out.Images[0].Caption != "Acapulco Resort Convention Spa Hotel" {
		t.Errorf("galeri: %d %+v", len(out.Images), out.Images[:1])
	}
	for _, im := range out.Images {
		if strings.Contains(im.URL, "?") {
			t.Errorf("gorsel URL'inde sorgu kalmis: %s", im.URL)
		}
	}
	if len(out.Boards) != 1 || out.Boards[0] != "Tam Pansiyon Plus" {
		t.Errorf("pansiyon: %v", out.Boards)
	}

	ft := []string{}
	for _, f := range out.Facilities {
		ft = append(ft, f.Title)
	}
	if strings.Join(ft, "|") != "Otel Özellikleri|Ücretsiz Aktiviteler|Ücretli Aktiviteler|Çocuklar İçin" {
		t.Fatalf("ozellik gruplari: %v", ft)
	}
	if items := out.Facilities[0].Items; len(items) != 15 || items[0] != "A la Carte Restoran (4 Adet)" {
		t.Errorf("Otel Ozellikleri: %v", items)
	}
	if items := out.Facilities[1].Items; len(items) != 13 || items[0] != "Otopark" {
		t.Errorf("Ucretsiz: %v", items)
	}
	if items := strings.Join(out.Facilities[3].Items, "|"); items != "Çocuk Kulubü (Ücretsiz)|Çocuk Parkı (Ücretsiz)|Çocuk Havuzu (Ücretsiz)|Bebek Bakımı (Ücretli)" {
		t.Errorf("Cocuklar Icin: %v", items)
	}

	if len(out.Rooms) < 3 || len(roomIDs) != len(out.Rooms) || roomIDs[0] != "25" {
		t.Fatalf("odalar: %d ids=%v", len(out.Rooms), roomIDs)
	}
	r := out.Rooms[0]
	if r.Name != "Villa Oda" || r.Size == nil || *r.Size != 30 || r.Capacity != "En fazla 4 yetişkin + 1 çocuk veya 3 yetişkin + 2 çocuk." {
		t.Errorf("Villa Oda: %+v", r)
	}
	if len(r.Features) < 10 || r.Features[0] != "Oda genişliği 30 - 32 m2" || !containsStr(r.Features, "Saç Kurutma Makinesi") {
		t.Errorf("Villa Oda ozellikleri: %v", r.Features)
	}
	if len(r.Images) != 1 || !strings.HasPrefix(r.Images[0], "https://cdn.tatilsepeti.com/Files/Images/TesisOda/00229/") {
		t.Errorf("Villa Oda kart gorseli: %v", r.Images)
	}
	if out.Rooms[1].Name != "Garden Suit" || out.Rooms[2].Name != "Palm Suit" {
		t.Errorf("oda sirasi: %q %q", out.Rooms[1].Name, out.Rooms[2].Name)
	}

	p := out.Policies
	if p == nil || p.CheckIn != "14:00" || p.CheckOut != "11:00" || p.PetPolicy != "Evcil hayvan kabul edilmemektedir." || !strings.Contains(p.ChildPolicy, "çocuk kulübü") {
		t.Errorf("politikalar: %+v", p)
	}

	st := []string{}
	kinds := map[string]string{}
	for _, s := range out.Sections {
		st = append(st, s.Title)
		kinds[s.Title] = s.Kind
	}
	joined := strings.Join(st, "|")
	wantPrefix := "Acapulco Resort Convention Spa Hotel Genel Bilgiler|Konum Bilgileri|Otel Özellikleri|Otel Özellikleri (Liste)|Ücretsiz Aktiviteler|Ücretli Aktiviteler|Çocuklar İçin|Diğer Notlar: Plaj Bilgisi|Diğer Notlar: Havuz Bilgisi"
	if !strings.HasPrefix(joined, wantPrefix) || !strings.HasSuffix(joined, "Acapulco Resort Convention Spa Hotel Yorumları") {
		t.Errorf("bolumler: %v", st)
	}
	if kinds["Konum Bilgileri"] != "pairs" || kinds["Otel Özellikleri"] != "pairs" || kinds["Diğer Notlar: Plaj Bilgisi"] != "list" {
		t.Errorf("bolum turleri: %v", kinds)
	}
	for _, s := range out.Sections {
		switch s.Title {
		case "Konum Bilgileri":
			if len(s.Pairs) != 8 || s.Pairs[0].Key != "Hava Alanına Uzaklığı" || s.Pairs[0].Value != "Ercan HA 30 km" || s.Pairs[3].Value != "Özel Plaj" {
				t.Errorf("konum ciftleri: %+v", s.Pairs)
			}
		case "Otel Özellikleri":
			if len(s.Pairs) != 4 || s.Pairs[0].Key != "Otel Yapım Yılı" || s.Pairs[0].Value != "1982" {
				t.Errorf("otel ozellik ciftleri: %+v", s.Pairs)
			}
		case "Acapulco Resort Convention Spa Hotel Yorumları":
			if len(s.Pairs) < 5 || s.Pairs[0].Key != "Tatilsepeti Puanı" || s.Pairs[0].Value != "8.4" || s.Pairs[1].Key != "Değerlendirme" || s.Pairs[1].Value != "801" {
				t.Errorf("yorum ciftleri: %+v", s.Pairs)
			}
		}
	}
	if out.FAQ == nil || len(out.FAQ) != 0 {
		t.Errorf("FAQ bos dizi: %#v", out.FAQ)
	}

	a, _ := json.Marshal(out)
	out2, _, _ := parseDetails(html, "https://www.tatilsepeti.com/acapulco-resort-convention-spa-hotel")
	b, _ := json.Marshal(out2)
	if string(a) != string(b) {
		t.Error("parseDetails deterministik degil")
	}
}

func TestRoomImagesFromHTML(t *testing.T) {
	frag := `<div class="item"><img data-src="https://cdn.tatilsepeti.com/Files/Images/TesisOda/00229/1050X700/a.jpg?v=1" /></div>` +
		`<img src="https://cdn.tatilsepeti.com/Files/Images/TesisOda/00229/1050X700/b.jpg" /><img src="https://cdn.tatilsepeti.com/Files/Images/TesisOda/00229/1050X700/a.jpg" />`
	got := roomImagesFromHTML(frag)
	if len(got) != 2 || got[0] != "https://cdn.tatilsepeti.com/Files/Images/TesisOda/00229/1050X700/a.jpg" {
		t.Errorf("oda gorselleri: %v", got)
	}
}

func TestParseSearch(t *testing.T) {
	got, err := parseSearch(readFixture(t, "hotelsearch.json"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, c := range got {
		names = append(names, c.Name)
		if c.URL == "" || !strings.HasPrefix(c.URL, "https://www.tatilsepeti.com/") || strings.Contains(c.URL, "yurtdisi") || c.ProviderHotelID == "" {
			t.Errorf("aday: %+v", c)
		}
	}
	if len(got) != 9 { // 8 yurt ici "merit royal" + 1 "elexus"; bolge (TypeId 2/5) ve yurt disi oteller (6) atilir
		t.Fatalf("9 aday bekleniyordu (%d): %v", len(got), names)
	}
	c := got[0]
	if c.Name != "Merit Royal Hotel Spa" || c.URL != "https://www.tatilsepeti.com/merit-royal-hotel-spa" || c.City != "Alsancak, Girne, Kıbrıs" || c.ProviderHotelID != "4022" {
		t.Errorf("ilk aday: %+v", c)
	}
	if got[8].Name != "Elexus Hotel Resort" || got[8].ProviderHotelID != "5574" {
		t.Errorf("elexus: %+v", got[8])
	}
}

func TestRankCandidates(t *testing.T) {
	got, _ := parseSearch(readFixture(t, "hotelsearch.json"))
	ranked := rankCandidates(got, "merıt royal")
	if ranked[0].Name != "Merit Royal Hotel Spa" || ranked[3].Name != "Merit Royal Diamond Hotel Spa" || ranked[4].Name != "Royal Taj Mahal" {
		t.Errorf("siralama: %v", ranked)
	}
}

func TestTitleCaseTR(t *testing.T) {
	if titleCaseTR("tam pansiyon plus") != "Tam Pansiyon Plus" || titleCaseTR("her şey dahil") != "Her Şey Dahil" || titleCaseTR("iki") != "İki" {
		t.Error(titleCaseTR("tam pansiyon plus"), titleCaseTR("her şey dahil"), titleCaseTR("iki"))
	}
}
