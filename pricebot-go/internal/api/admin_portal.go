package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"pricebot/internal/apperr"
	"pricebot/internal/db"
)

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// ---- Admin: musteri ayarlari (tip, etkin siteler) ----

// GET /api/admin/clients/{id}/settings
func (s *Server) adminClientSettings(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	st, err := s.db.GetClientSettings(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	users, _ := s.db.ListPortalUsers(r.Context(), id)
	hotels, _ := s.db.ListClientHotels(r.Context(), id)
	schedules, _ := s.db.ListSchedules(r.Context(), id)
	writeJSON(w, 200, map[string]any{"settings": st, "providers": s.reg.Names(), "portalUsers": users, "hotels": hotels, "schedules": schedules})
}

// PUT /api/admin/clients/{id}/settings {clientType?, enabledProviders?}
func (s *Server) adminUpdateClientSettings(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var body struct {
		ClientType       *string  `json:"clientType"`
		EnabledProviders []string `json:"enabledProviders"`
	}
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if body.ClientType != nil {
		t := strings.ToLower(strings.TrimSpace(*body.ClientType))
		if t != "api" && t != "portal" && t != "both" {
			badRequest(w, "clientType api | portal | both olmali")
			return
		}
		body.ClientType = &t
	}
	if body.EnabledProviders != nil {
		if err := s.providerNamesValid(body.EnabledProviders); err != nil {
			writeError(w, r, err)
			return
		}
	}
	if err := s.db.UpdateClientSettings(r.Context(), id, body.ClientType, body.EnabledProviders); err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	InvalidateKeyCache()
	st, _ := s.db.GetClientSettings(r.Context(), id)
	writeJSON(w, 200, st)
}

// ---- Admin: portal kullanicilari ----

// POST /api/admin/clients/{id}/portal-users {email, password, name}
func (s *Server) adminCreatePortalUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var in struct {
		Email    string  `json:"email"`
		Password string  `json:"password"`
		Name     *string `json:"name"`
	}
	if err := decodeBody(r, &in); err != nil || !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
		badRequest(w, "gecerli email ve en az 8 karakter password zorunlu")
		return
	}
	if _, err := s.db.GetClient(r.Context(), id); err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	u, err := s.db.CreatePortalUser(r.Context(), id, in.Email, string(hash), in.Name)
	if err != nil {
		if db.IsUniqueViolation(err) {
			writeError(w, r, apperr.Conflictf("Bu e-posta zaten kayitli"))
			return
		}
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, u)
}

// PUT /api/admin/portal-users/{id} {password?, isActive?, name?}
func (s *Server) adminUpdatePortalUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var in struct {
		Password *string `json:"password"`
		IsActive *bool   `json:"isActive"`
		Name     *string `json:"name"`
	}
	if err := decodeBody(r, &in); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	var hash *string
	if in.Password != nil {
		if len(*in.Password) < 8 {
			badRequest(w, "password en az 8 karakter")
			return
		}
		h, _ := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
		hs := string(h)
		hash = &hs
	}
	if err := s.db.UpdatePortalUser(r.Context(), id, hash, in.IsActive, in.Name); err != nil {
		notFoundOr(w, r, err, "Kullanici bulunamadi")
		return
	}
	writeJSON(w, 200, map[string]any{"message": "Guncellendi.", "id": id})
}

// DELETE /api/admin/portal-users/{id}
func (s *Server) adminDeletePortalUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	if err := s.db.DeletePortalUser(r.Context(), id); err != nil {
		notFoundOr(w, r, err, "Kullanici bulunamadi")
		return
	}
	writeJSON(w, 200, map[string]any{"message": "Silindi.", "id": id})
}

// GET /api/admin/schedules — tum musterilerin planli aramalari
func (s *Server) adminSchedules(w http.ResponseWriter, r *http.Request) {
	list, err := s.db.ListSchedules(r.Context(), 0)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

// GET /api/admin/clients/{id}/searches — musterinin arama gecmisi
func (s *Server) adminClientSearches(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	list, err := s.db.ListSearches(r.Context(), id, nil, atoiDefault(r.URL.Query().Get("limit"), 50))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}
