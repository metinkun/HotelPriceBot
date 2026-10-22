package mngturizm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const (
	baseURL  = "https://www.mngturizm.com"
	indexURL = baseURL + "/index.php"
	// otelprice: oda/pansiyon fiyat listesi (HTML parcasi). transfervar=1 + ucus blob'lari ile paket fiyati doner.
	otelPriceURL = indexURL + "?s=otelprice"
	ua           = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
	maxChildren  = 4
)

// Cloudflare: Referer basligi (mngturizm.com) olmadan XHR uclari JS challenge (403) doner; cerez gerekmez.
func xhrHeaders(referer string) map[string]string {
	if referer == "" {
		referer = baseURL + "/"
	}
	return map[string]string{
		"accept":           "*/*",
		"accept-language":  "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
		"content-type":     "application/x-www-form-urlencoded; charset=UTF-8",
		"origin":           baseURL,
		"user-agent":       ua,
		"x-requested-with": "XMLHttpRequest",
		"referer":          referer,
	}
}

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

var (
	reISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
	reTR  = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}`)
)

// toMNGDate "2026-12-05" (veya "05.12.2026") -> "05.12.2026"
func toMNGDate(input string) (string, error) {
	if reISO.MatchString(input) {
		p := strings.Split(input[:10], "-")
		return p[2] + "." + p[1] + "." + p[0], nil
	}
	if reTR.MatchString(input) {
		return input[:10], nil
	}
	return "", fmt.Errorf("Gecersiz tarih formati: %s (yyyy-MM-dd bekleniyor)", input)
}

// childParams cocuk yaslarini sitenin cocuk1..cocuk4 / cocuksayi alanlarina cevirir.
// Site formu: -1 = yok, 0 = bebek, 1..17 yas. En fazla 4 cocuk.
func childParams(childAges []int) (ages [maxChildren]string, count string) {
	n := 0
	for i := 0; i < maxChildren; i++ {
		ages[i] = "-1"
		if i < len(childAges) {
			a := childAges[i]
			if a < 0 {
				a = 0
			}
			if a > 17 {
				a = 17
			}
			ages[i] = strconv.Itoa(a)
			n++
		}
	}
	return ages, strconv.Itoa(n)
}

// flightSelection paket sorgusu icin secilmis gidis/donus ucus blob'lari.
type flightSelection struct {
	MinToplam string
	Pax       string
	Outbound  Flight
	Return    Flight
}

// fetchRoomPrices otelprice ucunu cagirir; ham HTML parcasini doner.
// pkg nil ise "Sadece Otel", degilse "Otel + Ucak (+ transfer)" fiyatlari.
func fetchRoomPrices(ctx context.Context, hotelID, toID, referer, checkIn, checkOut string, adults int, childAges []int, pkg *flightSelection) (string, error) {
	giris, err := toMNGDate(checkIn)
	if err != nil {
		return "", err
	}
	cikis, err := toMNGDate(checkOut)
	if err != nil {
		return "", err
	}
	if adults <= 0 {
		adults = 2
	}
	ages, count := childParams(childAges)
	form := httpx.FormValues{
		{"otel", hotelID},
		{"giris", giris},
		{"cikis", cikis},
		{"yetiskin", strconv.Itoa(adults)},
		{"cocuk1", ages[0]},
		{"cocuk2", ages[1]},
		{"cocuk3", ages[2]},
		{"cocuk4", ages[3]},
		{"cocuksayi", count},
	}
	if pkg == nil {
		form = append(form, [2]string{"transfervar", "0"}, [2]string{"mintoplam", ""},
			[2]string{"gidisUcusDeger", ""}, [2]string{"donusUcusDeger", ""},
			[2]string{"gidisrezbilgi", ""}, [2]string{"donusrezbilgi", ""}, [2]string{"pax", ""})
	} else {
		form = append(form, [2]string{"transfervar", "1"}, [2]string{"mintoplam", pkg.MinToplam},
			[2]string{"gidisUcusDeger", pkg.Outbound.Fiyat}, [2]string{"donusUcusDeger", pkg.Return.Fiyat},
			[2]string{"gidisrezbilgi", pkg.Outbound.Book}, [2]string{"donusrezbilgi", pkg.Return.Book}, [2]string{"pax", pkg.Pax})
	}
	form = append(form, [2]string{"toid", toID})

	resp, err := httpx.Post(ctx, otelPriceURL, []byte(form.Encode()), xhrHeaders(referer), 40*time.Second)
	if err != nil {
		return "", err
	}
	return resp.Text(), nil
}

// ---- Ucus arama (paket) ----

// Flight flightsearch yanitindaki bir ucus kutusu.
type Flight struct {
	Direction string  // oneway-flight | return-flight
	Company   string  // PC, VF, KHY ...
	Provider  string  // data-xmlsaglayici: Pegasus, AJET, CyprusRoyal ...
	Surcharge float64 // data-total: pakete eklenen ucus farki (0 = dahil)
	Depart    string  // HH:MM
	Arrive    string
	Fiyat     string // sifreli data-fiyat (gidisUcusDeger/donusUcusDeger)
	Book      string // sifreli data-book (gidisrezbilgi/donusrezbilgi)
	Order     int
}

// FlightSearch flightsearch yanitinin cozulmus hali.
type FlightSearch struct {
	MinToplam string
	Pax       string
	Outbound  []Flight
	Return    []Flight
}

var (
	reFlightBox  = regexp.MustCompile(`(?s)<div class="flight-box ([a-z-]+)[^"]*"([^>]*)>(.*?)<input class="flight-box-select"([^>]*)>`)
	reMinToplam  = regexp.MustCompile(`<input[^>]*name="mintoplam"[^>]*value="([^"]*)"|<input[^>]*value="([^"]*)"[^>]*name="mintoplam"`)
	rePax        = regexp.MustCompile(`<input[^>]*value="([^"]*)"[^>]*id="pax"`)
	reDepartTime = regexp.MustCompile(`class="departure-time">([^<]*)<`)
	reArriveTime = regexp.MustCompile(`class="departure-time2">([^<]*)<`)
	reOrder      = regexp.MustCompile(`order:\s*(\d+)`)
)

func attr(block, name string) string {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `="([^"]*)"`)
	return provider.FirstMatch(re, block)
}

func num(s string) float64 {
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return n
}

// parseFlightSearch flightsearch HTML'inden ucus kutularini ve gizli alanlari cikarir.
func parseFlightSearch(html string) *FlightSearch {
	fs := &FlightSearch{Pax: provider.FirstMatch(rePax, html)}
	if m := reMinToplam.FindStringSubmatch(html); m != nil {
		fs.MinToplam = m[1] + m[2]
	}
	for _, m := range reFlightBox.FindAllStringSubmatch(html, -1) {
		box, inner, radio := m[2], m[3], m[4]
		f := Flight{
			Direction: m[1],
			Company:   attr(box, "data-company"),
			Provider:  attr(box, "data-xmlsaglayici"),
			Surcharge: num(attr(radio, "data-total")),
			Depart:    strings.TrimSpace(provider.FirstMatch(reDepartTime, inner)),
			Arrive:    strings.TrimSpace(provider.FirstMatch(reArriveTime, inner)),
			Fiyat:     attr(radio, "data-fiyat"),
			Book:      attr(radio, "data-book"),
			Order:     int(num(provider.FirstMatch(reOrder, box))),
		}
		if f.Fiyat == "" || f.Book == "" {
			continue
		}
		switch f.Direction {
		case "oneway-flight":
			fs.Outbound = append(fs.Outbound, f)
		case "return-flight":
			fs.Return = append(fs.Return, f)
		}
	}
	return fs
}

// selectFlights en dusuk ucus farkli gidis + ona uygun donus ucusunu secer.
// Site kurallari: gidis CyprusRoyal ise donus de CyprusRoyal olmali (degilse olmamali);
// KHY (Kibris Havayollari) tek yon biletlenemez, iki yon de KHY olmali.
func selectFlights(fs *FlightSearch) (*flightSelection, error) {
	if fs == nil || len(fs.Outbound) == 0 || len(fs.Return) == 0 {
		return nil, fmt.Errorf("MNG Turizm: secilen tarih/havalimani icin ucus bulunamadi")
	}
	cheapest := func(list []Flight) Flight {
		sorted := append([]Flight{}, list...)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Surcharge != sorted[j].Surcharge {
				return sorted[i].Surcharge < sorted[j].Surcharge
			}
			return sorted[i].Order < sorted[j].Order
		})
		return sorted[0]
	}
	out := cheapest(fs.Outbound)
	var candidates []Flight
	for _, r := range fs.Return {
		if (r.Provider == "CyprusRoyal") != (out.Provider == "CyprusRoyal") {
			continue
		}
		if (r.Company == "KHY") != (out.Company == "KHY") {
			continue
		}
		candidates = append(candidates, r)
	}
	if len(candidates) == 0 {
		candidates = fs.Return
	}
	return &flightSelection{MinToplam: fs.MinToplam, Pax: fs.Pax, Outbound: out, Return: cheapest(candidates)}, nil
}

// fetchFlights otel sayfasindaki "Otel + Ucak" sekmesinin flightsearch cagrisini tekrarlar.
// fromIATA: kalkis (orn. SAW); toID: otelin sayfasindaki varis kodu (orn. air-ECN).
func fetchFlights(ctx context.Context, hotelID, toID, referer, fromIATA, checkIn, checkOut string, adults int, childAges []int) (*FlightSearch, error) {
	dep, err := toMNGDate(checkIn)
	if err != nil {
		return nil, err
	}
	ret, err := toMNGDate(checkOut)
	if err != nil {
		return nil, err
	}
	if adults <= 0 {
		adults = 2
	}
	ages, count := childParams(childAges)
	from := strings.ToUpper(strings.TrimSpace(fromIATA))
	to := strings.TrimPrefix(toID, "air-")
	q := url.Values{
		"otelid": {hotelID}, "from": {from}, "to": {to},
		"departuredate": {dep}, "returndate": {ret},
		"adult": {strconv.Itoa(adults)}, "child": {count}, "infant": {"0"},
		"fromid": {"air-" + from}, "toid": {toID},
		"s": {"flightsearch"}, "search": {"otel"}, "ou": {"otel+ucak"}, "type": {"round"},
		"transfervar":  {"1"},
		"childrenages": {strings.Join(ages[:], ",")},
	}
	resp, err := httpx.Get(ctx, indexURL+"?"+q.Encode(), xhrHeaders(referer), 45*time.Second)
	if err != nil {
		return nil, err
	}
	return parseFlightSearch(resp.Text()), nil
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
	City            string
	ToID            string // ucus varis kodu (air-ECN, air-AYT ...)
	HasPriceForm    bool   // online fiyat formu var mi (yoksa "Talep Et" oteli)
	HasFlightTab    bool
}

var (
	reRezOtelID = regexp.MustCompile(`id="rezotelid"\s+value="(\d+)"`)
	reDataID    = regexp.MustCompile(`data-id="(\d+)"`)
	reToID      = regexp.MustCompile(`id="toid"\s+value="([^"]*)"`)
	reProdName  = regexp.MustCompile(`"CD_ProdName":\s*"([^"]*)"`)
	reArea      = regexp.MustCompile(`"CD_Area":\s*"([^"]*)"`)
	reTitle     = regexp.MustCompile(`<title>([^<]*)</title>`)
	reDomain    = regexp.MustCompile(`(?i)^https?://(www\.)?mngturizm\.com/[a-z0-9-]+`)
)

func parseDetailPage(html string) *resolved {
	r := &resolved{
		ProviderHotelID: provider.FirstMatch(reRezOtelID, html),
		ToID:            provider.FirstMatch(reToID, html),
		HasPriceForm:    strings.Contains(html, `id="fiyat-hesapla"`),
		HasFlightTab:    strings.Contains(html, `id="otelUcakTransferBtn"`),
	}
	if r.ProviderHotelID == "" {
		r.ProviderHotelID = provider.FirstMatch(reDataID, html)
	}
	r.HotelName = provider.DecodeEntities(provider.FirstMatch(reProdName, html))
	if r.HotelName == "" {
		t := provider.DecodeEntities(provider.FirstMatch(reTitle, html))
		t = strings.TrimSuffix(t, " - MNG Turizm")
		r.HotelName = strings.TrimSpace(strings.TrimSuffix(t, "Fiyatları"))
	}
	r.City = provider.DecodeEntities(provider.FirstMatch(reArea, html))
	return r
}

func resolveHotelFromURL(ctx context.Context, u string) (*resolved, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
	if err != nil {
		return nil, err
	}
	r := parseDetailPage(resp.Text())
	if r.ProviderHotelID == "" {
		return nil, fmt.Errorf("MNG Turizm sayfasinda otel id (rezotelid) bulunamadi (otel detay URL'i olmali)")
	}
	return r, nil
}
