package tatildukkani

// Isimle otel arama (provider.HotelSearcher): sitenin arama kutusunun kullandigi
// POST /api/autocomplate/hotel {"term","limit","type":"HOTEL"} ucu (misafir token'li). Yanit
// data.hotel[] (id, name, seoPage.slug, destination[]); destination/category dizileri alinmaz.
// Sunucu Turkce karakter/buyuk-kucuk harf duyarsiz eslesir ("merıt" -> Merit ...).

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/provider"
)

const autocompleteURL = apiBase + "/api/autocomplate/hotel"

type autocompleteHotel struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	AirportCode string `json:"airportCode"`
	SeoPage     *struct {
		Slug string `json:"slug"`
		Type string `json:"type"`
	} `json:"seoPage"`
	Destination []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"destination"`
}

// parseAutocomplete data nesnesinden otel adaylarini cikarir (yalnizca type=HOTEL ve slug'i olanlar; API sirasi korunur).
func parseAutocomplete(data json.RawMessage) ([]provider.HotelCandidate, error) {
	var d struct {
		Hotel []autocompleteHotel `json:"hotel"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("Tatildukkani autocomplete yaniti parse edilemedi: %w", err)
	}
	out := []provider.HotelCandidate{}
	seen := map[int]bool{}
	for _, h := range d.Hotel {
		if h.Type != "HOTEL" || h.SeoPage == nil || h.SeoPage.Slug == "" || h.ID <= 0 || seen[h.ID] {
			continue
		}
		seen[h.ID] = true
		city := ""
		if n := len(h.Destination); n > 0 {
			city = provider.DecodeEntities(h.Destination[n-1].Name)
		}
		out = append(out, provider.HotelCandidate{
			Name: provider.DecodeEntities(h.Name), URL: siteBase + "/" + h.SeoPage.Slug,
			City: city, ProviderHotelID: strconv.Itoa(h.ID),
		})
	}
	return out, nil
}

func searchOnce(ctx context.Context, term string) ([]provider.HotelCandidate, error) {
	data, err := postJSON(ctx, autocompleteURL, map[string]any{"term": term, "limit": 20, "type": "HOTEL"}, 20*time.Second, "autocomplate/hotel")
	if err != nil {
		return nil, err
	}
	return parseAutocomplete(data)
}

// SearchHotels otel adina gore aday listesi; sonuc yoksa sorgunun son kelimeleri atilarak tekrar denenir.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	words := strings.Fields(query)
	if len(words) == 0 {
		return []provider.HotelCandidate{}, nil
	}
	for n := len(words); n >= 1; n-- {
		term := strings.Join(words[:n], " ")
		if len(term) < 3 { // site 3 karakterden kisa sorgu gondermiyor
			break
		}
		res, err := searchOnce(ctx, term)
		if err != nil {
			return nil, err
		}
		if len(res) > 0 || n == 1 {
			return res, nil
		}
	}
	return []provider.HotelCandidate{}, nil
}
