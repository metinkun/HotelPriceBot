package services

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ---- Otel adi eslestirme: normalize + puanlama ----

// stripDiacritics aksanlari atar (NFD ayristirma + Mn isaretlerini silme). x/text transform.Chain bazi
// girdilerde "slice bounds out of range" panigi uretiyor; bu yuzden elle yapilir.
func stripDiacritics(s string) string {
	d := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(d))
	for _, r := range d {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return norm.NFC.String(b.String())
}

// Tur/gurultu kelimeler: otel adini ayirt etmez, acenteden acenteye degisir.
var genericWords = map[string]bool{
	"hotel": true, "otel": true, "and": true, "the": true, "casino": true, "spa": true, "resort": true,
	"convention": true, "group": true, "holiday": true, "village": true, "tatil": true, "koyu": true,
	"kibris": true, "cyprus": true, "kktc": true, "wellness": true, "pansiyon": false,
}

// Zayif kelimeler: ayirt edici gucu dusuk (eksik/fazla olmasi az ceza).
var weakWords = map[string]bool{
	"villa": true, "suite": true, "apart": true, "apartment": true, "residence": true, "residance": true,
	"inn": true, "boutique": true, "butik": true, "deluxe": true, "luxury": true, "beach": false,
}

// Konum kelimeleri (KKTC): esanlamlilar tek bicime indirgenir; iki tarafta farkli konum varsa ayri otel.
var locationWords = map[string]string{
	"girne": "kyrenia", "kyrenia": "kyrenia", "lefkosa": "nicosia", "nicosia": "nicosia", "lefkosia": "nicosia",
	"gazimagusa": "famagusta", "gazimagosa": "famagusta", "magusa": "famagusta", "magosa": "famagusta", "famagusta": "famagusta",
	"iskele": "iskele", "bafra": "bafra", "lapta": "lapta", "alsancak": "alsancak", "karpaz": "karpaz", "dipkarpaz": "karpaz",
	"esentepe": "esentepe", "catalkoy": "catalkoy", "karavas": "alsancak", "bogaz": "bogaz", "guzelyurt": "guzelyurt",
	"lefke": "lefke", "tatlisu": "tatlisu", "kumyali": "kumyali", "yenierenkoy": "yenierenkoy", "karaoglanoglu": "karaoglanoglu",
	"ozankoy": "ozankoy", "bellapais": "bellapais", "zeytinlik": "zeytinlik", "alagadi": "alagadi",
}

var synonyms = map[string]string{"otel": "hotel", "hotels": "hotel", "butik": "boutique", "villas": "villa", "suites": "suite", "apartments": "apartment", "gardens": "garden", "rocks": "rock"}

// normWords kucuk harf, aksansiz, noktalama temiz kelimeler.
func normWords(s string) []string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("ı", "i", "ş", "s", "ğ", "g", "ç", "c", "ö", "o", "ü", "u", "İ", "i", "&", " and ", "`", "'", "’", "'", "'s ", " ").Replace(s)
	s = stripDiacritics(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	words := strings.Fields(b.String())
	for i, w := range words {
		if syn, ok := synonyms[w]; ok {
			words[i] = syn
		}
	}
	return words
}

type nameTokens struct {
	strong, weak []string
	loc          map[string]bool
}

func tokenize(s string) nameTokens {
	t := nameTokens{loc: map[string]bool{}}
	for _, w := range normWords(s) {
		switch {
		case genericWords[w]:
		case locationWords[w] != "":
			t.loc[locationWords[w]] = true
		case weakWords[w]:
			t.weak = append(t.weak, w)
		default:
			t.strong = append(t.strong, w)
		}
	}
	return t
}

// normName Turkce karakter/noktalama/kucuk-buyuk farklarini, tur ve konum kelimelerini atar (karsilastirma anahtari).
func normName(s string) string {
	t := tokenize(s)
	out := append(append([]string{}, t.strong...), t.weak...)
	if len(out) == 0 {
		return strings.Join(normWords(s), " ")
	}
	return strings.Join(out, " ")
}

func countIn(words []string, set map[string]bool) (in, out int) {
	for _, w := range words {
		if set[w] {
			in++
		} else {
			out++
		}
	}
	return
}

func toSet(words []string) map[string]bool {
	m := map[string]bool{}
	for _, w := range words {
		m[w] = true
	}
	return m
}

// score iki otel adinin benzerligi (0..1).
//   - Tur kelimeleri (hotel, casino, spa, resort...) yok sayilir; konum kelimeleri (Girne/Kyrenia...) eslenir.
//   - Sorgudaki kelimenin adayda OLMAMASI agir ceza (Merit Royal Premium != Merit Royal),
//     adayda fazladan kelime olmasi hafif ceza (Dorana Hotel Girne == Dorana Hotel).
//   - Tek anlamli kelimeli adlar (Life, Oscar, Dome) adayla birebir ortusmek zorunda (Life != Life Point).
//   - Iki tarafta farkli konum varsa ayri otel (Grand Pasha Kyrenia != Grand Pasha Lefkosa).
func score(a, b string) float64 {
	ta, tb := tokenize(a), tokenize(b)
	if len(ta.strong)+len(ta.weak) == 0 || len(tb.strong)+len(tb.weak) == 0 {
		if normName(a) != "" && normName(a) == normName(b) {
			return 1
		}
		return 0
	}
	if len(ta.loc) > 0 && len(tb.loc) > 0 {
		shared := false
		for l := range ta.loc {
			if tb.loc[l] {
				shared = true
			}
		}
		if !shared {
			return 0.1
		}
	}
	sa, sb := toSet(ta.strong), toSet(tb.strong)
	wa, wb := toSet(ta.weak), toSet(tb.weak)
	interS, missS := countIn(ta.strong, sb) // sorgunun guclu kelimeleri adayda var/yok
	_, extraS := countIn(tb.strong, sa)     // adayin fazladan guclu kelimeleri
	interW, missW := countIn(ta.weak, wb)
	_, extraW := countIn(tb.weak, wa)
	inter := float64(interS) + 0.5*float64(interW)
	if inter == 0 {
		return 0
	}
	denom := inter + 1.5*float64(missS) + 0.5*float64(missW) + 0.5*float64(extraS) + 0.25*float64(extraW)
	sc := inter / denom
	if len(ta.strong) == 1 && extraS > 0 && sc > 0.5 {
		sc = 0.5
	}
	if sc > 1 {
		sc = 1
	}
	return sc
}

// Turkiye / yurt disi isaretleri: KKTC listesi icin bu sehirlerdeki adaylar elenir.
var nonTRNCMarkers = []string{"turkiye", "antalya", "alanya", "mugla", "marmaris", "bodrum", "istanbul", "izmir", "ankara", "balikesir",
	"ayvalik", "didim", "side", "kemer", "belek", "kusadasi", "aydin", "canakkale", "bozcaada", "kocaeli", "gebze", "fethiye", "finike",
	"avsa", "marmara", "eskisehir", "odunpazari", "cankaya", "azerbaycan", "italya", "yunanistan", "larnaka", "avsallar", "beldibi",
	"kundu", "davutlar", "sigacik", "altinkum", "geyikli", "ezine", "buca", "atasehir", "karaman", "gence", "milano", "tasos",
	"goynuk", "sisli", "mersin", "adana", "bursa", "sakarya", "trabzon", "rize", "nevsehir", "kapadokya", "denizli", "pamukkale",
	"afyon", "yalova", "kirklareli", "edirne", "tekirdag", "sapanca", "bolu", "abant", "kartepe", "uludag", "erzurum", "kayseri"}

// cityAllowed aday sehri, otelin ulkesiyle uyumlu mu? Ulke verilmemisse veya sehir bossa filtre yok.
func cityAllowed(city, country string) bool {
	c := strings.ToUpper(strings.TrimSpace(country))
	if city == "" || (c != "TRNC" && c != "KKTC" && c != "CYPRUS" && c != "KIBRIS") {
		return true
	}
	words := normWords(city)
	for _, w := range words {
		if locationWords[w] != "" || genericWords[w] && (w == "kibris" || w == "cyprus" || w == "kktc") {
			return true
		}
	}
	for _, w := range words {
		for _, m := range nonTRNCMarkers {
			if w == m {
				return false
			}
		}
	}
	return true // bilinmeyen sehir: engelleme
}
