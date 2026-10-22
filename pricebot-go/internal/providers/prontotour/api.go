package prontotour

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/httpx"
)

const (
	apiBase         = "https://webapi.prontotour.com/corewebapi"
	autoCompleteURL = apiBase + "/AutoComplete/Search"
	hotelDetailURL  = apiBase + "/Hotel/HotelDetail"
	bundleSearchURL = apiBase + "/Bundle/BundleSearch"
	ua              = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
	// HotelDetail ~1 sn, BundleSearch (ucus aramasi dahil) ~4-6 sn.
	apiTimeout = 45 * time.Second
	// Yas verilmeyen cocuklar icin varsayilan yas.
	defaultChildAge = 7
	// Cocuk yas secimi sitede 1-14; 0 yas "bebek" (Type 2) sayilir.
	maxChildAge = 14
	adultAge    = "35"
)

// Yolcu tipleri (site JS: Type 0 yetiskin, 1 cocuk, 2 bebek).
const (
	travelerAdult  = 0
	travelerChild  = 1
	travelerInfant = 2
)

// Cerez/anahtar gerekmez; API acik CORS ile tarayicidan cagriliyor.
var apiHeaders = map[string]string{
	"accept":          "application/json, text/javascript, */*; q=0.01",
	"accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"content-type":    "application/json",
	"origin":          "https://www.prontotour.com",
	"referer":         "https://www.prontotour.com/",
	"user-agent":      ua,
}

// Traveler: yetiskinlerde Age "35" (string), cocuk/bebekte sayi — site JS ile ayni.
type Traveler struct {
	Type int `json:"Type"`
	Age  any `json:"Age"`
}

type Room struct {
	HotelTravelers []Traveler `json:"HotelTravelers"`
}

// currencyRequest: Currency "TL" dolu olmadan BundleSearch "Sonuc bulunamadi" doner.
type currencyRequest struct {
	Dolar    string `json:"Dolar"`
	Euro     string `json:"Euro"`
	Currency string `json:"Currency"`
}

var currencyTL = currencyRequest{Currency: "TL"}

type hotelDetailRequest struct {
	Code             string          `json:"Code"`
	CityCode         string          `json:"CityCode"`
	Room             []Room          `json:"Room"`
	HotelServiceType string          `json:"HotelServiceType"`
	StartDate        string          `json:"StartDate"`
	EndDate          string          `json:"EndDate"`
	SearchKey        string          `json:"SearchKey"`
	CurrencyRequest  currencyRequest `json:"CurrencyRequest"`
	SourceType       string          `json:"SourceType"`
	Packing          bool            `json:"Packing"`
	Bundle           bool            `json:"Bundle"`
	SGUID            string          `json:"S_GUID"`
}

// HotelSummary HotelDetail.HotelListResponse (tek nesne) ve BundleSearch.HotelListResponse[] (liste) ortak alanlari.
type HotelSummary struct {
	Name             string   `json:"Name"`
	HotelCode        string   `json:"HotelCode"`
	City             *string  `json:"City"`
	Address          *string  `json:"Address"`
	Star             *string  `json:"Star"`
	PriceTL          float64  `json:"PriceTL"`
	DiscountPriceTL  float64  `json:"DiscountPriceTL"`
	MealType         *string  `json:"MealType"`
	HotelServiceType int      `json:"HotelServiceType"`
	MinimumStay      int      `json:"MinimumStay"`
	Error            *string  `json:"Error"`
	DiscountRate     *float64 `json:"DiscountRate"`
}

// RoomDetail HotelRoomDetailsList elemani. Paket modunda (Packing/Bundle + S_GUID) fiyatlar
// en ucuz odaya gore FARK olarak gelir (en ucuz oda 0).
type RoomDetail struct {
	Allotment       bool    `json:"Allotment"`
	Mealy           string  `json:"Mealy"`
	MealyCode       string  `json:"MealyCode"`
	RoomName        string  `json:"RoomName"`
	RoomCode        string  `json:"RoomCode"`
	PriceTl         float64 `json:"PriceTl"`
	DiscountPriceTl float64 `json:"DiscountPriceTl"`
	FreeCancel      bool    `json:"FreeCancel"`
	RoomSize        any     `json:"RoomSize"`
}

type HotelDetailResponse struct {
	HotelListResponse    *HotelSummary `json:"HotelListResponse"`
	HotelRoomDetailsList []RoomDetail  `json:"HotelRoomDetailsList"`
	HotelServiceType     int           `json:"HotelServiceType"`
	SGUID                *string       `json:"S_GUID"`
}

type flightRequest struct {
	IsTurkey           bool       `json:"IsTurkey"`
	DepartureDate      string     `json:"DepartureDate"`
	ArrivalDate        string     `json:"ArrivalDate"`
	RouteType          string     `json:"RouteType"`
	Travelers          []Traveler `json:"Travelers"`
	MultiFlightRequest []any      `json:"MultiFlightRequest"`
}

type locationRequest struct {
	FromType string `json:"FromType"`
	FromID   string `json:"FromId"`
	ToType   string `json:"ToType"`
	ToID     int    `json:"ToId"`
	TourCode string `json:"TourCode"`
}

type hotelRequest struct {
	Checkin      string `json:"Checkin"`
	Checkout     string `json:"Checkout"`
	Room         []Room `json:"Room"`
	Packing      bool   `json:"Packing"`
	HotelPackage string `json:"HotelPackage"`
	FirstHotel   bool   `json:"FirstHotel"`
	HotelCode    string `json:"HotelCode"`
	TagName      string `json:"TagName"`
	IsTag        bool   `json:"IsTag"`
	RCI          bool   `json:"RCI"`
}

type bundleRequest struct {
	FlightRequest   *flightRequest  `json:"FlightRequest"`
	LocationRequest locationRequest `json:"LocationRequest"`
	HotelRequest    hotelRequest    `json:"HotelRequest"`
	TourRequest     any             `json:"TourRequest"`
	CurrencyRequest currencyRequest `json:"CurrencyRequest"`
	SourceType      string          `json:"SourceType"`
}

type Flight struct {
	FlightNumber         string  `json:"FlightNumber"`
	CompanyName          string  `json:"CompanyName"`
	DepartureAirportCode string  `json:"DepartureAirportCode"`
	ArrivalAirportCode   string  `json:"ArrivalAirportCode"`
	DepartureDate        string  `json:"DepartureDate"`
	DepartureTime        string  `json:"DepartureTime"`
	ArrivalTime          string  `json:"ArrivalTime"`
	AdultPrice           float64 `json:"AdultPrice"`
	ChildPrice           float64 `json:"ChildPrice"`
	InfantPrice          float64 `json:"InfantPrice"`
	Selected             bool    `json:"Selected"`
	ListNumber           int     `json:"ListNumber"`
}

type BundleResponse struct {
	TotalPrice        float64        `json:"TotalPrice"`
	FlightResponse    []Flight       `json:"FlightResponse"`
	HotelListResponse []HotelSummary `json:"HotelListResponse"`
	ErrorText         *string        `json:"ErrorText"`
	SGUID             *string        `json:"S_GUID"`
}

var reISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// normDate girdiyi API'nin bekledigi yyyy-MM-dd'ye indirger (standart zaten bu).
func normDate(input string) (string, error) {
	if !reISO.MatchString(input) {
		return "", fmt.Errorf("Gecersiz tarih formati: %s (yyyy-MM-dd bekleniyor)", input)
	}
	return input[:10], nil
}

// buildTravelers yetiskin + cocuk listesini site JS'nin urettigi yolcu dizisine cevirir.
// Yas verilmeyen cocuklar defaultChildAge; 0 yas bebek (Type 2); 15+ yetiskin sayilir.
func buildTravelers(adults, children int, childAges []int) []Traveler {
	if adults < 1 {
		adults = 1
	}
	out := make([]Traveler, 0, adults+children)
	for i := 0; i < adults; i++ {
		out = append(out, Traveler{Type: travelerAdult, Age: adultAge})
	}
	n := children
	if len(childAges) > n {
		n = len(childAges)
	}
	for i := 0; i < n; i++ {
		age := defaultChildAge
		if i < len(childAges) {
			age = childAges[i]
		}
		switch {
		case age <= 0:
			out = append(out, Traveler{Type: travelerInfant, Age: 0})
		case age > maxChildAge:
			out = append(out, Traveler{Type: travelerAdult, Age: adultAge})
		default:
			out = append(out, Traveler{Type: travelerChild, Age: age})
		}
	}
	return out
}

func postJSON(ctx context.Context, url string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := httpx.Post(ctx, url, raw, apiHeaders, apiTimeout)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("Prontotour yaniti parse edilemedi (%s): %w", url, err)
	}
	return nil
}

// packageCtx paket modunda HotelDetail icin BundleSearch'ten gelen oturum kimligi.
type packageCtx struct {
	SGUID string
}

// fetchHotelDetail bir otelin oda/fiyat listesini ceker. Tarihler bos ise yalnizca otel kimligi
// (ad, adres) doner — ResolveFromURL bunu kullanir. pkg verilirse paket (ucakli) modunda sorgular.
func fetchHotelDetail(ctx context.Context, code, serviceType, checkIn, checkOut string, travelers []Traveler, pkg *packageCtx) (*HotelDetailResponse, error) {
	req := hotelDetailRequest{
		Code: code, HotelServiceType: serviceType,
		Room:            []Room{{HotelTravelers: travelers}},
		StartDate:       checkIn,
		EndDate:         checkOut,
		CurrencyRequest: currencyTL,
		SourceType:      "H",
	}
	if pkg != nil {
		req.SourceType = "HF"
		req.Packing, req.Bundle = true, true
		req.SGUID = pkg.SGUID
	}
	var out HotelDetailResponse
	if err := postJSON(ctx, hotelDetailURL, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type Airport struct {
	ID   int
	Code string
	Name string
}

type autoCompleteItem struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	IdDest      int    `json:"IdDest"`
	AirPortCode string `json:"AirPortCode"`
}

// matchAirport autocomplete listesinde IATA koduyla eslesen havalimanini secer.
func matchAirport(items []autoCompleteItem, iata string) *Airport {
	iata = strings.ToUpper(strings.TrimSpace(iata))
	for _, it := range items {
		if it.Type == "F" && strings.EqualFold(it.AirPortCode, iata) && it.IdDest > 0 {
			return &Airport{ID: it.IdDest, Code: iata, Name: it.Name}
		}
	}
	return nil
}

// lookupAirport IATA -> Prontotour destinasyon id'si (AutoComplete/Search, Type F).
func lookupAirport(ctx context.Context, iata string) (*Airport, error) {
	body := map[string]any{"Key": strings.ToUpper(iata), "Type": "F", "Id": 0, "Source": 1}
	var items []autoCompleteItem
	if err := postJSON(ctx, autoCompleteURL, body, &items); err != nil {
		return nil, err
	}
	a := matchAirport(items, iata)
	if a == nil {
		return nil, fmt.Errorf("Prontotour'da '%s' havalimani bulunamadi (IATA kodu kontrol edilmeli)", iata)
	}
	return a, nil
}

// fetchBundle otel+ucak paketini arar (detay sayfasindaki HotelDetailSearch akisi: FirstHotel + HotelCode "kod-servisTipi").
func fetchBundle(ctx context.Context, code, serviceType string, from *Airport, checkIn, checkOut string, travelers []Traveler) (*BundleResponse, error) {
	req := bundleRequest{
		FlightRequest: &flightRequest{
			IsTurkey: true, DepartureDate: checkIn, ArrivalDate: checkOut, RouteType: "1",
			Travelers: travelers, MultiFlightRequest: []any{},
		},
		LocationRequest: locationRequest{FromType: "F", FromID: strconv.Itoa(from.ID)},
		HotelRequest: hotelRequest{
			Checkin: checkIn, Checkout: checkOut, Room: []Room{{HotelTravelers: travelers}},
			Packing: true, HotelPackage: "NO", FirstHotel: true, HotelCode: code + "-" + serviceType,
		},
		CurrencyRequest: currencyTL,
		SourceType:      "HF",
	}
	var out BundleResponse
	if err := postJSON(ctx, bundleSearchURL, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---- URL'den otel verisi cozme ----

// Detay URL'i: https://www.prontotour.com/<Seo-Ad>-h<otelKodu>-<servisTipi>[?CheckIn=...]
var reDetailPath = regexp.MustCompile(`(?i)/([^/?#]*?)-h(\d+)-(\d+)(?:[/?#]|$)`)

type urlParts struct {
	Code        string
	ServiceType string
	Slug        string
}

func parseDetailURL(u string) (*urlParts, error) {
	m := reDetailPath.FindStringSubmatch(u)
	if m == nil {
		return nil, fmt.Errorf("Prontotour otel detay URL'i bekleniyor (.../<Otel-Adi>-h<kod>-<tip>)")
	}
	return &urlParts{Slug: m[1], Code: m[2], ServiceType: m[3]}, nil
}

func nameFromSlug(slug string) string {
	return strings.TrimSpace(strings.ReplaceAll(slug, "-", " "))
}

type resolved struct {
	ProviderHotelID string
	ServiceType     string
	HotelName       string
}

func resolveHotelFromURL(ctx context.Context, u string) (*resolved, error) {
	p, err := parseDetailURL(u)
	if err != nil {
		return nil, err
	}
	r := &resolved{ProviderHotelID: p.Code, ServiceType: p.ServiceType, HotelName: nameFromSlug(p.Slug)}
	// Tarihsiz HotelDetail otel adini verir (fiyat yok); hata olursa slug'dan turetilen ad kalir.
	d, err := fetchHotelDetail(ctx, p.Code, p.ServiceType, "", "", buildTravelers(1, 0, nil), nil)
	if err != nil {
		return nil, err
	}
	if d.HotelListResponse == nil || strings.TrimSpace(d.HotelListResponse.Name) == "" {
		return nil, fmt.Errorf("Prontotour otel bulunamadi (kod %s, tip %s)", p.Code, p.ServiceType)
	}
	r.HotelName = strings.TrimSpace(d.HotelListResponse.Name)
	return r, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
