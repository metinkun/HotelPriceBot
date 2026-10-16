// Package tatilsepeti TatilSepeti saglayicisi (cookie'li liste fiyati + GetFlyingPackages).
package tatilsepeti

import (
	"context"
	"fmt"
	"regexp"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct {
	cookie            string
	verificationToken string
}

func New(cookie, verificationToken string) *Provider {
	return &Provider{cookie: cookie, verificationToken: verificationToken}
}

func (p *Provider) Name() string { return "tatilsepeti" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true, SupportsBulkPrices: true, RequiresCookie: true}
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	data, err := p.fetchPrices(ctx, []string{m.ProviderHotelID}, q.CheckIn, q.CheckOut, q.Adults, q.Children)
	if err != nil {
		return nil, err
	}
	return &provider.PriceResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		Found: data != nil, Price: normalizeResponse(data, m.ProviderHotelID),
	}, nil
}

// GetBulkPrices virgulle ayrilmis HotelIds ile native bulk.
func (p *Provider) GetBulkPrices(ctx context.Context, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	ids := make([]string, 0, len(ms))
	for _, m := range ms {
		ids = append(ids, m.ProviderHotelID)
	}
	data, err := p.fetchPrices(ctx, ids, q.CheckIn, q.CheckOut, q.Adults, q.Children)
	if err != nil {
		return nil, err
	}
	out := make([]provider.PriceResult, 0, len(ms))
	for _, m := range ms {
		out = append(out, provider.PriceResult{
			InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
			Found: data != nil, Price: normalizeResponse(data, m.ProviderHotelID),
		})
	}
	return out, nil
}

func (p *Provider) GetPackagePrices(ctx context.Context, m *provider.Mapping, q types.PackageQuery) (*provider.PackageResult, error) {
	dest := m.MetaString("destinationCode")
	if dest == "" {
		return nil, fmt.Errorf(`Mapping metadata'da destinationCode eksik (ornek: "ecnc")`)
	}
	nights := 0
	if n := types.NightsBetween(q.CheckIn, q.CheckOut); n != nil {
		nights = *n
	}
	ages := q.ChildAges
	if ages == nil {
		ages = []int{}
	}
	raw, err := p.fetchFlyingPackages(ctx, m.ProviderHotelID, q.AirportCode, dest, q.CheckIn, q.CheckOut, q.Adults, ages)
	if err != nil {
		return nil, err
	}
	rooms := parsePackageRoomList(raw.RoomList, nights)
	return &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms,
	}, nil
}

// ValidateMappingInput destinationCode sadece paket icin gerekli; fiyat icin metadata gerekmez.
func (p *Provider) ValidateMappingInput(_ provider.MappingInput) string { return "" }

func (p *Provider) RequiredMetadataFields() []string { return []string{} }

var reDomain = regexp.MustCompile(`(?i)tatilsepeti\.com`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz TatilSepeti URL'i (tatilsepeti.com bekleniyor)")
	}
	r, err := p.resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	meta := map[string]any{"sourceUrl": url}
	if r.DestinationCode != "" {
		meta["destinationCode"] = r.DestinationCode
	} else {
		meta["destinationCode"] = nil // paket icin gerekli; bulunamazsa null (duz fiyat yine calisir)
	}
	return &provider.ResolvedHotel{ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName), Metadata: meta}, nil
}
