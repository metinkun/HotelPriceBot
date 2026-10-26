package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/db"
	"pricebot/internal/services"
)

// ---- Otel statik detayi: ortak handler'lar (admin + portal) ----

// detailProviders otelin acenteleri + detay durumu.
func (s *Server) detailOverview(w http.ResponseWriter, r *http.Request, hotelID string) {
	hv, err := s.hotels.Get(r.Context(), hotelID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	rows, _ := s.db.ListDetailsForHotel(r.Context(), hotelID)
	byProv := map[string]db.HotelDetailRow{}
	for _, x := range rows {
		byProv[x.Provider] = x
	}
	kind, vid := viewerOf(r)
	out := []map[string]any{}
	for _, p := range hv.Providers {
		item := map[string]any{"provider": p.Provider, "sourceUrl": p.SourceURL, "supported": s.detailsSvc.Supports(p.Provider), "fetched": false}
		if x, ok := byProv[p.Provider]; ok && x.Version > 0 {
			item["fetched"] = true
			item["version"] = x.Version
			item["fetchedAt"] = x.FetchedAt
			item["changedAt"] = x.ChangedAt
			item["error"] = x.Error
			seen := s.db.SeenVersion(r.Context(), kind, vid, hotelID, p.Provider)
			item["unseenChange"] = seen > 0 && seen < x.Version
		} else if ok && x.Error != nil {
			item["error"] = x.Error
		}
		out = append(out, item)
	}
	writeJSON(w, 200, map[string]any{"hotel": hv, "providers": out, "changes": s.recentChanges(r, hotelID)})
}

func (s *Server) recentChanges(r *http.Request, hotelID string) []db.DetailChange {
	ch, _ := s.db.ListDetailChanges(r.Context(), hotelID, nil, 20)
	return ch
}

// viewerOf goruntuleyen kimligi: admin (id 0 veya e-posta hash) / portal user id.
func viewerOf(r *http.Request) (string, int) {
	if u := portalUserFrom(r.Context()); u != nil {
		return "portal", u.ID
	}
	id := auth.From(r.Context())
	if id != nil && id.IsAdmin {
		return "admin", int(hash32(id.ClientName) % 1000000)
	}
	return "anon", 0
}

func (s *Server) detailView(w http.ResponseWriter, r *http.Request, hotelID string) {
	prov := strings.TrimSpace(r.URL.Query().Get("provider"))
	if prov == "" {
		badRequest(w, "provider parametresi gerekli")
		return
	}
	kind, vid := viewerOf(r)
	mark := !boolParam(r.URL.Query().Get("peek"))
	v, err := s.detailsSvc.GetView(r.Context(), kind, vid, hotelID, prov, mark)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, v)
}

func (s *Server) detailRefresh(w http.ResponseWriter, r *http.Request, hotelID string) {
	prov := strings.TrimSpace(r.URL.Query().Get("provider"))
	if prov != "" {
		m, err := s.db.GetMapping(r.Context(), prov, hotelID)
		if err != nil {
			writeError(w, r, apperr.NotFoundf("Eslestirme bulunamadi: %s/%s", hotelID, prov))
			return
		}
		row, changed, err := s.detailsSvc.Refresh(r.Context(), m)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"provider": prov, "version": row.Version, "changed": changed, "fetchedAt": row.FetchedAt})
		return
	}
	rows, errs := s.detailsSvc.RefreshHotel(r.Context(), hotelID)
	writeJSON(w, 200, map[string]any{"refreshed": len(rows), "errors": errs})
}

// ---- Portal ----

func (s *Server) portalHotelDetails(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	hid := r.PathValue("id")
	if ok, _ := s.db.ClientOwnsHotel(r.Context(), id.ClientID, hid); !ok {
		writeError(w, r, apperr.NotFoundf("Otel bulunamadi: %s", hid))
		return
	}
	if r.URL.Query().Get("provider") != "" {
		// Plan/etkin site kontrolu
		pl := planFrom(r.Context())
		if pl != nil && len(pl.AllowedProviders) > 0 {
			ok := false
			for _, n := range pl.AllowedProviders {
				if n == r.URL.Query().Get("provider") {
					ok = true
				}
			}
			if !ok {
				writeError(w, r, apperr.New(apperr.Forbidden, "Bu site hesabiniz icin etkin degil"))
				return
			}
		}
		s.detailView(w, r, hid)
		return
	}
	s.detailOverview(w, r, hid)
}

func (s *Server) portalHotelDetailsRefresh(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	hid := r.PathValue("id")
	if ok, _ := s.db.ClientOwnsHotel(r.Context(), id.ClientID, hid); !ok {
		writeError(w, r, apperr.NotFoundf("Otel bulunamadi: %s", hid))
		return
	}
	s.detailRefresh(w, r, hid)
}

func (s *Server) portalNotifications(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	cid := id.ClientID
	list, err := s.db.ListNotifications(r.Context(), &cid, boolParam(r.URL.Query().Get("unread")), atoiDefault(r.URL.Query().Get("limit"), 50))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"unread": s.db.CountUnread(r.Context(), &cid), "items": list})
}

func (s *Server) portalNotificationsRead(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	cid := id.ClientID
	var body struct {
		IDs []int64 `json:"ids"`
	}
	_ = decodeBody(r, &body)
	if err := s.db.MarkNotificationsRead(r.Context(), &cid, body.IDs); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "unread": s.db.CountUnread(r.Context(), &cid)})
}

// ---- Admin ----

func (s *Server) adminHotelDetails(w http.ResponseWriter, r *http.Request) {
	hid := r.PathValue("id")
	if r.URL.Query().Get("provider") != "" {
		s.detailView(w, r, hid)
		return
	}
	s.detailOverview(w, r, hid)
}

func (s *Server) adminHotelDetailsRefresh(w http.ResponseWriter, r *http.Request) {
	s.detailRefresh(w, r, r.PathValue("id"))
}

// GET /api/admin/details/changes?limit=
func (s *Server) adminDetailChanges(w http.ResponseWriter, r *http.Request) {
	ch, err := s.db.ListDetailChanges(r.Context(), "", nil, atoiDefault(r.URL.Query().Get("limit"), 100))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": ch, "lastRun": s.detailsSvc.LastReport(), "supported": s.detailsSvc.SupportedProviders()})
}

// POST /api/admin/details/run — gunluk kontrolu simdi calistir (arka planda)
func (s *Server) adminDetailRunAll(w http.ResponseWriter, r *http.Request) {
	go s.detailsSvc.RunAll(contextWithoutCancel(r))
	writeJSON(w, 202, map[string]any{"message": "Statik detay kontrolu baslatildi.", "supported": s.detailsSvc.SupportedProviders()})
}

func (s *Server) adminNotifications(w http.ResponseWriter, r *http.Request) {
	list, err := s.db.ListNotifications(r.Context(), nil, boolParam(r.URL.Query().Get("unread")), atoiDefault(r.URL.Query().Get("limit"), 50))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"unread": s.db.CountUnread(r.Context(), nil), "items": list})
}

func (s *Server) adminNotificationsRead(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	_ = decodeBody(r, &body)
	if err := s.db.MarkNotificationsRead(r.Context(), nil, body.IDs); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "unread": s.db.CountUnread(r.Context(), nil)})
}

// ---- Admin: toplu otel ice aktarma ----

// POST /api/admin/hotels/import {hotels:[{externalId,name}], providers?:[], minScore?:0.6, apply?:false}
func (s *Server) adminImportHotels(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Hotels    []services.ImportHotel `json:"hotels"`
		Providers []string               `json:"providers"`
		MinScore  float64                `json:"minScore"`
		Apply     bool                   `json:"apply"`
	}
	if err := decodeBody(r, &body); err != nil || len(body.Hotels) == 0 {
		badRequest(w, "hotels dizisi zorunlu: [{externalId, name}]")
		return
	}
	if len(body.Hotels) > 200 {
		badRequest(w, "tek seferde en fazla 200 otel")
		return
	}
	if err := s.providerNamesValid(body.Providers); err != nil {
		writeError(w, r, err)
		return
	}
	rep := s.importer.Run(r.Context(), body.Hotels, body.Providers, body.MinScore, body.Apply, nil)
	writeJSON(w, 200, rep)
}

// GET /api/admin/hotels/import/providers — isimle arama destekleyen siteler
func (s *Server) adminImportProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"providers": s.importer.SearchProviders()})
}

func parseInt64s(s string) []int64 {
	out := []int64{}
	for _, p := range strings.Split(s, ",") {
		if n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}

var _ = json.Marshal
