package gezinomi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const (
	site         = "https://www.gezinomi.com"
	api          = "https://apigezinomi.gezinomi.com/api"
	pathPrices   = "Hotel/GetHotelRoomPrices"
	pathAirports = "Hotel/GetHotelAirports"
	ua           = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
)

var apiHeaders = map[string]string{
	"accept":          "*/*",
	"accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8",
	"content-type":    "application/json",
	"origin":          site,
	"referer":         site + "/",
	"user-agent":      ua,
}

var pageHeaders = map[string]string{
	"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"accept-language":           "tr-TR,tr;q=0.9,en-US;q=0.8",
	"user-agent":                ua,
	"sec-fetch-dest":            "document",
	"sec-fetch-mode":            "navigate",
	"sec-fetch-site":            "none",
	"upgrade-insecure-requests": "1",
}

func toAPIDate(iso string) string { return iso[:min(10, len(iso))] + "T00:00:00" }

// toFlightDate sitenin JS'i gibi: giris gunu + 3 saat, ISO (UTC).
func toFlightDate(iso string) string { return iso[:min(10, len(iso))] + "T03:00:00.000Z" }

type ReservationDetail struct {
	Absent                            bool     `json:"absent"`
	ConceptName                       string   `json:"conceptName"`
	HotelRoomName                     string   `json:"hotelRoomName"`
	TotalPrice                        *float64 `json:"totalPrice"`
	TotalDiscountedPrice              *float64 `json:"totalDiscountedPrice"`
	DiscountRate                      *float64 `json:"discountRate"`
	FlightPackageTotalPrice           *float64 `json:"flightPackageTotalPrice"`
	FlightPackageTotalDiscountedPrice *float64 `json:"flightPackageTotalDiscountedPrice"`
	FlightPackageDiscountRate         *float64 `json:"flightPackageDiscountRate"`
}

type RoomDetail struct {
	RoomID             *int                `json:"roomId"`
	RoomName           string              `json:"roomName"`
	ReservationDetails []ReservationDetail `json:"reservationDetails"`
}

type PriceData struct {
	RoomDetails []RoomDetail `json:"roomDetails"`
	SearchID    string       `json:"searchId"`
}

type PriceResponse struct {
	Data    *PriceData `json:"data"`
	Status  string     `json:"status"`
	Message string     `json:"message"`
}

type person struct {
	IsChild     bool `json:"isChild"`
	IsMen       bool `json:"isMen"`
	Age         int  `json:"age"`
	RoomOrderID int  `json:"roomOrderId"`
}

// buildPersons sitenin JS'i gibi: yetiskinler age 0; cocuk isChild = yas < 16.
func buildPersons(adults int, childAges []int) []person {
	ps := make([]person, 0, adults+len(childAges))
	for i := 0; i < adults; i++ {
		ps = append(ps, person{})
	}
	for _, a := range childAges {
		ps = append(ps, person{IsChild: a < 16, Age: a})
	}
	return ps
}

type flightLeg struct {
	OriginCode        string `json:"originCode"`
	IsOriginCity      bool   `json:"isOriginCity"`
	DestinationCode   string `json:"destinationCode"`
	IsDestinationCity bool   `json:"isDestinationCity"`
	FlightDate        string `json:"flightDate"`
}

// flightOpts paket sorgusu icin gidis-donus ucus bilgisi (nil: sadece otel).
type flightOpts struct {
	Origin            string
	Destination       string
	IsDestinationCity bool
}

// buildBody GetHotelRoomPrices govdesi. flight nil ise searchTypeId 1 (otel), doluysa 2 (otel+ucak).
func buildBody(hotelID int, checkIn, checkOut string, adults int, childAges []int, flight *flightOpts) []byte {
	passengers := map[string]int{"adult": 0, "child": 0, "infant": 0}
	flights := []flightLeg{}
	searchType := 1
	if flight != nil {
		searchType = 2
		passengers = map[string]int{"adult": adults, "child": len(childAges), "infant": 0}
		flights = []flightLeg{
			{OriginCode: flight.Origin, DestinationCode: flight.Destination, IsDestinationCity: flight.IsDestinationCity, FlightDate: toFlightDate(checkIn)},
			{OriginCode: flight.Destination, IsOriginCity: flight.IsDestinationCity, DestinationCode: flight.Origin, FlightDate: toFlightDate(checkOut)},
		}
	}
	body, _ := json.Marshal(map[string]any{
		"checkInDate":     toAPIDate(checkIn),
		"checkOutDate":    toAPIDate(checkOut),
		"hotelId":         hotelID,
		"apiId":           0,
		"searchTypeId":    searchType,
		"apiProviderName": "",
		"roomDetails":     []map[string]any{{"roomId": 1, "persons": buildPersons(adults, childAges)}},
		"flightSearch": map[string]any{
			"flights": flights, "passengers": passengers, "isMultiTrip": false, "directFlightsOnly": false, "cabinCodeType": "",
		},
	})
	return body
}

// fetchRoomPrices oda fiyatlari; flight doluysa ayni yanitta flightPackageTotal* alanlari da dolar.
func fetchRoomPrices(ctx context.Context, s *signer, providerHotelID, checkIn, checkOut string, adults int, childAges []int, flight *flightOpts) (*PriceResponse, error) {
	hotelID, _ := strconv.Atoi(providerHotelID)
	timeout := 40 * time.Second
	if flight != nil {
		timeout = 60 * time.Second // ucus aramasi daha yavas
	}
	resp, err := s.post(ctx, pathPrices, buildBody(hotelID, checkIn, checkOut, adults, childAges, flight), timeout)
	if err != nil {
		return nil, err
	}
	var out PriceResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("Gezinomi yaniti parse edilemedi: %w", err)
	}
	return &out, nil
}

type airport struct {
	AirportName string `json:"airportName"`
	AirportCode string `json:"airportCode"`
	IsCity      bool   `json:"isCity"`
}

type airportsResponse struct {
	Data    []airport `json:"data"`
	Status  string    `json:"status"`
	Message string    `json:"message"`
}

// fetchHotelAirports otelin varis havalimanlarini doner (hotelLocationId/parentId detay sayfasindan).
func fetchHotelAirports(ctx context.Context, s *signer, hotelID, locationID, parentID int) ([]airport, error) {
	body, _ := json.Marshal(map[string]int{"hotelId": hotelID, "hotelLocationId": locationID, "parentId": parentID})
	resp, err := s.post(ctx, pathAirports, body, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var out airportsResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("Gezinomi havalimani yaniti parse edilemedi: %w", err)
	}
	return out.Data, nil
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
	HotelLocationID string
	ParentID        string
}

var (
	reHotelID     = regexp.MustCompile(`"hotelId":(\d+)`)
	reHotelName   = regexp.MustCompile(`"hotelName":"([^"]{3,90})"`)
	reProductName = regexp.MustCompile(`"productName":"([^"]{3,90})"`)
	// Otel nesnesinde hotelLocationId'den kisa bir mesafe sonra parentId gelir (menu parentId'leriyle karismaz).
	reLocation = regexp.MustCompile(`"hotelLocationId":(\d+)[^{}]{0,400}?"parentId":(\d+)`)
)

func parseDetailPage(html string) (*resolved, error) {
	raw := strings.ReplaceAll(html, `\"`, `"`)
	id := provider.FirstMatch(reHotelID, raw)
	if id == "" {
		return nil, fmt.Errorf("Gezinomi sayfasinda hotelId bulunamadi (otel detay URL'i olmali)")
	}
	name := provider.FirstMatch(reHotelName, raw)
	if name == "" {
		name = provider.FirstMatch(reProductName, raw)
	}
	r := &resolved{ProviderHotelID: id, HotelName: strings.TrimSpace(name)}
	if m := reLocation.FindStringSubmatch(raw); len(m) == 3 {
		r.HotelLocationID, r.ParentID = m[1], m[2]
	}
	return r, nil
}

func resolveHotelFromURL(ctx context.Context, u string) (*resolved, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	return parseDetailPage(resp.Text())
}
