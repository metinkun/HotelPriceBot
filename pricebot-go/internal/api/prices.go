package api

import (
	"net/http"

	"pricebot/internal/types"
)

// GET /api/prices/all?hotelId=&checkIn=&checkOut=&adults=[&children=&childAges=&provider=a,b&fresh=true]
func (s *Server) getAllPrices(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("hotelId") == "" || q.Get("checkIn") == "" || q.Get("checkOut") == "" || q.Get("adults") == "" {
		badRequest(w, "Eksik parametre. hotelId, checkIn, checkOut, adults zorunlu.")
		return
	}
	pq := types.PriceQuery{
		HotelID: q.Get("hotelId"), CheckIn: q.Get("checkIn"), CheckOut: q.Get("checkOut"),
		Adults: atoiDefault(q.Get("adults"), 0), Children: atoiDefault(q.Get("children"), 0),
		ChildAges: parseChildAges(q.Get("childAges")),
	}
	if err := validatePriceQuery(&pq); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.requireClientHotels(r, pq.HotelID); err != nil {
		writeError(w, r, err)
		return
	}
	providers := parseProviderList(q.Get("provider"))
	if err := s.providerNamesValid(providers); err != nil {
		writeError(w, r, err)
		return
	}
	providers, err := s.providersForPlan(r, providers)
	if err != nil {
		writeError(w, r, err)
		return
	}
	res, err := s.prices.GetMultiProviderPrices(r.Context(), pq, providers, s.freshAllowed(r, boolParam(q.Get("fresh"))))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, res)
}

// GET /api/prices?provider=&hotelId=&checkIn=&checkOut=&adults=
func (s *Server) getPrice(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("provider") == "" || q.Get("hotelId") == "" || q.Get("checkIn") == "" || q.Get("checkOut") == "" || q.Get("adults") == "" {
		badRequest(w, "Eksik parametre. provider, hotelId, checkIn, checkOut, adults zorunlu.")
		return
	}
	pq := types.PriceQuery{
		HotelID: q.Get("hotelId"), CheckIn: q.Get("checkIn"), CheckOut: q.Get("checkOut"),
		Adults: atoiDefault(q.Get("adults"), 0), Children: atoiDefault(q.Get("children"), 0),
		ChildAges: parseChildAges(q.Get("childAges")),
	}
	if err := validatePriceQuery(&pq); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.requireClientHotels(r, pq.HotelID); err != nil {
		writeError(w, r, err)
		return
	}
	if _, err := s.providersForPlan(r, []string{q.Get("provider")}); err != nil {
		writeError(w, r, err)
		return
	}
	res, err := s.prices.GetPrice(r.Context(), q.Get("provider"), pq, s.freshAllowed(r, boolParam(q.Get("fresh"))))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, res)
}

// POST /api/prices/bulk
func (s *Server) getBulkPrices(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider  string   `json:"provider"`
		HotelIDs  []string `json:"hotelIds"`
		CheckIn   string   `json:"checkIn"`
		CheckOut  string   `json:"checkOut"`
		Adults    int      `json:"adults"`
		Children  int      `json:"children"`
		ChildAges []int    `json:"childAges"`
	}
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if body.Provider == "" || len(body.HotelIDs) == 0 || body.CheckIn == "" || body.CheckOut == "" || body.Adults == 0 {
		badRequest(w, "Eksik parametre. provider, hotelIds (array), checkIn, checkOut, adults zorunlu.")
		return
	}
	if body.ChildAges == nil {
		body.ChildAges = []int{}
	}
	bq := types.BulkPriceQuery{
		HotelIDs: body.HotelIDs, CheckIn: body.CheckIn, CheckOut: body.CheckOut,
		Adults: body.Adults, Children: body.Children, ChildAges: body.ChildAges,
	}
	if err := validateBulkQuery(&bq); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.requireClientHotels(r, bq.HotelIDs...); err != nil {
		writeError(w, r, err)
		return
	}
	if _, err := s.providersForPlan(r, []string{body.Provider}); err != nil {
		writeError(w, r, err)
		return
	}
	res, err := s.prices.GetBulkPrices(r.Context(), body.Provider, bq)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, res)
}

// GET /api/packages?provider=[a,b]&hotelId=&airportCode=&checkIn=&checkOut=&adults=
func (s *Server) getPackages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("hotelId") == "" || q.Get("airportCode") == "" || q.Get("checkIn") == "" || q.Get("checkOut") == "" || q.Get("adults") == "" {
		badRequest(w, "Eksik parametre. hotelId, airportCode, checkIn, checkOut, adults zorunlu.")
		return
	}
	pq := types.PackageQuery{
		HotelID: q.Get("hotelId"), AirportCode: q.Get("airportCode"), CheckIn: q.Get("checkIn"), CheckOut: q.Get("checkOut"),
		Adults: atoiDefault(q.Get("adults"), 0), Children: atoiDefault(q.Get("children"), 0),
		ChildAges: parseChildAges(q.Get("childAges")),
	}
	if err := validatePackageQuery(&pq); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.requireClientHotels(r, pq.HotelID); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.packagesAllowed(r); err != nil {
		writeError(w, r, err)
		return
	}
	list := parseProviderList(q.Get("provider"))
	if err := s.providerNamesValid(list); err != nil {
		writeError(w, r, err)
		return
	}
	list, err := s.providersForPlan(r, list)
	if err != nil {
		writeError(w, r, err)
		return
	}
	fresh := s.freshAllowed(r, boolParam(q.Get("fresh")))
	if len(list) == 1 {
		res, err := s.packages.GetPackagePrices(r.Context(), list[0], pq, fresh)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, 200, res)
		return
	}
	res, err := s.packages.GetMultiProviderPackagePrices(r.Context(), list, pq, fresh)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, res)
}
