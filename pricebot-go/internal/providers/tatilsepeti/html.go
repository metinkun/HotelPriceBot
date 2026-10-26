package tatilsepeti

// Detay sayfasi metinleri icin kucuk HTML yardimcilari (statik detay normalizasyonu).

import (
	"regexp"
	"strings"

	"pricebot/internal/provider"
)

var (
	reScriptStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	reComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	reBlockEnd    = regexp.MustCompile(`(?i)<br\s*/?>|</(p|div|li|tr|h[1-6]|table|ul|ol|figure|blockquote)>`)
	reLiStart     = regexp.MustCompile(`(?i)<li[^>]*>`)
	reSupSub      = regexp.MustCompile(`(?i)</?su[bp]>`)
	reCellEnd     = regexp.MustCompile(`(?i)</t[dh]>`)
	reLi          = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`)
	reAttrs       = regexp.MustCompile(`(?i)<([a-z][a-z0-9]*)(\s[^>]*)?>`)
	reKeepAttr    = regexp.MustCompile(`(?i)\s(href|src|colspan|rowspan)="[^"]*"`)
	reEmptyTag    = regexp.MustCompile(`(?i)<(p|div|span|strong|b|em|i|u)>\s*</(p|div|span|strong|b|em|i|u)>`)
	reMultiNL     = regexp.MustCompile(`\n{2,}`)
	reTable       = regexp.MustCompile(`(?i)<table`)
)

// htmlLines HTML'i satirlara boler: blok sonlari satir sonu, <li> "- " ile baslar, tablo hucreleri " | " ile ayrilir.
func htmlLines(s string) []string {
	s = reScriptStyle.ReplaceAllString(s, "")
	s = reComment.ReplaceAllString(s, "")
	s = reSupSub.ReplaceAllString(s, "") // "m<sup>2</sup>" -> "m2" (bosluk eklenmesin)
	s = reLiStart.ReplaceAllString(s, "\n- ")
	s = reCellEnd.ReplaceAllString(s, " | ")
	s = reBlockEnd.ReplaceAllString(s, "\n")
	s = provider.StripTags(s)
	out := []string{}
	for _, ln := range strings.Split(s, "\n") {
		ln = provider.DecodeEntities(strings.ReplaceAll(ln, " ", " "))
		ln = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(ln), "|"))
		if ln == "" || ln == "-" {
			continue
		}
		out = append(out, ln)
	}
	return out
}

// htmlText HTML -> duz metin (satirlar \n ile).
func htmlText(s string) string { return strings.Join(htmlLines(s), "\n") }

// htmlListItems yalnizca <li> icerikleri (duz metin).
func htmlListItems(s string) []string {
	out := []string{}
	for _, m := range reLi.FindAllStringSubmatch(s, -1) {
		if t := htmlText(m[1]); t != "" {
			out = append(out, strings.TrimPrefix(t, "- "))
		}
	}
	return out
}

// cleanHTML script/style/yorum kaldirir, href/src/colspan/rowspan disindaki attribute'lari atar, nbsp -> bosluk.
func cleanHTML(s string) string {
	s = reScriptStyle.ReplaceAllString(s, "")
	s = reComment.ReplaceAllString(s, "")
	s = reAttrs.ReplaceAllStringFunc(s, func(tag string) string {
		m := reAttrs.FindStringSubmatch(tag)
		keep := strings.Join(reKeepAttr.FindAllString(m[2], -1), "")
		return "<" + strings.ToLower(m[1]) + keep + ">"
	})
	s = strings.NewReplacer("&nbsp;", " ", " ", " ").Replace(s)
	s = reEmptyTag.ReplaceAllString(s, "")
	s = strings.TrimSpace(reMultiNL.ReplaceAllString(s, "\n"))
	return s
}

// sectionFromHTML HTML icerigi en uygun Section turune cevirir: sadece liste -> list, tablo var -> html, aksi -> text.
func sectionFromHTML(title, raw string) (provider.Section, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return provider.Section{}, false
	}
	if reTable.MatchString(raw) {
		h := cleanHTML(raw)
		if h == "" {
			return provider.Section{}, false
		}
		return provider.Section{Title: title, Kind: "html", HTML: h, Text: htmlText(raw)}, true
	}
	items := htmlListItems(raw)
	lines := htmlLines(raw)
	if len(items) > 0 && len(items) == len(lines) {
		return provider.Section{Title: title, Kind: "list", Items: items}, true
	}
	t := strings.Join(lines, "\n")
	if t == "" {
		return provider.Section{}, false
	}
	return provider.Section{Title: title, Kind: "text", Text: t}, true
}

// foldTR Turkce karakterleri sadelestirip kucuk harfe cevirir; harf/rakam disini bosluk yapar (arama icin).
func foldTR(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.NewReplacer("İ", "i", "I", "ı").Replace(s)) {
		switch r {
		case 'ı':
			r = 'i'
		case 'ş':
			r = 's'
		case 'ç':
			r = 'c'
		case 'ğ':
			r = 'g'
		case 'ü':
			r = 'u'
		case 'ö':
			r = 'o'
		case 'â':
			r = 'a'
		case 'î':
			r = 'i'
		case 'û':
			r = 'u'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
