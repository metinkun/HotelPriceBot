package jolly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const (
	reservationURL = "https://www.jollytur.com/hotel/GetReservationCompletePartial"
	searchURL      = "https://www.jollytur.com/Shared/Search"
	// packageTypeFlightTransfer: 0 = Ucak + Transfer Dahil, 1 = Ucak Dahil (transfersiz)
	packageTypeFlightTransfer = "0"
	ua                        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
)

var xhrHeaders = map[string]string{
	"accept":           "application/json",
	"accept-language":  "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"content-type":     "application/x-www-form-urlencoded; charset=UTF-8",
	"origin":           "https://www.jollytur.com",
	"user-agent":       ua,
	"x-requested-with": "XMLHttpRequest",
	"referer":          "https://www.jollytur.com/",
}

var pageHeaders = map[string]string{
	"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"accept-language":           "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"user-agent":                ua,
	"sec-fetch-dest":            "document",
	"sec-fetch-mode":            "navigate",
	"sec-fetch-site":            "none",
	"upgrade-insecure-requests": "1",
}

// Analytics analyticsDataModel (temiz lead-fiyat + kimlik verisi)
type Analytics struct {
	ProductID       any      `json:"productId"`
	ProductName     string   `json:"productName"`
	CityName        string   `json:"cityName"`
	CountryName     string   `json:"countryName"`
	Currency        string   `json:"currency"`
	TotalPrice      *float64 `json:"totalPrice"`
	DiscountedPrice *float64 `json:"discountedPrice"`
}

type ReservationResponse struct {
	HTML               string     `json:"html"`
	AnalyticsDataModel *Analytics `json:"analyticsDataModel"`
	IsAvailableHotel   bool       `json:"isAvailableHotel"`
}

var (
	reISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
	reTR  = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}`)
)

// toJollyDate "2026-08-31" (veya "31.08.2026") -> "2026.08.31"
func toJollyDate(input string) (string, error) {
	if reISO.MatchString(input) {
		return strings.ReplaceAll(input[:10], "-", "."), nil
	}
	if reTR.MatchString(input) {
		p := strings.Split(input[:10], ".")
		return p[2] + "." + p[1] + "." + p[0], nil
	}
	return "", fmt.Errorf("Gecersiz tarih formati: %s (yyyy-MM-dd bekleniyor)", input)
}

// buildRoomsParam "<yetiskin>[-<cocukYas>...]" — 0-1 yas -> 1.
func buildRoomsParam(adults int, childAges []int) string {
	parts := []string{strconv.Itoa(adults)}
	for _, a := range childAges {
		if a < 2 {
			a = 1
		}
		parts = append(parts, strconv.Itoa(a))
	}
	return strings.Join(parts, "-")
}

type Origin struct {
	ID   int
	Name string
}

// lookupAirport IATA -> Jolly ic havalimani id'si (destinationType === 7).
func lookupAirport(ctx context.Context, iata string) (*Origin, error) {
	u := searchURL + "?" + url.Values{"key": {strings.ToUpper(iata)}, "type": {"FlightPlanner"}}.Encode()
	resp, err := httpx.Get(ctx, u, map[string]string{"user-agent": ua, "x-requested-with": "XMLHttpRequest"}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var list []struct {
		ID              float64 `json:"id"`
		DestinationType float64 `json:"destinationType"`
		Value           string  `json:"value"`
	}
	_ = json.Unmarshal(resp.Body, &list)
	for _, x := range list {
		if x.DestinationType == 7 && x.ID > 0 {
			name := x.Value
			if name == "" {
				name = iata
			}
			return &Origin{ID: int(x.ID), Name: name}, nil
		}
	}
	return nil, fmt.Errorf("Jolly'de '%s' havalimani bulunamadi (IATA kodu kontrol edilmeli)", iata)
}

type packageOpts struct {
	Origin            *Origin
	PackageSearchType string
}

// fetchHotelReservation bir otel + tarih + kisi icin oda/fiyat yanitini ceker.
func fetchHotelReservation(ctx context.Context, providerHotelID, hotelType, checkIn, checkOut string, adults int, childAges []int, pkg *packageOpts) (*ReservationResponse, error) {
	start, err := toJollyDate(checkIn)
	if err != nil {
		return nil, err
	}
	end, err := toJollyDate(checkOut)
	if err != nil {
		return nil, err
	}
	originID, originType, originName, pst := "", "Zone", "", ""
	if pkg != nil {
		originID = strconv.Itoa(pkg.Origin.ID)
		originType = "Airport"
		originName = pkg.Origin.Name
		pst = pkg.PackageSearchType
	}
	form := httpx.FormValues{
		{"id", providerHotelID},
		{"startDate", start},
		{"endDate", end},
		{"rooms", buildRoomsParam(adults, childAges)},
		{"originId", originID},
		{"originType", originType},
		{"originName", originName},
		{"hotelType", hotelType},
		{"searchType", "Product"},
		{"customerTrackId", "00000000-0000-0000-0000-000000000000"},
		{"packageSearchType", pst},
		{"trivagoReferenceId", ""},
		{"neredeKalReferenceId", ""},
		{"hadsKey", ""},
		{"utmSource", ""},
	}
	resp, err := httpx.Post(ctx, reservationURL, []byte(form.Encode()), xhrHeaders, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var out ReservationResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("Jolly yaniti parse edilemedi: %w", err)
	}
	return &out, nil
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
	HotelType       string
}

var (
	reSPID   = regexp.MustCompile(`name="SearchParameters\.Id"[^>]*value="([^"]*)"`)
	reSPType = regexp.MustCompile(`name="SearchParameters\.HotelType"[^>]*value="([^"]*)"`)
	reSPName = regexp.MustCompile(`name="SearchParameters\.HotelName"[^>]*value="([^"]*)"`)
)

func resolveHotelFromURL(ctx context.Context, u string) (*resolved, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: 20 * time.Second})
	if err != nil {
		return nil, err
	}
	html := resp.Text()
	id := provider.FirstMatch(reSPID, html)
	if id == "" {
		return nil, fmt.Errorf("Jolly sayfasinda SearchParameters.Id bulunamadi (otel detay URL'i olmali)")
	}
	return &resolved{
		ProviderHotelID: id,
		HotelType:       provider.FirstMatch(reSPType, html),
		HotelName:       provider.DecodeEntities(provider.FirstMatch(reSPName, html)),
	}, nil
}
