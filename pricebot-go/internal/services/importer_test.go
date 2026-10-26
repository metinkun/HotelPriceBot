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
