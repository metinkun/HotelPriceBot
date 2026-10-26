package tatilcikus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"pricebot/internal/httpx"
)

// Tatilcikus tek sayfa uygulamasi (React); tum veri https://api.tatilcikus.com/<Servis>/ JSON API'sinden gelir.
// Otel fiyati, otelin `priceOperator` alanina gore farkli tedarikci servisinden cekilir:
//
//	2 = Coral    -> Coral/GetAvailableRooms    (hotelId = coralId)
//	3 = Gezinomi -> Gezinomi/GetAvailableRooms (hotelId = gezinomiId)
//	4 = Akgunler -> Akgunler/GetAvailableRooms (hotelId = akgunlerId)
//	-1 = online fiyat yok ("Sizi Arayalim"); 0 (Periods) ve 1 (Puzzle) sitede kullanilmiyor.
//
// Cerez/token gerekmez (Authorization yalnizca giris yapilmis kullanicida eklenir).
const (
	apiBase = "https://api.tatilcikus.com/"
	siteURL = "https://www.tatilcikus.com"
	ua      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

	opNone     = -1
	opPeriods  = 0
	opPuzzle   = 1
	opCoral    = 2
	opGezinomi = 3
	opAkgunler = 4

	detailCacheTTL = 10 * time.Minute
)

var apiHeaders = map[string]string{
	"accept":          "application/json, text/plain, */*",
	"accept-language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
	"origin":          siteURL,
	"referer":         siteURL + "/",
	"user-agent":      ua,
}

var postHeaders = httpx.Merge(apiHeaders, map[string]string{"content-type": "application/json"})

// ---- Hotels/GetByUrlWithDetail ----

type Hotel struct {
	ID                                 int     `json:"id"`
	Name                               string  `json:"name"`
	URL                                string  `json:"url"`
	CountryID                          int     `json:"countryId"`
	CityID                             int     `json:"cityId"`
	DistrictID                         int     `json:"districtId"`
	PriceOperator                      int     `json:"priceOperator"`
	PuzzleID                           *string `json:"puzzleId"`
	CoralID                            *string `json:"coralId"`
	GezinomiID                         *string `json:"gezinomiId"`
	AkgunlerID                         *string `json:"akgunlerId"`
	CommissionRate                     float64 `json:"commissionRate"`
	IsActiveCustomerNetPriceCommission bool    `json:"isActiveCustomerNetPriceCommission"`
	CustomerNetPriceCommission         float64 `json:"customerNetPriceCommission"`
	HasPackage                         bool    `json:"hasPackage"`
	PackagePrice                       float64 `json:"packagePrice"`
	PackagePriceMinNight               int     `json:"packagePriceMinNight"`
	FirstChildAge                      int     `json:"firstChildAge"` // bu yasin ustu tedarikcide yetiskin sayilir
	IsShown                            bool    `json:"isShown"`

	// Statik detay alanlari (GetDetails)
	FolderName             string         `json:"folderName"` // gorsel klasoru: /images/hotels/<folderName>/gallery/optimized/<url>.jpg
	GoogleMapsURL          string         `json:"googleMapsUrl"`
	GeneralText            string         `json:"generalText"`    // "<Otel> Hakkında" (HTML)
	ChildText              string         `json:"childText"`      // cocuk bilgileri (HTML)
	MarriedText            string         `json:"marriedText"`    // balayi (HTML)
	CallCenterText         string         `json:"callCenterText"` // "Çağrı Merkezi Bilgilendirmesi" (HTML)
	Score                  float64        `json:"score"`          // Tatilcikus puani (10 uzerinden)
	CancelOption           bool           `json:"cancelOption"`
	MoneyOption            bool           `json:"moneyOption"`
	TripadvisorRating      float64        `json:"tripadvisorRating"`
	TripadvisorRatingCount int            `json:"tripadvisorRatingCount"`
	TripadvisorName        string         `json:"tripadvisorName"`
	ShowTripadvisor        bool           `json:"showTripadvisor"`
	TagGroupHotelTexts     []TagGroupText `json:"tagGroupHotelTexts"`
}

// TagGroupText tag grubu basligi altinda gosterilen serbest metin (HTML).
type TagGroupText struct {
	TagGroupID int    `json:"tagGroupId"`
	Text       string `json:"text"`
}

// Item otel odasi veya pansiyon (concept); tedarikci id'leri ile eslenir.
type Item struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"` // HTML (oda/pansiyon aciklamasi)
	IsShown     bool     `json:"isShown"`
	OpenForSale bool     `json:"openForSale"`
	CoralIDs    []string `json:"coralIds"`
	GezinomiIDs []string `json:"gezinomiIds"`
	AkgunlerIDs []string `json:"akgunlerIds"`
}

type HotelDetail struct {
	Hotel       Hotel        `json:"hotel"`
	Rooms       []Item       `json:"rooms"`
	Concepts    []Item       `json:"concepts"`
	Highlights  []Highlight  `json:"highlights"`
	HotelTags   []HotelTag   `json:"hotelTags"`
	RoomTags    []RoomTag    `json:"roomTags"`
	HotelImages []HotelImage `json:"hotelImages"`
	HotelVideos []HotelVideo `json:"hotelVideos"`
	SubRatings  []SubRating  `json:"subRatings"`
}

type Highlight struct {
	Name string `json:"name"`
}

// HotelTag otelin ozelligi; ad Tags/ listesinden (tagId) cozulur.
type HotelTag struct {
	TagGroupID int  `json:"tagGroupId"`
	TagID      int  `json:"tagId"`
	IsPaid     bool `json:"isPaid"`
}

type RoomTag struct {
	RoomID int  `json:"roomId"`
	TagID  int  `json:"tagId"`
	IsPaid bool `json:"isPaid"`
}

// HotelImage gorsel; URL = siteURL + /images/hotels/<folderName>/gallery/optimized/<url>.jpg.
// groupId: 1 Genel, 2 Odalar, 3 Yeme Icme, 4 Cocuk, 5 Aktivite. roomId > 0 ise oda gorseli.
type HotelImage struct {
	ID          int     `json:"id"`
	GroupID     int     `json:"groupId"`
	RoomID      int     `json:"roomId"`
	URL         string  `json:"url"`
	Description *string `json:"description"`
	Score       int     `json:"score"`
	IsShown     bool    `json:"isShown"`
}

type HotelVideo struct {
	YoutubeVideoID string `json:"youtubeVideoId"`
}

// SubRating Tripadvisor alt puani (5 uzerinden).
type SubRating struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type detailEnvelope struct {
	Data    *HotelDetail `json:"data"`
	Success bool         `json:"success"`
	Message *string      `json:"message"`
}

// supplierID priceOperator'a gore tedarikci otel kodu.
func (h *Hotel) supplierID() string {
	var p *string
	switch h.PriceOperator {
	case opCoral:
		p = h.CoralID
	case opGezinomi:
		p = h.GezinomiID
	case opAkgunler:
		p = h.AkgunlerID
	case opPuzzle:
		p = h.PuzzleID
	}
	if p == nil {
		return ""
	}
	return *p
}

func operatorName(op int) string {
	switch op {
	case opCoral:
		return "Coral"
	case opGezinomi:
		return "Gezinomi"
	case opAkgunler:
		return "Akgunler"
	case opPuzzle:
		return "Puzzle"
	case opPeriods:
		return "Periods"
	}
	return "none"
}

type cachedDetail struct {
	d   *HotelDetail
	exp time.Time
}

var (
	detailMu    sync.Mutex
	detailCache = map[string]cachedDetail{}
)

// fetchHotelDetail slug ile otel detayini (operator, tedarikci kodlari, oda/pansiyon adlari) ceker; kisa sureli cache'ler.
func fetchHotelDetail(ctx context.Context, slug string) (*HotelDetail, error) {
	detailMu.Lock()
	if c, ok := detailCache[slug]; ok && time.Now().Before(c.exp) {
		detailMu.Unlock()
		return c.d, nil
	}
	detailMu.Unlock()

	u := apiBase + "Hotels/GetByUrlWithDetail?url=" + url.QueryEscape(slug)
	resp, err := httpx.Get(ctx, u, apiHeaders, 25*time.Second)
	if err != nil {
		return nil, err
	}
	d, err := parseDetail(resp.Body)
	if err != nil {
		return nil, err
	}
	detailMu.Lock()
	detailCache[slug] = cachedDetail{d: d, exp: time.Now().Add(detailCacheTTL)}
	detailMu.Unlock()
	return d, nil
}

func parseDetail(body []byte) (*HotelDetail, error) {
	var env detailEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("Tatilcikus otel detayi parse edilemedi: %w", err)
	}
	if env.Data == nil || env.Data.Hotel.ID == 0 {
		msg := ""
		if env.Message != nil {
			msg = ": " + *env.Message
		}
		return nil, fmt.Errorf("Tatilcikus'ta otel bulunamadi%s", msg)
	}
	return env.Data, nil
}

// ---- Musaitlik (fiyat) cagrilari ----

type availabilityRequest struct {
	HotelID        string   `json:"hotelId"`
	StartDate      string   `json:"startDate"` // DD.MM.YYYY
	EndDate        string   `json:"endDate"`   // DD.MM.YYYY
	AdultCount     int      `json:"adultCount"`
	ChildrenBirths []string `json:"childrenBirths"` // cocuk dogum tarihleri DD.MM.YYYY
}

// Akgunler (Kibris otelleri) yaniti.
type AkgunlerDaily struct {
	SaleAmount  float64 `json:"SaleAmount"`
	NetAmount   float64 `json:"NetAmount"`
	GrossAmount float64 `json:"GrossAmount"`
	Date        string  `json:"Date"` // 2026-12-05T00:00:00
}

type AkgunlerRoom struct {
	BoardID             string          `json:"BoardId"`
	RoomID              string          `json:"RoomId"`
	SalePrice           float64         `json:"SalePrice"`
	GrossPrice          float64         `json:"GrossPrice"`
	NetPrice            float64         `json:"NetPrice"`
	CommissionAmount    float64         `json:"CommissionAmount"`
	CommissionPercentEB float64         `json:"CommissionPercentEB"`
	StopSale            bool            `json:"StopSale"`
	Release             bool            `json:"Release"`
	AskSale             bool            `json:"AskSale"`
	DailyPrices         []AkgunlerDaily `json:"DailyPrices"`
}

type AkgunlerResponse struct {
	Data []struct {
		HotelID   string `json:"HotelId"`
		RoomItems []struct {
			Rooms []AkgunlerRoom `json:"Rooms"`
		} `json:"RoomItems"`
	} `json:"data"`
	Success bool `json:"success"`
}

// Coral yaniti (yurt ici otellerin cogu).
type CoralOffer struct {
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"originalPrice"`
	Discount      float64 `json:"discount"`
	SaleStatus    int     `json:"saleStatus"` // 0 musait, 1/3 stop, 2 sorulacak
	Night         int     `json:"night"`
	Currency      struct {
		SName string `json:"sname"`
	} `json:"currency"`
	Room struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"room"`
	Meal struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"meal"`
}

type CoralResponse struct {
	Data    []CoralOffer `json:"data"`
	Success bool         `json:"success"`
}

// Gezinomi yaniti.
type GezinomiOffer struct {
	NetFiyat           float64  `json:"net_fiyat"`
	BrutFiyat          float64  `json:"brut_fiyat"`
	AfiseFiyat         float64  `json:"afise_fiyat"`
	IntDiscountPercent float64  `json:"intDiscountPercent"`
	RoomID             int      `json:"room_id"`
	ConceptTypeID      int      `json:"concept_type_id"`
	Reasons            []string `json:"reasons"`
}

type GezinomiResponse struct {
	Data    []GezinomiOffer `json:"data"`
	Success bool            `json:"success"`
}

// CustomerCommission net-fiyat komisyonlu (isActiveCustomerNetPriceCommission) oteller icin tarih araligi bazli oran.
type CustomerCommission struct {
	HotelID        int     `json:"hotelId"`
	CommissionRate float64 `json:"commissionRate"`
	DateOfStart    string  `json:"dateOfStart"`
	DateOfEnd      string  `json:"dateOfEnd"`
}

func postJSON(ctx context.Context, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := httpx.Post(ctx, apiBase+path, b, postHeaders, 40*time.Second)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("Tatilcikus %s yaniti parse edilemedi: %w", path, err)
	}
	return nil
}

func fetchAkgunler(ctx context.Context, req availabilityRequest) (*AkgunlerResponse, error) {
	var out AkgunlerResponse
	if err := postJSON(ctx, "Akgunler/GetAvailableRooms", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func fetchCoral(ctx context.Context, req availabilityRequest) (*CoralResponse, error) {
	var out CoralResponse
	if err := postJSON(ctx, "Coral/GetAvailableRooms", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func fetchGezinomi(ctx context.Context, req availabilityRequest) (*GezinomiResponse, error) {
	var out GezinomiResponse
	if err := postJSON(ctx, "Gezinomi/GetAvailableRooms", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// fetchCustomerCommissions otele ait tarih bazli musteri komisyon oranlarini doner (sadece net-fiyat otelleri icin gerekir).
func fetchCustomerCommissions(ctx context.Context, hotelID int) ([]CustomerCommission, error) {
	resp, err := httpx.Get(ctx, apiBase+"CustomerCommissions/", apiHeaders, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data []CustomerCommission `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, err
	}
	out := []CustomerCommission{}
	for _, c := range env.Data {
		if c.HotelID == hotelID {
			out = append(out, c)
		}
	}
	return out, nil
}

// ---- Sehir adi (Cities/ listesi, bir kez cekilir) ----

var (
	cityOnce sync.Once
	cityMap  map[int]string
)

func cityName(ctx context.Context, cityID int) string {
	cityOnce.Do(func() {
		cityMap = map[int]string{}
		resp, err := httpx.Get(ctx, apiBase+"Cities/", apiHeaders, 15*time.Second)
		if err != nil {
			return
		}
		var env struct {
			Data []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		if json.Unmarshal(resp.Body, &env) == nil {
			for _, c := range env.Data {
				cityMap[c.ID] = c.Name
			}
		}
	})
	return cityMap[cityID]
}

// ---- Tarih / kisi donusumleri ----

var reISO = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})`)

// toSiteDate "2026-12-05" -> "05.12.2026" (site DD.MM.YYYY bekler; ISO formati 500 doner).
func toSiteDate(iso string) (string, error) {
	m := reISO.FindStringSubmatch(iso)
	if m == nil {
		return "", fmt.Errorf("Gecersiz tarih formati: %s (yyyy-MM-dd bekleniyor)", iso)
	}
	return m[3] + "." + m[2] + "." + m[1], nil
}

// childBirthDates yas listesini giris tarihine gore dogum tarihine cevirir (giris - yas yil - 6 ay; DD.MM.YYYY).
// Site cocuk yasi degil dogum tarihi ister; oda basina en fazla 2 cocuk.
func childBirthDates(checkIn string, ages []int) ([]string, error) {
	ci, err := time.Parse("2006-01-02", checkIn[:min(10, len(checkIn))])
	if err != nil {
		return nil, fmt.Errorf("Gecersiz tarih formati: %s", checkIn)
	}
	out := make([]string, 0, len(ages))
	for _, a := range ages {
		if a < 0 {
			a = 0
		}
		b := ci.AddDate(-a, -6, 0)
		out = append(out, b.Format("02.01.2006"))
	}
	return out, nil
}

func buildRequest(supplierID, checkIn, checkOut string, adults int, childAges []int) (availabilityRequest, error) {
	start, err := toSiteDate(checkIn)
	if err != nil {
		return availabilityRequest{}, err
	}
	end, err := toSiteDate(checkOut)
	if err != nil {
		return availabilityRequest{}, err
	}
	births, err := childBirthDates(checkIn, childAges)
	if err != nil {
		return availabilityRequest{}, err
	}
	if adults <= 0 {
		adults = 2
	}
	return availabilityRequest{HotelID: supplierID, StartDate: start, EndDate: end, AdultCount: adults, ChildrenBirths: births}, nil
}

// ---- URL -> slug ----

var reDomain = regexp.MustCompile(`(?i)^https?://(www\.)?tatilcikus\.com/`)

// slugFromURL "https://www.tatilcikus.com/acapulco-resort-hotel-spa?start=..." -> "acapulco-resort-hotel-spa".
func slugFromURL(raw string) (string, error) {
	if !reDomain.MatchString(strings.TrimSpace(raw)) {
		return "", fmt.Errorf("Gecersiz Tatilcikus URL'i (tatilcikus.com bekleniyor)")
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	slug := parts[0]
	if slug == "" || len(parts) > 1 || strings.ContainsAny(slug, ".") {
		return "", fmt.Errorf("Tatilcikus otel detay URL'i olmali (ornek: https://www.tatilcikus.com/acapulco-resort-hotel-spa)")
	}
	return slug, nil
}
