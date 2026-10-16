package tatilsepeti

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"pricebot/internal/httpx"
)

const (
	priceURL   = "https://www.tatilsepeti.com/hotel/GetHotelListPrice/"
	packageURL = "https://www.tatilsepeti.com/Hotel/GetFlyingPackages"
)

var defaultHeaders = map[string]string{
	"accept":           "application/json, text/javascript, */*; q=0.01",
	"accept-language":  "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"cache-control":    "no-cache",
	"content-type":     "application/x-www-form-urlencoded; charset=UTF-8",
	"origin":           "https://www.tatilsepeti.com",
	"pragma":           "no-cache",
	"user-agent":       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
	"x-requested-with": "XMLHttpRequest",
	"referer":          "https://www.tatilsepeti.com/",
}

var pageHeaders = map[string]string{
	"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"accept-language":           "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"user-agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
	"sec-fetch-dest":            "document",
	"sec-fetch-mode":            "navigate",
	"sec-fetch-site":            "none",
	"upgrade-insecure-requests": "1",
}

// yyyy-MM-dd -> dd.MM.yyyy
func toTSDate(iso string) string {
	p := strings.Split(iso[:min(10, len(iso))], "-")
	if len(p) != 3 {
		return iso
	}
	return p[2] + "." + p[1] + "." + p[0]
}

// fetchPrices GetHotelListPrice yanitini ham JSON (any) olarak doner.
func (p *Provider) fetchPrices(ctx context.Context, hotelIDs []string, checkIn, checkOut string, adults, children int) (any, error) {
	form := httpx.FormValues{
		{"AdultCount", fmt.Sprint(adults)},
		{"ChildCount", fmt.Sprint(children)},
		{"CampaignType", "0"},
		{"CampaignId", ""},
		{"checkinDate", toTSDate(checkIn) + " "},
		{"checkoutDate", toTSDate(checkOut)},
		{"HotelIds", strings.Join(hotelIDs, ",")},
	}
	headers := httpx.Merge(defaultHeaders, map[string]string{"cookie": p.cookie})
	resp, err := httpx.Post(ctx, priceURL, []byte(form.Encode()), headers, 15*time.Second)
	if err != nil {
		return nil, err
	}
	var data any
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		// JSON degilse ham metni don (normalizer musait degil olarak isler)
		return nil, fmt.Errorf("TatilSepeti yaniti JSON degil (cookie suresi dolmus olabilir)")
	}
	return data, nil
}

// ---- Paket (Ucak + Otel + Transfer) ----

type flyingPackagesResponse struct {
	RoomList              string `json:"roomList"`
	FlightListResult      string `json:"flightListResult"`
	TransferList          string `json:"transferList"`
	IsFlyingPackageOnline bool   `json:"IsFlyingPackageOnline"`
	CityCode              any    `json:"CityCode"`
}

// buildPackageSearchString: oda:2;tarih:26.07.2026,31.07.2026;click:true  (cocuklu: oda:2,5,8)
func buildPackageSearchString(adults int, childAges []int, checkIn, checkOut string) string {
	oda := fmt.Sprintf("oda:%d", adults)
	for _, a := range childAges {
		oda += fmt.Sprintf(",%d", a)
	}
	return fmt.Sprintf("%s;tarih:%s,%s;click:true", oda, toTSDate(checkIn), toTSDate(checkOut))
}

func (p *Provider) fetchFlyingPackages(ctx context.Context, hotelID, departureCode, destinationCode, checkIn, checkOut string, adults int, childAges []int) (*flyingPackagesResponse, error) {
	form := httpx.FormValues{
		{"Search", buildPackageSearchString(adults, childAges, checkIn, checkOut)},
		{"Id", hotelID},
		{"IsFlightPocket", "true"},
		{"depatureCode", strings.ToLower(departureCode)},
		{"destinationCode", strings.ToLower(destinationCode)},
	}
	headers := httpx.Merge(defaultHeaders)
	if p.cookie != "" {
		headers["cookie"] = p.cookie
	}
	if p.verificationToken != "" {
		headers["verificationtoken"] = p.verificationToken
	}
	resp, err := httpx.Post(ctx, packageURL, []byte(form.Encode()), headers, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var out flyingPackagesResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("TatilSepeti paket yaniti parse edilemedi: %w", err)
	}
	return &out, nil
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
	DestinationCode string
}

var (
	reHotelID   = regexp.MustCompile(`hidHotelId"[^>]*value="(\d+)"`)
	reDest      = regexp.MustCompile(`id="nereye"[^>]*data-search-value="([^"]+)"`)
	reTitle     = regexp.MustCompile(`<title>([^<]+)</title>`)
	reTitleTail = regexp.MustCompile(`\s*[-|].*$`)
)

func (p *Provider) resolveHotelFromURL(ctx context.Context, url string) (*resolved, error) {
	headers := httpx.Merge(pageHeaders)
	if p.cookie != "" {
		headers["cookie"] = p.cookie
	}
	resp, err := httpx.Do(ctx, http.MethodGet, url, httpx.Options{Headers: headers, Timeout: 20 * time.Second})
	if err != nil {
		return nil, err
	}
	html := resp.Text()
	idM := reHotelID.FindStringSubmatch(html)
	if idM == nil {
		return nil, fmt.Errorf("TatilSepeti sayfasinda hidHotelId bulunamadi (otel detay URL'i olmali)")
	}
	r := &resolved{ProviderHotelID: idM[1]}
	if m := reDest.FindStringSubmatch(html); m != nil {
		r.DestinationCode = m[1]
	}
	if m := reTitle.FindStringSubmatch(html); m != nil {
		r.HotelName = strings.TrimSpace(reTitleTail.ReplaceAllString(m[1], ""))
	}
	return r, nil
}
