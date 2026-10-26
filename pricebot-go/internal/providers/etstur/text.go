package etstur

import (
	"html"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"pricebot/internal/provider"
)

var (
	reBlockClose = regexp.MustCompile(`(?i)<\s*(br|/p|/li|/div|/h[1-6]|/tr)\s*/?>`)
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reBlankLines = regexp.MustCompile(`\n\s*\n+`)
	reWS         = regexp.MustCompile(`[\r\n\t]+`)
)

// cleanHTML HTML parcasini duz metne cevirir: blok kapanislari satir sonu olur,
// etiketler kaldirilir, entity'ler cozulur, fazla bosluk/satir sadelesir.
// Cift escape'li (&lt;p&gt;) icerik de desteklenir.
func cleanHTML(s string) string {
	if strings.Contains(s, "&lt;") && !strings.Contains(s, "<") {
		s = html.UnescapeString(s)
	}
	s = reWS.ReplaceAllString(s, " ") // kaynak HTML'deki satir sonlari anlamsiz
	s = strings.NewReplacer("<sup>2</sup>", "²", "<sup>3</sup>", "³").Replace(s)
	s = reBlockClose.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = provider.DecodeEntities(html.UnescapeString(l))
		l = strings.TrimSpace(strings.ReplaceAll(l, " ", " "))
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.TrimSpace(reBlankLines.ReplaceAllString(strings.Join(out, "\n"), "\n"))
}

// stripQuery gorsel URL'lerinden cache-buster/sorgu parametrelerini atar.
func stripQuery(u string) string {
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return strings.TrimSpace(u)
}

// absURL goreli yolu tam URL'ye cevirir.
func absURL(base, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "//") {
		return "https:" + p
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	b, err := url.Parse(base)
	if err != nil {
		return p
	}
	r, err := url.Parse(p)
	if err != nil {
		return p
	}
	return b.ResolveReference(r).String()
}

// uniq sirayi koruyarak bos ve tekrarlari atar.
func uniq(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

// staticStayWindow oda listesi icin sabit bir konaklama penceresi: iki ay sonrasinin
// ilk gunu, 3 gece. Ay icinde degismez; gunluk snapshot farklarini azaltir.
func staticStayWindow(now time.Time) (checkIn, checkOut string) {
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 2, 0)
	return first.Format("2006-01-02"), first.AddDate(0, 0, 3).Format("2006-01-02")
}
