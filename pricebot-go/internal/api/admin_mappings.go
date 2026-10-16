package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"pricebot/internal/db"
	"pricebot/internal/provider"
)

// GET /api/admin/mappings?provider=&search=&active=
func (s *Server) listMappings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var active *bool
	if v := q.Get("active"); v != "" {
		b := v == "true"
		active = &b
	}
	res, err := s.db.ListMappings(r.Context(), db.MappingFilter{Provider: q.Get("provider"), Search: q.Get("search"), Active: active})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, res)
}

type mappingBody struct {
	Provider        string          `json:"provider"`
	InternalHotelID string          `json:"internalHotelId"`
	ProviderHotelID string          `json:"providerHotelId"`
	HotelName       *string         `json:"hotelName"`
	Metadata        json.RawMessage `json:"metadata"`
	IsActive        *bool           `json:"isActive"`
}

func (b *mappingBody) metadata() map[string]any {
	if len(b.Metadata) == 0 || string(b.Metadata) == "null" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b.Metadata, &m); err != nil {
		return nil
	}
	return m
}

func (b *mappingBody) active() bool {
	return b.IsActive == nil || *b.IsActive
}

func (b *mappingBody) input() provider.MappingInput {
	return provider.MappingInput{
		InternalHotelID: b.InternalHotelID, ProviderHotelID: b.ProviderHotelID,
		HotelName: b.HotelName, IsActive: b.IsActive, Metadata: b.metadata(),
	}
}

func emptyToNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// POST /api/admin/mappings
func (s *Server) createMapping(w http.ResponseWriter, r *http.Request) {
	var body mappingBody
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if body.Provider == "" || body.InternalHotelID == "" || body.ProviderHotelID == "" {
		badRequest(w, "provider, internalHotelId ve providerHotelId zorunlu.")
		return
	}
	p, err := s.reg.Get(body.Provider)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if msg := p.ValidateMappingInput(body.input()); msg != "" {
		badRequest(w, msg)
		return
	}
	m, err := s.db.CreateMapping(r.Context(), db.MappingUpsert{
		Provider: body.Provider, InternalHotelID: body.InternalHotelID, ProviderHotelID: body.ProviderHotelID,
		HotelName: emptyToNil(body.HotelName), Metadata: body.metadata(), IsActive: body.active(),
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			writeJSON(w, 409, map[string]any{"error": "Bu provider + internalHotelId kombinasyonu zaten mevcut."})
			return
		}
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, m)
}

// POST /api/admin/mappings/bulk
func (s *Server) bulkMappings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string        `json:"provider"`
		Mappings []mappingBody `json:"mappings"`
	}
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if body.Provider == "" || len(body.Mappings) == 0 {
		badRequest(w, "provider ve mappings dizisi zorunlu.")
		return
	}
	p, err := s.reg.Get(body.Provider)
	if err != nil {
		writeError(w, r, err)
		return
	}
	ups := make([]db.MappingUpsert, 0, len(body.Mappings))
	for _, m := range body.Mappings {
		if msg := p.ValidateMappingInput(m.input()); msg != "" {
			badRequest(w, fmt.Sprintf("%s: %s", m.InternalHotelID, msg))
			return
		}
		ups = append(ups, db.MappingUpsert{
			Provider: body.Provider, InternalHotelID: m.InternalHotelID, ProviderHotelID: m.ProviderHotelID,
			HotelName: emptyToNil(m.HotelName), Metadata: m.metadata(), IsActive: m.active(),
		})
	}
	res, err := s.db.UpsertMappingsTx(r.Context(), ups)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"count": len(res), "mappings": res})
}

func pathID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	return id, err == nil
}

// PUT /api/admin/mappings/{id}
func (s *Server) updateMapping(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var body struct {
		InternalHotelID *string         `json:"internalHotelId"`
		ProviderHotelID *string         `json:"providerHotelId"`
		HotelName       *string         `json:"hotelName"`
		Metadata        json.RawMessage `json:"metadata"`
		IsActive        *bool           `json:"isActive"`
	}
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	patch := db.MappingPatch{InternalHotelID: body.InternalHotelID, ProviderHotelID: body.ProviderHotelID, IsActive: body.IsActive}
	if body.HotelName != nil {
		patch.HotelNameSet = true
		patch.HotelName = body.HotelName
	}
	if len(body.Metadata) > 0 {
		patch.MetadataSet = true
		if string(body.Metadata) != "null" {
			_ = json.Unmarshal(body.Metadata, &patch.Metadata)
		}
	}
	m, err := s.db.UpdateMappingByID(r.Context(), id, patch)
	if errors.Is(err, db.ErrNotFound) {
		writeJSON(w, 404, map[string]any{"error": "Eslestirme bulunamadi."})
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, m)
}

// DELETE /api/admin/mappings/{id}
func (s *Server) deleteMapping(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	err := s.db.DeleteMappingByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeJSON(w, 404, map[string]any{"error": "Eslestirme bulunamadi."})
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"message": "Eslestirme silindi."})
}
