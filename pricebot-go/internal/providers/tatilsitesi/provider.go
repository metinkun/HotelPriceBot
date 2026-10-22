// Package tatilsitesi Tatilsitesi.com saglayicisi (ASP.NET WebMethod JSON; cerez/token gerekmez).
//
// Fiyat: POST /Methods.aspx/hotelDetailSearch (Type=Hotel) -> oda+pansiyon satirlari, en ucuz rezervasyona acik satir lead.
// Paket: hotelDetailSearch (Type=Package) + GetAirSearch (en ucuz gidis+donus) + GetPackageTransfer (en ucuz transfer);
// toplam sayfadaki Package.Calc ile ayni sekilde istemci tarafinda hesaplanir.
package tatilsitesi

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"pricebot/internal/provider"
	"pricebot/internal/types"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "tatilsitesi" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{SupportsPackages: true}
}

func refererOf(m *provider.Mapping) string {
	if u := m.MetaString("sourceUrl"); u != "" {
		return u
	}
	return base + "/"
}

func (p *Provider) GetPrice(ctx context.Context, m *provider.Mapping, q types.PriceQuery) (*provider.PriceResult, error) {
	if m.ProviderHotelID == "" {
		return nil, fmt.Errorf("providerHotelId (hotelCODE) eksik")
	}
	cp, err := buildChildParams(q.Children, q.ChildAges)
	if err != nil {
		return nil, err
	}
	rows, err := fetchRooms(ctx, m.ProviderHotelID, types.Deref(m.HotelName), refererOf(m), q.CheckIn, q.CheckOut, q.Adults, cp, "Hotel")
	if err != nil {
		return nil, err
	}
	price := normalizePrice(rows)
	price.HotelName = m.HotelName
	price.City = types.StrOrNil(m.MetaString("city"))
	return &provider.PriceResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		Found: price.Available, Price: price,
	}, nil
}

func (p *Provider) GetBulkPrices(ctx context.Context, ms []provider.Mapping, q types.BulkPriceQuery) ([]provider.PriceResult, error) {
	return provider.DefaultBulk(ctx, p, ms, q)
}

func (p *Provider) GetPackagePrices(ctx context.Context, m *provider.Mapping, q types.PackageQuery) (*provider.PackageResult, error) {
	if m.ProviderHotelID == "" {
		return nil, fmt.Errorf("providerHotelId (hotelCODE) eksik")
	}
	cityCode, cityID := m.MetaString("cityCode"), m.MetaString("cityId")
	if cityCode == "" || cityID == "" {
		return nil, fmt.Errorf("Mapping metadata'da cityCode/cityId eksik (resolveFromUrl ile olusturulmali)")
	}
	cp, err := buildChildParams(q.Children, q.ChildAges)
	if err != nil {
		return nil, err
	}
	ref := refererOf(m)

	// Uc cagri birbirinden bagimsiz; sayfa da paralel atiyor. Ucus en yavasi (~35 sn).
	type roomsRes struct {
		rows []HotelRow
		err  error
	}
	type airRes struct {
		air *AirResponse
		err error
	}
	type trRes struct {
		tr  *TransferResponse
		err error
	}
	roomsCh, airCh, trCh := make(chan roomsRes, 1), make(chan airRes, 1), make(chan trRes, 1)
	go func() {
		rows, err := fetchRooms(ctx, m.ProviderHotelID, types.Deref(m.HotelName), ref, q.CheckIn, q.CheckOut, q.Adults, cp, "Package")
		roomsCh <- roomsRes{rows, err}
	}()
	go func() {
		air, err := fetchFlights(ctx, q.AirportCode, cityCode, ref, q.CheckIn, q.CheckOut, q.Adults, cp)
		airCh <- airRes{air, err}
	}()
	go func() {
		tr, err := fetchTransfers(ctx, m.ProviderHotelID, cityID, ref, q.CheckIn, q.CheckOut, q.Adults, cp)
		trCh <- trRes{tr, err}
	}()
	rr, ar, tr := <-roomsCh, <-airCh, <-trCh
	if rr.err != nil {
		return nil, rr.err
	}
	if ar.err != nil {
		return nil, ar.err
	}

	result := &provider.PackageResult{
		InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID, HotelName: m.HotelName,
		AirportCode: q.AirportCode, CheckIn: q.CheckIn, CheckOut: q.CheckOut, Rooms: []types.NormalizedPackageRoom{},
	}
	if len(rr.rows) == 0 {
		result.Note = "Paket icin oda bulunamadi."
		return result, nil
	}
	fp, err := pickFlights(ar.air)
	if err != nil {
		return nil, err
	}
	notes := []string{}
	if fp.Note != "" {
		notes = append(notes, fp.Note)
	}
	transferTotal, transferDesc := 0.0, ""
	if tr.err != nil {
		notes = append(notes, "Transfer fiyati alinamadi, pakete eklenmedi: "+tr.err.Error())
	} else {
		transferTotal, transferDesc = pickTransfer(tr.tr)
		if transferDesc == "" {
			notes = append(notes, "Bu otel/tarih icin transfer secenegi yok; paket otel+ucak olarak hesaplandi.")
		}
	}
	notes = append(notes, fmt.Sprintf("Ucus (en ucuz gidis+donus, %s): %.2f %s; transfer: %.2f (%s). Oda fiyatlari paket sozlesmesinden.",
		strings.ToUpper(q.AirportCode), fp.Total, fp.Currency, transferTotal, transferDesc))

	rooms := normalizePackages(rr.rows, fp.Total, transferTotal, types.NightsBetween(q.CheckIn, q.CheckOut))
	result.Available = len(rooms) > 0
	result.RoomCount = len(rooms)
	result.Rooms = rooms
	result.Note = strings.Join(notes, " ")
	return result, nil
}

func (p *Provider) ValidateMappingInput(in provider.MappingInput) string {
	if in.ProviderHotelID == "" {
		return "providerHotelId (hotelCODE, orn. ACAPULCO) zorunlu"
	}
	if !in.HasMeta("cityCode") || !in.HasMeta("cityId") {
		return "metadata.cityCode ve metadata.cityId zorunlu (paket icin; resolveFromUrl otomatik doldurur)"
	}
	return ""
}

func (p *Provider) RequiredMetadataFields() []string { return []string{"cityCode", "cityId"} }

var reDomain = regexp.MustCompile(`(?i)tatilsitesi\.com`)

func (p *Provider) ResolveFromURL(ctx context.Context, url string) (*provider.ResolvedHotel, error) {
	if !reDomain.MatchString(url) {
		return nil, fmt.Errorf("Gecersiz Tatilsitesi URL'i (tatilsitesi.com bekleniyor)")
	}
	r, err := resolveHotelFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	return &provider.ResolvedHotel{
		ProviderHotelID: r.ProviderHotelID, HotelName: types.StrOrNil(r.HotelName),
		Metadata: map[string]any{"sourceUrl": url, "cityCode": r.CityCode, "cityId": r.CityID, "townId": r.TownID},
	}, nil
}
