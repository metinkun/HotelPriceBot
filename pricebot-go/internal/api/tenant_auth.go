package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/db"
)

// ---- API anahtari dogrulama (musteri uclari) ----

type cachedIdentity struct {
	id   *auth.Identity
	plan *PlanLimits
	exp  time.Time
}

var (
	idCacheMu sync.Mutex
	idCache   = map[string]cachedIdentity{}
)

func planLimitsOf(p *db.Plan) *PlanLimits {
	if p == nil {
		return &PlanLimits{AllowPackages: true}
	}
	return &PlanLimits{
		AllowedProviders: p.AllowedProviders, AllowFresh: p.AllowFresh, AllowPackages: p.AllowPackages,
		DailyQuota: p.DailyQuota, RatePerMinute: p.RatePerMinute, PlanCode: p.Code,
	}
}

// resolveKey anahtari cozer (60 sn bellek cache). Admin super-anahtar da kabul edilir.
func (s *Server) resolveKey(ctx context.Context, key string) (*auth.Identity, *PlanLimits, error) {
	if key == "" {
		return nil, nil, apperr.New(apperr.Unauthorized, "API anahtari gerekli (X-API-Key)")
	}
	if s.cfg.AdminAPIKey != "" && key == s.cfg.AdminAPIKey {
		return &auth.Identity{IsAdmin: true, PlanCode: "admin"}, &PlanLimits{AllowFresh: true, AllowPackages: true, PlanCode: "admin"}, nil
	}
	hash := auth.HashKey(key)
	idCacheMu.Lock()
	if c, ok := idCache[hash]; ok && time.Now().Before(c.exp) {
		idCacheMu.Unlock()
		return c.id, c.plan, nil
	}
	idCacheMu.Unlock()

	ki, err := s.db.ResolveAPIKey(ctx, hash)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrKeyInvalid), errors.Is(err, db.ErrKeyExpired), errors.Is(err, db.ErrClientOff):
			return nil, nil, apperr.New(apperr.Unauthorized, "%s", err.Error())
		default:
			return nil, nil, err
		}
	}
	id := &auth.Identity{ClientID: ki.Client.ID, ClientName: ki.Client.Name, APIKeyID: ki.Key.ID}
	if ki.Plan != nil {
		id.PlanCode = ki.Plan.Code
	}
	pl := planLimitsOf(ki.Plan)
	idCacheMu.Lock()
	idCache[hash] = cachedIdentity{id: id, plan: pl, exp: time.Now().Add(60 * time.Second)}
	idCacheMu.Unlock()
	go s.db.TouchAPIKey(context.Background(), ki.Key.ID)
	return id, pl, nil
}

// InvalidateKeyCache anahtar iptalinde cache'i bosaltir.
func InvalidateKeyCache() {
	idCacheMu.Lock()
	idCache = map[string]cachedIdentity{}
	idCacheMu.Unlock()
}

func apiKeyFrom(r *http.Request) string {
	if k := strings.TrimSpace(r.Header.Get("X-API-Key")); k != "" {
		return k
	}
	if a := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(a), "apikey ") {
		return strings.TrimSpace(a[7:])
	}
	return strings.TrimSpace(r.URL.Query().Get("api_key"))
}

// client musteri uclarini korur: anahtar -> kimlik + plan; dakikalik sinir; gunluk kota.
func (s *Server) client(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := apiKeyFrom(r)
		if key == "" && !s.cfg.RequireAPIKey {
			h(w, r) // gelistirme modu: anonim erisim
			return
		}
		id, pl, err := s.resolveKey(r.Context(), key)
		if err != nil {
			writeError(w, r, err)
			return
		}
		if !id.IsAdmin {
			if !s.usage.RateAllow(r.Context(), id.APIKeyID, pl.RatePerMinute) {
				w.Header().Set("Retry-After", "60")
				writeError(w, r, apperr.New(apperr.TooMany, "Dakikalik istek siniri asildi (%d/dk)", pl.RatePerMinute))
				return
			}
			if pl.DailyQuota > 0 {
				used := s.usage.UsedToday(r.Context(), id.ClientID)
				w.Header().Set("X-Quota-Limit", fmt.Sprint(pl.DailyQuota))
				w.Header().Set("X-Quota-Used", fmt.Sprint(used))
				if used >= pl.DailyQuota {
					writeError(w, r, apperr.New(apperr.TooMany, "Gunluk kredi kotasi doldu (%d/%d). Kota her gun 00:00 UTC'de yenilenir.", used, pl.DailyQuota))
					return
				}
			}
			s.usage.RecordRequest(id)
		}
		ctx := auth.WithIdentity(r.Context(), id)
		ctx = context.WithValue(ctx, planKey, pl)
		ctx = context.WithValue(ctx, clientIDKey, fmt.Sprint(id.ClientID))
		sw := &statusWriter{ResponseWriter: w}
		start := time.Now()
		h(sw, r.WithContext(ctx))
		s.logRequest(r, id, sw.status, time.Since(start))
	})
}

// logRequest musteri istegini request_logs tablosuna asenkron yazar.
func (s *Server) logRequest(r *http.Request, id *auth.Identity, status int, dur time.Duration) {
	if id == nil || id.IsAdmin {
		return
	}
	reqID := RequestIDFrom(r.Context())
	ip := clientIP(r)
	cid, kid := id.ClientID, id.APIKeyID
	l := db.RequestLog{RequestID: &reqID, ClientID: &cid, APIKeyID: &kid, Method: r.Method, Path: r.URL.Path, Status: status, DurationMs: int(dur.Milliseconds()), IP: &ip}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.db.InsertRequestLog(ctx, l)
	}()
}

// providersForPlan istenen provider listesini plana gore daraltir; izinsiz istek varsa 403.
func (s *Server) providersForPlan(r *http.Request, requested []string) ([]string, error) {
	pl := planFrom(r.Context())
	if pl == nil || len(pl.AllowedProviders) == 0 {
		return requested, nil
	}
	allowed := map[string]bool{}
	for _, a := range pl.AllowedProviders {
		allowed[a] = true
	}
	for _, n := range requested {
		if !allowed[n] {
			return nil, apperr.New(apperr.Forbidden, "Planiniz '%s' saglayicisini kapsamiyor. Izinli: %s", n, strings.Join(pl.AllowedProviders, ", "))
		}
	}
	if len(requested) == 0 {
		return append([]string{}, pl.AllowedProviders...), nil
	}
	return requested, nil
}

func (s *Server) freshAllowed(r *http.Request, want bool) bool {
	if !want {
		return false
	}
	pl := planFrom(r.Context())
	return pl == nil || pl.AllowFresh
}

func (s *Server) packagesAllowed(r *http.Request) error {
	pl := planFrom(r.Context())
	if pl != nil && !pl.AllowPackages {
		return apperr.New(apperr.Forbidden, "Planiniz paket (otel+ucak+transfer) sorgusunu kapsamiyor")
	}
	return nil
}

// ---- Admin oturumu (JWT) ----

func (s *Server) jwtSecret() []byte {
	if s.cfg.JWTSecret != "" {
		return []byte(s.cfg.JWTSecret)
	}
	return []byte("pricebot-" + s.cfg.AdminAPIKey)
}

// admin: Bearer JWT (panel) veya eski x-api-key=ADMIN_API_KEY.
func (s *Server) admin(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
			claims, err := auth.ParseToken(s.jwtSecret(), a[7:])
			if err != nil || claims.Role != "admin" {
				writeJSON(w, 401, map[string]any{"error": "Gecersiz veya suresi dolmus oturum"})
				return
			}
			ctx := auth.WithIdentity(r.Context(), &auth.Identity{IsAdmin: true, ClientName: claims.Email, PlanCode: "admin"})
			h(w, r.WithContext(ctx))
			return
		}
		if s.cfg.AdminAPIKey == "" {
			writeJSON(w, 500, map[string]any{"error": "ADMIN_API_KEY ayarlanmamis."})
			return
		}
		if apiKeyFrom(r) != s.cfg.AdminAPIKey {
			writeJSON(w, 401, map[string]any{"error": "Gecersiz API anahtari"})
			return
		}
		ctx := auth.WithIdentity(r.Context(), &auth.Identity{IsAdmin: true, PlanCode: "admin"})
		h(w, r.WithContext(ctx))
	})
}

// POST /api/admin/auth/login {email, password} -> {token, expiresAt}
func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeBody(r, &body); err != nil || body.Email == "" || body.Password == "" {
		badRequest(w, "email ve password zorunlu")
		return
	}
	// Kaba kuvvete karsi: IP bazli dakikalik sinir
	if !s.usage.RateAllow(r.Context(), -1*int(hash32(clientIP(r))), 10) {
		writeError(w, r, apperr.New(apperr.TooMany, "Cok fazla giris denemesi; 1 dk bekleyin"))
		return
	}
	a, err := s.db.GetAdminByEmail(r.Context(), body.Email)
	if err != nil || !a.IsActive || bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(body.Password)) != nil {
		writeJSON(w, 401, map[string]any{"error": "E-posta veya sifre hatali"})
		return
	}
	exp := time.Now().Add(12 * time.Hour)
	tok, err := auth.SignToken(s.jwtSecret(), auth.Claims{Sub: fmt.Sprint(a.ID), Email: a.Email, Role: "admin", Exp: exp.Unix(), Iat: time.Now().Unix()})
	if err != nil {
		writeError(w, r, err)
		return
	}
	go s.db.TouchAdminLogin(context.Background(), a.ID)
	writeJSON(w, 200, map[string]any{"token": tok, "expiresAt": exp, "admin": map[string]any{"id": a.ID, "email": a.Email, "name": a.Name}})
}

// GET /api/admin/auth/me
func (s *Server) adminMe(w http.ResponseWriter, r *http.Request) {
	id := auth.From(r.Context())
	writeJSON(w, 200, map[string]any{"role": "admin", "email": id.ClientName})
}

func hash32(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// BootstrapAdmin ilk admin'i ADMIN_EMAIL/ADMIN_PASSWORD ile olusturur (admin yoksa).
func BootstrapAdmin(ctx context.Context, d *db.DB, email, password string) {
	if email == "" || password == "" {
		return
	}
	n, err := d.CountAdmins(ctx)
	if err != nil || n > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	if _, err := d.CreateAdmin(ctx, email, string(hash), nil); err != nil {
		slog.Warn("bootstrap admin olusturulamadi", "err", err)
		return
	}
	slog.Info("ilk admin olusturuldu", "email", email)
}
