package details

import (
	"regexp"
	"strconv"
	"strings"

	"pricebot/internal/provider"
)

// Bolum basliklarindan turetilen gorunumler: yorumlar, puanlar, etkinlikler, kampanyalar.
// Her acente kendi basligiyla yazar; burada ortak bir siniflandirma yapilir. Siniflandirilan
// bolumler diff'te "sections.*" yerine kendi etiketleriyle raporlanir (yorum/puan/etkinlik/kampanya).

type sectionClass int

const (
	classNone sectionClass = iota
	classReviews
	classScores
	classEvents
	classCampaigns
)

var reScore = regexp.MustCompile(`^\s*(\d{1,2}(?:[.,]\d{1,2})?)\s*(?:/\s*(?:5|10|100))?\s*$`)

func lowerTR(s string) string {
	r := strings.NewReplacer("İ", "i", "I", "ı", "Ğ", "ğ", "Ü", "ü", "Ş", "ş", "Ö", "ö", "Ç", "ç")
	return strings.ToLower(r.Replace(s))
}

func classify(s provider.Section) sectionClass {
	t := lowerTR(s.Title)
	switch {
	case strings.Contains(t, "kampanya") || strings.Contains(t, "fırsat") || strings.Contains(t, "firsat") || strings.Contains(t, "promosyon") || strings.Contains(t, "erken rezervasyon") || strings.Contains(t, "indirim"):
		return classCampaigns
	case strings.Contains(t, "etkinlik") || strings.Contains(t, "sanatçı") || strings.Contains(t, "sanatci") || strings.Contains(t, "konser") || strings.Contains(t, "program"):
		return classEvents
	case strings.Contains(t, "yorum") || strings.Contains(t, "değerlendirme") || strings.Contains(t, "degerlendirme") || strings.Contains(t, "puan"):
		if s.Kind == "pairs" {
			return classScores
		}
		return classReviews
	}
	return classNone
}

// classifiedTitles diff'te atlanacak bolum basliklari.
func classifiedTitles(d *provider.HotelDetails) map[string]bool {
	out := map[string]bool{}
	for _, s := range d.Sections {
		if classify(s) != classNone {
			out[s.Title] = true
		}
	}
	return out
}

// Enrich bos olan turetilmis alanlari Sections'tan doldurur (idempotent).
func Enrich(d *provider.HotelDetails) {
	if d == nil {
		return
	}
	var reviews []provider.Review
	var scores []provider.KV
	var events []provider.Event
	var camps []provider.Campaign
	for _, s := range d.Sections {
		switch classify(s) {
		case classReviews:
			for _, it := range sectionItems(s) {
				reviews = append(reviews, provider.Review{Text: it})
			}
		case classScores:
			for _, p := range s.Pairs {
				k := strings.TrimSpace(p.Key)
				if k == "" || strings.Contains(lowerTR(k), "yorum sayısı") || strings.Contains(lowerTR(k), "yorum sayisi") {
					continue
				}
				scores = append(scores, provider.KV{Key: k, Value: strings.TrimSpace(p.Value)})
			}
			if s.Kind != "pairs" {
				for _, it := range sectionItems(s) {
					reviews = append(reviews, provider.Review{Text: it})
				}
			}
		case classEvents:
			for _, it := range sectionItems(s) {
				events = append(events, provider.Event{Title: it})
			}
			for _, p := range s.Pairs {
				events = append(events, provider.Event{Title: p.Key, Text: p.Value})
			}
		case classCampaigns:
			for _, p := range s.Pairs {
				camps = append(camps, provider.Campaign{Title: p.Key, Text: p.Value})
			}
			if s.Kind != "pairs" {
				for _, it := range sectionItems(s) {
					camps = append(camps, provider.Campaign{Title: s.Title, Text: it})
				}
			}
		}
	}
	if len(d.Reviews) == 0 {
		d.Reviews = reviews
	}
	if len(d.Scores) == 0 {
		d.Scores = scores
	}
	if len(d.Events) == 0 {
		d.Events = events
	}
	if len(d.Campaigns) == 0 {
		d.Campaigns = camps
	}
}

// sectionItems liste bolumunde ogeler, metin bolumunde satirlar.
func sectionItems(s provider.Section) []string {
	var raw []string
	switch s.Kind {
	case "list":
		raw = s.Items
	case "pairs":
		for _, p := range s.Pairs {
			raw = append(raw, strings.TrimSpace(p.Key+": "+p.Value))
		}
	case "html":
		raw = strings.Split(stripTags(s.HTML), "\n")
	default:
		raw = strings.Split(s.Text, "\n")
	}
	out := []string{}
	for _, x := range raw {
		x = strings.TrimSpace(x)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

var reTags = regexp.MustCompile(`<[^>]+>`)

func stripTags(h string) string {
	h = strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "</p>", "\n", "</li>", "\n", "</div>", "\n").Replace(h)
	return reTags.ReplaceAllString(h, " ")
}

// ScoreValue "8,6" / "4.5/5" gibi metni sayiya cevirir (bulunamazsa nil).
func ScoreValue(v string) *float64 {
	m := reScore.FindStringSubmatch(v)
	if m == nil {
		return nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
	if err != nil {
		return nil
	}
	return &f
}
