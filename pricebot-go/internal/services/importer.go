package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"pricebot/internal/provider"
	"pricebot/internal/providers"
)

// ---- Toplu otel ekleme: isimden acente URL'i bulma ----

type ImportHotel struct {
	ExternalID int    `json:"externalId"`
	Name       string `json:"name"`
	Country    string `json:"country,omitempty"`
}

type ImportMatch struct {
	Provider   string  `json:"provider"`
	Name       string  `json:"name"`
	URL        string  `json:"url"`
	City       string  `json:"city,omitempty"`
	Score      float64 `json:"score"` // 0..1
	Ambiguous  bool    `json:"ambiguous,omitempty"`
	Candidates int     `json:"candidates"`
	Error      string  `json:"error,omitempty"`
}

type ImportResult struct {
	ExternalID      int           `json:"externalId"`
	Name            string        `json:"name"`
	Matches         []ImportMatch `json:"matches"`         // esik ustu eslesmeler
	LowConfidence   []ImportMatch `json:"lowConfidence"`   // esik alti en iyi aday (inceleme icin)
	InternalHotelID string        `json:"internalHotelId"` // apply ile olusturulduysa
	Errors          []string      `json:"errors,omitempty"`
}

type ImportReport struct {
	ID        string         `json:"id,omitempty"`
	Status    string         `json:"status"` // running | done
	Done      int            `json:"done"`
	Total     int            `json:"total"`
	Providers []string       `json:"providers"`
	Applied   bool           `json:"applied"`
	MinScore  float64        `json:"minScore"`
	Results   []ImportResult `json:"results"`
	Duration  string         `json:"duration"`
	StartedAt time.Time      `json:"startedAt"`
}

var stripper = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// normName Turkce karakter/noktalama/kucuk-buyuk farklarini ve gurultu kelimeleri atar.
func normName(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("ı", "i", "ş", "s", "ğ", "g", "ç", "c", "ö", "o", "ü", "u", "İ", "i", "&", " and ", "`", "'", "’", "'").Replace(s)
	if t, _, err := transform.String(stripper, s); err == nil {
		s = t
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	words := strings.Fields(b.String())
	noise := map[string]bool{"hotel": true, "otel": true, "and": true, "the": true, "casino": true, "spa": true, "resort": true, "convention": true, "boutique": true, "butik": true, "deluxe": true, "premium": true, "luxury": true, "holiday": true, "village": true, "tatil": true, "koyu": true, "koyü": true, "hotels": true, "group": true, "cyprus": true, "kibris": true, "kyrenia": true, "girne": true, "lefkosa": true, "nicosia": true, "beach": true, "city": true, "residence": true, "residance": true, "inn": true, "suites": true, "suite": true}
	out := []string{}
	for _, w := range words {
		if !noise[w] {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		return strings.Join(words, " ")
	}
	return strings.Join(out, " ")
}

// score iki otel adinin benzerligi (0..1): kelime kesisimi (Jaccard) + on ek bonusu.
func score(a, b string) float64 {
	na, nb := normName(a), normName(b)
	if na == "" || nb == "" {
		return 0
	}
	if na == nb {
		return 1
	}
	wa, wb := strings.Fields(na), strings.Fields(nb)
	set := map[string]bool{}
	for _, w := range wa {
		set[w] = true
	}
	inter := 0
	for _, w := range wb {
		if set[w] {
			inter++
		}
	}
	union := len(wa) + len(wb) - inter
	if union == 0 {
		return 0
	}
	j := float64(inter) / float64(union)
	// Tum sorgu kelimeleri adayda geciyorsa (aday daha uzun olsa da) guclu eslesme
	if inter == len(wa) || inter == len(wb) {
		j = (j + 1) / 2
	}
	if strings.HasPrefix(nb, na) || strings.HasPrefix(na, nb) {
		j = (j + 1) / 2
	}
	return j
}

// Importer isim listesinden acente URL'leri bulur ve istenirse otelleri olusturur.
type Importer struct {
	reg    *providers.Registry
	hotels *HotelService

	mu   sync.Mutex
	jobs map[string]*ImportReport
}

func NewImporter(reg *providers.Registry, hs *HotelService) *Importer {
	return &Importer{reg: reg, hotels: hs, jobs: map[string]*ImportReport{}}
}

// Start ice aktarmayi arka planda baslatir; rapor ilerledikce Get ile okunur (uzun surer: otel x acente).
func (im *Importer) Start(hotels []ImportHotel, providerNames []string, minScore float64, apply bool) *ImportReport {
	id := fmt.Sprintf("imp_%d", time.Now().UnixNano())
	rep := &ImportReport{ID: id, Status: "running", Total: len(hotels), Providers: providerNames, Applied: apply, MinScore: minScore, Results: []ImportResult{}, StartedAt: time.Now()}
	if len(rep.Providers) == 0 {
		rep.Providers = im.SearchProviders()
	}
	im.mu.Lock()
	im.jobs[id] = rep
	// eski raporlari temizle (son 20)
	if len(im.jobs) > 20 {
		oldest, oldestT := "", time.Now()
		for k, v := range im.jobs {
			if v.StartedAt.Before(oldestT) {
				oldest, oldestT = k, v.StartedAt
			}
		}
		delete(im.jobs, oldest)
	}
	im.mu.Unlock()
	go im.Run(context.Background(), hotels, providerNames, minScore, apply, nil, rep)
	return rep
}

// Get ilerleyen/biten raporun kopyasini doner.
func (im *Importer) Get(id string) (*ImportReport, bool) {
	im.mu.Lock()
	defer im.mu.Unlock()
	r, ok := im.jobs[id]
	if !ok {
		return nil, false
	}
	cp := *r
	cp.Results = append([]ImportResult{}, r.Results...)
	if cp.Status == "running" {
		cp.Duration = time.Since(r.StartedAt).Round(time.Second).String()
	}
	return &cp, true
}

// SearchProviders isimle arama destekleyen provider'lar.
func (im *Importer) SearchProviders() []string {
	out := []string{}
	for _, p := range im.reg.All() {
		if _, ok := p.(provider.HotelSearcher); ok {
			out = append(out, p.Name())
		}
	}
	return out
}

// Run her otel icin tum (veya secili) acentelerde arama yapar; apply ile esik ustu eslesmelerden otel olusturur.
func (im *Importer) Run(ctx context.Context, hotels []ImportHotel, providerNames []string, minScore float64, apply bool, hotelName func(ImportHotel) string, rep *ImportReport) *ImportReport {
	start := time.Now()
	if minScore <= 0 || minScore > 1 {
		minScore = 0.6
	}
	provs := providerNames
	if len(provs) == 0 {
		provs = im.SearchProviders()
	}
	if rep == nil {
		rep = &ImportReport{Status: "running", Total: len(hotels), Providers: provs, Applied: apply, MinScore: minScore, StartedAt: start}
	}

	// Otel bazinda sirali (siteye nezaket), acenteler paralel
	for _, h := range hotels {
		res := ImportResult{ExternalID: h.ExternalID, Name: h.Name, Matches: []ImportMatch{}, LowConfidence: []ImportMatch{}}
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, pn := range provs {
			p, err := im.reg.Get(pn)
			if err != nil {
				continue
			}
			hs, ok := p.(provider.HotelSearcher)
			if !ok {
				continue
			}
			wg.Add(1)
			go func(pn string, hs provider.HotelSearcher) {
				defer wg.Done()
				sctx, cancel := context.WithTimeout(ctx, 40*time.Second)
				defer cancel()
				cands, err := hs.SearchHotels(sctx, searchQuery(h.Name))
				if (err != nil || len(cands) == 0) && searchQuery(h.Name) != shortQuery(h.Name) {
					cands, err = hs.SearchHotels(sctx, shortQuery(h.Name))
				}
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					res.Errors = append(res.Errors, pn+": "+err.Error())
					return
				}
				if len(cands) == 0 {
					return
				}
				best, second := ImportMatch{}, 0.0
				for _, c := range cands {
					sc := score(h.Name, c.Name)
					if sc > best.Score {
						second = best.Score
						best = ImportMatch{Provider: pn, Name: c.Name, URL: c.URL, City: c.City, Score: sc, Candidates: len(cands)}
					} else if sc > second {
						second = sc
					}
				}
				best.Ambiguous = best.Score > 0 && second > 0 && best.Score-second < 0.1 && second >= minScore
				if best.Score >= minScore {
					res.Matches = append(res.Matches, best)
				} else if best.URL != "" {
					res.LowConfidence = append(res.LowConfidence, best)
				}
			}(pn, hs)
		}
		wg.Wait()
		sort.Slice(res.Matches, func(i, j int) bool { return res.Matches[i].Provider < res.Matches[j].Provider })

		if apply && len(res.Matches) > 0 {
			urls := map[string]string{}
			for _, m := range res.Matches {
				if !m.Ambiguous {
					urls[m.Provider] = m.URL
				}
			}
			if len(urls) > 0 {
				name := h.Name
				if hotelName != nil {
					name = hotelName(h)
				}
				out, err := im.hotels.AddFromURLs(ctx, AddHotelInput{URLs: urls, HotelName: name})
				if err != nil {
					res.Errors = append(res.Errors, "ekleme: "+err.Error())
				} else {
					res.InternalHotelID = out.InternalHotelID
					for _, e := range out.Errors {
						res.Errors = append(res.Errors, e.Provider+": "+e.Error)
					}
				}
			}
		}
		im.mu.Lock()
		rep.Results = append(rep.Results, res)
		rep.Done = len(rep.Results)
		im.mu.Unlock()
	}
	im.mu.Lock()
	rep.Status = "done"
	rep.Duration = time.Since(start).Round(time.Second).String()
	im.mu.Unlock()
	return rep
}

// searchQuery arama kutusuna yazilacak sorgu: baslik bicimli, "&" sade.
func searchQuery(name string) string {
	s := strings.NewReplacer("&", " ", "`", "'").Replace(name)
	s = strings.Join(strings.Fields(s), " ")
	// Baslik bicimi (API'ler buyuk/kucuk duyarsiz; ancak bazilari tam buyuk harfi sevmez)
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		r := []rune(w)
		if len(r) > 0 {
			r[0] = unicode.ToUpper(r[0])
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// shortQuery gurultu kelimeler atilmis ilk iki anlamli kelime.
func shortQuery(name string) string {
	w := strings.Fields(normName(name))
	if len(w) > 2 {
		w = w[:2]
	}
	return searchQuery(strings.Join(w, " "))
}
