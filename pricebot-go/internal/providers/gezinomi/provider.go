// Package gezinomi Gezinomi saglayicisi (apigezinomi GetHotelRoomPrices).
package gezinomi

import (
	"context"
	"fmt"
	"regexp"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "gezinomi" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true}
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	data, err := fetchRoomPrices(ctx, m.ProviderHotelID, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges, false)
	if err != nil {
		return nil, err
	}
	price := normalizePrice(data, m.HotelName)
	return &provider.PriceResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		Found: price.Available, Price: price,
	}, nil
}

func (p *Provider) GetBulkPrices(ctx context.Context, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	return provider.DefaultBulk(ctx, p, ms, q)
}

func (p *Provider) GetPackagePrices(ctx context.Context, m *provider.Mapping, q types.PackageQuery) (*provider.PackageResult, error) {
	data, err := fetchRoomPrices(ctx, m.ProviderHotelID, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges, true)
	if err != nil {
		return nil, err
	}
	rooms := normalizePackages(data, types.NightsBetween(q.CheckIn, q.CheckOut))
	return &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms,
		Note: "Gezinomi paket fiyatlari otelin varsayilan ucus secenegiyle doner; kalkis havalimani (airportCode) sorguya dahil edilmiyor.",
	}, nil
}

func (p *Provider) ValidateMappingInput(_ provider.MappingInput) string { return "" }

func (p *Provider) RequiredMetadataFields() []string { return []string{} }

var reDomain = regexp.MustCompile(`(?i)gezinomi\.com`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Gezinomi URL'i (gezinomi.com bekleniyor)")
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
