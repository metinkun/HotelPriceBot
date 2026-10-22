// Package enuygun Enuygun otel saglayicisi (search-core/detail-search JSON API; cerez/tarayici gerekmez).
package enuygun

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "enuygun" }

// Capabilities paket yok: Enuygun "Otel + Ucak" dinamik paketi, otel teklifi + ayri ucus aramasinin
// tarayicida birlestirilmesidir; tek bir paket fiyat API'si sunmaz.
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}

func hotelIDOf(m *provider.Mapping) (int, error) {
	id, err := strconv.Atoi(m.ProviderHotelID)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("Enuygun providerHotelId sayisal olmali (ornek: 580512), gelen: %q", m.ProviderHotelID)
	}
	return id, nil
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	id, err := hotelIDOf(m)
	if err != nil {
		return nil, err
	}
	data, err := fetchDetailSearch(ctx, id, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges)
	if err != nil {
		return nil, err
	}
	price := normalizePrice(data, id)
	if price.HotelName == nil {
		price.HotelName = m.HotelName
	}
	return &provider.PriceResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		Found: price.Available, Price: price,
	}, nil
}

func (p *Provider) GetBulkPrices(ctx context.Context, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	return provider.DefaultBulk(ctx, p, ms, q)
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if n, err := strconv.Atoi(in.ProviderHotelID); err != nil || n <= 0 {
		return "providerHotelId sayisal Enuygun otel id'si olmali (ornek: 580512)"
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{} }

var reDomain = regexp.MustCompile(`(?i)enuygun\.com`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Enuygun URL'i (enuygun.com bekleniyor)")
	}
	r, err := resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName),
		Metadata: map[string]any{
			"slug": r.Slug, "sourceUrl": url, "region": r.Region,
			"isDomestic": r.IsDomestic, "isCyprus": r.IsCyprus,
		},
	}, nil
}
