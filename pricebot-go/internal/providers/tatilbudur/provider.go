// Package tatilbudur Tatilbudur saglayicisi (Laravel CSRF + Akamai cookie, calculate-room-price).
package tatilbudur

import (
	"context"
	"fmt"
	"regexp"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "tatilbudur" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true}
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	src := m.MetaString("sourceUrl")
	if src == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (otel detay URL'i)")
	}
	data, err := fetchRoomPrice(ctx, src, m.ProviderHotelID, q.CheckIn, q.CheckOut, q.Adults, q.Children)
	if err != nil {
		return nil, err
	}
	price := normalizePrice(data)
	price.HotelName = m.HotelName
	return &provider.PriceResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		Found: price.Available, Price: price,
	}, nil
}

func (p *Provider) GetBulkPrices(ctx context.Context, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	return provider.DefaultBulk(ctx, p, ms, q)
}

func (p *Provider) GetPackagePrices(ctx context.Context, m *provider.Mapping, q types.PackageQuery) (*provider.PackageResult, error) {
	src := m.MetaString("sourceUrl")
	if src == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (otel detay URL'i)")
	}
	data, _, err := fetchPackagePrice(ctx, src, m.ProviderHotelID, q.AirportCode, q.CheckIn, q.CheckOut, q.Adults, q.Children)
	if err != nil {
		return nil, err
	}
	rooms := normalizePackages(data)
	return &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms,
	}, nil
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if !in.HasMeta("sourceUrl") {
		return "metadata.sourceUrl zorunlu (otel detay URL'i)"
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"sourceUrl"} }

var reDomain = regexp.MustCompile(`(?i)tatilbudur\.com`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Tatilbudur URL'i (tatilbudur.com bekleniyor)")
	}
	r, err := resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName),
		Metadata: map[string]any{"sourceUrl": url},
	}, nil
}
