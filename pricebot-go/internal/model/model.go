package model

import "time"

// HotelMapping = hotel_mappings tablosu satiri.
type HotelMapping struct {
	ID              int            `json:"id"`
	Provider        string         `json:"provider"`
	InternalHotelID string         `json:"internalHotelId"`
	ProviderHotelID string         `json:"providerHotelId"`
	HotelName       *string        `json:"hotelName"`
	Metadata        map[string]any `json:"metadata"`
	IsActive        bool           `json:"isActive"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

// MetaString metadata icindeki bir alani string olarak doner (yoksa "").
func (m *HotelMapping) MetaString(key string) string {
	if m == nil || m.Metadata == nil {
		return ""
	}
	v, ok := m.Metadata[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Proxy = proxies tablosu satiri.
type Proxy struct {
	ID               int        `json:"id"`
	WebshareID       string     `json:"webshareId"`
	ProxyAddress     string     `json:"proxyAddress"`
	Port             int        `json:"port"`
	Username         string     `json:"username"`
	Password         string     `json:"password"`
	CountryCode      *string    `json:"countryCode"`
	Valid            bool       `json:"valid"`
	LastVerification *time.Time `json:"lastVerification"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}
