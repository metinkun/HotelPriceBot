package tatilsepeti

// Isimle otel arama (provider.HotelSearcher): sitenin arama kutusunun (jQuery UI autocomplete, main.min.js
// tsAutocomplete) kullandigi GET /common/hotelsearch?value=<sorgu> ucu. Yanit JSON dizi: Title, Type
// ("Yurt İçi Oteller" | "Yurt Dışı Oteller" | "Bölge / Şehir" | kampanya/tema...), TypeId (1 = yurt ici otel),
// Id (otel id), Link ("/<slug>"), AreaPlace ("Çatalköy, Girne, Kıbrıs"), TitleHighlighted. Cerez/token gerekmez;
// sunucu Turkce karakter / buyuk-kucuk harf duyarsiz eslestirir.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const hotelSearchURL = "https://www.tatilsepeti.com/common/hotelsearch"

type searchItem struct {
	Title      string `json:"Title"`
	Type       string `json:"Type"`
	TypeID     int    `json:"TypeId"`
	ID         string `json:"Id"`
	Link       string `json:"Link"`
	IsDomestic bool   `json:"IsDomestic"`
	AreaPlace  string `json:"AreaPlace"`
}

// parseSearch yalnizca yurt ici otel kayitlarini (TypeId 1; detay sayfasi sablonu bot'un parse ettigi sablon) aday yapar.
func parseSearch(body []byte) ([]provider.HotelCandidate, error) {
	var items []searchItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("TatilSepeti arama yaniti parse edilemedi: %w", err)
	}
	out := []provider.HotelCandidate{}
	seen := map[string]bool{}
	for _, it := range items {
		if it.TypeID != 1 || it.ID == "" || !strings.HasPrefix(it.Link, "/") || seen[it.ID] {
			continue
		}
		seen[it.ID] = true
		out = append(out, provider.HotelCandidate{
			Name: provider.DecodeEntities(it.Title), URL: strings.TrimSuffix(homeURL, "/") + it.Link,
			City: provider.DecodeEntities(it.AreaPlace), ProviderHotelID: it.ID,
		})
	}
	return out, nil
}

func (p *Provider) searchOnce(ctx context.Context, term string) ([]provider.HotelCandidate, error) {
	headers := httpx.Merge(defaultHeaders, map[string]string{"accept": "application/json, text/javascript, */*; q=0.01"})
	delete(headers, "content-type")
	// Varsa mevcut oturum cerezi eklenir (zorunlu degil); oturum edinmek icin ek istek yapilmaz.
	p.sess.mu.Lock()
	if p.sess.cur != nil && p.sess.cur.Cookie != "" {
		headers["cookie"] = p.sess.cur.Cookie
	}
	p.sess.mu.Unlock()
	resp, err := httpx.Get(ctx, hotelSearchURL+"?value="+url.QueryEscape(term), headers, 20*time.Second)
	if err != nil {
		if looksBlocked(resp, err) {
			return nil, fmt.Errorf("TatilSepeti arama istegi engellendi: %w", err)
		}
		return nil, err
	}
	if !looksJSON(resp.Body) {
		return nil, fmt.Errorf("TatilSepeti arama yaniti JSON degil (engel olabilir)")
	}
	return parseSearch(resp.Body)
}

// SearchHotels otel adina gore aday listesi; sonuc yoksa sorgunun son kelimeleri atilarak tekrar denenir.
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	words := strings.Fields(query)
	if len(words) == 0 {
		return []provider.HotelCandidate{}, nil
	}
	for n := len(words); n >= 1; n-- {
		res, err := p.searchOnce(ctx, strings.Join(words[:n], " "))
		if err != nil {
			return nil, err
		}
		if len(res) > 0 || n == 1 {
			return rankCandidates(res, query), nil
		}
	}
	return []provider.HotelCandidate{}, nil
}

// rankCandidates sunucunun bulanik sonuclarini siralar: once sorgunun tum kelimelerini (Turkce duyarsiz)
// iceren adlar, sonra digerleri; gruplar icinde sunucu sirasi korunur.
func rankCandidates(list []provider.HotelCandidate, query string) []provider.HotelCandidate {
	words := strings.Fields(foldTR(query))
	full, rest := []provider.HotelCandidate{}, []provider.HotelCandidate{}
	for _, c := range list {
		name := foldTR(c.Name)
		ok := true
		for _, w := range words {
			if !strings.Contains(name, w) {
				ok = false
				break
			}
		}
		if ok {
			full = append(full, c)
		} else {
			rest = append(rest, c)
		}
	}
	return append(full, rest...)
}
