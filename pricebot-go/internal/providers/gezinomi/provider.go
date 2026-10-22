// Package gezinomi Gezinomi saglayicisi (apigezinomi GetHotelRoomPrices).
//
// Hibrit akis: fiyat/paket istekleri duz HTTP; yalnizca x-client-key uretimi icin kalici,
// hafif bir tarayici sekmesi kullanilir (bkz. session.go ve docs/providers/gezinomi.md).
package gezinomi

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"pricebot/internal/browser"
	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct {
	signer *signer

	airportMu sync.Mutex
	airports  map[string]airport // providerHotelId -> varis havalimani
}

func New(b *browser.Manager) *Provider {
	return &Provider{signer: newSigner(b), airports: map[string]airport{}}
}

func (p *Provider) Name() string { return "gezinomi" }

// Stats oturum/sekme durumu (provider health).
func (p *Provider) Stats() map[string]any { return p.signer.Stats() }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true, RequiresBrowser: true}
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	data, err := fetchRoomPrices(ctx, p.signer, m.ProviderHotelID, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges, nil)
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

// destinationAirport otelin varis havalimanini bulur (bellekte onbelleklenir).
// hotelLocationId/parentId metadata'da yoksa sourceUrl'den yeniden cozulur.
func (p *Provider) destinationAirport(ctx context.Context, m *provider.Mapping) (airport, error) {
	p.airportMu.Lock()
	a, ok := p.airports[m.ProviderHotelID]
	p.airportMu.Unlock()
	if ok {
		return a, nil
	}
	loc, _ := strconv.Atoi(m.MetaString("hotelLocationId"))
	parent, _ := strconv.Atoi(m.MetaString("parentId"))
	if loc == 0 {
		src := m.MetaString("sourceUrl")
		if src == "" {
			return airport{}, fmt.Errorf("Mapping metadata'da hotelLocationId/sourceUrl eksik (paket icin gerekli)")
		}
		r, err := resolveHotelFromURL(ctx, src)
		if err != nil {
			return airport{}, err
		}
		loc, _ = strconv.Atoi(r.HotelLocationID)
		parent, _ = strconv.Atoi(r.ParentID)
	}
	hotelID, _ := strconv.Atoi(m.ProviderHotelID)
	list, err := fetchHotelAirports(ctx, p.signer, hotelID, loc, parent)
	if err != nil {
		return airport{}, err
	}
	if len(list) == 0 {
		return airport{}, fmt.Errorf("Gezinomi bu otel icin varis havalimani dondurmedi")
	}
	p.airportMu.Lock()
	p.airports[m.ProviderHotelID] = list[0]
	p.airportMu.Unlock()
	return list[0], nil
}

func (p *Provider) GetPackagePrices(ctx context.Context, m *provider.Mapping, q types.PackageQuery) (*provider.PackageResult, error) {
	dest, err := p.destinationAirport(ctx, m)
	if err != nil {
		return nil, err
	}
	flight := &flightOpts{Origin: q.AirportCode, Destination: dest.AirportCode, IsDestinationCity: dest.IsCity}
	data, err := fetchRoomPrices(ctx, p.signer, m.ProviderHotelID, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges, flight)
	if err != nil {
		return nil, err
	}
	rooms := normalizePackages(data, types.NightsBetween(q.CheckIn, q.CheckOut))
	return &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms,
		Note: fmt.Sprintf("Gezinomi paket fiyati %s -> %s gidis-donus, sitenin varsayilan (en ucuz) ucus secimiyle hesaplanir.", q.AirportCode, dest.AirportCode),
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
	meta := map[string]any{"sourceUrl": url}
	if r.HotelLocationID != "" {
		meta["hotelLocationId"] = r.HotelLocationID
		meta["parentId"] = r.ParentID
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName), Metadata: meta,
	}, nil
}
