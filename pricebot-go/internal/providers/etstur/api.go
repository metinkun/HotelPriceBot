package etstur

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
	searchURL  = "https://www.etstur.com/services/api/search/v2/hotels"
	// Ekim 2026: Etstur oda/paket akisini "multi" uclarina tasidi (eski /room ve /room/package hata donuyor).
	roomURL    = "https://www.etstur.com/services/api/room/multi"
	packageURL = "https://www.etstur.com/services/api/room/multi/package"
	pageLimit  = 100
	maxPages   = 20
)

var defaultHeaders = map[string]string{
	"accept":           "application/json, text/plain, */*",
	"accept-language":  "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"content-type":     "application/json",
	"origin":           "https://www.etstur.com",
	"referer":          "https://www.etstur.com/",
	"user-agent":       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
	"x-requested-with": "XMLHttpRequest",
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

// ---- Istek/yanit modelleri ----

type roomRequest struct {
	AdultCount  int   `json:"adultCount"`
	ChildCount  int   `json:"childCount"`
	ChildAges   []int `json:"childAges"`
	InfantCount int   `json:"infantCount"`
}

type sortSpec struct {
	Type      string `json:"type"`
	Direction string `json:"direction"`
}

type searchRequest struct {
	CheckIn         string        `json:"checkIn"`
	CheckOut        string        `json:"checkOut"`
	Rooms           []roomRequest `json:"rooms"`
	URL             string        `json:"url"`
	Offset          int           `json:"offset"`
	Limit           int           `json:"limit"`
	MinPrice        *float64      `json:"minPrice"`
	MaxPrice        *float64      `json:"maxPrice"`
	Filters         []any         `json:"filters"`
	Loc             any           `json:"loc"`
	Zoom            any           `json:"zoom"`
	RadiusKm        any           `json:"radiusKm"`
	ExcludeHotelIDs []string      `json:"excludeHotelIds"`
	Sort            sortSpec      `json:"sort"`
}

type PriceBlock struct {
	Currency        string   `json:"currency"`
	Amount          float64  `json:"amount"`
	DiscountedPrice *float64 `json:"discountedPrice"`
	DiscountRate    float64  `json:"discountRate"`
}

type Room struct {
	RoomNames      []string `json:"roomNames"`
	BoardType      []string `json:"boardType"`
	BoardTypeLabel []string `json:"boardTypeLabel"`
	NightCount     int      `json:"nightCount"`
	Availability   *struct {
		Type       string `json:"type"`
		NightCount int    `json:"nightCount"`
	} `json:"availability"`
	Price                    *PriceBlock `json:"price"`
	CampaignHighlightedPrice *struct {
		Label string      `json:"label"`
		Type  string      `json:"type"`
		Price *PriceBlock `json:"price"`
	} `json:"campaignHighlightedPrice"`
}

type Hotel struct {
	HotelID  string `json:"hotelId"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Location *struct {
		City  string `json:"city"`
		State string `json:"state"`
		Label string `json:"label"`
	} `json:"location"`
	Room *Room `json:"room"`
}

type searchResponse struct {
	Success      bool    `json:"success"`
	ErrorCode    *string `json:"errorCode"`
	ErrorMessage *string `json:"errorMessage"`
	Result       *struct {
		TotalCount int     `json:"totalCount"`
		Offset     int     `json:"offset"`
		Limit      int     `json:"limit"`
		Hotels     []Hotel `json:"hotels"`
	} `json:"result"`
}

type roomSearchResponse struct {
	Success      bool    `json:"success"`
	ErrorMessage *string `json:"errorMessage"`
	Result       *struct {
		RoomSearchID string `json:"roomSearchId"`
	} `json:"result"`
}

type PackageBoard struct {
	ID        string `json:"id"`
	BoardType *struct {
		Code  string `json:"code"`
		Label string `json:"label"`
	} `json:"boardType"`
	Price        *PriceBlock `json:"price"`
	Cancellation *string     `json:"cancellation"`
}

type PackageRoom struct {
	RoomID     string         `json:"roomId"`
	RoomName   string         `json:"roomName"`
	RoomSize   *int           `json:"roomSize"`
	NightCount *int           `json:"nightCount"`
	SubBoards  []PackageBoard `json:"subBoards"`
}

type packageResponse struct {
	Success      bool    `json:"success"`
	ErrorMessage *string `json:"errorMessage"`
	Result       *struct {
		RoomSearchID string        `json:"roomSearchId"`
		Rooms        []PackageRoom `json:"rooms"`
	} `json:"result"`
}

// Occupancy tarih + kisi bilgisi.
type Occupancy struct {
	CheckIn   string
	CheckOut  string
	Adults    int
	Children  int
	ChildAges []int
}

var (
	reISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
	reTR  = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}`)
)

// ToEtsturDate "2026-06-24" veya "24.06.2026" -> "2026-06-24T00:00:00.000"
func ToEtsturDate(input string) (string, error) {
	var y, m, d string
	switch {
	case reISO.MatchString(input):
		p := strings.Split(input[:10], "-")
		y, m, d = p[0], p[1], p[2]
	case reTR.MatchString(input):
		p := strings.Split(input[:10], ".")
		d, m, y = p[0], p[1], p[2]
	default:
		return "", fmt.Errorf("Gecersiz tarih formati: %s (yyyy-MM-dd bekleniyor)", input)
	}
	return fmt.Sprintf("%s-%s-%sT00:00:00.000", y, m, d), nil
}

func toRoomDate(input string) (string, error) {
	s, err := ToEtsturDate(input)
	if err != nil {
		return "", err
	}
	return s[:10], nil
}

func buildSearchBody(destinationURL string, occ Occupancy, offset, limit int) (*searchRequest, error) {
	ci, err := ToEtsturDate(occ.CheckIn)
	if err != nil {
		return nil, err
	}
	co, err := ToEtsturDate(occ.CheckOut)
	if err != nil {
		return nil, err
	}
	ages := occ.ChildAges
	if ages == nil {
		ages = []int{}
	}
	return &searchRequest{
		CheckIn: ci, CheckOut: co,
		Rooms:           []roomRequest{{AdultCount: occ.Adults, ChildCount: occ.Children, ChildAges: ages, InfantCount: 0}},
		URL:             destinationURL,
		Offset:          offset,
		Limit:           limit,
		Filters:         []any{},
		ExcludeHotelIDs: []string{},
		Sort:            sortSpec{},
	}, nil
}

func (p *Provider) search(ctx context.Context, body *searchRequest) (*searchResponse, error) {
	b, _ := json.Marshal(body)
	headers := httpx.Merge(defaultHeaders)
	if p.cookie != "" {
		headers["cookie"] = p.cookie
	}
	resp, err := httpx.Post(ctx, searchURL, b, headers, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var out searchResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("Etstur yaniti parse edilemedi: %w", err)
	}
	return &out, nil
}

// collectHotelsByIDs destinasyon sayfalarini tarayip istenen otelleri toplar.
func (p *Provider) collectHotelsByIDs(ctx context.Context, destinationURL string, wantedIDs []string, occ Occupancy) (map[string]*Hotel, error) {
	wanted := map[string]bool{}
	for _, id := range wantedIDs {
		wanted[id] = true
	}
	found := map[string]*Hotel{}
	offset := 0
	total := int(^uint(0) >> 1)

	for page := 0; page < maxPages && len(found) < len(wanted); page++ {
		if offset >= total {
			break
		}
		body, err := buildSearchBody(destinationURL, occ, offset, pageLimit)
		if err != nil {
			return nil, err
		}
		data, err := p.search(ctx, body)
		if err != nil {
			return nil, err
		}
		if !data.Success || data.Result == nil {
			msg := "Etstur arama basarisiz"
			if data.ErrorMessage != nil && *data.ErrorMessage != "" {
				msg = *data.ErrorMessage
			}
			return nil, fmt.Errorf("%s", msg)
		}
		total = data.Result.TotalCount
		for i := range data.Result.Hotels {
			h := &data.Result.Hotels[i]
			if wanted[h.HotelID] {
				found[h.HotelID] = h
			}
		}
		if len(data.Result.Hotels) < pageLimit {
			break
		}
		offset += pageLimit
	}
	return found, nil
}

// ---- Paket akisi ----

func (p *Provider) fetchRoomSearch(ctx context.Context, etsturHotelID string, occ Occupancy) (roomSearchID, cookie string, err error) {
	ci, err := toRoomDate(occ.CheckIn)
	if err != nil {
		return "", "", err
	}
	co, err := toRoomDate(occ.CheckOut)
	if err != nil {
		return "", "", err
	}
	ages := occ.ChildAges
	if ages == nil {
		ages = []int{}
	}
	// Yeni format: tek "room" yerine "rooms" dizisi (tarayici capture'i, 2026-10-16)
	body, _ := json.Marshal(map[string]any{
		"hotelId":  etsturHotelID,
		"checkIn":  ci,
		"checkOut": co,
		"rooms":    []roomRequest{{AdultCount: occ.Adults, ChildCount: occ.Children, ChildAges: ages, InfantCount: 0}},
	})
	resp, err := httpx.Post(ctx, roomURL, body, httpx.Merge(defaultHeaders), 20*time.Second)
	if err != nil {
		return "", "", err
	}
	var data roomSearchResponse
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		return "", "", fmt.Errorf("Etstur oda yaniti parse edilemedi: %w", err)
	}
	if !data.Success || data.Result == nil || data.Result.RoomSearchID == "" {
		msg := "Oda aramasi basarisiz (roomSearchId yok)"
		if data.ErrorMessage != nil && *data.ErrorMessage != "" {
			msg = *data.ErrorMessage
		}
		return "", "", fmt.Errorf("%s", msg)
	}
	return data.Result.RoomSearchID, resp.SetCookies(), nil
}

func (p *Provider) fetchPackage(ctx context.Context, roomSearchID, airportCode, cookie string) (*packageResponse, error) {
	body, _ := json.Marshal(map[string]string{"roomSearchId": roomSearchID, "airportCode": airportCode})
	headers := httpx.Merge(defaultHeaders)
	if cookie != "" {
		headers["cookie"] = cookie
	}
	resp, err := httpx.Post(ctx, packageURL, body, headers, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var data packageResponse
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		return nil, fmt.Errorf("Etstur paket yaniti parse edilemedi: %w", err)
	}
	return &data, nil
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
	DestinationURL  string
	Slug            string
}

// resolveHotelFromURL detay sayfasindaki __NEXT_DATA__ icinden hotelId, ad ve destinationUrl cikarir.
func resolveHotelFromURL(ctx context.Context, url string) (*resolved, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, url, httpx.Options{Headers: pageHeaders, Timeout: 20 * time.Second})
	if err != nil {
		return nil, err
	}
	html := resp.Text()
	i := strings.Index(html, "__NEXT_DATA__")
	if i == -1 {
		return nil, fmt.Errorf("Etstur sayfasinda __NEXT_DATA__ bulunamadi (URL gecersiz olabilir)")
	}
	j := strings.Index(html[i:], ">") + i + 1
	k := strings.Index(html[j:], "</script>")
	if k == -1 {
		return nil, fmt.Errorf("Etstur __NEXT_DATA__ parse edilemedi")
	}
	var data struct {
		Props struct {
			PageProps struct {
				Data struct {
					HotelID  any    `json:"hotelId"`
					Name     string `json:"name"`
					URL      string `json:"url"`
					Location struct {
						LocationBreadCrumbs []struct {
							URL string `json:"url"`
						} `json:"locationBreadCrumbs"`
					} `json:"location"`
				} `json:"data"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal([]byte(html[j:j+k]), &data); err != nil {
		return nil, fmt.Errorf("Etstur __NEXT_DATA__ parse edilemedi")
	}
	d := data.Props.PageProps.Data
	hotelID := ""
	switch v := d.HotelID.(type) {
	case string:
		hotelID = v
	case float64:
		hotelID = fmt.Sprintf("%.0f", v)
	}
	if hotelID == "" {
		return nil, fmt.Errorf("Etstur sayfasinda hotelId bulunamadi (otel detay URL'i olmali)")
	}
	dest := ""
	if len(d.Location.LocationBreadCrumbs) > 0 {
		dest = d.Location.LocationBreadCrumbs[0].URL
	}
	return &resolved{ProviderHotelID: hotelID, HotelName: d.Name, DestinationURL: dest, Slug: d.URL}, nil
}
