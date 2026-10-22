package tatilsitesi

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

// PriceDetailInfos.inType: 1 = kampanya/indirim mesaji, 6 = min. geceleme.
const (
	infoCampaign = 1
	infoMinStay  = 6
)

var reMinStay = regexp.MustCompile(`(\d+)\s*Min\.?\s*Geceleme`)

func currencyOf(c string) *string {
	c = strings.TrimSpace(c)
	switch strings.ToUpper(c) {
	case "", "TRY":
		return types.Str("TL")
	case "EURO":
		return types.Str("EUR")
	}
	return types.Str(c)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// listPriceOf NotDiscountAmount sadece satis fiyatindan buyukse liste fiyatidir.
func listPriceOf(r HotelRow) *float64 {
	if r.NotDiscountAmount > r.SaleAmount && r.SaleAmount > 0 {
		return types.F64(r.NotDiscountAmount)
	}
	return nil
}

func rowInfo(r HotelRow) (campaign *string, minStay *int) {
	for _, i := range r.PriceDetailInfos {
		msg := provider.DecodeEntities(i.StMessage)
		switch i.InType {
		case infoCampaign:
			if campaign == nil && msg != "" {
				campaign = types.Str(msg)
			}
		case infoMinStay:
			if m := reMinStay.FindStringSubmatch(msg); m != nil {
				if n, err := strconv.Atoi(m[1]); err == nil {
					minStay = types.Int(n)
				}
			}
		}
	}
	return
}

// cheapestRow rezervasyona acik (Talep Et olmayan) en ucuz satir; yoksa nil.
func cheapestRow(rows []HotelRow) *HotelRow {
	var best *HotelRow
	for i := range rows {
		r := &rows[i]
		if !r.Bookable() {
			continue
		}
		if best == nil || r.SaleAmount < best.SaleAmount {
			best = r
		}
	}
	return best
}

// normalizePrice hotelDetailSearch (Type=Hotel) satirlarini lead/en ucuz teklife cevirir.
func normalizePrice(rows []HotelRow) *types.NormalizedPrice {
	r := cheapestRow(rows)
	if r == nil {
		return &types.NormalizedPrice{Available: false}
	}
	listPrice := listPriceOf(*r)
	price := types.F64(r.SaleAmount)
	campaign, minStay := rowInfo(*r)
	return &types.NormalizedPrice{
		Available:     true,
		RoomName:      types.StrOrNil(provider.DecodeEntities(r.RoomCategoryName)),
		BoardType:     types.StrOrNil(provider.DecodeEntities(r.PansionName)),
		Currency:      currencyOf(r.SaleCurrency),
		ListPrice:     listPrice,
		Price:         price,
		DiscountRate:  types.DiscountRateOf(listPrice, price),
		CampaignName:  campaign,
		MinStayNights: minStay,
	}
}

// ---- Paket bilesenleri ----

type flightPick struct {
	Total    float64
	Currency string
	Note     string
}

func isContractedPegasus(o FlightOption) bool {
	if len(o.Segments) == 0 {
		return false
	}
	s := o.Segments[0]
	return strings.EqualFold(s.OperatingAirline, "pc") && strings.EqualFold(s.Cabin, "contracted")
}

func isPegasus(o FlightOption) bool {
	return len(o.Segments) > 0 && strings.EqualFold(o.Segments[0].OperatingAirline, "pc")
}

func sortByFare(list []FlightOption) []FlightOption {
	out := make([]FlightOption, 0, len(list))
	for _, o := range list {
		if o.TotalFare > 0 {
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TotalFare < out[j].TotalFare })
	return out
}

func firstPegasus(list []FlightOption) *FlightOption {
	for i := range list {
		if isPegasus(list[i]) {
			return &list[i]
		}
	}
	return nil
}

// pickFlights sayfanin otomatik secimini taklit eder: gidis ve donus listeleri fiyata gore siralanir, ilkler secilir.
// Anlasmali (contracted) Pegasus sinifi tek basina secilemez; bir yon o sinifsa diger yon da Pegasus olmali.
// Recommendations (gidis-donus kombinasyon) doluysa en ucuz kombinasyon kullanilir.
func pickFlights(air *AirResponse) (*flightPick, error) {
	if air == nil || air.D.Data == nil {
		return nil, fmt.Errorf("Tatilsitesi ucus sonucu bulunamadi")
	}
	if recs := sortByFare(air.D.Data.Recommendations); len(recs) > 0 {
		return &flightPick{Total: recs[0].TotalFare, Currency: recs[0].Currency}, nil
	}
	var outs, ins []FlightOption
	for _, o := range air.D.Data.FlightOptions {
		switch strings.ToUpper(o.OptionFlag) {
		case "OUT":
			outs = append(outs, o)
		case "IN":
			ins = append(ins, o)
		}
	}
	outs, ins = sortByFare(outs), sortByFare(ins)
	if len(outs) == 0 || len(ins) == 0 {
		return nil, fmt.Errorf("Tatilsitesi gidis/donus ucusu bulunamadi")
	}
	dep, ret := &outs[0], &ins[0]
	if isContractedPegasus(*dep) && !isPegasus(*ret) {
		if p := firstPegasus(ins); p != nil {
			ret = p
		}
	} else if isContractedPegasus(*ret) && !isPegasus(*dep) {
		if p := firstPegasus(outs); p != nil {
			dep = p
		}
	}
	cur := dep.Currency
	if cur == "" {
		cur = ret.Currency
	}
	note := ""
	if ret.Currency != "" && dep.Currency != "" && !strings.EqualFold(dep.Currency, ret.Currency) {
		note = "Gidis ve donus ucus para birimleri farkli."
	}
	return &flightPick{Total: round2(dep.TotalFare + ret.TotalFare), Currency: cur, Note: note}, nil
}

func cheapestTransfer(list []Transfer) *Transfer {
	var best *Transfer
	for i := range list {
		t := &list[i]
		if t.SaleAmount < 0 {
			continue
		}
		if best == nil || t.SaleAmount < best.SaleAmount {
			best = t
		}
	}
	return best
}

// pickTransfer sayfadaki secim: gidis-donus (RoundTrip) tek paket varsa ve ciftten ucuzsa o; yoksa gidis + donus en ucuzlari.
// Transfer yoksa 0 doner (sayfa da "Sonuc Bulunamadi" ile devam eder).
func pickTransfer(tr *TransferResponse) (total float64, desc string) {
	if tr == nil {
		return 0, ""
	}
	rt := cheapestTransfer(tr.D.RoundTripTransferList)
	dep := cheapestTransfer(tr.D.DepartureTransferList)
	arr := cheapestTransfer(tr.D.ArrivalTransferList)
	pair := math.Inf(1)
	if dep != nil && arr != nil {
		pair = dep.SaleAmount + arr.SaleAmount
	}
	rtv := math.Inf(1)
	if rt != nil {
		rtv = rt.SaleAmount
	}
	switch {
	case math.IsInf(rtv, 1) && math.IsInf(pair, 1):
		return 0, ""
	case rtv <= pair:
		return rtv, rt.Description
	default:
		return pair, dep.Description + " + " + arr.Description
	}
}

// normalizePackages Type=Package oda satirlarini ucak+transfer toplamiyla birlestirir (sayfadaki Package.Calc esdegeri).
func normalizePackages(rows []HotelRow, flightTotal, transferTotal float64, nightCount *int) []types.NormalizedPackageRoom {
	rooms := []types.NormalizedPackageRoom{}
	index := map[string]int{}
	for _, r := range rows {
		if r.SaleAmount <= 0 {
			continue
		}
		extra := flightTotal + transferTotal
		price := types.F64(round2(r.SaleAmount + extra))
		var listPrice *float64
		if lp := listPriceOf(r); lp != nil {
			listPrice = types.F64(round2(*lp + extra))
		}
		var cancel *string
		if r.NonRefundable {
			cancel = types.Str("İade edilemez")
		}
		if !r.Bookable() {
			cancel = types.Str("Talep üzerine")
		}
		board := types.NormalizedPackageBoard{
			BoardType: types.StrOrNil(provider.DecodeEntities(r.PansionName)), Currency: currencyOf(r.SaleCurrency),
			ListPrice: listPrice, Price: price, DiscountRate: types.DiscountRateOf(listPrice, price), Cancellation: cancel,
		}
		name := provider.DecodeEntities(r.RoomCategoryName)
		if name == "" {
			name = "Oda"
		}
		if i, ok := index[name]; ok {
			rooms[i].Boards = append(rooms[i].Boards, board)
			continue
		}
		id := r.RoomCategory
		if id == "" {
			id = fmt.Sprint(len(rooms))
		}
		index[name] = len(rooms)
		rooms = append(rooms, types.NormalizedPackageRoom{RoomID: id, RoomName: name, NightCount: nightCount, Boards: []types.NormalizedPackageBoard{board}})
	}
	sort.SliceStable(rooms, func(i, j int) bool {
		return minBoard(rooms[i]) < minBoard(rooms[j])
	})
	return rooms
}

func minBoard(r types.NormalizedPackageRoom) float64 {
	m := math.Inf(1)
	for _, b := range r.Boards {
		if b.Price != nil && *b.Price < m {
			m = *b.Price
		}
	}
	return m
}
