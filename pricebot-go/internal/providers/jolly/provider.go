// Package jolly Jolly Tur saglayicisi (GetReservationCompletePartial; cookie gerekmez).
package jolly

import (
	"context"
	"fmt"
	"regexp"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "jolly" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true}
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	hotelType := m.MetaString("hotelType")
	if hotelType == "" {
		return nil, fmt.Errorf(`Mapping metadata'da hotelType eksik (ornek: "Cyprus", "Domestic")`)
	}
	data, err := fetchHotelReservation(ctx, m.ProviderHotelID, hotelType, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges, nil)
	if err != nil {
		return nil, err
	}
	price := normalizePrice(data)
	return &provider.PriceResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		Found: price.Available, Price: price,
	}, nil
}

func (p *Provider) GetBulkPrices(ctx context.Context, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	return provider.DefaultBulk(ctx, p, ms, q)
}

func (p *Provider) GetPackagePrices(ctx context.Context, m *provider.Mapping, q types.PackageQuery) (*provider.PackageResult, error) {
	hotelType := m.MetaString("hotelType")
	if hotelType == "" {
		return nil, fmt.Errorf("Mapping metadata'da hotelType eksik")
	}
	origin, err := lookupAirport(ctx, q.AirportCode)
	if err != nil {
		return nil, err
	}
	data, err := fetchHotelReservation(ctx, m.ProviderHotelID, hotelType, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges,
		&packageOpts{Origin: origin, PackageSearchType: packageTypeFlightTransfer})
	if err != nil {
		return nil, err
	}
	rooms := normalizePackages(data, types.NightsBetween(q.CheckIn, q.CheckOut))
	return &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms,
	}, nil
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if !in.HasMeta("hotelType") {
		return `metadata.hotelType zorunlu (ornek: "Cyprus", "Domestic")`
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"hotelType"} }

var reDomain = regexp.MustCompile(`(?i)jollytur\.com`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Jolly URL'i (jollytur.com bekleniyor)")
	}
	r, err := resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	if r.HotelType == "" {
		return nil, fmt.Errorf("Jolly hotelType cozulemedi (otel detay sayfasi URL'i olmali)")
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName),
		Metadata: map[string]any{"hotelType": r.HotelType, "sourceUrl": url},
	}, nil
}
