// Package etstur Etstur saglayicisi (destinasyon arama API'si + paket API'si).
package etstur

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct {
	cookie string
}

func New(cookie string) *Provider { return &Provider{cookie: cookie} }

func (p *Provider) Name() string { return "etstur" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true, SupportsBulkPrices: true}
}

func occupancyOf(checkIn, checkOut string, adults, children int, ages []int) Occupancy {
	if ages == nil {
		ages = []int{}
	}
	return Occupancy{CheckIn: checkIn, CheckOut: checkOut, Adults: adults, Children: children, ChildAges: ages}
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	dest := m.MetaString("destinationUrl")
	if dest == "" {
		return nil, fmt.Errorf("Mapping metadata'da destinationUrl eksik")
	}
	found, err := p.collectHotelsByIDs(ctx, dest, []string{m.ProviderHotelID}, occupancyOf(q.CheckIn, q.CheckOut, q.Adults, q.Children, q.ChildAges))
	if err != nil {
		return nil, err
	}
	h := found[m.ProviderHotelID]
	res := &provider.PriceResult{InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName, Found: h != nil}
	if h != nil {
		res.Price = normalizeHotel(h)
	}
	return res, nil
}

// GetBulkPrices destinasyon bazli gruplama ile optimize edilmis bulk sorgu.
func (p *Provider) GetBulkPrices(ctx context.Context, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	occ := occupancyOf(q.CheckIn, q.CheckOut, q.Adults, q.Children, q.ChildAges)

	byDest := map[string][]*provider.Mapping{}
	order := []string{}
	for i := range ms {
		dest := ms[i].MetaString("destinationUrl")
		if dest == "" {
			continue
		}
		if _, ok := byDest[dest]; !ok {
			order = append(order, dest)
		}
		byDest[dest] = append(byDest[dest], &ms[i])
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	results := []provider.PriceResult{}

	for _, dest := range order {
		group := byDest[dest]
		wg.Add(1)
		go func(dest string, group []*provider.Mapping) {
			defer wg.Done()
			ids := make([]string, 0, len(group))
			for _, m := range group {
				ids = append(ids, m.ProviderHotelID)
			}
			found, err := p.collectHotelsByIDs(ctx, dest, ids, occ)
			mu.Lock()
			defer mu.Unlock()
			for _, m := range group {
				r := provider.PriceResult{InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName}
				if err != nil {
					r.Error = err.Error()
				} else if h := found[m.ProviderHotelID]; h != nil {
					r.Found = true
					r.Price = normalizeHotel(h)
				}
				results = append(results, r)
			}
		}(dest, group)
	}
	wg.Wait()
	return results, nil
}

func (p *Provider) GetPackagePrices(ctx context.Context, m *provider.Mapping, q types.PackageQuery) (*provider.PackageResult, error) {
	occ := occupancyOf(q.CheckIn, q.CheckOut, q.Adults, q.Children, q.ChildAges)

	// 1) Oda aramasi -> roomSearchId + oturum cookie'si
	roomSearchID, cookie, err := p.fetchRoomSearch(ctx, m.ProviderHotelID, occ)
	if err != nil {
		return nil, err
	}
	// 2) Paket fiyatlari (ayni oturum)
	pkg, err := p.fetchPackage(ctx, roomSearchID, q.AirportCode, cookie)
	if err != nil {
		return nil, err
	}
	rooms := []types.NormalizedPackageRoom{}
	if pkg.Result != nil {
		for _, r := range pkg.Result.Rooms {
			rooms = append(rooms, normalizePackageRoom(r))
		}
	}
	return &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms,
	}, nil
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if !in.HasMeta("destinationUrl") {
		return `metadata.destinationUrl zorunlu (ornek: "Kemer-Otelleri")`
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"destinationUrl"} }

var reDomain = regexp.MustCompile(`(?i)etstur\.com`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Etstur URL'i (etstur.com bekleniyor)")
	}
	r, err := resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	if r.DestinationURL == "" {
		return nil, fmt.Errorf("Etstur destinationUrl cozulemedi (otel detay sayfasi URL'i olmali)")
	}
	meta := map[string]any{"destinationUrl": r.DestinationURL, "sourceUrl": url}
	if r.Slug != "" {
		meta["slug"] = r.Slug
	} else {
		meta["slug"] = nil
	}
	return &provider.ResolvedHotel{ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName), Metadata: meta}, nil
}
