package types

import (
	"math"
	"time"
)

// ---- Ortak sorgu tipleri (standart: yyyy-MM-dd) ----

type PriceQuery struct {
	HotelID   string
	CheckIn   string
	CheckOut  string
	Adults    int
	Children  int
	ChildAges []int
}

type BulkPriceQuery struct {
	HotelIDs  []string
	CheckIn   string
	CheckOut  string
	Adults    int
	Children  int
	ChildAges []int
}

type PackageQuery struct {
	HotelID     string
	AirportCode string
	CheckIn     string
	CheckOut    string
	Adults      int
	Children    int
	ChildAges   []int
}

// ---- Normalize cikti tipleri (tum provider'lar bu formatta doner) ----

type NormalizedPrice struct {
	Available         bool     `json:"available"`
	HotelName         *string  `json:"hotelName"`
	City              *string  `json:"city"`
	RoomName          *string  `json:"roomName"`
	BoardType         *string  `json:"boardType"`
	Currency          *string  `json:"currency"`
	ListPrice         *float64 `json:"listPrice"`
	Price             *float64 `json:"price"`
	DiscountRate      float64  `json:"discountRate"`
	DiscountPrice     *float64 `json:"discountPrice,omitempty"`
	CampaignName      *string  `json:"campaignName,omitempty"`
	BankCampaignPrice *float64 `json:"bankCampaignPrice,omitempty"`
	BankCampaignLabel *string  `json:"bankCampaignLabel,omitempty"`
	MinStayNights     *int     `json:"minStayNights,omitempty"`
	Note              *string  `json:"note,omitempty"` // orn. "musait degil; sitede gosterilen fiyat"
}

// Unavailable bos (musait degil) fiyat kaydi uretir.
func Unavailable(hotelName, city *string) *NormalizedPrice {
	return &NormalizedPrice{Available: false, HotelName: hotelName, City: city}
}

type NormalizedPackageBoard struct {
	BoardType    *string  `json:"boardType"`
	Currency     *string  `json:"currency"`
	ListPrice    *float64 `json:"listPrice"`
	Price        *float64 `json:"price"`
	DiscountRate float64  `json:"discountRate"`
	Cancellation *string  `json:"cancellation"`
}

type NormalizedPackageRoom struct {
	RoomID     string                   `json:"roomId"`
	RoomName   string                   `json:"roomName"`
	RoomSize   *int                     `json:"roomSize"`
	NightCount *int                     `json:"nightCount"`
	Boards     []NormalizedPackageBoard `json:"boards"`
}

// ---- Pointer yardimcilari ----

func Str(s string) *string { return &s }

// StrOrNil bos string icin nil doner.
func StrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func F64(v float64) *float64 { return &v }

// F64Pos sadece pozitif ve sonlu degerler icin pointer doner.
func F64Pos(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return nil
	}
	return &v
}

func Int(v int) *int { return &v }

func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// DiscountRateOf liste ve satis fiyatindan yuvarlanmis indirim yuzdesi hesaplar.
func DiscountRateOf(listPrice, price *float64) float64 {
	if listPrice == nil || price == nil || *listPrice <= *price || *price <= 0 {
		return 0
	}
	return math.Round((1 - *price / *listPrice) * 100)
}

// NightsBetween iki yyyy-MM-dd tarihi arasindaki gece sayisi (parse edilemezse nil).
func NightsBetween(checkIn, checkOut string) *int {
	a, err1 := time.Parse("2006-01-02", checkIn[:min(10, len(checkIn))])
	b, err2 := time.Parse("2006-01-02", checkOut[:min(10, len(checkOut))])
	if err1 != nil || err2 != nil {
		return nil
	}
	n := int(math.Round(b.Sub(a).Hours() / 24))
	return &n
}
