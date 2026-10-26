package services

import (
	"encoding/json"
	"os"
	"testing"
)

// Tum KKTC otel adlari normalize edilebilmeli (eski transform.Chain bazi adlarda panikliyordu).
func TestNormNameAllTRNC(t *testing.T) {
	b, err := os.ReadFile("../../data/hotels-trnc.json")
	if err != nil {
		t.Skip("liste yok")
	}
	var hotels []struct{ Name string }
	if err := json.Unmarshal(b, &hotels); err != nil {
		t.Fatal(err)
	}
	for _, h := range hotels {
		if normName(h.Name) == "" {
			t.Errorf("bos normalize: %q", h.Name)
		}
		_ = searchQuery(h.Name)
		_ = shortQuery(h.Name)
	}
	for _, s := range []string{"ARDIÇ AĞACI TATIL KÖYÜ", "MERYEM`S HOTEL", "OLİVİA PALM HOTEL", "CHAMADA PRESTİGE HOTEL CASINO & SPA", "İ", "ı", ""} {
		_ = normName(s)
	}
}

func TestScore(t *testing.T) {
	if s := score("MERIT CRYSTAL COVE HOTEL & CASINO", "Merit Crystal Cove Hotel"); s < 0.9 {
		t.Errorf("yuksek skor bekleniyordu: %v", s)
	}
	if s := score("PORT VIEW HOTEL", "Cratos Premium Hotel & Port & Spa"); s >= 0.6 {
		t.Errorf("dusuk skor bekleniyordu: %v", s)
	}
	if s := score("ARDIÇ AĞACI TATIL KÖYÜ", "Ardıç Ağacı Tatil Köyü"); s < 0.99 {
		t.Errorf("Turkce karakter farki skoru dusurmemeli: %v", s)
	}
}

func TestScoreStrict(t *testing.T) {
	accept := [][2]string{
		{"MERIT ROYAL PREMIUM HOTEL & CASINO", "Merit Royal Premium Hotel & Spa"},
		{"DORANA HOTEL", "Dorana Hotel Girne"},
		{"ROCKS HOTEL & CASINO", "Rocks Hotel"},
		{"DOME HOTEL & CASINO", "Dome Hotel"},
		{"CRATOS PREMIUM HOTEL & CASINO & SPA", "Cratos Premium Hotel & Port & Spa"},
		{"THE COLONY ARKIN GROUP HOTEL & CASINO", "The Arkın Colony Hotel"},
		{"UPTOWN BUYUK ANADOLU GIRNE HOTEL", "Uptown Büyük Anadolu Hotel Girne"},
		{"PARK PALACE HOTEL KYRENIA", "Hotel Park Palace"},
		{"GRAND CENTER BOUTIQUE HOTEL", "Grand Center Butik Hotel"},
		{"MOUNTAIN VIEW HOTEL & VILLAS", "Mountain View Hotel"},
		{"ACAPULCO RESORT CONVENTION SPA", "Acapulco Resort Hotel"},
		{"ALTINKAYA HOLIDAY VILLAGE", "Altınkaya Tatil Köyü"},
		{"ELEXUS HOTEL RESORT SPA CASINO", "Elexus Hotel Resort"},
		{"GRAND PASHA KYRENIA HOTEL & CASINO & SPA", "Grand Pasha Kyrenia Hotel & Spa"},
		{"MERYEM`S HOTEL", "Meryem's Otel"},
		{"NOYANLAR HOLIDAYS HOMES", "Noyanlar Holiday Homes"},
		{"PREMIUM INN BOUTIQUE HOTEL", "Premium Inn Hotel"},
		{"NOVEL CENTRE POINT INN HOTEL", "Novel Centre Point Hotel"},
		{"BELLA VIEW ART BOUTIQUE HOTEL", "Bella View Art Botique Hotel"},
		{"GILLHAM VINEYARD HOTEL", "Gillham Vineyard Hotel - Adults Only"},
		{"OLİVİA PALM HOTEL", "Olivia Palm Butik Hotel"},
		{"MERIT ROYAL DIAMOND & SPA", "Merit Royal Diamond Hotel & Spa +14"},
	}
	reject := [][2]string{
		{"DORANA RESİDANCE", "Dorana Hotel"},
		{"ROYAL PALACE HOTEL", "Royal Alhambra Palace"},
		{"ADA BEACH HOTEL", "Kleopatra Ada Beach Hotel"},
		{"ADA BEACH HOTEL", "Bodrum Ada Suites Beach Hotel"},
		{"BLUE SEA KARPAZ", "Protaras Blue Sea Apartment"},
		{"EDELWEISS HOLIDAY RESIDENCE", "Hotel Edelweiss"},
		{"MERIT ROYAL PREMIUM HOTEL & CASINO", "Merit Royal Hotel & Spa"},
		{"OSCAR RESORT HOTEL & CASINO", "Oscar Park Hotel"},
		{"LIFE HOTEL", "Life Point Hotel"},
		{"LIFE HOTEL", "Ak Life Hotel Suite"},
		{"ADA BEACH HOTEL", "Ada Life Otel"},
		{"GRAND PASHA KYRENIA HOTEL & CASINO & SPA", "Grand Pasha Lefkoşa Hotel & Spa"},
		{"DENIZKIZI ROYAL HOTEL", "Denizkızı Hotel"},
		{"DENIZKIZI HOTEL", "Denizkızı Sevilay Pansiyon"},
		{"DENIZKIZI HOTEL", "Denizkızı Royal Hotel"},
		{"SEA LIFE LONG BEACH HOTEL", "Sea Life Family Resort Hotel"},
		{"SEA LIFE LONG BEACH HOTEL", "Life Hotel"},
		{"DEL MARE PARK", "Holiday Park Resort"},
		{"BELLA VIEW ART BOUTIQUE HOTEL", "Bella Hotel"},
		{"PORT VIEW HOTEL", "Cratos Premium Hotel & Port & Spa"},
	}
	for _, c := range accept {
		if s := score(c[0], c[1]); s < 0.6 {
			t.Errorf("kabul bekleniyordu %q ~ %q: %.2f", c[0], c[1], s)
		}
	}
	for _, c := range reject {
		if s := score(c[0], c[1]); s >= 0.6 {
			t.Errorf("red bekleniyordu %q ~ %q: %.2f", c[0], c[1], s)
		}
	}
}

func TestCityConflict(t *testing.T) {
	if !cityConflict("GRAND SAPPHIRE RESORT ISKELE", "Grand Sapphire City Hotel", "Gazimağusa") {
		t.Error("Iskele vs Gazimagusa catisma bekleniyordu")
	}
	if cityConflict("GRAND SAPPHIRE RESORT ISKELE", "Grand Sapphire Resort", "Yeni İskele") || cityConflict("DORANA HOTEL", "Dorana Hotel", "Gazimağusa") || cityConflict("GRAND SAPPHIRE RESORT ISKELE", "Grand Sapphire Resort", "") {
		t.Error("catisma beklenmiyordu")
	}
}

func TestCityAllowed(t *testing.T) {
	for _, c := range []string{"", "Girne", "Girne, Kıbrıs", "Lefkoşa", "Gazimagosa", "Kıbrıs Magosa", "Kyrenia, Kıbrıs", "Merkez", "Lower Kyrenia, Girne"} {
		if !cityAllowed(c, "TRNC") {
			t.Errorf("izin bekleniyordu: %q", c)
		}
	}
	for _, c := range []string{"Muğla", "Alanya Merkez, Alanya", "Belek, Serik, Antalya, Türkiye", "Milano, İtalya", "Bodrum", "Göynük, Kemer, Antalya", "Turgutreis", "Kapparis, Paralimni", "Cenevre, İsviçre", "Çolaklı"} {
		if cityAllowed(c, "TRNC") {
			t.Errorf("red bekleniyordu: %q", c)
		}
		if !cityAllowed(c, "") {
			t.Errorf("ulke yoksa filtre olmamali: %q", c)
		}
	}
}

func TestDisplayName(t *testing.T) {
	cases := map[string]string{
		"MERIT CRYSTAL COVE HOTEL & CASINO": "Merit Crystal Cove Hotel & Casino",
		"MERYEM`S HOTEL":                    "Meryem's Hotel",
		"Dorana Hotel":                      "Dorana Hotel",
		"THE ARKIN ISKELE HOTEL & CASINO":   "The Arkin Iskele Hotel & Casino",
	}
	for in, want := range cases {
		if got := displayName(in); got != want {
			t.Errorf("displayName(%q) = %q, bekleniyordu %q", in, got, want)
		}
	}
}
