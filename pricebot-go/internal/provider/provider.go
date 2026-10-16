// Package provider tum otel saglayicilarinin ortak arayuzu ve sonuc tipleri.
package provider

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/model"
	"pricebot/internal/types"
)

// Mapping = DB'deki hotel_mappings satiri.
type Mapping = model.HotelMapping

type PriceResult struct {
	InternalHotelID string                 `json:"internalHotelId"`
	ProviderHotelID string                 `json:"providerHotelId"`
	HotelName       *string                `json:"hotelName"`
	Found           bool                   `json:"found"`
	Price           *types.NormalizedPrice `json:"price"`
	DurationMs      *int64                 `json:"durationMs,omitempty"`
	Error           string                 `json:"error,omitempty"`
	// Cached true ise sonuc Redis cache'ten geldi; FetchedAt siteden cekilme zamani.
	Cached    bool       `json:"cached"`
	FetchedAt *time.Time `json:"fetchedAt,omitempty"`
}

type PackageResult struct {
	InternalHotelID string                        `json:"internalHotelId"`
	ProviderHotelID string                        `json:"providerHotelId"`
	HotelName       *string                       `json:"hotelName"`
	AirportCode     string                        `json:"airportCode"`
	CheckIn         string                        `json:"checkIn"`
	CheckOut        string                        `json:"checkOut"`
	Available       bool                          `json:"available"`
	RoomCount       int                           `json:"roomCount"`
	Rooms           []types.NormalizedPackageRoom `json:"rooms"`
	DurationMs      *int64                        `json:"durationMs,omitempty"`
	// Provider'a ozgu uyari (orn. kalkis havalimani secimi desteklenmiyor).
	Note      string     `json:"note,omitempty"`
	Cached    bool       `json:"cached"`
	FetchedAt *time.Time `json:"fetchedAt,omitempty"`
}

// MappingInput admin mapping olusturma girdisi (provider-spesifik validasyon icin).
type MappingInput struct {
	InternalHotelID string         `json:"internalHotelId"`
	ProviderHotelID string         `json:"providerHotelId"`
	HotelName       *string        `json:"hotelName"`
	IsActive        *bool          `json:"isActive"`
	Metadata        map[string]any `json:"metadata"`
}

// HasMeta metadata'da dolu bir string alan var mi?
func (in MappingInput) HasMeta(key string) bool {
	if in.Metadata == nil {
		return false
	}
	v, ok := in.Metadata[key]
	if !ok || v == nil {
		return false
	}
	if s, ok := v.(string); ok {
		return s != ""
	}
	return true
}

// ResolvedHotel bir provider URL'inden cozulen, mapping olusturmak icin gereken veri.
type ResolvedHotel struct {
	ProviderHotelID string
	HotelName       *string
	Metadata        map[string]any // destinationUrl / destinationCode + sourceUrl
}

type Capabilities struct {
	SupportsPackages   bool `json:"supportsPackages"`
	SupportsBulkPrices bool `json:"supportsBulkPrices"`
	RequiresCookie     bool `json:"requiresCookie"`
	RequiresBrowser    bool `json:"requiresPuppeteer"`
}

// Provider tum saglayicilarin uygulamasi gereken arayuz.
type Provider interface {
	Name() string
	Capabilities() Capabilities
	GetPrice(ctx context.Context, m *Mapping, q types.PriceQuery) (*PriceResult, error)
	GetBulkPrices(ctx context.Context, ms []Mapping, q types.BulkPriceQuery) ([]PriceResult, error)
	// ValidateMappingInput hata mesaji doner, gecerliyse "".
	ValidateMappingInput(in MappingInput) string
	RequiredMetadataFields() []string
}

// PackageProvider paket (otel+ucak+transfer) sorgusu destekleyen provider.
type PackageProvider interface {
	GetPackagePrices(ctx context.Context, m *Mapping, q types.PackageQuery) (*PackageResult, error)
}

// URLResolver otel URL'inden mapping verisi cozebilen provider.
type URLResolver interface {
	ResolveFromURL(ctx context.Context, url string) (*ResolvedHotel, error)
}

// DefaultBulk tek tek sorgulayan varsayilan bulk uygulamasi (BaseProvider.getBulkPrices karsiligi).
func DefaultBulk(ctx context.Context, p Provider, ms []Mapping, q types.BulkPriceQuery) ([]PriceResult, error) {
	results := make([]PriceResult, 0, len(ms))
	for i := range ms {
		m := &ms[i]
		r, err := p.GetPrice(ctx, m, types.PriceQuery{
			HotelID: m.InternalHotelID, CheckIn: q.CheckIn, CheckOut: q.CheckOut,
			Adults: q.Adults, Children: q.Children, ChildAges: q.ChildAges,
		})
		if err != nil {
			results = append(results, PriceResult{
				InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID,
				HotelName: m.HotelName, Found: false, Price: nil, Error: err.Error(),
			})
			continue
		}
		results = append(results, *r)
	}
	return results, nil
}

// ---- Ortak yardimcilar ----

// ParseTRPrice "92.558,00 TL" / "104.546,04" -> 92558 / 104546.04 (TR formati). Gecersizse 0.
func ParseTRPrice(s string) float64 {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' {
			b.WriteRune(r)
		}
	}
	cleaned := strings.ReplaceAll(b.String(), ".", "")
	cleaned = strings.Replace(cleaned, ",", ".", 1)
	n, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}
	return n
}

var (
	reHexEntity = regexp.MustCompile(`&#x([0-9a-fA-F]+);`)
	reDecEntity = regexp.MustCompile(`&#(\d+);`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// DecodeEntities HTML entity'lerini (hex/ondalik dahil) cozer ve bosluklari sadelestirir.
func DecodeEntities(s string) string {
	s = reHexEntity.ReplaceAllStringFunc(s, func(m string) string {
		n, err := strconv.ParseInt(reHexEntity.FindStringSubmatch(m)[1], 16, 32)
		if err != nil {
			return m
		}
		return string(rune(n))
	})
	s = reDecEntity.ReplaceAllStringFunc(s, func(m string) string {
		n, err := strconv.Atoi(reDecEntity.FindStringSubmatch(m)[1])
		if err != nil {
			return m
		}
		return string(rune(n))
	})
	s = strings.NewReplacer("&amp;", "&", "&nbsp;", " ", "&quot;", `"`, "&#39;", "'").Replace(s)
	return strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
}

// FirstMatch regex'in ilk yakalama grubunu doner (yoksa "").
func FirstMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// StripTags HTML etiketlerini kaldirir.
var reTags = regexp.MustCompile(`<[^>]+>`)

func StripTags(s string) string { return reTags.ReplaceAllString(s, " ") }
