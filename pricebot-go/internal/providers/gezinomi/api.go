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
	site          = "https://www.gezinomi.com"
	api           = "https://apigezinomi.gezinomi.com/api"
	roomPricesURL = api + "/Hotel/GetHotelRoomPrices"
	ua            = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
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

type ReservationDetail struct {
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

type PriceResponse struct {
	Data *struct {
		RoomDetails []RoomDetail `json:"roomDetails"`
		SearchID    string       `json:"searchId"`
	} `json:"data"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type person struct {
	IsChild     bool `json:"isChild"`
	IsMen       bool `json:"isMen"`
	Age         int  `json:"age"`
	RoomOrderID int  `json:"roomOrderId"`
}

func buildPersons(adults int, childAges []int) []person {
	ps := make([]person, 0, adults+len(childAges))
	for i := 0; i < adults; i++ {
		ps = append(ps, person{})
	}
	for _, a := range childAges {
		ps = append(ps, person{IsChild: true, Age: a})
	}
	return ps
}

// fetchRoomPrices withFlight true ise ayni yanitta paket (flightPackage*) alanlari da dolar.
func fetchRoomPrices(ctx context.Context, providerHotelID, checkIn, checkOut string, adults int, childAges []int, withFlight bool) (*PriceResponse, error) {
	hotelID, _ := strconv.Atoi(providerHotelID)
	passengers := map[string]int{"adult": 0, "child": 0, "infant": 0}
	if withFlight {
		passengers = map[string]int{"adult": adults, "child": len(childAges), "infant": 0}
	}
	body, _ := json.Marshal(map[string]any{
		"checkInDate":     toAPIDate(checkIn),
		"checkOutDate":    toAPIDate(checkOut),
		"hotelId":         hotelID,
		"apiId":           0,
		"searchTypeId":    1,
		"apiProviderName": "",
		"roomDetails":     []map[string]any{{"roomId": 1, "persons": buildPersons(adults, childAges)}},
		"flightSearch": map[string]any{
			"flights": []any{}, "passengers": passengers, "isMultiTrip": false, "directFlightsOnly": false, "cabinCodeType": "",
		},
	})
	resp, err := httpx.Post(ctx, roomPricesURL, body, apiHeaders, 40*time.Second)
	if err != nil {
		return nil, err
	}
	var out PriceResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("Gezinomi yaniti parse edilemedi: %w", err)
	}
	return &out, nil
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
}

var (
	reHotelID     = regexp.MustCompile(`"hotelId":(\d+)`)
	reHotelName   = regexp.MustCompile(`"hotelName":"([^"]{3,90})"`)
	reProductName = regexp.MustCompile(`"productName":"([^"]{3,90})"`)
)

func resolveHotelFromURL(ctx context.Context, u string) (*resolved, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	raw := strings.ReplaceAll(resp.Text(), `\"`, `"`)
	id := provider.FirstMatch(reHotelID, raw)
	if id == "" {
		return nil, fmt.Errorf("Gezinomi sayfasinda hotelId bulunamadi (otel detay URL'i olmali)")
	}
	name := provider.FirstMatch(reHotelName, raw)
	if name == "" {
		name = provider.FirstMatch(reProductName, raw)
	}
	return &resolved{ProviderHotelID: id, HotelName: strings.TrimSpace(name)}, nil
}
