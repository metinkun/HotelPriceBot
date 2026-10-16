package setur

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
	base      = "https://www.setur.com.tr"
	searchURL = base + "/api/services/v4/SearchesService/searchesUrl"
	ua        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
	// Setur detay sayfasi tarihli sorguda yavas render ediyor (~20-40sn).
	pageTimeout = 90 * time.Second
)

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

type LowestPrice struct {
	Price                   *float64 `json:"price"`
	CurrencyCode            string   `json:"currencyCode"`
	OriginalPrice           *float64 `json:"originalPrice"`
	OriginalDiscountedPrice *float64 `json:"originalDiscountedPrice"`
	DiscountRatio           *float64 `json:"discountRatio"`
	RoomInformation         string   `json:"roomInformation"`
}

type HotelData struct {
	LowestPrice     *LowestPrice
	RoomInformation string
	GuestAdultCount *int
	GuestChildCount *int
	BoardType       string
	HotelName       string
	City            string
	Country         string
	NightCount      *int
	HasPackage      bool
}

// birthDateFromAge yastan yaklasik dogum tarihi uretir (yyyy-MM-dd).
func birthDateFromAge(age int, today time.Time) string {
	if age < 0 {
		age = 0
	}
	return fmt.Sprintf("%04d-%02d-%02d", today.Year()-age, int(today.Month()), today.Day())
}

// buildRoomParam `<yetiskin>[_<cocukDogumTarihi>...]` — virgul YENI ODA demektir, kullanilmaz.
func buildRoomParam(adults int, childAges []int) string {
	parts := []string{strconv.Itoa(adults)}
	now := time.Now()
	for _, a := range childAges {
		parts = append(parts, birthDateFromAge(a, now))
	}
	return strings.Join(parts, "_")
}

var (
	reLowest     = regexp.MustCompile(`"lowestPrice":\{[^}]*\}`)
	reNights     = regexp.MustCompile(`"nightCount":(\d+)`)
	reMetaTitle  = regexp.MustCompile(`"metaTitle":"([^"]{3,120})"`)
	reGuestAd    = regexp.MustCompile(`"guests":\{"adultCount":(\d+)`)
	reGuestCh    = regexp.MustCompile(`"guests":\{"adultCount":\d+,"childCount":(\d+)`)
	reBoard      = regexp.MustCompile(`"boardType":"([^"]{2,40})"`)
	reCity       = regexp.MustCompile(`"city":"([^"]{2,40})"`)
	reCountry    = regexp.MustCompile(`"country":"([^"]{2,40})"`)
	reHasPackage = regexp.MustCompile(`"hasPackage":true`)
)

func parseLowest(seg string) *LowestPrice {
	m := reLowest.FindString(seg)
	if m == "" {
		return nil
	}
	var lp LowestPrice
	if err := json.Unmarshal([]byte(strings.TrimPrefix(m, `"lowestPrice":`)), &lp); err != nil {
		return nil
	}
	return &lp
}

func intPtr(s string) *int {
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

// fetchHotelPage detay sayfasini tarih/kisi ile ceker ve gomulu (RSC) JSON'dan fiyat alanlarini ayiklar.
func fetchHotelPage(ctx context.Context, slug, checkIn, checkOut string, adults int, childAges []int) (*HotelData, error) {
	u := fmt.Sprintf("%s/%s?in=%s&out=%s&room=%s", base, slug, checkIn, checkOut, buildRoomParam(adults, childAges))
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: pageTimeout})
	if err != nil {
		return nil, err
	}
	raw := strings.ReplaceAll(resp.Text(), `\"`, `"`)

	lp := parseLowest(raw)
	d := &HotelData{
		LowestPrice:     lp,
		GuestAdultCount: intPtr(provider.FirstMatch(reGuestAd, raw)),
		GuestChildCount: intPtr(provider.FirstMatch(reGuestCh, raw)),
		BoardType:       provider.FirstMatch(reBoard, raw),
		HotelName:       provider.FirstMatch(reMetaTitle, raw),
		City:            provider.FirstMatch(reCity, raw),
		Country:         provider.FirstMatch(reCountry, raw),
		NightCount:      intPtr(provider.FirstMatch(reNights, raw)),
		HasPackage:      reHasPackage.MatchString(raw),
	}
	if lp != nil {
		d.RoomInformation = lp.RoomInformation
	}
	return d, nil
}

// fetchPackagePage paket fiyati `packageInitialData.data.hotel.lowestPrice` icindedir.
func fetchPackagePage(ctx context.Context, slug, checkIn, checkOut string, adults int, childAges []int, departureCode, arrivalCode string) (*HotelData, error) {
	params := []string{
		"in=" + checkIn, "out=" + checkOut, "room=" + buildRoomParam(adults, childAges),
		"isPackage=true", "dep=" + url.QueryEscape(strings.ToUpper(departureCode)), "depIsCity=true",
	}
	if arrivalCode != "" {
		params = append(params, "arr="+url.QueryEscape(strings.ToUpper(arrivalCode)))
	}
	u := fmt.Sprintf("%s/%s?%s", base, slug, strings.Join(params, "&"))
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: pageTimeout})
	if err != nil {
		return nil, err
	}
	raw := strings.ReplaceAll(resp.Text(), `\"`, `"`)

	idx := strings.Index(raw, `"packageInitialData"`)
	if idx == -1 {
		return &HotelData{}, nil
	}
	end := idx + 4000
	if end > len(raw) {
		end = len(raw)
	}
	seg := raw[idx:end]
	lp := parseLowest(seg)
	d := &HotelData{
		LowestPrice: lp,
		BoardType:   provider.FirstMatch(reBoard, seg),
		HotelName:   provider.FirstMatch(reMetaTitle, seg),
		City:        provider.FirstMatch(reCity, seg),
		Country:     provider.FirstMatch(reCountry, seg),
		NightCount:  intPtr(provider.FirstMatch(reNights, seg)),
		HasPackage:  true,
	}
	if lp != nil {
		d.RoomInformation = lp.RoomInformation
	}
	return d, nil
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
	Slug            string
	HasPackage      bool
}

var reHost = regexp.MustCompile(`^https?://[^/]+/`)

// slugFromURL https://www.setur.com.tr/<slug>
func slugFromURL(u string) string {
	path := reHost.ReplaceAllString(u, "")
	path = strings.SplitN(path, "?", 2)[0]
	return strings.TrimRight(path, "/")
}

func resolveHotelFromURL(ctx context.Context, u string) (*resolved, error) {
	slug := slugFromURL(u)
	if slug == "" {
		return nil, fmt.Errorf("Setur URL'inden slug cozulemedi")
	}
	body, _ := json.Marshal(map[string]any{"args": []map[string]string{{"s": slug}}})
	resp, err := httpx.Post(ctx, searchURL, body, map[string]string{
		"user-agent": ua, "content-type": "application/json", "accept": "application/json",
		"origin": base, "referer": base + "/" + slug,
	}, 25*time.Second)
	if err != nil {
		return nil, err
	}
	var d struct {
		ID      any    `json:"id"`
		Text    string `json:"text"`
		Type    string `json:"type"`
		Package bool   `json:"package"`
		URL     string `json:"url"`
	}
	_ = json.Unmarshal(resp.Body, &d)
	id := ""
	switch v := d.ID.(type) {
	case float64:
		id = strconv.FormatInt(int64(v), 10)
	case string:
		id = v
	}
	if id == "" || id == "0" || d.Type != "Hotel" {
		return nil, fmt.Errorf("Setur otel bulunamadi (otel detay sayfasi URL'i olmali)")
	}
	r := &resolved{ProviderHotelID: id, HotelName: strings.TrimSpace(d.Text), Slug: slug, HasPackage: d.Package}
	if d.URL != "" {
		r.Slug = d.URL
	}
	return r, nil
}
