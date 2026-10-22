package enuygun

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
)

const (
	base = "https://www.enuygun.com"
	// detailSearchURL otel detay sayfasinin oda/teklif listesini ceken JSON API (search-core).
	detailSearchURL = base + "/otel/search-core/detail-search"
	// apiDetailURL slug -> otel kimligi/ad/bolge (Next.js detay verisi, tarihsiz).
	apiDetailURL = base + "/otel/api-detail/%s.json"
	ua           = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
	timeout      = 30 * time.Second
)

var apiHeaders = map[string]string{
	"accept":          "application/json, text/plain, */*",
	"accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"content-type":    "application/json",
	"origin":          base,
	"referer":         base + "/otel/",
	"user-agent":      ua,
	"x-app-market":    "TR",
}

// ---- detail-search istek/yanit tipleri ----

type searchRoom struct {
	Adults   int   `json:"adults"`
	Children []int `json:"children"`
}

type searchRequest struct {
	HotelID      []int        `json:"hotelId"`
	Hos          bool         `json:"hos"`
	CheckInDate  string       `json:"checkInDate"`
	CheckOutDate string       `json:"checkOutDate"`
	Rooms        []searchRoom `json:"rooms"`
	Locale       string       `json:"locale"`
	Currency     string       `json:"currency"`
	Nationality  string       `json:"nationality"`
	Pagination   pagination   `json:"pagination"`
}

type pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type PriceBlock struct {
	Total Money `json:"total"`
}

type DiscountTag struct {
	Tag            string  `json:"tag"`
	DiscountString string  `json:"discountString"`
	Label          string  `json:"label"`
	Amount         float64 `json:"amount"`
}

type Offer struct {
	ID            string `json:"id"`
	Provider      int    `json:"provider"`
	OfferType     string `json:"offerType"`
	FreeChildAges string `json:"freeChildAgeRange"`
	Concept       struct {
		Description string `json:"description"`
		ParentCode  string `json:"parentCode"`
	} `json:"concept"`
	RoomClass struct {
		Code        string `json:"code"`
		Description string `json:"description"`
		Refundable  bool   `json:"refundable"`
	} `json:"roomClass"`
	HiddenState struct {
		Hidden bool `json:"hidden"`
	} `json:"hiddenState"`
	DiscountTags  []DiscountTag `json:"discountTags"`
	AdultCount    int           `json:"adultCount"`
	ChildrenCount int           `json:"childrenCount"`
	DiscountFrom  float64       `json:"discountFrom"`
	Saleable      bool          `json:"saleable"`
	IsBestOffer   bool          `json:"isBestOffer"`
	IsRefundable  bool          `json:"isRefundable"`
	Price         PriceBlock    `json:"price"`
	TryPrice      PriceBlock    `json:"tryPrice"`
}

type Room struct {
	RoomID      string `json:"roomId"`
	HiddenState struct {
		Hidden bool `json:"hidden"`
	} `json:"hiddenState"`
	Type struct {
		Name string `json:"name"`
	} `json:"type"`
	Offers []Offer `json:"offers"`
}

type Hotel struct {
	ID   int `json:"id"`
	Data struct {
		Name    string `json:"name"`
		Slug    string `json:"slug"`
		Address struct {
			City struct {
				Name string `json:"name"`
			} `json:"city"`
			Country struct {
				Name string `json:"name"`
			} `json:"country"`
		} `json:"address"`
	} `json:"data"`
	AvailabilityState struct {
		Availability int `json:"Availability"`
	} `json:"availabilityState"`
	Rooms []Room `json:"rooms"`
}

// SearchResponse search-core/detail-search yaniti (sadece kullanilan alanlar).
type SearchResponse struct {
	RequestID string  `json:"requestId"`
	Status    bool    `json:"status"`
	Message   string  `json:"message"`
	Code      int     `json:"code"`
	Hotels    []Hotel `json:"hotels"`
}

var reISO = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})`)

// toEnuygunDate "2026-12-05" -> "05.12.2026" (detail-search DD.MM.YYYY bekler).
func toEnuygunDate(input string) (string, error) {
	m := reISO.FindStringSubmatch(input)
	if m == nil {
		return "", fmt.Errorf("Gecersiz tarih formati: %s (yyyy-MM-dd bekleniyor)", input)
	}
	return m[3] + "." + m[2] + "." + m[1], nil
}

// childAgesParam negatif yaslari 0'a ceker; nil yerine bos dizi doner (API null sevmiyor).
func childAgesParam(ages []int) []int {
	out := make([]int, 0, len(ages))
	for _, a := range ages {
		if a < 0 {
			a = 0
		}
		out = append(out, a)
	}
	return out
}

// fetchDetailSearch bir otel + tarih + kisi icin tum oda/teklif listesini ceker.
func fetchDetailSearch(ctx context.Context, hotelID int, checkIn, checkOut string, adults int, childAges []int) (*SearchResponse, error) {
	in, err := toEnuygunDate(checkIn)
	if err != nil {
		return nil, err
	}
	out, err := toEnuygunDate(checkOut)
	if err != nil {
		return nil, err
	}
	if adults <= 0 {
		adults = 2
	}
	body, _ := json.Marshal(searchRequest{
		HotelID: []int{hotelID}, Hos: true,
		CheckInDate: in, CheckOutDate: out,
		Rooms:  []searchRoom{{Adults: adults, Children: childAgesParam(childAges)}},
		Locale: "tr_TR", Currency: "TRY", Nationality: "tr",
		Pagination: pagination{Page: 1, Limit: 30},
	})
	resp, err := httpx.Post(ctx, detailSearchURL, body, apiHeaders, timeout)
	if err != nil {
		return nil, err
	}
	var sr SearchResponse
	if err := json.Unmarshal(resp.Body, &sr); err != nil {
		return nil, fmt.Errorf("Enuygun detail-search yaniti parse edilemedi: %w", err)
	}
	if !sr.Status {
		msg := sr.Message
		if msg == "" {
			msg = "status=false"
		}
		return nil, fmt.Errorf("Enuygun detail-search hatasi: %s", msg)
	}
	return &sr, nil
}

// ---- URL'den otel verisi cozme ----

type resolved struct {
	ProviderHotelID string
	HotelName       string
	Slug            string
	Region          string
	IsDomestic      bool
	IsCyprus        bool
}

// apiDetailResponse /otel/api-detail/<slug>.json (sadece kullanilan alanlar).
type apiDetailResponse struct {
	Data struct {
		Slug  string `json:"slug"`
		Hotel struct {
			Details struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				Slug     string `json:"slug"`
				Domestic bool   `json:"domestic"`
				IsCyprus bool   `json:"isCyprus"`
			} `json:"details"`
		} `json:"hotel"`
		Region struct {
			Name string `json:"name"`
		} `json:"region"`
		DefaultParameters struct {
			Hotel struct {
				ID   int    `json:"id"`
				Slug string `json:"slug"`
				Name string `json:"name"`
			} `json:"hotel"`
			Region struct {
				Name       string `json:"name"`
				IsDomestic bool   `json:"isDomestic"`
			} `json:"region"`
		} `json:"defaultParameters"`
	} `json:"data"`
}

var (
	// /otel/detay/<slug>/ veya /otel/<slug>/ (bolge/tema/arama gibi alt yollar otel degil).
	reSlug     = regexp.MustCompile(`(?i)enuygun\.com/(?:[a-z]{2}/)?(?:otel|hotels)/(?:detay/|detail/)?([a-z0-9][a-z0-9-]*)/?(?:[?#]|$)`)
	reTrailID  = regexp.MustCompile(`-(\d{3,})$`)
	nonHotelRe = regexp.MustCompile(`(?i)^(bolge|tema|arama|detay|detail|region|search|paket-arama|paket-detay|kis-tatili|search-core|api-detail)$`)
)

// slugFromURL detay URL'inden otel slug'ini ayiklar ("" bulunamazsa).
func slugFromURL(u string) string {
	m := reSlug.FindStringSubmatch(u)
	if m == nil {
		return ""
	}
	s := strings.ToLower(m[1])
	if nonHotelRe.MatchString(s) {
		return ""
	}
	return s
}

// idFromSlug "acapulco-resort-convention-spa-580512" -> 580512 (0 yoksa).
func idFromSlug(slug string) int {
	m := reTrailID.FindStringSubmatch(slug)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func parseAPIDetail(body []byte) (*resolved, error) {
	var d apiDetailResponse
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("Enuygun api-detail yaniti parse edilemedi: %w", err)
	}
	id := d.Data.DefaultParameters.Hotel.ID
	name := d.Data.DefaultParameters.Hotel.Name
	slug := d.Data.DefaultParameters.Hotel.Slug
	if id == 0 {
		id = d.Data.Hotel.Details.ID
	}
	if name == "" {
		name = d.Data.Hotel.Details.Name
	}
	if slug == "" {
		slug = d.Data.Hotel.Details.Slug
	}
	if slug == "" {
		slug = d.Data.Slug
	}
	if id == 0 {
		return nil, fmt.Errorf("Enuygun otel bulunamadi (otel detay sayfasi URL'i olmali)")
	}
	region := d.Data.Region.Name
	if region == "" {
		region = d.Data.DefaultParameters.Region.Name
	}
	return &resolved{
		ProviderHotelID: strconv.Itoa(id), HotelName: strings.TrimSpace(name), Slug: slug, Region: region,
		IsDomestic: d.Data.Hotel.Details.Domestic || d.Data.DefaultParameters.Region.IsDomestic,
		IsCyprus:   d.Data.Hotel.Details.IsCyprus,
	}, nil
}

// resolveHotelFromURL slug'i api-detail ile cozer (slug sonundaki sayi her otelde yok; orn. club-simena).
func resolveHotelFromURL(ctx context.Context, u string) (*resolved, error) {
	slug := slugFromURL(u)
	if slug == "" {
		return nil, fmt.Errorf("Enuygun URL'inden otel slug'i cozulemedi (/otel/detay/<slug>/ bekleniyor)")
	}
	q := url.Values{"slug": {slug}, "type": {"hotel"}}
	endpoint := fmt.Sprintf(apiDetailURL, url.PathEscape(slug)) + "?" + q.Encode()
	headers := httpx.Merge(apiHeaders, map[string]string{"referer": base + "/otel/detay/" + slug + "/"})
	resp, err := httpx.Do(ctx, http.MethodGet, endpoint, httpx.Options{Headers: headers, Timeout: timeout})
	if err == nil {
		r, perr := parseAPIDetail(resp.Body)
		if perr == nil {
			return r, nil
		}
		err = perr
	}
	// api-detail cevap vermezse slug sonundaki sayisal id ile (ad/bolge olmadan) devam et.
	if id := idFromSlug(slug); id > 0 {
		return &resolved{ProviderHotelID: strconv.Itoa(id), Slug: slug}, nil
	}
	return nil, err
}
