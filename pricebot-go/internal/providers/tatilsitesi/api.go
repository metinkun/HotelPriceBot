package tatilsitesi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const (
	base       = "https://www.tatilsitesi.com"
	methodsURL = base + "/Methods.aspx/"
	ua         = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

	// hotelDetailSearch ~3-5 sn; GetAirSearch ilk cagrida ~35 sn surebiliyor (saglayici tarafinda senkron arama).
	roomTimeout     = 45 * time.Second
	transferTimeout = 30 * time.Second
	airTimeout      = 75 * time.Second
	airMaxPolls     = 10
	airPollDelay    = 2 * time.Second
	// flightProvider: sayfa JS'inde sabit (TestID=37 degilse 16).
	flightProvider = 16
	maxChildren    = 3
)

// ASP.NET WebMethod cagrilari: JSON govde, yanit {"d": ...}. Cerez/token gerekmiyor.
var jsonHeaders = map[string]string{
	"accept":           "application/json, text/javascript, */*; q=0.01",
	"accept-language":  "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"content-type":     "application/json; charset=UTF-8",
	"origin":           base,
	"referer":          base + "/",
	"user-agent":       ua,
	"x-requested-with": "XMLHttpRequest",
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

// ---- Yanit tipleri ----

type PriceDetailInfo struct {
	InType    int    `json:"inType"`
	StMessage string `json:"stMessage"`
}

// HotelRow hotelDetailSearch yanitindaki bir oda+pansiyon satiri (konaklama toplami, tum indirimler uygulanmis).
type HotelRow struct {
	SaleCurrency      string            `json:"SaleCurrency"`
	SaleAmount        float64           `json:"SaleAmount"`
	NotDiscountAmount float64           `json:"NotDiscountAmount"`
	RoomCategoryName  string            `json:"RoomCategoryName"`
	PansionName       string            `json:"PansionName"`
	Pansion           string            `json:"Pansion"`
	RoomCategory      string            `json:"RoomCategory"`
	ContractID        int               `json:"ContractID"`
	Release           bool              `json:"Release"`
	StopSale          bool              `json:"StopSale"`
	AskSale           bool              `json:"AskSale"`
	NonRefundable     bool              `json:"NonRefundable"`
	Allotment         int               `json:"Allotment"`
	PriceDetailInfos  []PriceDetailInfo `json:"PriceDetailInfos"`
}

// Bookable sayfadaki "Rezervasyon" butonu kosulu (aksi halde "Talep Et").
func (r HotelRow) Bookable() bool {
	return !r.Release && !r.StopSale && !r.AskSale && r.SaleAmount > 0
}

type hotelSearchResponse struct {
	D []HotelRow `json:"d"`
}

type Transfer struct {
	TransferID       int     `json:"TransferID"`
	Description      string  `json:"Description"`
	SaleAmount       float64 `json:"SaleAmount"`
	SaleCurrencyName string  `json:"SaleCurrencyName"`
	RouteTypeName    string  `json:"RouteTypeName"`
}

type TransferResponse struct {
	D struct {
		RoundTripTransferList []Transfer `json:"RoundTripTransferList"`
		DepartureTransferList []Transfer `json:"DepartureTransferList"`
		ArrivalTransferList   []Transfer `json:"ArrivalTransferList"`
	} `json:"d"`
}

type FlightSegment struct {
	DepartureAirport string `json:"DepartureAirport"`
	ArrivalAirport   string `json:"ArrivalAirport"`
	OperatingAirline string `json:"OperatingAirline"`
	AirlineName      string `json:"AirlineName"`
	Cabin            string `json:"Cabin"`
	DepartureTime    string `json:"DepartureTime"`
}

// FlightOption tek yon (OptionFlag OUT/IN) veya Recommendations icinde gidis-donus kombinasyonu.
type FlightOption struct {
	OptionFlag string          `json:"OptionFlag"`
	ProductID  string          `json:"ProductId"`
	TotalFare  float64         `json:"TotalFare"`
	Currency   string          `json:"Currency"`
	Segments   []FlightSegment `json:"Segments"`
}

type AirResponse struct {
	D struct {
		Status       string `json:"Status"`
		Success      bool   `json:"Success"`
		ErrorMessage string `json:"ErrorMessage"`
		Data         *struct {
			Recommendations []FlightOption `json:"Recommendations"`
			FlightOptions   []FlightOption `json:"FlightOptions"`
		} `json:"Data"`
	} `json:"d"`
}

// ---- Yardimcilar ----

// newGUID sayfadaki uuidv4() karsiligi (GetAirSearch asGuid: ayni aramanin polling anahtari).
func newGUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%08d-0000-4000-8000-000000000000", time.Now().UnixNano()%100000000)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// toSlashDate yyyy-MM-dd -> dd/MM/yyyy (sayfanin "d/m/Y" formati; flatpickr sifirli gun/ay uretir).
func toSlashDate(iso string) (string, error) {
	if len(iso) < 10 {
		return "", fmt.Errorf("Gecersiz tarih formati: %s (yyyy-MM-dd bekleniyor)", iso)
	}
	p := strings.Split(iso[:10], "-")
	if len(p) != 3 || len(p[0]) != 4 {
		return "", fmt.Errorf("Gecersiz tarih formati: %s (yyyy-MM-dd bekleniyor)", iso)
	}
	return p[2] + "/" + p[1] + "/" + p[0], nil
}

// childParams sitenin childCount/childAge1..3 alanlari. Kullanilmayan yaslar 0, childAge4 her zaman -1.
type childParams struct {
	Count int
	Ages  [3]int
}

func buildChildParams(children int, ages []int) (childParams, error) {
	var cp childParams
	if len(ages) == 0 && children > 0 {
		return cp, fmt.Errorf("Tatilsitesi icin cocuk yaslari (childAges) gerekli")
	}
	if len(ages) > maxChildren {
		return cp, fmt.Errorf("Tatilsitesi en fazla %d cocuk destekliyor", maxChildren)
	}
	cp.Count = len(ages)
	for i, a := range ages {
		if a < 0 {
			a = 0
		}
		cp.Ages[i] = a
	}
	return cp, nil
}

func postMethod(ctx context.Context, method string, body any, referer string, timeout time.Duration, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	h := jsonHeaders
	if referer != "" {
		h = httpx.Merge(jsonHeaders, map[string]string{"referer": referer})
	}
	resp, err := httpx.Post(ctx, methodsURL+method, b, h, timeout)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("Tatilsitesi %s yaniti parse edilemedi: %w", method, err)
	}
	return nil
}

// fetchRooms hotelDetailSearch: searchType "Hotel" (sadece otel) veya "Package" (otel+ucak sozlesmesi; fiyatlar farkli).
func fetchRooms(ctx context.Context, hotelCode, hotelName, referer, checkIn, checkOut string, adults int, cp childParams, searchType string) ([]HotelRow, error) {
	ci, err := toSlashDate(checkIn)
	if err != nil {
		return nil, err
	}
	co, err := toSlashDate(checkOut)
	if err != nil {
		return nil, err
	}
	param := map[string]any{
		"hotelCode":             hotelCode,
		"checkInSTR":            ci,
		"checkOutSTR":           co,
		"adultCount":            adults,
		"childCount":            cp.Count,
		"childAge1":             cp.Ages[0],
		"childAge2":             cp.Ages[1],
		"childAge3":             cp.Ages[2],
		"childAge4":             -1,
		"HotelsProCoralAllRoom": true,
	}
	if hotelName != "" {
		param["hotelname"] = hotelName
	}
	var out hotelSearchResponse
	if err := postMethod(ctx, "hotelDetailSearch", map[string]any{"Parameter": param, "Type": searchType}, referer, roomTimeout, &out); err != nil {
		return nil, err
	}
	return out.D, nil
}

// fetchTransfers GetPackageTransfer: havalimani <-> otel bolgesi transfer secenekleri (CityCode = sayfadaki CityID).
func fetchTransfers(ctx context.Context, hotelCode, cityID, referer, checkIn, checkOut string, adults int, cp childParams) (*TransferResponse, error) {
	ci, err := toSlashDate(checkIn)
	if err != nil {
		return nil, err
	}
	co, err := toSlashDate(checkOut)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"parameter": map[string]any{
		"HotelCode": hotelCode, "CityCode": cityID, "CheckIn": ci, "CheckOut": co,
		"Adult": adults, "Child": cp.Count,
		"ChildAge1": cp.Ages[0], "ChildAge2": cp.Ages[1], "ChildAge3": cp.Ages[2], "ChildAge4": -1, "ChildAge5": -1,
	}}
	var out TransferResponse
	if err := postMethod(ctx, "GetPackageTransfer", body, referer, transferTimeout, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// fetchFlights GetAirSearch: kalkis IATA -> otel sehri (CityCode, orn. ECN). Status "Ready" olana kadar ayni asGuid ile tekrar sorulur.
func fetchFlights(ctx context.Context, departure, arrivalCity, referer, checkIn, checkOut string, adults int, cp childParams) (*AirResponse, error) {
	ci, err := toSlashDate(checkIn)
	if err != nil {
		return nil, err
	}
	co, err := toSlashDate(checkOut)
	if err != nil {
		return nil, err
	}
	age := func(i int) int {
		if cp.Count > i {
			return cp.Ages[i]
		}
		return -1
	}
	params := map[string]any{
		"departure": strings.ToUpper(departure), "arrival": "-1", "departureCity": -1,
		"depSearchType": "City", "retSearchType": "Apt", "arrivalCity": arrivalCity,
		"departureDate": ci, "returnDate": co,
		"adultCount": adults, "childCount": cp.Count,
		"CAge1": age(0), "CAge2": age(1), "CAge3": age(2), "CAge4": -1, "CAge5": -1, "CAge6": -1,
		"provider": flightProvider, "CAge7": -1, "CAge8": -1,
		"isRoundTrip": true, "isInternational": false, "classType": "Economy", "parityControl": "",
		"asGuid": newGUID(),
	}
	var last *AirResponse
	for i := 0; i < airMaxPolls; i++ {
		var out AirResponse
		if err := postMethod(ctx, "GetAirSearch", map[string]any{"flightSearchParams": params}, referer, airTimeout, &out); err != nil {
			return nil, err
		}
		last = &out
		if out.D.Status == "Ready" {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(airPollDelay):
		}
	}
	if last == nil || last.D.Data == nil {
		msg := ""
		if last != nil {
			msg = last.D.ErrorMessage
		}
		return nil, fmt.Errorf("Tatilsitesi ucus sonucu alinamadi (%s)", msg)
	}
	return last, nil
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
	CityCode        string
	CityID          string
	TownID          string
}

var (
	reHotelCode = regexp.MustCompile(`var\s+hotelCODE\s*=\s*'([^']*)'`)
	reHotelName = regexp.MustCompile(`var\s+HotelName\s*=\s*'([^']*)'`)
	reCityCode  = regexp.MustCompile(`var\s+CityCode\s*=\s*'([^']*)'`)
	reCityID    = regexp.MustCompile(`var\s+CityID\s*=\s*'([^']*)'`)
	reTownID    = regexp.MustCompile(`var\s+TownID\s*=\s*'([^']*)'`)
)

func parseDetailPage(html string) *resolved {
	return &resolved{
		ProviderHotelID: provider.FirstMatch(reHotelCode, html),
		HotelName:       provider.DecodeEntities(provider.FirstMatch(reHotelName, html)),
		CityCode:        provider.FirstMatch(reCityCode, html),
		CityID:          provider.FirstMatch(reCityID, html),
		TownID:          provider.FirstMatch(reTownID, html),
	}
}

func resolveHotelFromURL(ctx context.Context, u string) (*resolved, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
	if err != nil {
		return nil, err
	}
	if strings.Contains(resp.FinalURL, "/pagenotfound") {
		return nil, fmt.Errorf("Tatilsitesi otel sayfasi bulunamadi (URL yanlis olabilir)")
	}
	r := parseDetailPage(resp.Text())
	if r.ProviderHotelID == "" {
		return nil, fmt.Errorf("Tatilsitesi sayfasinda hotelCODE bulunamadi (otel detay URL'i olmali)")
	}
	return r, nil
}
