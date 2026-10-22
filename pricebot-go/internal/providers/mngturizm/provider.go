// Package mngturizm MNG Turizm saglayicisi (index.php?s=otelprice; cerez gerekmez, Referer sart).
package mngturizm

import (
	"context"
	"fmt"
	"strings"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "mngturizm" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true}
}

// referer mapping'deki sourceUrl (Cloudflare icin mngturizm.com Referer'i yeterli).
func referer(m *provider.Mapping) string {
	if u := m.MetaString("sourceUrl"); u != "" {
		return u
	}
	return baseURL + "/"
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	html, err := fetchRoomPrices(ctx, m.ProviderHotelID, m.MetaString("toId"), referer(m), q.CheckIn, q.CheckOut, q.Adults, q.ChildAges, nil)
	if err != nil {
		return nil, err
	}
	price := normalizePrice(html, m.HotelName, types.StrOrNil(m.MetaString("city")))
	if !price.Available && !isUnavailable(html) && !strings.Contains(html, "otelfiyatsablon") {
		return nil, fmt.Errorf("MNG Turizm fiyat yaniti taninamadi (otel id %s)", m.ProviderHotelID)
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
	toID := m.MetaString("toId")
	if toID == "" {
		return nil, fmt.Errorf("Mapping metadata'da toId eksik (otel sayfasindaki varis kodu, ornek: air-ECN); URL'den yeniden cozun")
	}
	ref := referer(m)
	fs, err := fetchFlights(ctx, m.ProviderHotelID, toID, ref, q.AirportCode, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges)
	if err != nil {
		return nil, err
	}
	sel, err := selectFlights(fs)
	if err != nil {
		return nil, err
	}
	html, err := fetchRoomPrices(ctx, m.ProviderHotelID, toID, ref, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges, sel)
	if err != nil {
		return nil, err
	}
	rooms := normalizePackages(html, types.NightsBetween(q.CheckIn, q.CheckOut))
	note := fmt.Sprintf("Ucus: gidis %s %s-%s (%s %s, fark %.2f), donus %s %s-%s (%s %s, fark %.2f); en dusuk ucus farkli secenekler secildi, transfer dahil.",
		strings.ToUpper(q.AirportCode), sel.Outbound.Depart, sel.Outbound.Arrive, sel.Outbound.Provider, sel.Outbound.Company, sel.Outbound.Surcharge,
		strings.TrimPrefix(toID, "air-"), sel.Return.Depart, sel.Return.Arrive, sel.Return.Provider, sel.Return.Company, sel.Return.Surcharge)
	return &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms, Note: note,
	}, nil
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string { return "" }

func (p *Provider) RequiredMetadataFields() []string { return []string{} }

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(strings.TrimSpace(url)) {
		return nil, fmt.Errorf("Gecersiz MNG Turizm URL'i (https://www.mngturizm.com/<otel-slug> bekleniyor)")
	}
	r, err := resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	if !r.HasPriceForm {
		return nil, fmt.Errorf("MNG Turizm: bu otelde online fiyat formu yok (yalnizca 'Talep Et'); fiyat cekilemez")
	}
	meta := map[string]any{"sourceUrl": url}
	if r.ToID != "" {
		meta["toId"] = r.ToID
	}
	if r.City != "" {
		meta["city"] = r.City
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName), Metadata: meta,
	}, nil
}
