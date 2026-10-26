package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

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
				defer func() {
					if rec := recover(); rec != nil {
						mu.Lock()
						res.Errors = append(res.Errors, fmt.Sprintf("%s: panic: %v", pn, rec))
						mu.Unlock()
					}
				}()
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
				best, second, secondKey, rejected := ImportMatch{}, 0.0, "", 0
				for _, c := range cands {
					if !cityAllowed(c.City, h.Country) {
						rejected++
						continue
					}
					sc := score(h.Name, c.Name)
					if cityConflict(h.Name, c.Name, c.City) {
						sc *= 0.7
					}
					key := normName(c.Name)
					if sc > best.Score {
						if key != normName(best.Name) {
							second, secondKey = best.Score, normName(best.Name)
						}
						best = ImportMatch{Provider: pn, Name: c.Name, URL: c.URL, City: c.City, Score: sc, Candidates: len(cands)}
					} else if sc > second && key != normName(best.Name) {
						second, secondKey = sc, key
					}
				}
				_ = secondKey
				if best.URL == "" {
					if rejected > 0 {
						res.Errors = append(res.Errors, fmt.Sprintf("%s: %d aday baska bolgede (sehir filtresi)", pn, rejected))
					}
					return
				}
				// Belirsiz: farkli adli ikinci aday en iyiye cok yakin (ve esik ustu)
				best.Ambiguous = best.Score < 1 && second > 0 && best.Score-second < 0.1 && second >= minScore
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
				name := displayName(h.Name)
				if hotelName != nil {
					name = hotelName(h)
				}
				// Excel kimligi varsa kalici, izlenebilir otel ID'si (tekrar calistirmada ayni otel guncellenir)
				internalID := ""
				if h.ExternalID > 0 {
					internalID = fmt.Sprintf("ext_%d", h.ExternalID)
				}
				out, err := im.hotels.AddFromURLs(ctx, AddHotelInput{URLs: urls, HotelName: name, InternalHotelID: internalID})
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

// displayName BUYUK HARFLI listeden okunabilir ad: "MERIT CRYSTAL COVE HOTEL & CASINO" -> "Merit Crystal Cove Hotel & Casino".
func displayName(name string) string {
	name = strings.Join(strings.Fields(strings.ReplaceAll(name, "`", "'")), " ")
	if strings.ToUpper(name) != name {
		return name // zaten karisik harf
	}
	words := strings.Fields(strings.ToLower(strings.NewReplacer("I", "ı", "İ", "i").Replace(name)))
	for i, w := range words {
		r := []rune(w)
		if len(r) == 0 {
			continue
		}
		if r[0] == 'ı' {
			r[0] = 'I'
		} else if r[0] == 'i' {
			r[0] = 'İ'
		} else {
			r[0] = unicode.ToUpper(r[0])
		}
		// Ingilizce kelimelerde noktasiz i yanlis olur: ingilizce/ortak kelimelerde i'ye cevir
		words[i] = strings.NewReplacer("ı", "i").Replace(string(r))
		if r[0] == 'I' && len(r) > 1 {
			words[i] = "I" + strings.NewReplacer("ı", "i").Replace(string(r[1:]))
		}
	}
	return strings.Join(words, " ")
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
