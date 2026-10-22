// Package tatildukkani Tatil Dukkani saglayicisi (api.tatildukkani.com JSON + Socket.IO oda listesi;
// cerez/tarayici gerekmez, misafir token'i /guest ile alinir).
package tatildukkani

import (
	"context"
	"fmt"
	"regexp"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "tatildukkani" }

// Capabilities paket yok: ucus sekmesi misafire kapali (tabProductSettings.flight.showToGuest=false);
// "Otel + Ucak + Transfer" aramasi ayni otel fiyatlarini dondurup yalnizca transfer secimi ekliyor.
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}

// slugOf mapping'den slug: metadata.slug, yoksa sourceUrl'den.
func slugOf(m *provider.Mapping) (string, error) {
	if s := m.MetaString("slug"); s != "" {
		return s, nil
	}
	if u := m.MetaString("sourceUrl"); u != "" {
		if s := slugFromURL(u); s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("Mapping metadata'da slug (veya sourceUrl) eksik (ornek: acapulco-resort-convention-spa)")
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	slug, err := slugOf(m)
	if err != nil {
		return nil, err
	}
	page, av, err := fetchAvailability(ctx, slug, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges)
	if err != nil {
		return nil, err
	}
	if m.ProviderHotelID != "" && page != nil && itoa(page.HotelID) != m.ProviderHotelID {
		return nil, fmt.Errorf("Tatildukkani slug %q otel id %d verdi, mapping %s bekliyor", slug, page.HotelID, m.ProviderHotelID)
	}
	price := normalizePrice(page, av)
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
	if !in.HasMeta("slug") && !in.HasMeta("sourceUrl") {
		return "metadata.slug veya metadata.sourceUrl zorunlu (ornek slug: acapulco-resort-convention-spa)"
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"slug"} }

var reDomain = regexp.MustCompile(`(?i)tatildukkani\.com`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Tatildukkani URL'i (tatildukkani.com bekleniyor)")
	}
	page, err := resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	meta := map[string]any{"slug": page.Slug, "sourceUrl": url}
	if page.City != "" {
		meta["city"] = page.City
	}
	if page.AirportCode != "" {
		meta["airportCode"] = page.AirportCode
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: itoa(page.HotelID), HotelName: types.StrOrNil(page.Title), Metadata: meta,
	}, nil
}
