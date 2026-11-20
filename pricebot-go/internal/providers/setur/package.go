package setur

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"pricebot/internal/httpx"
	"pricebot/internal/types"
)

// Paket sayfasi (isPackage=true&dep=<kalkis>) RSC akisinda `packageInitialData.data`:
//   hotel.rooms[].informations[].variations[]  -> oda+pansiyon OTEL fiyati (konaklama toplami)
//   flights[]  (selected=true gidis + donus)   -> secili ucuslarin fiyati (tum yolcular)
//   transfers[] (selected=true Departure/Return) -> secili transferlerin fiyati
// Sitenin gosterdigi paket fiyati = oda fiyati + secili ucuslar + secili transferler
// (dogrulandi: Salamis Park 22.000 + 6.468 + 7.810 + 2.400 + 2.400 = 41.078 TL).

type pkgVariation struct {
	PromotionInfoName           string   `json:"promotionInfoName"`
	TotalSellingPrice           *float64 `json:"totalSellingPrice"`
	TotalDiscountedSellingPrice *float64 `json:"totalDiscountedSellingPrice"`
	DiscountPercent             float64  `json:"discountPercent"`
	Currency                    string   `json:"currency"`
}

type pkgRoom struct {
	ID           any    `json:"id"`
	Name         string `json:"name"`
	Informations []struct {
		AvailabilityStatus string         `json:"availabilityStatus"`
		BoardTypeName      string         `json:"boardTypeName"`
		IsRefundable       bool           `json:"isRefundable"`
		Variations         []pkgVariation `json:"variations"`
	} `json:"informations"`
}

type pkgFlightOption struct {
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Selected bool    `json:"selected"`
	Flights  []struct {
		LegNumber     int    `json:"legNumber"`
		DepartureDate string `json:"departureDate"`
		FlightNo      string `json:"flightno"`
		AirlineName   string `json:"airlineName"`
		Segments      []struct {
			Origin      string `json:"origin"`
			Destination string `json:"destination"`
			DepTime     string `json:"depTime"`
			FlightNo    string `json:"flightno"`
		} `json:"segments"`
	} `json:"flights"`
}

type pkgTransfer struct {
	Selected     bool     `json:"selected"`
	TransferType string   `json:"transferType"`
	Name         string   `json:"name"`
	TrySalePrice *float64 `json:"trySalePrice"`
}

type packageData struct {
	Data struct {
		Hotel struct {
			ID          any          `json:"id"`
			Name        string       `json:"name"`
			NightCount  *int         `json:"nightCount"`
			LowestPrice *LowestPrice `json:"lowestPrice"`
			Rooms       []pkgRoom    `json:"rooms"`
		} `json:"hotel"`
		Flights   []pkgFlightOption `json:"flights"`
		Transfers []pkgTransfer     `json:"transfers"`
	} `json:"data"`
	IsError bool `json:"isError"`
}

// parsePackageData RSC akisindan packageInitialData'yi cozer.
func parsePackageData(payload string) (*packageData, error) {
	var d packageData
	if err := rscExtract(payload, `"packageInitialData":`, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// pkgExtras secili ucus + transfer toplami ve aciklama notu.
func pkgExtras(d *packageData) (flights, transfers float64, note string) {
	var legs []string
	for _, o := range d.Data.Flights {
		if !o.Selected {
			continue
		}
		flights += o.Price
		for _, l := range o.Flights {
			if len(l.Segments) > 0 {
				s := l.Segments[0]
				fn := l.FlightNo
				if fn == "" {
					fn = s.FlightNo
				}
				legs = append(legs, strings.TrimSpace(fmt.Sprintf("%s→%s %s %s %s", s.Origin, l.Segments[len(l.Segments)-1].Destination, strings.TrimSpace(strings.SplitN(l.DepartureDate, "T", 2)[0]), s.DepTime, fn)))
			}
		}
	}
	var trs []string
	for _, t := range d.Data.Transfers {
		if !t.Selected || t.TrySalePrice == nil {
			continue
		}
		transfers += *t.TrySalePrice
		trs = append(trs, strings.TrimSpace(t.Name))
	}
	note = "Setur paket = oda + secili ucuslar + secili transfer (sitenin varsayilan secimi)."
	if len(legs) > 0 {
		note += " Ucuslar: " + strings.Join(legs, " / ") + fmt.Sprintf(" (%.0f TL)", flights)
	}
	if len(trs) > 0 {
		note += "; transfer: " + strings.Join(uniq(trs), ", ") + fmt.Sprintf(" (%.0f TL)", transfers)
	}
	return
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// normalizePackageRooms oda x pansiyon paket fiyatlari (oda + ucus + transfer).
func normalizePackageRooms(d *packageData) []types.NormalizedPackageRoom {
	out := []types.NormalizedPackageRoom{}
	if d == nil {
		return out
	}
	flights, transfers, _ := pkgExtras(d)
	add := flights + transfers
	for i, r := range d.Data.Hotel.Rooms {
		room := types.NormalizedPackageRoom{RoomID: idString(r.ID), RoomName: strings.TrimSpace(r.Name), NightCount: d.Data.Hotel.NightCount, Boards: []types.NormalizedPackageBoard{}}
		if room.RoomID == "" {
			room.RoomID = fmt.Sprint(i)
		}
		for _, inf := range r.Informations {
			if !strings.EqualFold(inf.AvailabilityStatus, "Available") {
				continue
			}
			for _, v := range inf.Variations {
				if v.TotalDiscountedSellingPrice == nil || *v.TotalDiscountedSellingPrice <= 0 {
					continue
				}
				price := types.F64(*v.TotalDiscountedSellingPrice + add)
				var list *float64
				if v.TotalSellingPrice != nil && *v.TotalSellingPrice > *v.TotalDiscountedSellingPrice {
					list = types.F64(*v.TotalSellingPrice + add)
				}
				cur := v.Currency
				if cur == "" {
					cur = "TL"
				}
				var cancel *string
				if inf.IsRefundable {
					cancel = types.Str("FREE_CANCELLATION")
				}
				room.Boards = append(room.Boards, types.NormalizedPackageBoard{
					BoardType: types.StrOrNil(inf.BoardTypeName), Currency: types.Str(cur),
					ListPrice: list, Price: price, DiscountRate: types.DiscountRateOf(list, price), Cancellation: cancel,
				})
			}
		}
		if len(room.Boards) > 0 {
			out = append(out, room)
		}
	}
	return out
}

// fetchPackageData paket sayfasini ceker ve packageInitialData'yi cozer.
func fetchPackageData(ctx context.Context, slug, checkIn, checkOut string, adults int, childAges []int, departureCode, arrivalCode string) (*packageData, error) {
	params := []string{
		"in=" + checkIn, "out=" + checkOut, "room=" + buildRoomParam(adults, childAges),
		"isPackage=true", "dep=" + url.QueryEscape(strings.ToUpper(departureCode)), "depIsCity=true",
	}
	if arrivalCode != "" {
		params = append(params, "arr="+url.QueryEscape(strings.ToUpper(arrivalCode)))
	}
	u := fmt.Sprintf("%s/%s?%s", base, slug, strings.Join(params, "&"))
	resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{Headers: pageHeaders, Timeout: pageTimeout})
	if err != nil {
		return nil, err
	}
	d, err := parsePackageData(rscPayload(resp.Text()))
	if err != nil {
		return nil, fmt.Errorf("Setur paket verisi bulunamadi (sayfa paket modunda degil?): %w", err)
	}
	return d, nil
}
