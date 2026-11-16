package coraltatil

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/browser"
	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct {
	sess *session
}

// New maxAge: kalici sekmenin yenilenme suresi.
func New(b *browser.Manager, maxAge time.Duration) *Provider {
	return &Provider{sess: newSession(b, maxAge)}
}

func (p *Provider) Name() string { return "coraltatil" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: false, RequiresBrowser: true}
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) }

func slugOf(m *provider.Mapping) string {
	if s := m.MetaString("slug"); s != "" {
		return s
	}
	return slugFromURL(m.MetaString("sourceUrl"))
}

func nightsOf(checkIn, checkOut string) (int, error) {
	in, err := time.Parse("2006-01-02", checkIn[:min(10, len(checkIn))])
	if err != nil {
		return 0, fmt.Errorf("Gecersiz checkIn (yyyy-MM-dd bekleniyor): %s", checkIn)
	}
	out, err := time.Parse("2006-01-02", checkOut[:min(10, len(checkOut))])
	if err != nil {
		return 0, fmt.Errorf("Gecersiz checkOut (yyyy-MM-dd bekleniyor): %s", checkOut)
	}
	n := int(out.Sub(in).Hours() / 24)
	if n <= 0 {
		return 0, fmt.Errorf("checkOut checkIn'den sonra olmali")
	}
	return n, nil
}

func (p *Provider) fetchPrice(ctx context.Context, hotelID, slug, name, checkIn, checkOut string, adults int, childAges []int) (*PriceData, error) {
	nights, err := nightsOf(checkIn, checkOut)
	if err != nil {
		return nil, err
	}
	crit := buildCriteria(hotelID, slug, name, checkIn[:10], nights, adults, childAges)
	var d PriceData
	if err := p.sess.run(ctx, jsPrice(slug, crit), &d, func() bool { return d.Stale }); err != nil {
		return nil, err
	}
	if d.Error != "" {
		return nil, fmt.Errorf("Coral Tatil: %s", d.Error)
	}
	return &d, nil
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	slug := slugOf(m)
	if slug == "" || m.ProviderHotelID == "" {
		return nil, fmt.Errorf("Mapping metadata'da slug/sourceUrl ve providerHotelId gerekli (ornek: https://www.coraltatil.com/salamis-bay-conti/)")
	}
	d, err := p.fetchPrice(ctx, m.ProviderHotelID, slug, types.Deref(m.HotelName), q.CheckIn, q.CheckOut, q.Adults, q.ChildAges)
	if err != nil {
		return nil, err
	}
	price := normalizePrice(d)
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

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if !in.HasMeta("slug") && !in.HasMeta("sourceUrl") {
		return "metadata.slug veya metadata.sourceUrl zorunlu (ornek: https://www.coraltatil.com/salamis-bay-conti/)"
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"slug"} }

// ResolveFromURL otel detay URL'inden kimlik/ad cozer.
func (p *Provider) ResolveFromURL(ctx context.Context, u string) (*provider.ResolvedHotel, error) {
	slug := slugFromURL(u)
	if slug == "" {
		return nil, fmt.Errorf("Gecersiz Coral Tatil URL'i (ornek: https://www.coraltatil.com/salamis-bay-conti/)")
	}
	var d PriceData
	if err := p.sess.run(ctx, jsResolve(slug), &d, func() bool { return d.Stale }); err != nil {
		return nil, err
	}
	if d.Error != "" || d.Hotel == nil || d.Hotel.ID == "" {
		msg := d.Error
		if msg == "" {
			msg = "otel bulunamadi"
		}
		return nil, fmt.Errorf("Coral Tatil: %s", msg)
	}
	fu := d.Hotel.FriendlyURL
	if fu == "" {
		fu = slug
	}
	meta := map[string]any{"sourceUrl": siteBase + "/" + fu + "/", "slug": fu, "city": d.Hotel.Place, "location": d.Hotel.LocationSummary}
	return &provider.ResolvedHotel{ProviderHotelID: d.Hotel.ID, HotelName: types.StrOrNil(d.Hotel.Name), Metadata: meta}, nil
}

// SearchHotels isimle otel adaylari (ListArrivalLocations, type 7).
func (p *Provider) SearchHotels(ctx context.Context, query string) ([]provider.HotelCandidate, error) {
	q := strings.TrimSpace(query)
	if len([]rune(q)) < 3 {
		return []provider.HotelCandidate{}, nil
	}
	var d searchData
	if err := p.sess.run(ctx, jsSearch(q), &d, nil); err != nil {
		return nil, err
	}
	if d.Error != "" {
		return nil, fmt.Errorf("Coral Tatil arama: %s", d.Error)
	}
	out := make([]provider.HotelCandidate, 0, len(d.Locations))
	for _, l := range d.Locations {
		id := strings.TrimSuffix(l.ID, "-7")
		if id == "" || l.FriendlyURL == "" {
			continue
		}
		city := l.Parent
		if i := strings.Index(city, ","); i > 0 {
			city = strings.TrimSpace(city[:i])
		}
		out = append(out, provider.HotelCandidate{Name: strings.TrimSpace(l.Name), URL: siteBase + "/" + l.FriendlyURL + "/", City: city, ProviderHotelID: id})
	}
	return out, nil
}
