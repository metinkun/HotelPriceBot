package api

import (
	"net/http"
	"strings"

	"pricebot/internal/services"
)

// POST /api/admin/hotels  body: { urls: {etstur: "...", ...}, internalHotelId?, hotelName }
func (s *Server) createHotel(w http.ResponseWriter, r *http.Request) {
	var body services.AddHotelInput
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if len(body.URLs) == 0 {
		badRequest(w, `urls zorunlu. Ornek: { "urls": { "etstur": "...", "tatilsepeti": "..." } }`)
		return
	}
	if strings.TrimSpace(body.HotelName) == "" {
		badRequest(w, "hotelName zorunlu.")
		return
	}
	res, err := s.hotels.AddFromURLs(r.Context(), body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, res)
}

// GET /api/admin/hotels?search=&active=true
func (s *Server) listHotels(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var active *bool
	if v := q.Get("active"); v != "" {
		b := v == "true"
		active = &b
	}
	res, err := s.hotels.List(r.Context(), q.Get("search"), active)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, res)
}

// GET /api/admin/hotels/{id}
func (s *Server) getHotel(w http.ResponseWriter, r *http.Request) {
	res, err := s.hotels.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, res)
}

// PUT /api/admin/hotels/{id}  body: { urls?, hotelName? }
func (s *Server) updateHotel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URLs      map[string]string `json:"urls"`
		HotelName *string           `json:"hotelName"`
	}
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	res, err := s.hotels.Update(r.Context(), r.PathValue("id"), body.URLs, body.HotelName)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, res)
}

// DELETE /api/admin/hotels/{id}
func (s *Server) deleteHotel(w http.ResponseWriter, r *http.Request) {
	res, err := s.hotels.Delete(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"message": "Otel silindi.", "internalHotelId": res.InternalHotelID, "deletedMappings": res.DeletedMappings})
}
