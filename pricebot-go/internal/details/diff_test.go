package details

import (
	"testing"
	"time"

	"pricebot/internal/provider"
)

func sample() *provider.HotelDetails {
	st := 5
	return &provider.HotelDetails{
		Provider: "etstur", ProviderHotelID: "1", Name: "Acapulco Resort", Description: "Deniz kenari", Stars: &st, City: "Girne",
		Images:     []provider.Image{{URL: "https://x/1.jpg"}, {URL: "https://x/2.jpg"}},
		Facilities: []provider.FacilityGroup{{Title: "Genel", Items: []string{"Havuz", "Spa"}}},
		Rooms:      []provider.RoomInfo{{Name: "Villa", Features: []string{"Klima"}}},
		Policies:   &provider.Policies{CheckIn: "14:00", CheckOut: "12:00"},
		FAQ:        []provider.QA{{Question: "Plaj var mi?", Answer: "Evet"}},
		Sections:   []provider.Section{{Title: "Genel Bilgiler", Kind: "text", Text: "Metin"}},
	}
}

func TestHashIgnoresFetchedAt(t *testing.T) {
	a, b := sample(), sample()
	a.FetchedAt = time.Now()
	b.FetchedAt = time.Now().Add(time.Hour)
	if Hash(a) != Hash(b) {
		t.Fatal("fetchedAt hash'i etkilememeli")
	}
	b.Description = "x"
	if Hash(a) == Hash(b) {
		t.Fatal("icerik degisince hash degismeli")
	}
}

func TestDiffAndSummary(t *testing.T) {
	a, b := sample(), sample()
	b.Images = append(b.Images, provider.Image{URL: "https://x/3.jpg"})
	b.Facilities[0].Items = []string{"Havuz"} // Spa kaldirildi
	b.Policies.CheckIn = "15:00"
	b.FAQ = append(b.FAQ, provider.QA{Question: "Otopark?", Answer: "Var"})
	b.Sections[0].Text = "Yeni metin"
	b.Rooms = append(b.Rooms, provider.RoomInfo{Name: "Suit"})

	ch := Diff(a, b)
	paths := map[string]string{}
	for _, c := range ch {
		paths[c.Path] = c.Kind
	}
	for p, k := range map[string]string{"images[2]": "added", "facilities.Genel[1]": "removed", "policies.checkIn": "changed", "faq.Otopark?": "added", "sections.Genel Bilgiler": "changed", "rooms.Suit": "added"} {
		if paths[p] != k {
			t.Errorf("%s: %q bekleniyordu, %q (tum: %v)", p, k, paths[p], paths)
		}
	}
	if len(Diff(a, a)) != 0 {
		t.Error("ayni snapshot icin fark olmamali")
	}
	s := Summary(ch)
	for _, want := range []string{"gorsel eklendi", "ozellik kaldirildi", "politika degisti", "soru-cevap eklendi", "bolum degisti", "oda eklendi"} {
		if !contains(s, want) {
			t.Errorf("ozet %q icermeli: %s", want, s)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
