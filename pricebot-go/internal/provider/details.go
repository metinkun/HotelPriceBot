package provider

import (
	"context"
	"time"
)

// ---- Otel statik verisi (detay sayfasi) ----
//
// Her acentenin otel detay sayfasindaki bilgiler bu ORTAK semaya normalize edilir
// (gorseller, ozellikler, odalar, politikalar, soru-cevap, konum). Acentenin kendi
// basliklariyla ham gosterim icin Sections kullanilir. Semadaki alanlar JSON'a yazilir,
// gunluk degisiklik kontrolu bu JSON uzerinde alan bazinda diff alir.

type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Image struct {
	URL     string `json:"url"`
	Caption string `json:"caption,omitempty"`
}

type FacilityGroup struct {
	Title string   `json:"title"` // acentedeki grup adi (orn. "Genel", "Plaj", "Spor")
	Items []string `json:"items"`
}

type RoomInfo struct {
	Name        string   `json:"name"`
	Size        *int     `json:"size,omitempty"` // m2
	Capacity    string   `json:"capacity,omitempty"`
	Description string   `json:"description,omitempty"`
	Features    []string `json:"features,omitempty"`
	Images      []string `json:"images,omitempty"`
}

type Policies struct {
	CheckIn      string   `json:"checkIn,omitempty"`
	CheckOut     string   `json:"checkOut,omitempty"`
	ChildPolicy  string   `json:"childPolicy,omitempty"`
	PetPolicy    string   `json:"petPolicy,omitempty"`
	Cancellation string   `json:"cancellation,omitempty"`
	Other        []string `json:"other,omitempty"`
}

type QA struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Section acentenin kendi basligi ve gosterim bicimiyle ham icerik.
type Section struct {
	Title string   `json:"title"`
	Kind  string   `json:"kind"` // text | list | pairs | html
	Text  string   `json:"text,omitempty"`
	Items []string `json:"items,omitempty"`
	Pairs []KV     `json:"pairs,omitempty"`
	HTML  string   `json:"html,omitempty"` // temizlenmis (script/style yok) HTML
}

type Contact struct {
	Phone   string `json:"phone,omitempty"`
	Email   string `json:"email,omitempty"`
	Website string `json:"website,omitempty"`
}

// HotelDetails bir acentedeki otel detay sayfasinin normalize edilmis hali.
type HotelDetails struct {
	Provider        string    `json:"provider"`
	ProviderHotelID string    `json:"providerHotelId"`
	SourceURL       string    `json:"sourceUrl"`
	FetchedAt       time.Time `json:"fetchedAt"`

	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Stars       *int      `json:"stars,omitempty"`
	Rating      *float64  `json:"rating,omitempty"`      // acentenin puani (10 uzerinden normalize)
	RatingCount *int      `json:"ratingCount,omitempty"` // yorum sayisi
	Address     string    `json:"address,omitempty"`
	City        string    `json:"city,omitempty"`
	Region      string    `json:"region,omitempty"`
	Country     string    `json:"country,omitempty"`
	Location    *GeoPoint `json:"location,omitempty"`
	Contact     *Contact  `json:"contact,omitempty"`

	Images     []Image         `json:"images"`
	Facilities []FacilityGroup `json:"facilities"`
	Rooms      []RoomInfo      `json:"rooms"`
	Boards     []string        `json:"boards,omitempty"` // pansiyon tipleri
	Policies   *Policies       `json:"policies,omitempty"`
	FAQ        []QA            `json:"faq"`

	// Acentenin kendi basliklariyla tum icerik (ham/orijinal gorunum icin)
	Sections []Section `json:"sections"`
}

// DetailProvider otel statik detayini cekebilen provider.
type DetailProvider interface {
	GetDetails(ctx context.Context, m *Mapping) (*HotelDetails, error)
}

// ---- Otel arama (isimden URL bulma) ----

// HotelCandidate bir acentede isimle aramada bulunan otel.
type HotelCandidate struct {
	Name            string `json:"name"`
	URL             string `json:"url"` // ResolveFromURL'in kabul ettigi detay URL'i
	City            string `json:"city,omitempty"`
	ProviderHotelID string `json:"providerHotelId,omitempty"`
}

// HotelSearcher acentede otel adiyla arama yapabilen provider (toplu otel ekleme icin).
type HotelSearcher interface {
	SearchHotels(ctx context.Context, query string) ([]HotelCandidate, error)
}
