package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/db"
)

func notFoundOr(w http.ResponseWriter, r *http.Request, err error, msg string) {
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, r, apperr.NotFoundf("%s", msg))
		return
	}
	writeError(w, r, err)
}

// ---- Planlar ----

func (s *Server) listPlans(w http.ResponseWriter, r *http.Request) {
	ps, err := s.db.ListPlans(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, ps)
}

func (s *Server) validatePlan(in *db.PlanInput) error {
	if strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.Name) == "" {
		return apperr.BadRequestf("code ve name zorunlu")
	}
	if in.DailyQuota < 0 || in.MaxSites < 0 || in.MaxHotels < 0 || in.MaxUsers < 0 || in.DailySearches < 0 || in.DailyScheduledRuns < 0 {
		return apperr.BadRequestf("Limitler negatif olamaz (0 = sinirsiz)")
	}
	if k := strings.ToLower(strings.TrimSpace(in.Kind)); k != "" && k != "acente" && k != "otel" && k != "both" {
		return apperr.BadRequestf("kind acente | otel | both olmali")
	}
	if in.MaxSites > 0 && len(in.AllowedProviders) > in.MaxSites {
		return apperr.BadRequestf("Izinli site listesi (%d) site sayisi limitini (%d) asiyor", len(in.AllowedProviders), in.MaxSites)
	}
	return s.providerNamesValid(in.AllowedProviders)
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	var in db.PlanInput
	if err := decodeBody(r, &in); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	in.IsActive = true
	if err := s.validatePlan(&in); err != nil {
		writeError(w, r, err)
		return
	}
	p, err := s.db.CreatePlan(r.Context(), in)
	if err != nil {
		if db.IsUniqueViolation(err) {
			writeError(w, r, apperr.Conflictf("Bu plan kodu zaten var"))
			return
		}
		writeError(w, r, err)
		return
	}
	InvalidateKeyCache()
	writeJSON(w, 201, p)
}

func (s *Server) updatePlan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var in db.PlanInput
	if err := decodeBody(r, &in); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if err := s.validatePlan(&in); err != nil {
		writeError(w, r, err)
		return
	}
	p, err := s.db.UpdatePlan(r.Context(), id, in)
	if err != nil {
		notFoundOr(w, r, err, "Plan bulunamadi")
		return
	}
	InvalidateKeyCache()
	writeJSON(w, 200, p)
}

// ---- Musteriler ----

func (s *Server) listClients(w http.ResponseWriter, r *http.Request) {
	cs, err := s.db.ListClients(r.Context(), r.URL.Query().Get("search"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, cs)
}

func (s *Server) createClient(w http.ResponseWriter, r *http.Request) {
	var in db.ClientInput
	if err := decodeBody(r, &in); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		badRequest(w, "name zorunlu")
		return
	}
	if in.PlanID != nil {
		if _, err := s.db.GetPlan(r.Context(), *in.PlanID); err != nil {
			writeError(w, r, apperr.BadRequestf("planId gecersiz"))
			return
		}
	}
	c, err := s.db.CreateClient(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, c)
}

func (s *Server) getClient(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	c, err := s.db.GetClient(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	keys, _ := s.db.ListAPIKeys(r.Context(), id)
	used := s.usage.UsedToday(r.Context(), id)
	writeJSON(w, 200, map[string]any{"client": c, "keys": keys, "usedToday": used})
}

func (s *Server) updateClient(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	var in db.ClientInput
	if err := decodeBody(r, &in); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if in.Kind != nil {
		k := strings.ToLower(strings.TrimSpace(*in.Kind))
		if k != "acente" && k != "otel" {
			badRequest(w, "kind acente | otel olmali")
			return
		}
		in.Kind = &k
	}
	if in.OwnHotelID != nil && strings.TrimSpace(*in.OwnHotelID) != "" {
		// Kendi oteli musterinin otel listesinde olmali (yoksa baglanir)
		hid := strings.TrimSpace(*in.OwnHotelID)
		if _, err := s.hotels.Get(r.Context(), hid); err != nil {
			writeError(w, r, err)
			return
		}
		if owns, _ := s.db.ClientOwnsHotel(r.Context(), id, hid); !owns {
			_ = s.db.LinkClientHotel(r.Context(), id, hid, nil)
		}
	}
	if in.PlanID != nil {
		if p, err := s.db.GetPlan(r.Context(), *in.PlanID); err == nil {
			cur, _ := s.db.GetClient(r.Context(), id)
			if cur != nil && p.Kind != "both" && p.Kind != cur.Kind && (in.Kind == nil || *in.Kind != p.Kind) {
				writeError(w, r, apperr.BadRequestf("'%s' paketi yalnizca %s turu musterilere atanabilir", p.Name, strings.ToUpper(p.Kind)))
				return
			}
		}
	}
	c, err := s.db.UpdateClient(r.Context(), id, in)
	if err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	InvalidateKeyCache()
	writeJSON(w, 200, c)
}


// ---- API anahtarlari ----

func generateAPIKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "pb_live_" + hex.EncodeToString(b)
}

// POST /api/admin/clients/{id}/keys {label?, expiresInDays?} -> duz metin anahtar SADECE bu yanitta.
func (s *Server) createKey(w http.ResponseWriter, r *http.Request) {
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
	if !db.EffectiveLimits(c, p).AllowAPI {
		writeError(w, r, apperr.New(apperr.Forbidden, "Bu musterinin paketi API erisimi icermiyor (limitlerden 'API kullanicisi' acilabilir)"))
		return
	}
	var in struct {
		Label         *string `json:"label"`
		ExpiresInDays int     `json:"expiresInDays"`
	}
	_ = decodeBody(r, &in)
	var exp *time.Time
	if in.ExpiresInDays > 0 {
		t := time.Now().AddDate(0, 0, in.ExpiresInDays)
		exp = &t
	}
	plain := generateAPIKey()
	k, err := s.db.CreateAPIKey(r.Context(), id, in.Label, auth.HashKey(plain), plain[:12], exp)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{
		"key":     plain,
		"apiKey":  k,
		"warning": "Anahtar yalnizca bu yanitta gosterilir; saklayin.",
	})
}

func (s *Server) listKeys(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	keys, err := s.db.ListAPIKeys(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, keys)
}

func (s *Server) revokeKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	if err := s.db.RevokeAPIKey(r.Context(), id); err != nil {
		notFoundOr(w, r, err, "Anahtar bulunamadi veya zaten iptal")
		return
	}
	InvalidateKeyCache()
	writeJSON(w, 200, map[string]any{"message": "Anahtar iptal edildi.", "id": id})
}

// ---- Kullanim / loglar ----

func parseDateOr(s string, def time.Time) time.Time {
	if t, err := time.Parse("2006-01-02", strings.TrimSpace(s)); err == nil {
		return t
	}
	return def
}

func optInt(s string) *int {
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

// GET /api/admin/usage?from=&to=&clientId=
func (s *Server) usageReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	to := parseDateOr(q.Get("to"), time.Now())
	from := parseDateOr(q.Get("from"), to.AddDate(0, 0, -30))
	rows, err := s.db.UsageSummary(r.Context(), from, to, optInt(q.Get("clientId")))
	if err != nil {
		writeError(w, r, err)
		return
	}
	totals := map[string]int{"credits": 0, "requests": 0, "cacheHits": 0}
	for _, x := range rows {
		totals["credits"] += x.Credits
		totals["requests"] += x.Requests
		totals["cacheHits"] += x.CacheHits
	}
	writeJSON(w, 200, map[string]any{"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"), "totals": totals, "rows": rows})
}

// GET /api/admin/logs?clientId=&limit=
func (s *Server) requestLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rows, err := s.db.ListRequestLogs(r.Context(), optInt(q.Get("clientId")), atoiDefault(q.Get("limit"), 100))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, rows)
}

// ---- Adminler ----

func (s *Server) listAdmins(w http.ResponseWriter, r *http.Request) {
	as, err := s.db.ListAdmins(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, 200, as)
}

func (s *Server) createAdmin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string  `json:"email"`
		Password string  `json:"password"`
		Name     *string `json:"name"`
	}
	if err := decodeBody(r, &in); err != nil || !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
		badRequest(w, "gecerli email ve en az 8 karakter password zorunlu")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, r, err)
		return
	}
	a, err := s.db.CreateAdmin(r.Context(), in.Email, string(hash), in.Name)
	if err != nil {
		if db.IsUniqueViolation(err) {
			writeError(w, r, apperr.Conflictf("Bu e-posta zaten kayitli"))
			return
		}
		writeError(w, r, err)
		return
	}
	writeJSON(w, 201, a)
}

// ---- Musteri self-servis ----

// GET /api/v1/me -> kimlik, plan, bugunku kullanim
func (s *Server) clientMe(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	pl := planFrom(r.Context())
	if id == nil {
		writeJSON(w, 200, map[string]any{"anonymous": true, "providers": s.reg.Names()})
		return
	}
	out := map[string]any{"clientId": id.ClientID, "clientName": id.ClientName, "plan": id.PlanCode, "admin": id.IsAdmin}
	if pl != nil {
		providers := pl.AllowedProviders
		if len(providers) == 0 {
			providers = s.reg.Names()
		}
		out["providers"] = providers
		out["allowPackages"] = pl.AllowPackages
		out["allowFresh"] = pl.AllowFresh
		out["dailyQuota"] = pl.DailyQuota
		out["ratePerMinute"] = pl.RatePerMinute
	}
	if !id.IsAdmin {
		out["usedToday"] = s.usage.UsedToday(r.Context(), id.ClientID)
	}
	writeJSON(w, 200, out)
}

// StartLogRetention eski request_logs kayitlarini gunde bir temizler.
func StartLogRetention(ctx context.Context, d *db.DB, days int) {
	if days <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			c, cancel := context.WithTimeout(ctx, 30*time.Second)
			_, _ = d.PurgeRequestLogs(c, days)
			cancel()
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}
