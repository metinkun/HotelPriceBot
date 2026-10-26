package jolly

import (
	"html"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"pricebot/internal/provider"
)

var (
	reBlockClose = regexp.MustCompile(`(?i)<\s*(br|/p|/li|/div|/h[1-6]|/tr|/pclass[^>]*)\s*/?>`)
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reBlankLines = regexp.MustCompile(`\n\s*\n+`)
	reWS         = regexp.MustCompile(`[\r\n\t]+`)
)

// cleanHTML HTML parcasini duz metne cevirir (blok kapanislari satir sonu, etiketler
// kaldirilir, entity'ler cozulur). Jolly'nin bozuk <pclass="msonormal"> etiketleri de atilir.
func cleanHTML(s string) string {
	s = reWS.ReplaceAllString(s, " ") // kaynak HTML'deki satir sonlari anlamsiz
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

// text secimin metnini tek satira indirger (entity cozumu + bosluk sadelestirme).
func text(sel *goquery.Selection) string {
	return provider.DecodeEntities(html.UnescapeString(strings.ReplaceAll(sel.Text(), " ", " ")))
}

// blockText secimin ic HTML'ini satir yapisini koruyarak metne cevirir.
func blockText(sel *goquery.Selection) string {
	h, err := sel.Html()
	if err != nil {
		return text(sel)
	}
	return cleanHTML(h)
}

func stripQuery(u string) string {
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return strings.TrimSpace(u)
}

func absURL(p string) string {
	p = strings.TrimSpace(p)
	switch {
	case p == "":
		return ""
	case strings.HasPrefix(p, "//"):
		return "https:" + p
	case strings.HasPrefix(p, "http://"), strings.HasPrefix(p, "https://"):
		return p
	case strings.HasPrefix(p, "/"):
		return "https://www.jollytur.com" + p
	}
	return "https://www.jollytur.com/" + p
}

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

// staticStayWindow oda listesi icin sabit pencere: iki ay sonrasinin ilk gunu, 3 gece.
func staticStayWindow(now time.Time) (checkIn, checkOut string) {
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 2, 0)
	return first.Format("2006-01-02"), first.AddDate(0, 0, 3).Format("2006-01-02")
}
