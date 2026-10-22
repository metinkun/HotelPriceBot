package tatildukkani

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"pricebot/internal/httpx"
)

const (
	siteBase    = "https://www.tatildukkani.com"
	apiBase     = "https://api.tatildukkani.com"
	guestURL    = apiBase + "/guest"
	socketLogin = apiBase + "/api/socket-login"
	slugURL     = apiBase + "/api/seo_page/slug"
	initURL     = apiBase + "/api/searchlog/hotel/init"
	ua          = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

	// adultBirthDate: site yetiskinleri sabit bir dogum tarihiyle temsil eder (sayi = virgulle ayrilmis tekrar).
	adultBirthDate = "1970-01-01"
	// guestTokenTTL misafir apiToken'inin yeniden cekilme suresi (site cerezi 23 saat tutuyor).
	guestTokenTTL = 6 * time.Hour
)

var baseHeaders = map[string]string{
	"accept":          "application/json, text/plain, */*",
	"accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"content-type":    "application/json",
	"user-agent":      ua,
	"origin":          siteBase,
	"referer":         siteBase + "/",
}

// envelope API'nin tum yanitlarini saran zarf.
type envelope struct {
	Success   bool            `json:"success"`
	Data      json.RawMessage `json:"data"`
	Error     any             `json:"error"`
	ErrorCode any             `json:"errorCode"`
}

func decodeEnvelope(body []byte, what string) (json.RawMessage, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("Tatildukkani %s yaniti parse edilemedi: %w", what, err)
	}
	if !env.Success {
		return nil, fmt.Errorf("Tatildukkani %s basarisiz: %v (kod %v)", what, env.Error, env.ErrorCode)
	}
	return env.Data, nil
}

// ---- Misafir token ----

type guest struct {
	APIToken string `json:"apiToken"`
	Type     string `json:"type"`
}

var (
	guestMu      sync.Mutex
	cachedGuest  *guest
	guestFetched time.Time
)

// getGuest misafir apiToken'ini (GET /guest) ceker; TTL boyunca cache'ler.
func getGuest(ctx context.Context) (*guest, error) {
	guestMu.Lock()
	defer guestMu.Unlock()
	if cachedGuest != nil && time.Since(guestFetched) < guestTokenTTL {
		return cachedGuest, nil
	}
	resp, err := httpx.Get(ctx, guestURL, baseHeaders, 20*time.Second)
	if err != nil {
		return nil, err
	}
	data, err := decodeEnvelope(resp.Body, "guest")
	if err != nil {
		return nil, err
	}
	var g guest
	if err := json.Unmarshal(data, &g); err != nil || g.APIToken == "" {
		return nil, fmt.Errorf("Tatildukkani misafir token alinamadi")
	}
	if g.Type == "" {
		g.Type = "GUEST"
	}
	cachedGuest, guestFetched = &g, time.Now()
	return &g, nil
}

func invalidateGuest() {
	guestMu.Lock()
	cachedGuest = nil
	guestMu.Unlock()
}

func authHeaders(g *guest) map[string]string {
	return httpx.Merge(baseHeaders, map[string]string{"x-auth-token": g.APIToken, "tokenType": g.Type})
}

// postJSON token'li POST; 401/403'te misafir token'i bir kez yenileyip tekrar dener.
func postJSON(ctx context.Context, url string, body any, timeout time.Duration, what string) (json.RawMessage, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		g, err := getGuest(ctx)
		if err != nil {
			return nil, err
		}
		resp, err := httpx.Post(ctx, url, raw, authHeaders(g), timeout)
		if err != nil {
			if se, ok := err.(*httpx.StatusError); ok && (se.Status == 401 || se.Status == 403) && attempt == 0 {
				invalidateGuest()
				continue
			}
			return nil, err
		}
		return decodeEnvelope(resp.Body, what)
	}
	return nil, fmt.Errorf("Tatildukkani %s: yetkilendirme basarisiz", what)
}

// fetchSocketToken POST /api/socket-login -> ws oturumu icin socketToken.
func fetchSocketToken(ctx context.Context) (string, error) {
	data, err := postJSON(ctx, socketLogin, map[string]any{}, 20*time.Second, "socket-login")
	if err != nil {
		return "", err
	}
	var d struct {
		SocketToken string `json:"socketToken"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.SocketToken == "" {
		return "", fmt.Errorf("Tatildukkani socketToken alinamadi")
	}
	return d.SocketToken, nil
}

// ---- Tarih / kisi formatlari ----

var reISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

func parseISO(s string) (time.Time, error) {
	if !reISO.MatchString(s) {
		return time.Time{}, fmt.Errorf("Gecersiz tarih formati: %s (yyyy-MM-dd bekleniyor)", s)
	}
	return time.Parse("2006-01-02", s[:10])
}

// addMonthsClamped moment.js gibi ay ekler: gun, hedef ayin son gununu asarsa kirpilir (tasma yok).
func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	if d > last {
		d = last
	}
	return time.Date(first.Year(), first.Month(), d, 0, 0, 0, 0, time.UTC)
}

// childBirthDate sitenin $calcChildAge'i: checkOut - yas yil - 1 ay - 10 gun (yyyy-MM-dd).
func childBirthDate(checkOut time.Time, age int) string {
	if age < 0 {
		age = 0
	}
	d := addMonthsClamped(checkOut, -age*12-1)
	return d.AddDate(0, 0, -10).Format("2006-01-02")
}

// adtParam "1970-01-01,1970-01-01" (yetiskin basina bir dogum tarihi).
func adtParam(adults int) string {
	if adults < 1 {
		adults = 1
	}
	parts := make([]string, adults)
	for i := range parts {
		parts[i] = adultBirthDate
	}
	return strings.Join(parts, ",")
}

// chdParam cocuk dogum tarihleri virgulle; cocuk yoksa "".
func chdParam(checkOut time.Time, childAges []int) string {
	parts := make([]string, 0, len(childAges))
	for _, a := range childAges {
		parts = append(parts, childBirthDate(checkOut, a))
	}
	return strings.Join(parts, ",")
}

// ---- Sayfa (seo_page/slug) ----

type pageData struct {
	Type        string          `json:"type"`
	TypeID      int             `json:"typeId"`
	Title       string          `json:"title"`
	Slug        string          `json:"slug"`
	AirportCode string          `json:"airportCode"`
	Result      json.RawMessage `json:"result"`
}

// hotelResult otel detay sayfasinin result nesnesi (yalnizca kullanilan alanlar).
type hotelResult struct {
	ID                int             `json:"id"`
	Name              string          `json:"name"`
	IsAvailable       bool            `json:"isAvailable"`
	IsPrice           bool            `json:"isPrice"`
	AvabilityCriteria json.RawMessage `json:"avabilityCriteria"`
	Destination       []struct {
		Name string `json:"name"`
	} `json:"destination"`
	HotelDestination struct {
		Name string `json:"name"`
	} `json:"hotelDestination"`
}

// HotelPage detay sayfasi + otel kriterleri (socket aramasinin girdisi).
type HotelPage struct {
	HotelID     int
	Title       string
	Slug        string
	AirportCode string
	City        string
	Criteria    json.RawMessage
}

// fetchHotelPage POST /api/seo_page/slug: slug -> otel id, ad, sehir ve avabilityCriteria.
// checkIn/checkOut bos olabilir (yalnizca cozumleme icin).
func fetchHotelPage(ctx context.Context, slug, checkIn, checkOut string, adults int, chd string) (*HotelPage, error) {
	adt := ""
	if checkIn != "" {
		adt = adtParam(adults)
	}
	body := map[string]any{
		"slug": slug, "checkIn": checkIn, "checkOut": checkOut, "origin": "",
		"periods": []string{}, "destination": []int{}, "adt": adt, "chd": chd,
	}
	data, err := postJSON(ctx, slugURL, body, 30*time.Second, "seo_page/slug")
	if err != nil {
		return nil, err
	}
	return parseHotelPage(data)
}

func parseHotelPage(data json.RawMessage) (*HotelPage, error) {
	var pd pageData
	if err := json.Unmarshal(data, &pd); err != nil {
		return nil, fmt.Errorf("Tatildukkani sayfa verisi parse edilemedi: %w", err)
	}
	if pd.Type != "HOTEL" || pd.TypeID <= 0 {
		return nil, fmt.Errorf("Tatildukkani URL'i otel detay sayfasi degil (tip: %q)", pd.Type)
	}
	var hr hotelResult
	if err := json.Unmarshal(pd.Result, &hr); err != nil {
		return nil, fmt.Errorf("Tatildukkani otel sonucu parse edilemedi: %w", err)
	}
	city := hr.HotelDestination.Name
	if city == "" && len(hr.Destination) > 0 {
		city = hr.Destination[len(hr.Destination)-1].Name
	}
	title := pd.Title
	if title == "" {
		title = hr.Name
	}
	return &HotelPage{
		HotelID: pd.TypeID, Title: title, Slug: pd.Slug, AirportCode: pd.AirportCode, City: city,
		Criteria: hr.AvabilityCriteria,
	}, nil
}

// ---- Arama baslatma (searchlog/hotel/init) ----

// initSearch sunucuda arama kaydi acar; donen uid socket uzerinden sonucu almak icin kullanilir.
// hotelList tam avabilityCriteria nesnesi olmali (yalnizca hotelId ile socket sonuc gondermiyor).
func initSearch(ctx context.Context, hotelID int, checkIn, checkOut string, adults int, chd string, criteria json.RawMessage) (string, error) {
	if len(criteria) == 0 || string(criteria) == "null" {
		return "", fmt.Errorf("Tatildukkani avabilityCriteria bos; otel sayfasi fiyat vermiyor olabilir")
	}
	body := map[string]any{
		"id": hotelID, "type": "HOTEL", "userId": nil,
		"checkIn": checkIn, "checkOut": checkOut,
		"adt": adtParam(adults), "chd": chd,
		"hotelList": []json.RawMessage{criteria},
	}
	data, err := postJSON(ctx, initURL, body, 20*time.Second, "searchlog/hotel/init")
	if err != nil {
		return "", err
	}
	var d struct {
		UID string `json:"uid"`
	}
	if err := json.Unmarshal(data, &d); err != nil || d.UID == "" {
		return "", fmt.Errorf("Tatildukkani arama uid alinamadi")
	}
	return d.UID, nil
}

// fetchAvailability tam akis: sayfa -> init -> socket; oda/fiyat listesini doner.
func fetchAvailability(ctx context.Context, slug, checkIn, checkOut string, adults int, childAges []int) (*HotelPage, *AvailableHotel, error) {
	in, err := parseISO(checkIn)
	if err != nil {
		return nil, nil, err
	}
	out, err := parseISO(checkOut)
	if err != nil {
		return nil, nil, err
	}
	if !out.After(in) {
		return nil, nil, fmt.Errorf("checkOut checkIn'den sonra olmali")
	}
	ci, co := in.Format("2006-01-02"), out.Format("2006-01-02")
	chd := chdParam(out, childAges)

	page, err := fetchHotelPage(ctx, slug, ci, co, adults, chd)
	if err != nil {
		return nil, nil, err
	}
	socketToken, err := fetchSocketToken(ctx)
	if err != nil {
		return page, nil, err
	}
	uid, err := initSearch(ctx, page.HotelID, ci, co, adults, chd, page.Criteria)
	if err != nil {
		return page, nil, err
	}
	av, err := waitAvailability(ctx, socketToken, uid)
	if err != nil {
		return page, nil, err
	}
	return page, av, nil
}

// ---- URL'den otel verisi cozme ----

var reHost = regexp.MustCompile(`^https?://[^/]+/`)

// slugFromURL https://www.tatildukkani.com/<slug>[?...] -> slug (ilk yol parcasi).
func slugFromURL(u string) string {
	p := reHost.ReplaceAllString(strings.TrimSpace(u), "")
	p = strings.SplitN(p, "?", 2)[0]
	p = strings.SplitN(p, "#", 2)[0]
	p = strings.Trim(p, "/")
	if i := strings.Index(p, "/"); i >= 0 {
		p = p[:i]
	}
	return p
}

func resolveHotelFromURL(ctx context.Context, u string) (*HotelPage, error) {
	slug := slugFromURL(u)
	if slug == "" || slug == "search" {
		return nil, fmt.Errorf("Tatildukkani URL'inden otel slug'i cozulemedi (ornek: https://www.tatildukkani.com/acapulco-resort-convention-spa)")
	}
	page, err := fetchHotelPage(ctx, slug, "", "", 0, "")
	if err != nil {
		if se, ok := err.(*httpx.StatusError); ok && se.Status == http.StatusNotFound {
			return nil, fmt.Errorf("Tatildukkani otel bulunamadi: %s", slug)
		}
		return nil, err
	}
	if page.Slug == "" {
		page.Slug = slug
	}
	return page, nil
}

func itoa(n int) string { return strconv.Itoa(n) }
