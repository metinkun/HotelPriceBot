package tatilcikus

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/types"
)

// Durumlar sitenin cartInfo.status degerleriyle ayni.
const (
	statusAvailable = "AVAILABLE"
	statusOnRequest = "ON_REQUEST"
	statusStopSale  = "STOP_SALE"
)

// offer tedarikciden bagimsiz normalize edilmis tek oda+pansiyon teklifi (sitede gosterilen musteri fiyati).
type offer struct {
	RoomName     string
	BoardName    string
	Price        float64 // sitede gosterilen satis fiyati (roomPrice)
	ListPrice    float64 // ustu cizili fiyat (roomPrevPrice); 0 ise yok
	DiscountRate float64 // sitenin gosterdigi % (yoksa hesaplanir)
	Currency     string
	Status       string
}

// findItem tedarikci id'si ile eslesen, gosterimde ve satista olan oda/pansiyon kaydini bulur.
func findItem(items []Item, op int, id string) *Item {
	for i := range items {
		it := &items[i]
		var ids []string
		switch op {
		case opCoral:
			ids = it.CoralIDs
		case opGezinomi:
			ids = it.GezinomiIDs
		case opAkgunler:
			ids = it.AkgunlerIDs
		}
		for _, x := range ids {
			if x == id {
				return it
			}
		}
	}
	return nil
}

func sellable(it *Item) bool { return it != nil && it.IsShown && it.OpenForSale }

func currencyLabel(c string) string {
	switch strings.ToUpper(strings.TrimSpace(c)) {
	case "", "TL", "TRY":
		return "TL"
	}
	return strings.ToUpper(strings.TrimSpace(c))
}

// jsParseInt JavaScript parseInt(number) davranisi: sifira dogru kirpma.
func jsParseInt(v float64) float64 { return math.Trunc(v) }

// ---- Akgunler (priceOperator 4) ----

// gezinomiStatus sitenin Wt.Ii yardimcisinin karsiligi: reasons listesinden durum uretir.
func gezinomiStatus(reasons []string) string {
	if len(reasons) == 0 {
		return statusAvailable
	}
	for _, r := range reasons {
		switch r {
		case "Realesa", "Stop", "Min Geceleme", "Minumum Geceleme", "CheckIn Kısıtı", "CheckOut Kısıtı", "Kontrat Yok", "Fiyat Yok":
			return statusStopSale
		}
	}
	for _, r := range reasons {
		if r == "Kota" || r == "Sot-Sat" {
			return statusOnRequest
		}
	}
	return statusAvailable
}

func akgunlerStatus(r *AkgunlerRoom) string {
	switch {
	case r.StopSale || r.Release:
		return statusStopSale
	case r.AskSale:
		return statusOnRequest
	}
	return statusAvailable
}

// commissionFor gun icin musteri komisyon orani: tarih araligi eslesen override, yoksa otelin varsayilani.
func commissionFor(day time.Time, overrides []CustomerCommission, def float64) float64 {
	for _, c := range overrides {
		s, err1 := time.Parse("2006-01-02", c.DateOfStart[:min(10, len(c.DateOfStart))])
		e, err2 := time.Parse("2006-01-02", c.DateOfEnd[:min(10, len(c.DateOfEnd))])
		if err1 != nil || err2 != nil {
			continue
		}
		if !day.Before(s) && !day.After(e) {
			return c.CommissionRate
		}
	}
	return def
}

// netCommissionPrice isActiveCustomerNetPriceCommission=true otellerde sitenin uyguladigi fiyat (GrossPrice - Mt)
// ve indirim yuzdesi (jt). Mt = SUM(gun) parseInt(GrossAmount * (komisyon + CommissionPercentEB) / 100).
// CommissionPercentEB 0 ise site Mt=null kabul eder -> fiyat GrossPrice, indirim (komisyon+0).
func netCommissionPrice(h *Hotel, r *AkgunlerRoom, overrides []CustomerCommission) (price, rate float64) {
	eb := jsParseInt(r.CommissionPercentEB)
	rate = h.CustomerNetPriceCommission + eb
	if eb == 0 {
		return r.GrossPrice, rate
	}
	var mt float64
	for _, d := range r.DailyPrices {
		day, err := time.Parse("2006-01-02", d.Date[:min(10, len(d.Date))])
		if err != nil {
			continue
		}
		w := commissionFor(day, overrides, h.CustomerNetPriceCommission)
		mt += jsParseInt(d.GrossAmount * (w + eb) / 100)
		rate = w + eb
	}
	return r.GrossPrice - mt, rate
}

func normalizeAkgunler(d *HotelDetail, resp *AkgunlerResponse, overrides []CustomerCommission) []offer {
	out := []offer{}
	if resp == nil {
		return out
	}
	h := &d.Hotel
	for _, hs := range resp.Data {
		for _, ri := range hs.RoomItems {
			for i := range ri.Rooms {
				r := &ri.Rooms[i]
				room := findItem(d.Rooms, opAkgunler, r.RoomID)
				board := findItem(d.Concepts, opAkgunler, r.BoardID)
				if !sellable(room) || !sellable(board) {
					continue
				}
				o := offer{RoomName: room.Name, BoardName: board.Name, Currency: "TL", Status: akgunlerStatus(r),
					Price: r.SalePrice, ListPrice: r.GrossPrice, DiscountRate: r.CommissionPercentEB}
				if h.IsActiveCustomerNetPriceCommission {
					o.Price, o.DiscountRate = netCommissionPrice(h, r, overrides)
				}
				out = append(out, o)
			}
		}
	}
	return out
}

// ---- Coral (priceOperator 2) ----

func coralStatus(s int) string {
	switch s {
	case 0:
		return statusAvailable
	case 2:
		return statusOnRequest
	}
	return statusStopSale
}

func normalizeCoral(d *HotelDetail, resp *CoralResponse) []offer {
	out := []offer{}
	if resp == nil {
		return out
	}
	for i := range resp.Data {
		c := &resp.Data[i]
		room := findItem(d.Rooms, opCoral, strconv.Itoa(c.Room.ID))
		board := findItem(d.Concepts, opCoral, strconv.Itoa(c.Meal.ID))
		if !sellable(room) || !sellable(board) {
			continue
		}
		out = append(out, offer{RoomName: room.Name, BoardName: board.Name, Currency: currencyLabel(c.Currency.SName),
			Status: coralStatus(c.SaleStatus), Price: c.Price, ListPrice: c.OriginalPrice, DiscountRate: c.Discount})
	}
	return out
}

// ---- Gezinomi (priceOperator 3) ----

func normalizeGezinomi(d *HotelDetail, resp *GezinomiResponse) []offer {
	out := []offer{}
	if resp == nil {
		return out
	}
	for i := range resp.Data {
		g := &resp.Data[i]
		room := findItem(d.Rooms, opGezinomi, strconv.Itoa(g.RoomID))
		board := findItem(d.Concepts, opGezinomi, strconv.Itoa(g.ConceptTypeID))
		if !sellable(room) || !sellable(board) {
			continue
		}
		out = append(out, offer{RoomName: room.Name, BoardName: board.Name, Currency: "TL",
			Status: gezinomiStatus(g.Reasons), Price: g.AfiseFiyat, ListPrice: g.BrutFiyat, DiscountRate: g.IntDiscountPercent})
	}
	return out
}

// ---- Lead fiyat ----

// leadOffer en ucuz AVAILABLE teklif; yoksa en ucuz ON_REQUEST (sitede "Talep Et" ile gosterilir). Stop-sale'ler alinmaz.
func leadOffer(offers []offer) (*offer, bool) {
	var avail, onReq []offer
	for _, o := range offers {
		if o.Price <= 0 {
			continue
		}
		switch o.Status {
		case statusAvailable:
			avail = append(avail, o)
		case statusOnRequest:
			onReq = append(onReq, o)
		}
	}
	byPrice := func(s []offer) { sort.SliceStable(s, func(i, j int) bool { return s[i].Price < s[j].Price }) }
	if len(avail) > 0 {
		byPrice(avail)
		return &avail[0], true
	}
	if len(onReq) > 0 {
		byPrice(onReq)
		return &onReq[0], false
	}
	return nil, false
}

// normalizePrice teklif listesini ortak NormalizedPrice formatina cevirir.
func normalizePrice(offers []offer, hotelName, city string) *types.NormalizedPrice {
	o, available := leadOffer(offers)
	if o == nil {
		return types.Unavailable(types.StrOrNil(hotelName), types.StrOrNil(city))
	}
	price := types.F64Pos(o.Price)
	var listPrice *float64
	if o.ListPrice > o.Price {
		listPrice = types.F64Pos(o.ListPrice)
	}
	rate := math.Round(o.DiscountRate)
	if rate <= 0 {
		rate = types.DiscountRateOf(listPrice, price)
	}
	np := &types.NormalizedPrice{
		Available:    available,
		HotelName:    types.StrOrNil(hotelName),
		City:         types.StrOrNil(city),
		RoomName:     types.StrOrNil(o.RoomName),
		BoardType:    types.StrOrNil(o.BoardName),
		Currency:     types.Str(o.Currency),
		ListPrice:    listPrice,
		Price:        price,
		DiscountRate: rate,
	}
	if !available {
		np.CampaignName = types.Str("Sorulacak (ON_REQUEST) - anlik musaitlik yok, site 'Talep Et' gosteriyor")
	}
	return np
}
