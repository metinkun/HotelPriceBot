package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/db"
	"pricebot/internal/portal"
)

// ---- Portal oturumu (musteri tipi 2) ----

const portalUserKey ctxKey = "portalUser"

func portalUserFrom(ctx context.Context) *db.PortalUser {
	if v, ok := ctx.Value(portalUserKey).(*db.PortalUser); ok {
		return v
	}
	return nil
}

// portal: Bearer JWT (role=portal) -> kullanici + musteri + plan + etkin siteler.
func (s *Server) portal(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := r.Header.Get("Authorization")
		if !strings.HasPrefix(a, "Bearer ") {
			writeJSON(w, 401, map[string]any{"error": "Oturum gerekli"})
			return
		}
		claims, err := auth.ParseToken(s.jwtSecret(), a[7:])
		if err != nil || claims.Role != "portal" {
			writeJSON(w, 401, map[string]any{"error": "Gecersiz veya suresi dolmus oturum"})
			return
		}
		uid, _ := strconv.Atoi(claims.Sub)
		u, err := s.db.GetPortalUser(r.Context(), uid)
		if err != nil || !u.IsActive {
			writeJSON(w, 401, map[string]any{"error": "Kullanici bulunamadi veya pasif"})
			return
		}
		c, err := s.db.GetClient(r.Context(), u.ClientID)
		if err != nil || !c.IsActive {
			writeJSON(w, 403, map[string]any{"error": "Hesap pasif"})
			return
		}
		if demoExpired(c) {
			writeJSON(w, 403, map[string]any{"error": "Deneme sureniz doldu; devam etmek icin yetkilinizle iletisime gecin", "code": "demo_expired"})
			return
		}
		var dev *db.Device
		if claims.Dev > 0 {
			d, err := s.db.GetDevice(r.Context(), claims.Dev)
			if err != nil || d.RemovedAt != nil || d.UserID != u.ID {
				writeJSON(w, 401, map[string]any{"error": "Bu cihazin kaydi kaldirilmis; lutfen yeniden giris yapin", "code": "device_removed"})
				return
			}
			dev = d
		} else if c.RequireDeviceAuth {
			// Cihaz dogrulamasi acikken cihazsiz (eski) oturum kabul edilmez
			writeJSON(w, 401, map[string]any{"error": "Oturumunuz cihaz dogrulamasi gerektiriyor; lutfen yeniden giris yapin", "code": "device_required"})
			return
		}
		id := &auth.Identity{ClientID: c.ID, ClientName: c.Name, PlanCode: "portal"}
		var plan *db.Plan
		if c.PlanID != nil {
			if pl, err := s.db.GetPlan(r.Context(), *c.PlanID); err == nil && pl.IsActive {
				plan = pl
				id.PlanCode = pl.Code
			}
		}
		pl := limitsOf(c, plan)
		eff, err := s.portalSvc.EffectiveProviders(r.Context(), c.ID, pl.AllowedProviders)
		if err != nil {
			writeError(w, r, err)
			return
		}
		if pl.MaxSites > 0 && len(eff) > pl.MaxSites {
			eff = eff[:pl.MaxSites]
		}
		pl.AllowedProviders = eff // portalda etkin siteler = plan ∩ musteri ayari ∩ acik siteler
		if pl.DailySearches > 0 {
			w.Header().Set("X-Quota-Limit", fmt.Sprint(pl.DailySearches))
			w.Header().Set("X-Quota-Used", fmt.Sprint(s.db.ClientDailyToday(r.Context(), c.ID).Searches))
		}
		ctx := auth.WithIdentity(r.Context(), id)
		ctx = context.WithValue(ctx, planKey, pl)
		ctx = context.WithValue(ctx, clientIDKey, fmt.Sprint(c.ID))
		ctx = context.WithValue(ctx, portalUserKey, u)
		if dev != nil {
			ctx = context.WithValue(ctx, portalDeviceKey, dev)
		}
		h(w, r.WithContext(ctx))
	})
}

// GET /api/portal/me
func (s *Server) portalMe(w http.ResponseWriter, r *http.Request) {
	u := portalUserFrom(r.Context())
	id := auth.From(r.Context())
	pl := planFrom(r.Context())
	c, _ := s.db.GetClient(r.Context(), id.ClientID)
	daily := s.db.ClientDailyToday(r.Context(), id.ClientID)
	var planName string
	if c != nil && c.PlanID != nil {
		if p, err := s.db.GetPlan(r.Context(), *c.PlanID); err == nil {
			planName = p.Name
		}
	}
	dev := portalDeviceFrom(r.Context())
	out := map[string]any{
		"user":      map[string]any{"id": u.ID, "email": u.Email, "name": u.Name},
		"client":    c,
		"providers": s.portalSvc.ProviderList(pl.AllowedProviders),
		// dailyQuota = gunluk ARAMA sayisi (musteriye gosterilen birim); site cagrisi sadece admin panelde
		"plan":      map[string]any{"code": id.PlanCode, "name": planName, "dailyQuota": pl.DailySearches, "allowPackages": pl.AllowPackages, "allowFresh": pl.AllowFresh, "kind": pl.Kind},
		"limits":    pl.Limits,
		"usage":     map[string]any{"searches": daily.Searches, "scheduledRuns": daily.ScheduledRuns, "hotels": s.db.CountClientHotels(r.Context(), id.ClientID), "users": s.db.CountPortalUsers(r.Context(), id.ClientID)},
		"usedToday": daily.Searches,
		"device":    dev,
	}
	writeJSON(w, 200, out)
}

// ---- Oteller ----

// GET /api/portal/hotels
func (s *Server) portalHotels(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	links, err := s.db.ListClientHotels(r.Context(), id.ClientID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	pl := planFrom(r.Context())
	eff := map[string]bool{}
	for _, n := range pl.AllowedProviders {
		eff[n] = true
	}
	out := []map[string]any{}
	for _, l := range links {
		item := map[string]any{"internalHotelId": l.InternalHotelID, "label": l.Label, "createdAt": l.CreatedAt, "hotelName": nil, "providers": []any{}}
		if hv, err := s.hotels.Get(r.Context(), l.InternalHotelID); err == nil {
			item["hotelName"] = hv.HotelName
			provs := []map[string]any{}
			for _, p := range hv.Providers {
				provs = append(provs, map[string]any{"provider": p.Provider, "sourceUrl": p.SourceURL, "enabled": eff[p.Provider], "isActive": p.IsActive})
			}
			item["providers"] = provs
		}
		out = append(out, item)
	}
	writeJSON(w, 200, out)
}


// PUT /api/portal/hotels/{id} {urls?, hotelName?}  (ek site URL'i / ad)
func (s *Server) portalUpdateHotel(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	hid := r.PathValue("id")
	if ok, _ := s.db.ClientOwnsHotel(r.Context(), id.ClientID, hid); !ok {
		writeError(w, r, apperr.NotFoundf("Otel bulunamadi: %s", hid))
		return
	}
	var body struct {
		URLs      map[string]string `json:"urls"`
		HotelName *string           `json:"hotelName"`
	}
	if err := decodeBody(r, &body); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	pl := planFrom(r.Context())
	eff := map[string]bool{}
	for _, n := range pl.AllowedProviders {
		eff[n] = true
	}
	for k := range body.URLs {
		if !eff[k] {
			writeError(w, r, apperr.New(apperr.Forbidden, "'%s' sitesi bu hesap icin etkin degil", k))
			return
		}
	}
	res, err := s.hotels.Update(r.Context(), hid, body.URLs, body.HotelName)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, res)
}


// ---- Aramalar ----

func (s *Server) parseSearchRequest(r *http.Request) (portal.SearchRequest, error) {
	var req portal.SearchRequest
	if err := decodeBody(r, &req); err != nil {
		return req, apperr.BadRequestf("Gecersiz JSON govde")
	}
	if req.CheckIn != "" || req.CheckOut != "" {
		ci, co, err := validateStay(req.CheckIn, req.CheckOut, max(req.Adults, 1), req.Children, req.ChildAges)
		if err != nil {
			return req, err
		}
		req.CheckIn, req.CheckOut = ci, co
	} else {
		if req.DaysAhead <= 0 || req.DaysAhead > 400 {
			return req, apperr.BadRequestf("daysAhead 1-400 arasinda olmali (veya checkIn/checkOut verin)")
		}
		if req.Nights <= 0 || req.Nights > 30 {
			return req, apperr.BadRequestf("nights 1-30 arasinda olmali")
		}
	}
	if req.Kind == "packages" {
		if !reIATA.MatchString(strings.TrimSpace(req.AirportCode)) {
			return req, apperr.BadRequestf("Paket aramasi icin airportCode (IATA) gerekli")
		}
		if pl := planFrom(r.Context()); pl != nil && !pl.AllowPackages {
			return req, apperr.New(apperr.Forbidden, "Paketiniz otel + ucak paket sorgusunu kapsamiyor")
		}
	}
	if err := s.providerNamesValid(req.Providers); err != nil {
		return req, err
	}
	return req, nil
}

// POST /api/portal/searches
func (s *Server) portalRunSearch(w http.ResponseWriter, r *http.Request) {
	req, err := s.parseSearchRequest(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := auth.From(r.Context())
	u := portalUserFrom(r.Context())
	pl := planFrom(r.Context())
	if pl.DailySearches > 0 && s.db.ClientDailyToday(r.Context(), id.ClientID).Searches >= pl.DailySearches {
		writeError(w, r, apperr.New(apperr.TooMany, "Gunluk arama hakkiniz doldu (%d). Hak her gun 00:00'da yenilenir.", pl.DailySearches))
		return
	}
	uid := u.ID
	res, err := s.portalSvc.RunSearch(r.Context(), id, &uid, nil, req, pl.AllowedProviders)
	if err != nil {
		writeError(w, r, err)
		return
	}
	_ = s.db.IncClientDaily(r.Context(), id.ClientID, "searches", 1)
	writeJSON(w, 202, res)
}

// GET /api/portal/searches?limit=&scheduleId=
func (s *Server) portalSearches(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	q := r.URL.Query()
	list, err := s.db.ListSearches(r.Context(), id.ClientID, optInt(q.Get("scheduleId")), atoiDefault(q.Get("limit"), 50))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

// GET /api/portal/searches/{id}
func (s *Server) portalSearch(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	sid, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "Gecersiz id")
		return
	}
	sr, err := s.db.GetSearch(r.Context(), id.ClientID, sid)
	if err != nil {
		notFoundOr(w, r, err, "Arama bulunamadi")
		return
	}
	if sr.Status == "queued" || sr.Status == "running" {
		w.Header().Set("Retry-After", "2")
	}
	writeJSON(w, 200, sr)
}

// ---- Planli aramalar ----

type scheduleBody struct {
	Name     string               `json:"name"`
	Kind     string               `json:"kind"`
	Request  portal.SearchRequest `json:"request"`
	Weekdays []int16              `json:"weekdays"`
	RunTime  string               `json:"runTime"`
	Timezone string               `json:"timezone"`
	IsActive *bool                `json:"isActive"`
	StartsOn string               `json:"startsOn"` // YYYY-MM-DD (bos = hemen)
	EndsOn   string               `json:"endsOn"`   // YYYY-MM-DD (bos = suresiz)
}

func (s *Server) scheduleInput(r *http.Request, b scheduleBody) (db.ScheduleInput, error) {
	if strings.TrimSpace(b.Name) == "" {
		return db.ScheduleInput{}, apperr.BadRequestf("name zorunlu")
	}
	if b.Kind == "" {
		b.Kind = "prices"
	}
	if b.Timezone == "" {
		b.Timezone = "Europe/Istanbul"
	}
	if b.RunTime == "" {
		b.RunTime = "09:00"
	}
	req := b.Request
	req.Kind = b.Kind
	if len(req.HotelIDs) == 0 {
		return db.ScheduleInput{}, apperr.BadRequestf("En az bir otel secilmeli")
	}
	if req.DaysAhead <= 0 {
		req.DaysAhead = 30
	}
	if req.Nights <= 0 {
		req.Nights = 3
	}
	if req.Adults <= 0 {
		req.Adults = 2
	}
	if b.Kind == "packages" && !reIATA.MatchString(strings.TrimSpace(req.AirportCode)) {
		return db.ScheduleInput{}, apperr.BadRequestf("Paket plani icin airportCode (IATA) gerekli")
	}
	if err := s.providerNamesValid(req.Providers); err != nil {
		return db.ScheduleInput{}, err
	}
	id := auth.From(r.Context())
	for _, h := range req.HotelIDs {
		if ok, _ := s.db.ClientOwnsHotel(r.Context(), id.ClientID, h); !ok {
			return db.ScheduleInput{}, apperr.New(apperr.Forbidden, "Otel bu hesaba ait degil: %s", h)
		}
	}
	var startsOn, endsOn *string
	for _, x := range []struct {
		v   string
		dst **string
		lbl string
	}{{b.StartsOn, &startsOn, "startsOn"}, {b.EndsOn, &endsOn, "endsOn"}} {
		v := strings.TrimSpace(x.v)
		if v == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return db.ScheduleInput{}, apperr.BadRequestf("%s YYYY-MM-DD bicimnde olmali", x.lbl)
		}
		vv := v
		*x.dst = &vv
	}
	if startsOn != nil && endsOn != nil && *endsOn < *startsOn {
		return db.ScheduleInput{}, apperr.BadRequestf("Bitis tarihi baslangictan once olamaz")
	}
	active := b.IsActive == nil || *b.IsActive
	var next *time.Time
	if active {
		n, err := nextRunFrom(b.Weekdays, b.RunTime, b.Timezone, startsOn)
		if err != nil {
			return db.ScheduleInput{}, err
		}
		next = &n
	}
	if b.Weekdays == nil {
		b.Weekdays = []int16{}
	}
	return db.ScheduleInput{Name: b.Name, Kind: b.Kind, Request: req, Weekdays: b.Weekdays, RunTime: b.RunTime, Timezone: b.Timezone, IsActive: active, NextRun: next, StartsOn: startsOn, EndsOn: endsOn}, nil
}

func (s *Server) portalSchedules(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	list, err := s.db.ListSchedules(r.Context(), id.ClientID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) portalCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var b scheduleBody
	if err := decodeBody(r, &b); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	in, err := s.scheduleInput(r, b)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := auth.From(r.Context())
	uid := portalUserFrom(r.Context()).ID
	sc, err := s.db.CreateSchedule(r.Context(), id.ClientID, &uid, in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, sc)
}

func (s *Server) portalUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	sid, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var b scheduleBody
	if err := decodeBody(r, &b); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	in, err := s.scheduleInput(r, b)
	if err != nil {
		writeError(w, r, err)
		return
	}
	id := auth.From(r.Context())
	sc, err := s.db.UpdateSchedule(r.Context(), id.ClientID, sid, in)
	if err != nil {
		notFoundOr(w, r, err, "Plan bulunamadi")
		return
	}
	writeJSON(w, 200, sc)
}


// POST /api/portal/schedules/{id}/run — plani simdi calistir
func (s *Server) portalRunSchedule(w http.ResponseWriter, r *http.Request) {
	sid, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	id := auth.From(r.Context())
	sc, err := s.db.GetSchedule(r.Context(), id.ClientID, sid)
	if err != nil {
		notFoundOr(w, r, err, "Plan bulunamadi")
		return
	}
	var req portal.SearchRequest
	if err := jsonUnmarshal(sc.Request, &req); err != nil {
		writeError(w, r, err)
		return
	}
	req.Kind = sc.Kind
	pl := planFrom(r.Context())
	uid := portalUserFrom(r.Context()).ID
	res, err := s.portalSvc.RunSearch(r.Context(), id, &uid, &sid, req, pl.AllowedProviders)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 202, res)
}

func notFoundErr(err error) bool { return errors.Is(err, db.ErrNotFound) }
