// Package api HTTP katmani: router, middleware ve handler'lar.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/cache"
	"pricebot/internal/canary"
	"pricebot/internal/config"
	"pricebot/internal/db"
	"pricebot/internal/jobs"
	"pricebot/internal/limiter"
	"pricebot/internal/providers"
	"pricebot/internal/services"
)

// Deps sunucunun bagimliliklari.
type Deps struct {
	Cache   *cache.Cache
	Limiter *limiter.Limiter
	Fetcher *services.Fetcher
	Canary  *canary.Canary
	Jobs    *jobs.Manager
	Usage   *services.Usage
}

type Server struct {
	cfg      *config.Config
	db       *db.DB
	reg      *providers.Registry
	deps     Deps
	hotels   *services.HotelService
	prices   *services.PriceService
	packages *services.PackageService
	usage    *services.Usage
	started  time.Time
}

func New(cfg *config.Config, d *db.DB, reg *providers.Registry, deps Deps) *Server {
	return &Server{
		cfg: cfg, db: d, reg: reg, deps: deps,
		hotels:   services.NewHotelService(d, reg),
		prices:   services.NewPriceService(d, reg, deps.Fetcher),
		packages: services.NewPackageService(d, reg, deps.Fetcher),
		usage:    deps.Usage,
		started:  time.Now(),
	}
}

// Handler tum route'lari bagli http.Handler doner.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)

	if s.cfg.SwaggerOn {
		mux.HandleFunc("GET /docs", s.swaggerUI)
		mux.HandleFunc("GET /docs/", s.swaggerUI)
		mux.HandleFunc("GET /docs/openapi.json", s.openAPISpec)
	}

	// Musteri uclari (API anahtari; REQUIRE_API_KEY=false ise anonim de calisir)
	mux.Handle("GET /api/prices", s.client(s.getPrice))
	mux.Handle("GET /api/prices/all", s.client(s.getAllPrices))
	mux.Handle("POST /api/prices/bulk", s.client(s.getBulkPrices))
	mux.Handle("GET /api/packages", s.client(s.getPackages))
	mux.Handle("POST /api/v1/jobs", s.client(s.createJob))
	mux.Handle("GET /api/v1/jobs/{id}", s.client(s.getJob))
	mux.Handle("GET /api/v1/me", s.client(s.clientMe))

	// Admin oturumu
	mux.HandleFunc("POST /api/admin/auth/login", s.adminLogin)
	mux.Handle("GET /api/admin/auth/me", s.admin(s.adminMe))
	mux.Handle("GET /api/admin/admins", s.admin(s.listAdmins))
	mux.Handle("POST /api/admin/admins", s.admin(s.createAdmin))

	// Admin - URL bazli otel
	mux.Handle("POST /api/admin/hotels", s.admin(s.createHotel))
	mux.Handle("GET /api/admin/hotels", s.admin(s.listHotels))
	mux.Handle("GET /api/admin/hotels/{id}", s.admin(s.getHotel))
	mux.Handle("PUT /api/admin/hotels/{id}", s.admin(s.updateHotel))
	mux.Handle("DELETE /api/admin/hotels/{id}", s.admin(s.deleteHotel))

	// Admin - mapping
	mux.Handle("GET /api/admin/mappings", s.admin(s.listMappings))
	mux.Handle("POST /api/admin/mappings", s.admin(s.createMapping))
	mux.Handle("POST /api/admin/mappings/bulk", s.admin(s.bulkMappings))
	mux.Handle("PUT /api/admin/mappings/{id}", s.admin(s.updateMapping))
	mux.Handle("DELETE /api/admin/mappings/{id}", s.admin(s.deleteMapping))

	// Admin - musteriler / planlar / anahtarlar / kullanim
	mux.Handle("GET /api/admin/plans", s.admin(s.listPlans))
	mux.Handle("POST /api/admin/plans", s.admin(s.createPlan))
	mux.Handle("PUT /api/admin/plans/{id}", s.admin(s.updatePlan))
	mux.Handle("GET /api/admin/clients", s.admin(s.listClients))
	mux.Handle("POST /api/admin/clients", s.admin(s.createClient))
	mux.Handle("GET /api/admin/clients/{id}", s.admin(s.getClient))
	mux.Handle("PUT /api/admin/clients/{id}", s.admin(s.updateClient))
	mux.Handle("DELETE /api/admin/clients/{id}", s.admin(s.deleteClient))
	mux.Handle("GET /api/admin/clients/{id}/keys", s.admin(s.listKeys))
	mux.Handle("POST /api/admin/clients/{id}/keys", s.admin(s.createKey))
	mux.Handle("DELETE /api/admin/keys/{id}", s.admin(s.revokeKey))
	mux.Handle("GET /api/admin/usage", s.admin(s.usageReport))
	mux.Handle("GET /api/admin/logs", s.admin(s.requestLogs))

	// Admin - provider sagligi / kuyruk
	mux.Handle("GET /api/admin/providers/health", s.admin(s.providersHealth))
	mux.Handle("POST /api/admin/providers/{name}/reset-circuit", s.admin(s.resetCircuit))
	mux.Handle("POST /api/admin/canary/run", s.admin(s.runCanary))
	mux.Handle("GET /api/admin/jobs/stats", s.admin(s.jobStats))

	// Admin paneli (SPA)
	s.mountAdminUI(mux)

	var h http.Handler = mux
	h = corsWith(s.cfg.CORSOrigins)(h)
	h = requestID(h)
	h = recoverer(h)
	return h
}

// ---- Middleware ----

// ---- Yardimcilar ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	body := map[string]any{"error": err.Error(), "requestId": RequestIDFrom(r.Context())}
	if d := apperr.DetailsOf(err); d != nil {
		body["details"] = d
	}
	writeJSON(w, apperr.Status(err), body)
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, 400, map[string]any{"error": msg})
}

func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 4<<20))
	return dec.Decode(v)
}

// parseChildAges "5,8" -> [5 8]
func parseChildAges(s string) []int {
	out := []int{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if n, err := strconv.Atoi(p); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func parseProviderList(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

// ---- Sistem uclari ----

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status": "ok", "providers": s.reg.Names(),
		"uptimeSeconds": int(time.Since(s.started).Seconds()),
	})
}

// ready: DB zorunlu (503), Redis opsiyonel (degraded).
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	checks := map[string]string{}
	status := 200
	overall := "ok"
	if err := s.db.Pool.Ping(ctx); err != nil {
		checks["postgres"] = "down: " + err.Error()
		status, overall = 503, "down"
	} else {
		checks["postgres"] = "ok"
	}
	if err := s.deps.Cache.Ping(ctx); err != nil {
		checks["redis"] = "down: " + err.Error()
		if overall == "ok" {
			overall = "degraded"
		}
	} else {
		checks["redis"] = "ok"
	}
	writeJSON(w, status, map[string]any{"status": overall, "checks": checks})
}

func (s *Server) providersHealth(w http.ResponseWriter, _ *http.Request) {
	stats := s.deps.Limiter.Snapshot(s.reg.Names())
	caps := map[string]any{}
	for _, p := range s.reg.All() {
		caps[p.Name()] = p.Capabilities()
	}
	var last any
	if s.deps.Canary != nil {
		if rep := s.deps.Canary.Last(); rep != nil {
			last = rep
		}
	}
	writeJSON(w, 200, map[string]any{
		"providers": stats, "capabilities": caps, "canary": last,
		"cache": map[string]any{"enabled": s.deps.Cache.Enabled(), "ttlSeconds": int(s.deps.Cache.TTL().Seconds())},
	})
}

func (s *Server) resetCircuit(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := s.reg.Get(name); err != nil {
		writeError(w, r, err)
		return
	}
	s.deps.Limiter.Reset(name)
	writeJSON(w, 200, map[string]any{"message": "Devre kesici sifirlandi.", "provider": name})
}

func (s *Server) runCanary(w http.ResponseWriter, r *http.Request) {
	if s.deps.Canary == nil || s.cfg.CanaryHotelID == "" {
		badRequest(w, "CANARY_HOTEL_ID ayarlanmamis")
		return
	}
	rep := s.deps.Canary.Run(r.Context())
	writeJSON(w, 200, rep)
}
