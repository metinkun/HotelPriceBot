package tatilbudur

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const (
	base           = "https://www.tatilbudur.com"
	calcURL        = base + "/hotel/calculate-room-price"
	calcPackageURL = base + "/hotel/calculate-package-room-price"
	ua             = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
)

// Tam HTML sayfa cekerken tarayici-gezinme header'lari (403'u asan set).
var pageHeaders = map[string]string{
	"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"accept-language":           "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"user-agent":                ua,
	"sec-ch-ua":                 `"Chromium";v="150", "Google Chrome";v="150", "Not;A=Brand";v="8"`,
	"sec-ch-ua-mobile":          "?0",
	"sec-ch-ua-platform":        `"macOS"`,
	"sec-fetch-dest":            "document",
	"sec-fetch-mode":            "navigate",
	"sec-fetch-site":            "none",
	"upgrade-insecure-requests": "1",
}

type RoomPriceResponse struct {
	View            string `json:"view"`
	AlternativeHTML string `json:"alternativeHtml"`
}

// yyyy-MM-dd -> "d.M.yyyy" (sifirsiz)
func toDotDate(iso string) string {
	p := strings.Split(iso[:min(10, len(iso))], "-")
	if len(p) != 3 {
		return iso
	}
	return fmt.Sprintf("%d.%d.%s", atoi(p[2]), atoi(p[1]), p[0])
}

// yyyy-MM-dd -> "dd/MM/yyyy"
func toSlashDate(iso string) string {
	p := strings.Split(iso[:min(10, len(iso))], "-")
	if len(p) != 3 {
		return iso
	}
	return p[2] + "/" + p[1] + "/" + p[0]
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
		}
	}
	return n
}

type tokenSession struct {
	Token  string
	Cookie string
	HTML   string
}

var (
	reToken     = regexp.MustCompile(`name="_token"\s+value="([^"]+)"`)
	reCSRFMeta  = regexp.MustCompile(`csrf-token"\s+content="([^"]+)"`)
	reToCode    = regexp.MustCompile(`&to=([A-Z]{3})&from=`)
	reHotelID   = regexp.MustCompile(`hotelId=(\d+)`)
	reHotelName = regexp.MustCompile(`<h1[^>]*hotel-name[^>]*>\s*([^<]{2,80})`)
)

// fetchTokenAndCookies detay sayfasini ceker; CSRF _token + oturum cookie'lerini (Akamai dahil) doner.
func fetchTokenAndCookies(ctx context.Context, u string) (*tokenSession, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
	if err != nil {
		return nil, err
	}
	html := resp.Text()
	token := provider.FirstMatch(reToken, html)
	if token == "" {
		token = provider.FirstMatch(reCSRFMeta, html)
	}
	if token == "" {
		return nil, fmt.Errorf("Tatilbudur sayfasinda _token bulunamadi")
	}
	return &tokenSession{Token: token, Cookie: resp.SetCookies(), HTML: html}, nil
}

// buildBaseForm iki fiyat cagrisinin da kullandigi ortak form govdesi (#allHotelTabForm).
func buildBaseForm(token, providerHotelID, checkIn, checkOut string, adults, children int) string {
	quick := fmt.Sprintf("%d Yetişkin", adults)
	if children > 0 {
		quick += fmt.Sprintf(" %d Çocuk", children)
	}
	form := httpx.FormValues{
		{"_token", token},
		{"productType", "hotel"},
		{"hotelId", providerHotelID},
		{"selectedRoom", ""},
		{"selectedPricingId", ""},
		{"selectedMealType", ""},
		{"hotelOldId", ""},
		{"productLoc", ""},
		{"productTypeId", ""},
		{"code", ""},
		{"alertRoom", "0"},
		{"actionPricingId", ""},
		{"currencyId", ""},
		{"actions[]", "0"},
		{"selectedActionCategory", ""},
		{"hidePrice", "0"},
		{"googleRemarketingCategory", ""},
		{"autoPost", "0"},
		{"isFlightPackage", "0"},
		{"adult", fmt.Sprint(adults)},
		{"child", fmt.Sprint(children)},
		{"isCyprusPackageManual", "0"},
		{"priceConfig", ""},
		{"loyaltyPoint", "0"},
		{"daterange-1", toSlashDate(checkIn) + " - " + toSlashDate(checkOut)},
		{"checkInDate", toDotDate(checkIn)},
		{"checkOutDate", toDotDate(checkOut)},
		{"quickPersonCount", quick},
		{"type", ""},
	}
	return form.Encode()
}

func priceHeaders(token, detailURL, cookie string) map[string]string {
	return map[string]string{
		"user-agent":       ua,
		"x-requested-with": "XMLHttpRequest",
		"x-csrf-token":     token,
		"content-type":     "application/x-www-form-urlencoded; charset=UTF-8",
		"accept":           "*/*",
		"origin":           base,
		"referer":          detailURL,
		"cookie":           cookie,
	}
}

func parseView(body []byte) (*RoomPriceResponse, error) {
	var out RoomPriceResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("Tatilbudur yaniti parse edilemedi: %w", err)
	}
	return &out, nil
}

// fetchRoomPrice: detay sayfasindan taze token+cookie -> calculate-room-price POST.
func fetchRoomPrice(ctx context.Context, detailURL, providerHotelID, checkIn, checkOut string, adults, children int) (*RoomPriceResponse, error) {
	s, err := fetchTokenAndCookies(ctx, detailURL)
	if err != nil {
		return nil, err
	}
	body := buildBaseForm(s.Token, providerHotelID, checkIn, checkOut, adults, children)
	resp, err := httpx.Post(ctx, calcURL, []byte(body), priceHeaders(s.Token, detailURL, s.Cookie), 30*time.Second)
	if err != nil {
		return nil, err
	}
	return parseView(resp.Body)
}

// fetchPackagePrice paket (Otel + Ucak + Transfer): ayni form + to (sayfa JS'inden) + from (kullanicidan).
func fetchPackagePrice(ctx context.Context, detailURL, providerHotelID, departureCode, checkIn, checkOut string, adults, children int) (*RoomPriceResponse, string, error) {
	s, err := fetchTokenAndCookies(ctx, detailURL)
	if err != nil {
		return nil, "", err
	}
	dest := provider.FirstMatch(reToCode, s.HTML)
	if dest == "" {
		return nil, "", fmt.Errorf("Tatilbudur varis havalimani kodu bulunamadi (bu otel icin paket sunulmuyor olabilir)")
	}
	body := buildBaseForm(s.Token, providerHotelID, checkIn, checkOut, adults, children) +
		"&to=" + dest + "&from=" + url.QueryEscape(strings.ToUpper(departureCode))
	resp, err := httpx.Post(ctx, calcPackageURL, []byte(body), priceHeaders(s.Token, detailURL, s.Cookie), 45*time.Second)
	if err != nil {
		return nil, "", err
	}
	data, err := parseView(resp.Body)
	return data, dest, err
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
}

func resolveHotelFromURL(ctx context.Context, u string) (*resolved, error) {
	s, err := fetchTokenAndCookies(ctx, u)
	if err != nil {
		return nil, err
	}
	id := provider.FirstMatch(reHotelID, s.HTML)
	if id == "" {
		return nil, fmt.Errorf("Tatilbudur sayfasinda hotelId bulunamadi (otel detay URL'i olmali)")
	}
	return &resolved{ProviderHotelID: id, HotelName: provider.DecodeEntities(provider.FirstMatch(reHotelName, s.HTML))}, nil
}
