// Package tatilcikus Tatilcikus (tatilcikus.com) saglayicisi: duz HTTP JSON API, cerez/tarayici gerekmez.
//
// Fiyat akisi: mapping metadata'daki slug ile Hotels/GetByUrlWithDetail -> priceOperator'a gore
// Coral/Gezinomi/Akgunler GetAvailableRooms -> oda/pansiyon eslemesi -> en ucuz musait teklif.
// Paket (otel+ucak+transfer) sitede canli sorgulanmaz; sadece "Sizi Arayalim" ile statik baslangic fiyati gosterilir,
// bu nedenle paket destegi yok.
package tatilcikus

import (
	"context"
	"fmt"
	"strconv"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "tatilcikus" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: false}
}

// slugOf mapping'den otel slug'ini cozer: metadata.slug, yoksa sourceUrl, yoksa sayisal olmayan providerHotelId.
func slugOf(m *provider.Mapping) (string, error) {
	if s := m.MetaString("slug"); s != "" {
		return s, nil
	}
	if u := m.MetaString("sourceUrl"); u != "" {
		if s, err := slugFromURL(u); err == nil {
			return s, nil
		}
	}
	if m.ProviderHotelID != "" {
		if _, err := strconv.Atoi(m.ProviderHotelID); err != nil {
			return m.ProviderHotelID, nil
		}
	}
	return "", fmt.Errorf("Mapping metadata'da slug eksik (ornek: acapulco-resort-hotel-spa)")
}

// promoteOlderChildren otelin cocuk yas sinirini (firstChildAge) asan cocuklari yetiskin sayar.
// Tedarikciler (orn. Akgunler) sinir ustu cocuk icin bos liste doner; sitede kullanici bunu yetiskin olarak aratir.
func promoteOlderChildren(h *Hotel, adults int, ages []int) (int, []int, bool) {
	if h.FirstChildAge <= 0 {
		return adults, ages, false
	}
	kept := make([]int, 0, len(ages))
	promoted := 0
	for _, a := range ages {
		if a > h.FirstChildAge {
			promoted++
		} else {
			kept = append(kept, a)
		}
	}
	return adults + promoted, kept, promoted > 0
}

// fetchOffers otel detayini ceker ve operatorune gore tedarikci fiyatlarini normalize eder.
// Sonuc bos gelir ve yas siniri ustu cocuk varsa, cocuklar yetiskin sayilarak bir kez daha denenir.
func fetchOffers(ctx context.Context, slug, checkIn, checkOut string, adults int, childAges []int) (*HotelDetail, []offer, error) {
	d, err := fetchHotelDetail(ctx, slug)
	if err != nil {
		return nil, nil, err
	}
	offers, err := fetchSupplierOffers(ctx, d, checkIn, checkOut, adults, childAges)
	if err != nil || len(offers) > 0 {
		return d, offers, err
	}
	if a2, c2, changed := promoteOlderChildren(&d.Hotel, adults, childAges); changed {
		offers, err = fetchSupplierOffers(ctx, d, checkIn, checkOut, a2, c2)
	}
	return d, offers, err
}

func fetchSupplierOffers(ctx context.Context, d *HotelDetail, checkIn, checkOut string, adults int, childAges []int) ([]offer, error) {
	h := &d.Hotel
	sid := h.supplierID()
	if h.PriceOperator == opNone || sid == "" {
		return nil, fmt.Errorf("Tatilcikus bu otel icin online fiyat vermiyor (priceOperator=%d, sadece 'Sizi Arayalim')", h.PriceOperator)
	}
	req, err := buildRequest(sid, checkIn, checkOut, adults, childAges)
	if err != nil {
		return nil, err
	}
	switch h.PriceOperator {
	case opAkgunler:
		resp, err := fetchAkgunler(ctx, req)
		if err != nil {
			return nil, err
		}
		var overrides []CustomerCommission
		if h.IsActiveCustomerNetPriceCommission {
			// Override listesi alinamazsa otelin varsayilan oraniyla devam edilir.
			overrides, _ = fetchCustomerCommissions(ctx, h.ID)
		}
		return normalizeAkgunler(d, resp, overrides), nil
	case opCoral:
		resp, err := fetchCoral(ctx, req)
		if err != nil {
			return nil, err
		}
		return normalizeCoral(d, resp), nil
	case opGezinomi:
		resp, err := fetchGezinomi(ctx, req)
		if err != nil {
			return nil, err
		}
		return normalizeGezinomi(d, resp), nil
	}
	return nil, fmt.Errorf("Tatilcikus desteklenmeyen fiyat operatoru: %s (%d)", operatorName(h.PriceOperator), h.PriceOperator)
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	slug, err := slugOf(m)
	if err != nil {
		return nil, err
	}
	d, offers, err := fetchOffers(ctx, slug, q.CheckIn, q.CheckOut, q.Adults, q.ChildAges)
	if err != nil {
		return nil, err
	}
	price := normalizePrice(offers, d.Hotel.Name, cityName(ctx, d.Hotel.CityID))
	hotelName := m.HotelName
	if hotelName == nil {
		hotelName = types.StrOrNil(d.Hotel.Name)
	}
	return &provider.PriceResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: hotelName,
		Found: price.Available, Price: price,
	}, nil
}

func (p *Provider) GetBulkPrices(ctx context.Context, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	return provider.DefaultBulk(ctx, p, ms, q)
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if !in.HasMeta("slug") && !in.HasMeta("sourceUrl") {
		return `metadata.slug zorunlu (otel detay URL'indeki yol, ornek: "acapulco-resort-hotel-spa")`
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"slug"} }

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	slug, err := slugFromURL(url)
	if err != nil {
		return nil, err
	}
	d, err := fetchHotelDetail(ctx, slug)
	if err != nil {
		return nil, err
	}
	h := &d.Hotel
	meta := map[string]any{
		"slug":          h.URL,
		"sourceUrl":     url,
		"priceOperator": operatorName(h.PriceOperator),
		"supplierId":    h.supplierID(),
	}
	if h.HasPackage && h.PackagePrice > 0 {
		meta["packageFromPricePerNight"] = h.PackagePrice
		meta["packageMinNight"] = h.PackagePriceMinNight
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: strconv.Itoa(h.ID), HotelName: types.StrOrNil(h.Name), Metadata: meta,
	}, nil
}
