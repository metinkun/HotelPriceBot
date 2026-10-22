// Package prontotour Prontotour saglayicisi (webapi.prontotour.com/corewebapi JSON API; cookie gerekmez).
package prontotour

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "prontotour" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true}
}

// serviceTypeOf metadata.serviceType, yoksa sourceUrl'deki "-h<kod>-<tip>" ekinden okur.
func serviceTypeOf(m *provider.Mapping) string {
	if s := strings.TrimSpace(m.MetaString("serviceType")); s != "" {
		return s
	}
	if u := m.MetaString("sourceUrl"); u != "" {
		if parts, err := parseDetailURL(u); err == nil {
			return parts.ServiceType
		}
	}
	return ""
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	st := serviceTypeOf(m)
	if st == "" {
		return nil, fmt.Errorf("Mapping metadata'da serviceType (veya -h<kod>-<tip> iceren sourceUrl) eksik")
	}
	ci, err := normDate(q.CheckIn)
	if err != nil {
		return nil, err
	}
	co, err := normDate(q.CheckOut)
	if err != nil {
		return nil, err
	}
	data, err := fetchHotelDetail(ctx, m.ProviderHotelID, st, ci, co, buildTravelers(q.Adults, q.Children, q.ChildAges), nil)
	if err != nil {
		return nil, err
	}
	price := normalizePrice(data)
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
	st := serviceTypeOf(m)
	if st == "" {
		return nil, fmt.Errorf("Mapping metadata'da serviceType (veya -h<kod>-<tip> iceren sourceUrl) eksik")
	}
	ci, err := normDate(q.CheckIn)
	if err != nil {
		return nil, err
	}
	co, err := normDate(q.CheckOut)
	if err != nil {
		return nil, err
	}
	from, err := lookupAirport(ctx, q.AirportCode)
	if err != nil {
		return nil, err
	}
	travelers := buildTravelers(q.Adults, q.Children, q.ChildAges)
	bundle, err := fetchBundle(ctx, m.ProviderHotelID, st, from, ci, co, travelers)
	if err != nil {
		return nil, err
	}
	res := &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: ci, CheckOut: co, Rooms: []types.NormalizedPackageRoom{},
	}
	if packageHotel(bundle) == nil {
		if bundle.ErrorText != nil && *bundle.ErrorText != "" {
			res.Note = "Prontotour: " + *bundle.ErrorText
		}
		return res, nil
	}
	// Oda bazli fiyatlar icin paket modunda HotelDetail (S_GUID ile); basarisiz olursa taban paket yeter.
	var detail *HotelDetailResponse
	if bundle.SGUID != nil && *bundle.SGUID != "" {
		detail, _ = fetchHotelDetail(ctx, m.ProviderHotelID, st, ci, co, travelers, &packageCtx{SGUID: *bundle.SGUID})
	}
	res.Rooms = normalizePackages(bundle, detail, types.NightsBetween(ci, co))
	res.Available = len(res.Rooms) > 0
	res.RoomCount = len(res.Rooms)
	res.Note = flightSummary(bundle)
	if res.HotelName == nil {
		res.HotelName = types.StrOrNil(strings.TrimSpace(packageHotel(bundle).Name))
	}
	return res, nil
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if !in.HasMeta("serviceType") && !in.HasMeta("sourceUrl") {
		return "metadata.serviceType veya metadata.sourceUrl (otel detay URL'i) zorunlu"
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"serviceType"} }

var reDomain = regexp.MustCompile(`(?i)prontotour\.com`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Prontotour URL'i (prontotour.com bekleniyor)")
	}
	r, err := resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName),
		Metadata: map[string]any{"serviceType": r.ServiceType, "sourceUrl": url},
	}, nil
}
