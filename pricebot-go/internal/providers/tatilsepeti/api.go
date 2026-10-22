package tatilsepeti

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"pricebot/internal/browser"
	"pricebot/internal/httpx"
)

const (
	priceURL   = "https://www.tatilsepeti.com/hotel/GetHotelListPrice/"
	packageURL = "https://www.tatilsepeti.com/Hotel/GetFlyingPackages"
)

// XHR basliklari. UA tarayici ile ayni tutulur: cf_clearance cerezi UA'ya baglidir.
var defaultHeaders = map[string]string{
	"accept":             "application/json, text/javascript, */*; q=0.01",
	"accept-language":    "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"content-type":       "application/x-www-form-urlencoded; charset=UTF-8",
	"origin":             "https://www.tatilsepeti.com",
	"user-agent":         browser.UA,
	"sec-ch-ua":          `"Chromium";v="150", "Google Chrome";v="150", "Not;A=Brand";v="8"`,
	"sec-ch-ua-mobile":   "?0",
	"sec-ch-ua-platform": `"macOS"`,
	"sec-fetch-dest":     "empty",
	"sec-fetch-mode":     "cors",
	"sec-fetch-site":     "same-origin",
	"x-requested-with":   "XMLHttpRequest",
	"referer":            homeURL,
}

// Tam HTML sayfa (detay) cekerken tarayici-gezinme basliklari. XHR basliklari (x-requested-with,
// origin) ile GET yapilirsa site parca (partial) HTML doner; hidHotelId/forgeryToken bulunmaz.
var pageHeaders = map[string]string{
	"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"accept-language":           "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"user-agent":                browser.UA,
	"sec-ch-ua":                 `"Chromium";v="150", "Google Chrome";v="150", "Not;A=Brand";v="8"`,
	"sec-ch-ua-mobile":          "?0",
	"sec-ch-ua-platform":        `"macOS"`,
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

// postJSON oturumlu form POST; yanit JSON degilse / engellenirse oturumu yenileyip bir kez daha dener.
// referer: istegin yapildigi otel sayfasi (oturum edinimi icin de ayni sayfa acilir).
func (p *Provider) postJSON(ctx context.Context, url string, form httpx.FormValues, referer string, timeout time.Duration) ([]byte, error) {
	if referer == "" {
		referer = homeURL
	}
	body := []byte(form.Encode())
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		s, err := p.sess.get(ctx, referer)
		if err != nil {
			return nil, err
		}
		headers := httpx.Merge(defaultHeaders, map[string]string{"referer": referer})
		if s.Cookie != "" {
			headers["cookie"] = s.Cookie
		}
		if s.Token != "" {
			headers["verificationtoken"] = s.Token
		}
		resp, err := httpx.Post(ctx, url, body, headers, timeout)
		switch {
		case err != nil && looksBlocked(resp, err):
			lastErr = fmt.Errorf("TatilSepeti istegi engellendi: %w", err)
			p.sess.invalidate(s, true)
		case err != nil:
			return nil, err
		case !looksJSON(resp.Body):
			lastErr = fmt.Errorf("TatilSepeti yaniti JSON degil (oturum gecersiz olabilir)")
			p.sess.invalidate(s, looksBlocked(resp, nil))
		default:
			return resp.Body, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

// fetchPrices GetHotelListPrice yanitini ham JSON (any) olarak doner.
func (p *Provider) fetchPrices(ctx context.Context, hotelIDs []string, checkIn, checkOut string, adults, children int, referer string) (any, error) {
	form := httpx.FormValues{
		{"AdultCount", fmt.Sprint(adults)},
		{"ChildCount", fmt.Sprint(children)},
		{"CampaignType", "0"},
		{"CampaignId", ""},
		{"checkinDate", toTSDate(checkIn) + " "},
		{"checkoutDate", toTSDate(checkOut)},
		{"HotelIds", strings.Join(hotelIDs, ",")},
	}
	raw, err := p.postJSON(ctx, priceURL, form, referer, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("TatilSepeti fiyat yaniti parse edilemedi: %w", err)
	}
	return data, nil
}

// ---- Paket (Ucak + Otel + Transfer) ----

type flyingPackagesResponse struct {
	RoomList              string `json:"roomList"`
	FlightListResult      string `json:"flightListResult"`
	TransferList          string `json:"transferList"`
	DetailMessage         string `json:"detailMessage"`
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

func (p *Provider) fetchFlyingPackages(ctx context.Context, hotelID, departureCode, destinationCode, checkIn, checkOut string, adults int, childAges []int, referer string) (*flyingPackagesResponse, error) {
	form := httpx.FormValues{
		{"Search", buildPackageSearchString(adults, childAges, checkIn, checkOut)},
		{"Id", hotelID},
		{"IsFlightPocket", "true"},
		{"depatureCode", strings.ToLower(departureCode)},
		{"destinationCode", strings.ToLower(destinationCode)},
	}
	raw, err := p.postJSON(ctx, packageURL, form, referer, 40*time.Second)
	if err != nil {
		return nil, err
	}
	var out flyingPackagesResponse
	if err := json.Unmarshal(raw, &out); err != nil {
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
	reHotelName = regexp.MustCompile(`id="hidHotelName"[^>]*value="([^"]+)"`)
	reDest      = regexp.MustCompile(`id="nereye"[^>]*data-search-value="([^"]+)"`)
	reTitle     = regexp.MustCompile(`<title>([^<]+)</title>`)
	reTitleTail = regexp.MustCompile(`\s*[-|].*$`)
)

// parseDetailPage detay HTML'inden otel id/ad/destinasyon kodunu cikarir.
func parseDetailPage(html string) (*resolved, error) {
	idM := reHotelID.FindStringSubmatch(html)
	if idM == nil {
		return nil, fmt.Errorf("TatilSepeti sayfasinda hidHotelId bulunamadi (otel detay URL'i olmali)")
	}
	r := &resolved{ProviderHotelID: idM[1]}
	if m := reDest.FindStringSubmatch(html); m != nil {
		r.DestinationCode = m[1]
	}
	if m := reHotelName.FindStringSubmatch(html); m != nil {
		r.HotelName = strings.TrimSpace(m[1])
	} else if m := reTitle.FindStringSubmatch(html); m != nil {
		r.HotelName = strings.TrimSpace(reTitleTail.ReplaceAllString(m[1], ""))
	}
	return r, nil
}

func (p *Provider) resolveHotelFromURL(ctx context.Context, url string) (*resolved, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, url, httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
	if err != nil {
		if looksBlocked(resp, err) && p.sess.bm != nil {
			// Duz HTTP engellendi: sayfayi tarayicida acip HTML'i oradan al.
			return p.resolveViaBrowser(ctx, url)
		}
		return nil, err
	}
	// Ayni GET'ten cikan cerez/token'i oturum olarak sakla (ek istek gerekmez).
	if token := reForgeryTokenMatch(resp.Text()); token != "" || resp.SetCookies() != "" {
		p.sess.store(&session{Cookie: resp.SetCookies(), Token: token, At: time.Now(), Source: sourceHTTP})
	}
	return parseDetailPage(resp.Text())
}

func reForgeryTokenMatch(html string) string {
	m := reForgeryToken.FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	return m[1]
}

func (p *Provider) resolveViaBrowser(ctx context.Context, url string) (*resolved, error) {
	page, cancel, err := p.sess.bm.NewPage(ctx, 90*time.Second)
	if err != nil {
		return nil, fmt.Errorf("tatilsepeti: sekme acilamadi: %w", err)
	}
	defer cancel()
	if err := browser.Navigate(page, url, 1500*time.Millisecond); err != nil {
		return nil, fmt.Errorf("tatilsepeti: sayfa acilamadi: %w", err)
	}
	var html string
	if err := browser.Eval(page, `document.documentElement.outerHTML`, &html); err != nil {
		return nil, err
	}
	return parseDetailPage(html)
}
