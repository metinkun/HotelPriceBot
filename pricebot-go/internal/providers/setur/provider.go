// Package setur Setur saglayicisi (detay sayfasi RSC payload'indan lowestPrice).
package setur

import (
	"context"
	"fmt"
	"regexp"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "setur" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true}
}

func slugOf(m *provider.Mapping) string {
	if s := m.MetaString("slug"); s != "" {
		return s
	}
	return slugFromURL(m.MetaString("sourceUrl"))
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	slug := slugOf(m)
	if slug == "" {
		return nil, fmt.Errorf("Mapping metadata'da slug/sourceUrl eksik")
	}
	data, err := fetchHotelPage(ctx, slug, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges)
	if err != nil {
		return nil, err
	}
	price := normalize(data)
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
	slug := slugOf(m)
	if slug == "" {
		return nil, fmt.Errorf("Mapping metadata'da slug/sourceUrl eksik")
	}
	data, err := fetchPackagePage(ctx, slug, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges, q.AirportCode, m.MetaString("arrivalCode"))
	if err != nil {
		return nil, err
	}
	pr := normalize(data)
	rooms := []types.NormalizedPackageRoom{}
	if pr.Available {
		rooms = append(rooms, types.NormalizedPackageRoom{
			RoomID: "lowest", RoomName: "En uygun paket", NightCount: data.NightCount,
			Boards: []types.NormalizedPackageBoard{{
				BoardType: pr.BoardType, Currency: pr.Currency, ListPrice: pr.ListPrice, Price: pr.Price, DiscountRate: pr.DiscountRate,
			}},
		})
	}
	hotelName := m.HotelName
	if hotelName == nil {
		hotelName = pr.HotelName
	}
	return &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: hotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms,
		Note: "Setur paket sayfasi yalnizca en uygun (lowest) paket fiyatini veriyor; oda bazli liste sunmuyor.",
	}, nil
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if !in.HasMeta("slug") && !in.HasMeta("sourceUrl") {
		return "metadata.slug veya metadata.sourceUrl zorunlu"
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"slug"} }

var reDomain = regexp.MustCompile(`(?i)setur\.com\.tr`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Setur URL'i (setur.com.tr bekleniyor)")
	}
	r, err := resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName),
		Metadata: map[string]any{"slug": r.Slug, "sourceUrl": url, "hasPackage": r.HasPackage},
	}, nil
}
