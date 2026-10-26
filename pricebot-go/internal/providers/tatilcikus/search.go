package tatilcikus

// Isimle otel arama (provider.HotelSearcher).
//
// Tatilcikus'ta ayri bir autocomplete ucu yok; sitenin arama kutusu Hotels/GetSummary (tum oteller: id, ad,
// slug, ulke; ~320 kayit, anonim) listesini istemci tarafinda filtreler. Burada da ayni liste cekilir
// (10 dk cache) ve Turkce karakter / buyuk-kucuk harf duyarsiz kelime eslesmesi yapilir.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"pricebot/internal/provider"
)

// HotelSummary Hotels/GetSummary kaydi.
type HotelSummary struct {
	ID        int    `json:"id"`
	CountryID int    `json:"countryId"`
	Name      string `json:"name"`
	URL       string `json:"url"`
}

func parseSummary(body []byte) ([]HotelSummary, error) {
	var env struct {
		Data []HotelSummary `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("Tatilcikus otel listesi parse edilemedi: %w", err)
	}
	return env.Data, nil
}

// matchHotels sorgunun tum kelimelerini (Turkce duyarsiz) otel adinda/slug'inda arar; once tam ad, sonra
// adi sorguyla baslayanlar, sonra digerleri; esitlikte ada gore sirali.
func matchHotels(list []HotelSummary, query string, countries map[int]string) []provider.HotelCandidate {
	q := foldTR(query)
	if q == "" {
		return []provider.HotelCandidate{}
	}
	words := strings.Fields(q)
	type scored struct {
		c     provider.HotelCandidate
		score int
	}
	out := []scored{}
	for _, h := range list {
		if h.URL == "" || h.Name == "" {
			continue
		}
		fn := foldTR(h.Name)
		hay := fn + " " + foldTR(h.URL)
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		score := 2
		switch {
		case fn == q:
			score = 0
		case strings.HasPrefix(fn, q):
			score = 1
		}
		out = append(out, scored{score: score, c: provider.HotelCandidate{
			Name: provider.DecodeEntities(h.Name), URL: siteURL + "/" + h.URL,
			City: countries[h.CountryID], ProviderHotelID: strconv.Itoa(h.ID),
		}})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score < out[j].score
		}
		return out[i].c.Name < out[j].c.Name
	})
	res := make([]provider.HotelCandidate, 0, len(out))
	for _, s := range out {
		res = append(res, s.c)
	}
	return res
}

// SearchHotels otel adina gore aday listesi (yalnizca oteller; City alaninda ulke adi — liste sehir vermiyor).
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	if strings.TrimSpace(query) == "" {
		return []provider.HotelCandidate{}, nil
	}
	body, err := getListCached(ctx, "Hotels/GetSummary", detailCacheTTL)
	if err != nil {
		return nil, err
	}
	list, err := parseSummary(body)
	if err != nil {
		return nil, err
	}
	countries := map[int]string{}
	if b, err := getListCached(ctx, "Countries/", lookupTTL); err == nil {
		countries = parseNameMap(b)
	}
	res := matchHotels(list, query, countries)
	// Sonuc yoksa kisaltilmis sorgu (ilk kelimeler) ile tekrar dene.
	for words := strings.Fields(query); len(res) == 0 && len(words) > 1; words = words[:len(words)-1] {
		res = matchHotels(list, strings.Join(words[:len(words)-1], " "), countries)
	}
	return res, nil
}
