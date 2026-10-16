// Package touristica Touristica saglayicisi (Cloudflare -> headless Chrome UI akisi).
package touristica

import (
	"context"
	"fmt"
	"regexp"

	"pricebot/internal/browser"
	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct {
	browser *browser.Manager
}

func New(b *browser.Manager) *Provider { return &Provider{browser: b} }

func (p *Provider) Name() string { return "touristica" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true, RequiresBrowser: true}
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	src := m.MetaString("sourceUrl")
	if src == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (otel detay URL'i)")
	}
	raw, err := p.fetchRoomPrices(ctx, src, q.CheckIn, q.CheckOut, q.Adults, q.Children)
	if err != nil {
		return nil, err
	}
	price := normalize(raw)
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

func (p *Provider) GetPackagePrices(ctx context.Context, m *provider.Mapping, q types.PackageQuery) (*provider.PackageResult, error) {
	src := m.MetaString("sourceUrl")
	if src == "" {
		return nil, fmt.Errorf("Mapping metadata'da sourceUrl eksik (otel detay URL'i)")
	}
	cards, hotelName, err := p.fetchPackagePrices(ctx, src, q.CheckIn, q.CheckOut, q.Adults, q.Children)
	if err != nil {
		return nil, err
	}
	nights := types.NightsBetween(q.CheckIn, q.CheckOut)

	order := []string{}
	byRoom := map[string]*types.NormalizedPackageRoom{}
	for _, c := range cards {
		if c.Price == nil {
			continue
		}
		name := "Oda"
		if c.RoomName != nil && *c.RoomName != "" {
			name = *c.RoomName
		}
		if _, ok := byRoom[name]; !ok {
			byRoom[name] = &types.NormalizedPackageRoom{RoomID: name, RoomName: name, NightCount: nights, Boards: []types.NormalizedPackageBoard{}}
			order = append(order, name)
		}
		byRoom[name].Boards = append(byRoom[name].Boards, types.NormalizedPackageBoard{
			BoardType: c.BoardType, Currency: types.Str("TL"), Price: c.Price,
		})
	}
	rooms := make([]types.NormalizedPackageRoom, 0, len(order))
	for _, k := range order {
		rooms = append(rooms, *byRoom[k])
	}
	hn := m.HotelName
	if hn == nil {
		hn = hotelName
	}
	return &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: hn,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms,
		Note: "Touristica otel sayfasinda kalkis havalimani secimi yoktur; paket fiyatlari sitenin varsayilan kalkisi ile dondurulur (airportCode dikkate alinmaz).",
	}, nil
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if !in.HasMeta("sourceUrl") {
		return "metadata.sourceUrl zorunlu (otel detay URL'i)"
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"sourceUrl"} }

var reDomain = regexp.MustCompile(`(?i)touristica\.com\.tr`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Touristica URL'i (touristica.com.tr bekleniyor)")
	}
	r, err := p.resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	meta := map[string]any{"sourceUrl": url, "slug": r.Slug}
	if r.City != nil {
		meta["city"] = *r.City
	} else {
		meta["city"] = nil
	}
	return &provider.ResolvedHotel{ProviderHotelID: r.ProviderHotelID, HotelName: r.HotelName, Metadata: meta}, nil
}
