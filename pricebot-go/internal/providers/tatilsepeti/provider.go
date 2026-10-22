// Package tatilsepeti TatilSepeti saglayicisi (liste fiyati GetHotelListPrice + paket GetFlyingPackages).
//
// Oturum (cerez + VerificationToken) otomatik edinilir: once duz HTTP, engellenirse headless Chrome
// (yalnizca cerez/token okumak icin, sekme hemen kapanir). Bkz. session.go ve docs/providers/tatilsepeti.md.
package tatilsepeti

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"pricebot/internal/browser"
	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct {
	sess *sessionCache
}

// Option New icin istege bagli ayarlar.
type Option func(*Provider)

// WithBrowser duz HTTP engellendiginde oturum edinmek icin kullanilacak tarayici yoneticisi.
// Verilmezse yalnizca duz HTTP denenir (engelde hata doner).
func WithBrowser(b *browser.Manager) Option {
	return func(p *Provider) { p.sess.bm = b }
}

// WithSessionAge oturumun yenilenme suresi (varsayilan 20 dk).
func WithSessionAge(d time.Duration) Option {
	return func(p *Provider) {
		if d > 0 {
			p.sess.maxAge = d
		}
	}
}

// WithForceBrowser true ise oturum her zaman tarayici ile edinilir (duz HTTP atlanir).
func WithForceBrowser(on bool) Option {
	return func(p *Provider) { p.sess.forceBrowser = on }
}

// New cookie/verificationToken bos olabilir (otomatik edinim); doluysa once onlar denenir,
// basarisiz olursa otomatik edinime gecilir.
func New(cookie, verificationToken string, opts ...Option) *Provider {
	p := &Provider{sess: newSessionCache(cookie, verificationToken, defaultSessionAge)}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *Provider) Name() string { return "tatilsepeti" }

// Stats oturum sayaclari (provider health icin).
func (p *Provider) Stats() map[string]int64 { return p.sess.Stats() }

func (p *Provider) Capabilities() provider.Capabilities {
	// RequiresCookie false: cerez/token artik otomatik; env degerleri yalnizca istege bagli override.
	return provider.Capabilities{SupportsPackages: true, SupportsBulkPrices: true}
}

// pageURLOf oturum edinimi / referer icin otel detay sayfasi (yoksa ana sayfa).
func pageURLOf(m *provider.Mapping) string {
	if m == nil {
		return homeURL
	}
	if u := m.MetaString("sourceUrl"); u != "" {
		return u
	}
	return homeURL
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	data, err := p.fetchPrices(ctx, []string{m.ProviderHotelID}, q.CheckIn, q.CheckOut, q.Adults, q.Children, pageURLOf(m))
	if err != nil {
		return nil, err
	}
	price := normalizeResponse(data, m.ProviderHotelID)
	if price.HotelName == nil {
		price.HotelName = m.HotelName
	}
	return &provider.PriceResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		Found: price.Available, Price: price,
	}, nil
}

// GetBulkPrices virgulle ayrilmis HotelIds ile native bulk.
func (p *Provider) GetBulkPrices(ctx context.Context, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	if len(ms) == 0 {
		return []provider.PriceResult{}, nil
	}
	ids := make([]string, 0, len(ms))
	for _, m := range ms {
		ids = append(ids, m.ProviderHotelID)
	}
	data, err := p.fetchPrices(ctx, ids, q.CheckIn, q.CheckOut, q.Adults, q.Children, pageURLOf(&ms[0]))
	if err != nil {
		return nil, err
	}
	out := make([]provider.PriceResult, 0, len(ms))
	for _, m := range ms {
		price := normalizeResponse(data, m.ProviderHotelID)
		if price.HotelName == nil {
			price.HotelName = m.HotelName
		}
		out = append(out, provider.PriceResult{
			InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
			Found: price.Available, Price: price,
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
	raw, err := p.fetchFlyingPackages(ctx, m.ProviderHotelID, q.AirportCode, dest, q.CheckIn, q.CheckOut, q.Adults, ages, pageURLOf(m))
	if err != nil {
		return nil, err
	}
	rooms := parsePackageRoomList(raw.RoomList, nights)
	res := &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
		Available: len(rooms) > 0, RoomCount: len(rooms), Rooms: rooms,
	}
	if len(rooms) == 0 {
		if msg := provider.DecodeEntities(provider.StripTags(raw.DetailMessage)); msg != "" {
			res.Note = msg
		}
	}
	return res, nil
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
