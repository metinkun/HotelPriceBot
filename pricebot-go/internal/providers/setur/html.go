package setur

// HTML -> metin / Section yardimcilari (statik detay icin). Ayni yardimcilar gezinomi ve enuygun
// paketlerinde de bulunur (paketler birbirini import etmez).

import (
	"html"
	"regexp"
	"strings"

	"pricebot/internal/provider"
)

var (
	reScriptStyle = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)\s*>`)
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reBlockTag    = regexp.MustCompile(`(?i)<\s*(?:br|/p|/div|/li|/h[1-6]|/tr|/section|/ul|/ol|/table|/blockquote)\b[^>]*>`)
	reListItem    = regexp.MustCompile(`(?is)<li\b[^>]*>(.*?)</li\s*>`)
	reAnyTag      = regexp.MustCompile(`<[^>]+>`)
	reOpenTag     = regexp.MustCompile(`<[^>]*$`) // kesilmis/kapanmamis son etiket
	reHSpace      = regexp.MustCompile("[ \t ​]+")
	reURLQuery    = regexp.MustCompile(`[?#].*$`)
	reMultiSpace  = regexp.MustCompile(`\s+`)
)

// htmlToText HTML'i duz metne cevirir: script/style/yorum atilir, blok etiketleri satir sonu olur,
// entity'ler cozulur, bosluklar sadelesir. Satirlar "\n" ile ayrilir.
func htmlToText(s string) string {
	if s == "" {
		return ""
	}
	s = reScriptStyle.ReplaceAllString(s, " ")
	s = reHTMLComment.ReplaceAllString(s, " ")
	s = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(s)
	s = reBlockTag.ReplaceAllString(s, "\n")
	s = reAnyTag.ReplaceAllString(s, " ")
	s = reOpenTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(reHSpace.ReplaceAllString(ln, " "))
		if ln != "" {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

// oneLine HTML/metni tek satira indirger (ad, politika gibi kisa alanlar icin).
func oneLine(s string) string {
	return strings.TrimSpace(reMultiSpace.ReplaceAllString(htmlToText(s), " "))
}

// htmlSection HTML icerigi acentenin basligiyla Section'a cevirir: <li> varsa kind=list (liste disi
// metin Text'e), degilse kind=text. Icerik bos ise nil.
func htmlSection(title, raw string) *provider.Section {
	items := []string{}
	for _, m := range reListItem.FindAllStringSubmatch(raw, -1) {
		if t := oneLine(m[1]); t != "" {
			items = append(items, t)
		}
	}
	rest := htmlToText(reListItem.ReplaceAllString(raw, " "))
	if len(items) == 0 && rest == "" {
		return nil
	}
	sec := &provider.Section{Title: strings.TrimSpace(title), Kind: "text", Text: rest}
	if len(items) > 0 {
		sec.Kind = "list"
		sec.Items = items
	}
	return sec
}

// listSection metin listesinden kind=list Section uretir (bos ise nil).
func listSection(title string, items []string) *provider.Section {
	items = uniqStrings(items)
	if len(items) == 0 {
		return nil
	}
	return &provider.Section{Title: title, Kind: "list", Items: items}
}

// pairsSection anahtar/deger listesinden kind=pairs Section uretir (bos ise nil).
func pairsSection(title string, pairs []provider.KV) *provider.Section {
	if len(pairs) == 0 {
		return nil
	}
	return &provider.Section{Title: title, Kind: "pairs", Pairs: pairs}
}

// cleanImageURL cache-buster/sorgu parametrelerini atar, protokolsuz URL'leri https yapar.
func cleanImageURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	u = reURLQuery.ReplaceAllString(u, "")
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	return u
}

// uniqStrings sirayi koruyarak bos ve tekrarlari atar.
func uniqStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
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

// appendSection nil olmayan section'lari ekler.
func appendSection(list []provider.Section, secs ...*provider.Section) []provider.Section {
	for _, s := range secs {
		if s != nil {
			list = append(list, *s)
		}
	}
	return list
}
