package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/db"
)

// ---- Admin: musteri limitleri (paketten kopyalanir, ozellestirilebilir) ----

func (s *Server) clientWithPlan(ctx context.Context, id int) (*db.Client, *db.Plan, error) {
	c, err := s.db.GetClient(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	var p *db.Plan
	if c.PlanID != nil {
		p, _ = s.db.GetPlan(ctx, *c.PlanID)
	}
	return c, p, nil
}

// GET /api/admin/clients/{id}/limits
func (s *Server) adminClientLimits(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	c, p, err := s.clientWithPlan(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	daily := s.db.ClientDailyToday(r.Context(), id)
	writeJSON(w, 200, map[string]any{
		"limits": db.EffectiveLimits(c, p), "planLimits": db.LimitsFromPlan(p), "customized": c.LimitsCustomized, "plan": p,
		"usage":         map[string]any{"today": daily, "hotels": s.db.CountClientHotels(r.Context(), id), "users": s.db.CountPortalUsers(r.Context(), id)},
		"demoExpiresAt": c.DemoExpiresAt, "planAssignedAt": c.PlanAssignedAt,
	})
}

// PUT /api/admin/clients/{id}/limits {limits}
func (s *Server) adminSetClientLimits(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var in db.Limits
	if err := decodeBody(r, &in); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	for _, v := range []int{in.MaxSites, in.MaxHotels, in.MaxUsers, in.DailySearches, in.DailyScheduledRuns, in.DailyQuota, in.RatePerMinute, in.MaxConcurrency, in.MaxDevicesPC, in.MaxDevicesMobile} {
		if v < 0 {
			badRequest(w, "Limitler negatif olamaz (0 = sinirsiz)")
			return
		}
	}
	c, p, err := s.clientWithPlan(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	customized := in != db.LimitsFromPlan(p)
	if err := s.db.SetClientLimits(r.Context(), c.ID, in, customized); err != nil {
		writeError(w, r, err)
		return
	}
	InvalidateKeyCache()
	writeJSON(w, 200, map[string]any{"limits": in, "customized": customized})
}

// POST /api/admin/clients/{id}/limits/reset — paket degerlerine don
func (s *Server) adminResetClientLimits(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	if err := s.db.ResetClientLimits(r.Context(), id); err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	InvalidateKeyCache()
	c, p, _ := s.clientWithPlan(r.Context(), id)
	writeJSON(w, 200, map[string]any{"limits": db.EffectiveLimits(c, p), "customized": false})
}

// ---- Admin: giris loglari ----

// GET /api/admin/login-logs?clientId=&userId=&limit=
func (s *Server) adminLoginLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rows, err := s.db.ListLoginLogs(r.Context(), optInt(q.Get("clientId")), optInt(q.Get("userId")), atoiDefault(q.Get("limit"), 200))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, rows)
}

// ---- Admin: onay bekleyen islemler ----

// GET /api/admin/approvals?status=pending&limit=
func (s *Server) adminApprovals(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	if status == "all" {
		status = ""
	} else if status == "" {
		status = "pending"
	}
	list, err := s.db.ListApprovals(r.Context(), status, optInt(q.Get("clientId")), nil, atoiDefault(q.Get("limit"), 100))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"pending": s.db.CountPendingApprovals(r.Context()), "items": list})
}

// POST /api/admin/approvals/{id}/approve | reject {note}
func (s *Server) adminDecideApproval(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r)
		if !ok {
			badRequest(w, "Gecersiz id")
			return
		}
		var body struct {
			Note *string `json:"note"`
		}
		_ = decodeBody(r, &body)
		a, err := s.db.GetApproval(r.Context(), id)
		if err != nil {
			notFoundOr(w, r, err, "Talep bulunamadi")
			return
		}
		if a.Status != "pending" {
			writeError(w, r, apperr.Conflictf("Talep zaten karara baglanmis (%s)", a.Status))
			return
		}
		by := "admin"
		if ad := auth.From(r.Context()); ad != nil && ad.ClientName != "" {
			by = ad.ClientName
		}
		if approve {
			switch a.Type {
			case "device_remove", "device_replace":
				var p struct {
					RemoveDeviceID int `json:"removeDeviceId"`
				}
				_ = json.Unmarshal(a.Payload, &p)
				if p.RemoveDeviceID > 0 {
					if err := s.db.RemoveDevice(r.Context(), p.RemoveDeviceID, "approval"); err != nil && err != db.ErrNotFound {
						writeError(w, r, err)
						return
					}
				}
			}
			out, err := s.db.DecideApproval(r.Context(), id, "approved", body.Note, by)
			if err != nil {
				writeError(w, r, err)
				return
			}
			writeJSON(w, 200, out)
			return
		}
		out, err := s.db.DecideApproval(r.Context(), id, "rejected", body.Note, by)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, 200, out)
	}
}

// ---- Admin: kullanici cihazlari ----

// GET /api/admin/portal-users/{id}/devices
func (s *Server) adminUserDevices(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	devs, err := s.db.ListDevices(r.Context(), id, true)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, devs)
}

// DELETE /api/admin/devices/{id} — admin tarafinda sinirsiz
func (s *Server) adminRemoveDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	if err := s.db.RemoveDevice(r.Context(), id, "admin"); err != nil {
		notFoundOr(w, r, err, "Cihaz bulunamadi veya zaten kaldirilmis")
		return
	}
	writeJSON(w, 200, map[string]any{"message": "Cihaz kaldirildi.", "id": id})
}

// ---- Admin: site yonetimi ----

// GET /api/admin/sites
func (s *Server) adminSites(w http.ResponseWriter, r *http.Request) {
	stats := map[string]any{}
	if s.deps.Limiter != nil {
		for _, st := range s.deps.Limiter.Snapshot(s.reg.Names()) {
			b, _ := json.Marshal(st)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			if n, ok := m["provider"].(string); ok {
				stats[n] = m
			}
		}
	}
	var canary map[string]any
	if s.deps.Canary != nil {
		if rep := s.deps.Canary.Last(); rep != nil {
			b, _ := json.Marshal(rep)
			var m struct {
				Checks []map[string]any `json:"checks"`
			}
			_ = json.Unmarshal(b, &m)
			canary = map[string]any{}
			for _, c := range m.Checks {
				if n, ok := c["provider"].(string); ok {
					canary[n] = c
				}
			}
		}
	}
	out := []map[string]any{}
	for _, p := range s.reg.All() {
		name := p.Name()
		set := s.sites.Get(name)
		info := s.portalSvc.ProviderList([]string{name})
		label, domain := name, ""
		for _, pi := range info {
			if pi.Name == name {
				label, domain = pi.Label, pi.Domain
			}
		}
		_, hasDetails := p.(interface {
			GetDetails(ctx context.Context, m any) (any, error)
		})
		out = append(out, map[string]any{
			"name": name, "label": label, "domain": domain, "enabled": set.Enabled, "displayName": set.DisplayName, "sortOrder": set.SortOrder, "note": set.Note, "updatedAt": set.UpdatedAt,
			"capabilities": p.Capabilities(), "supportsDetails": hasDetails || s.detailsSvc.Supports(name), "health": stats[name], "canary": canary[name],
			"clientsUsing": s.db.ClientsUsingProvider(r.Context(), name),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i]["sortOrder"].(int), out[j]["sortOrder"].(int)
		if a != b {
			return a < b
		}
		return out[i]["name"].(string) < out[j]["name"].(string)
	})
	writeJSON(w, 200, out)
}

// PUT /api/admin/sites/{name} {enabled?, displayName?, sortOrder?, note?}
func (s *Server) adminUpdateSite(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := s.reg.Get(name); err != nil {
		writeError(w, r, apperr.NotFoundf("Site bulunamadi: %s", name))
		return
	}
	var body struct {
		Enabled     *bool   `json:"enabled"`
		DisplayName *string `json:"displayName"`
		SortOrder   *int    `json:"sortOrder"`
		Note        *string `json:"note"`
	}
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if err := s.db.UpsertSiteSetting(r.Context(), name, body.Enabled, body.DisplayName, body.SortOrder, body.Note); err != nil {
		writeError(w, r, err)
		return
	}
	s.sites.Reload(r.Context())
	InvalidateKeyCache()
	writeJSON(w, 200, s.sites.Get(name))
}

// ---- Admin: otel kunyesi ve musteri atamalari ----

// PUT /api/admin/hotels/{id}/meta {name, city, region, country, stars, chain, externalId, isActive, notes}
func (s *Server) adminUpdateHotelMeta(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.hotels.Get(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}
	var in db.HotelMetaInput
	if err := decodeBody(r, &in); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if in.Stars != nil && (*in.Stars < 0 || *in.Stars > 7) {
		badRequest(w, "stars 0-7 arasinda olmali")
		return
	}
	m, err := s.db.UpdateHotelMeta(r.Context(), id, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, m)
}

// GET /api/admin/hotels/{id}/clients — otele bagli musteriler
func (s *Server) adminHotelClients(w http.ResponseWriter, r *http.Request) {
	list, err := s.db.ClientsOfHotel(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

// POST /api/admin/clients/{id}/hotels {internalHotelId, label?} — mevcut sistem otelini musteriye bagla
func (s *Server) adminLinkClientHotel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var body struct {
		InternalHotelID string  `json:"internalHotelId"`
		Label           *string `json:"label"`
	}
	if err := decodeBody(r, &body); err != nil || strings.TrimSpace(body.InternalHotelID) == "" {
		badRequest(w, "internalHotelId zorunlu")
		return
	}
	c, p, err := s.clientWithPlan(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	if _, err := s.hotels.Get(r.Context(), body.InternalHotelID); err != nil {
		writeError(w, r, err)
		return
	}
	lim := db.EffectiveLimits(c, p)
	if owns, _ := s.db.ClientOwnsHotel(r.Context(), id, body.InternalHotelID); !owns && lim.MaxHotels > 0 && s.db.CountClientHotels(r.Context(), id) >= lim.MaxHotels {
		writeError(w, r, apperr.New(apperr.Forbidden, "Musterinin otel siniri dolu (%d). Limiti ozellestirin veya paketi degistirin.", lim.MaxHotels))
		return
	}
	if err := s.db.LinkClientHotel(r.Context(), id, body.InternalHotelID, body.Label); err != nil {
		writeError(w, r, err)
		return
	}
	hotels, _ := s.db.ListClientHotels(r.Context(), id)
	writeJSON(w, 201, hotels)
}

// DELETE /api/admin/clients/{id}/hotels/{hotelId}
func (s *Server) adminUnlinkClientHotel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	if err := s.db.UnlinkClientHotel(r.Context(), id, r.PathValue("hotelId")); err != nil {
		notFoundOr(w, r, err, "Baglanti bulunamadi")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// GET /api/admin/usage/daily?from=&to=&clientId= — musteri bazinda gunluk arama / planli / API / site cagrisi
func (s *Server) adminUsageDaily(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	to := parseDateOr(q.Get("to"), time.Now())
	from := parseDateOr(q.Get("from"), to.AddDate(0, 0, -30))
	rows, err := s.db.ClientDailyRange(r.Context(), from, to, optInt(q.Get("clientId")))
	if err != nil {
		writeError(w, r, err)
		return
	}
	tot := map[string]int{"searches": 0, "scheduledRuns": 0, "apiJobs": 0, "siteCalls": 0}
	for _, x := range rows {
		tot["searches"] += x.Searches
		tot["scheduledRuns"] += x.ScheduledRuns
		tot["apiJobs"] += x.APIJobs
		tot["siteCalls"] += x.SiteCalls
	}
	writeJSON(w, 200, map[string]any{"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"), "totals": tot, "rows": rows})
}
