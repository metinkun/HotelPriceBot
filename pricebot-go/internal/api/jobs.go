package api

import (
	"fmt"
	"net/http"
	"pricebot/internal/auth"
	"strings"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/jobs"
	"pricebot/internal/types"
)

// POST /api/v1/jobs  body: jobs.Request  -> 202 {requestId, status, pollAfterMs, statusUrl}
func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	if s.deps.Jobs == nil {
		writeError(w, r, apperr.New(apperr.Unavailable, "Asenkron is kuyrugu bu ortamda etkin degil (Redis gerekir)"))
		return
	}
	var req jobs.Request
	if err := decodeBody(r, &req); err != nil {
		badRequest(w, "Gecersiz JSON govde")
		return
	}
	if req.Type == "" {
		req.Type = jobs.TypePrices
	}
	if req.ChildAges == nil {
		req.ChildAges = []int{}
	}
	req.HotelID = strings.TrimSpace(req.HotelID)
	req.Providers = normalizeProviders(req.Providers)
	if req.MaxWaitSeconds < 0 || req.MaxWaitSeconds > 120 {
		badRequest(w, "maxWaitSeconds 0-120 araliginda olmali (0 = tum siteler bitene kadar bekle)")
		return
	}
	switch req.Type {
	case jobs.TypePrices:
		pq := types.PriceQuery{HotelID: req.HotelID, CheckIn: req.CheckIn, CheckOut: req.CheckOut, Adults: req.Adults, Children: req.Children, ChildAges: req.ChildAges}
		if err := validatePriceQuery(&pq); err != nil {
			writeError(w, r, err)
			return
		}
		req.CheckIn, req.CheckOut = pq.CheckIn, pq.CheckOut
	case jobs.TypePackages:
		pq := types.PackageQuery{HotelID: req.HotelID, AirportCode: req.AirportCode, CheckIn: req.CheckIn, CheckOut: req.CheckOut, Adults: req.Adults, Children: req.Children, ChildAges: req.ChildAges}
		if err := validatePackageQuery(&pq); err != nil {
			writeError(w, r, err)
			return
		}
		req.CheckIn, req.CheckOut, req.AirportCode = pq.CheckIn, pq.CheckOut, pq.AirportCode
	default:
		badRequest(w, "type 'prices' veya 'packages' olmali")
		return
	}
	if err := s.providerNamesValid(req.Providers); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.requireClientHotels(r, req.HotelID); err != nil {
		writeError(w, r, err)
		return
	}
	if req.Type == jobs.TypePackages {
		if err := s.packagesAllowed(r); err != nil {
			writeError(w, r, err)
			return
		}
	}
	provs, err := s.providersForPlan(r, req.Providers)
	if err != nil {
		writeError(w, r, err)
		return
	}
	// Secilen sitelerde otelin eslesmesi (ve paket destegi) var mi? Yoksa hak dusmeden 400.
	// Kismen gecersizse calismayan siteler nedeniyle birlikte isin sonucunda raporlanir (progress "skipped").
	skipped := map[string]string{}
	for _, n := range s.closedSites(req.Providers) {
		skipped[n] = skippedMaintenance
	}
	if len(req.Providers) > 0 {
		mapped := map[string]bool{}
		if hv, err := s.hotels.Get(r.Context(), req.HotelID); err == nil {
			for _, p := range hv.Providers {
				if p.IsActive {
					mapped[p.Provider] = true
				}
			}
		}
		usable := []string{}
		unsupported := []string{}
		for _, n := range provs {
			if !mapped[n] {
				skipped[n] = "Bu otelin bu sitede eslesmesi yok (GET /api/v1/hotels -> providers); atlandi"
				continue
			}
			if req.Type == jobs.TypePackages {
				if p, err := s.reg.Get(n); err != nil || !p.Capabilities().SupportsPackages {
					unsupported = append(unsupported, n)
					skipped[n] = "Site paket (otel+ucak) sorgusunu desteklemiyor; atlandi"
					continue
				}
			}
			usable = append(usable, n)
		}
		if len(usable) == 0 {
			if len(unsupported) > 0 {
				writeError(w, r, apperr.BadRequestf("Secilen siteler paket (otel+ucak) sorgusunu desteklemiyor: %s", strings.Join(unsupported, ", ")))
				return
			}
			writeError(w, r, apperr.BadRequestf("Secilen sitelerde bu otelin eslesmesi yok: %s (GET /api/v1/hotels -> providers)", strings.Join(provs, ", ")))
			return
		}
		provs = usable
	}
	req.Providers = provs
	if len(skipped) > 0 {
		req.Skipped = skipped
	}
	req.Fresh = s.freshAllowed(r, req.Fresh)

	idn := auth.From(r.Context())
	pl := planFrom(r.Context())
	if idn != nil && !idn.IsAdmin && idn.ClientID > 0 && pl != nil && pl.DailySearches > 0 {
		used := s.db.ClientDailyToday(r.Context(), idn.ClientID).Searches
		w.Header().Set("X-Quota-Limit", fmt.Sprint(pl.DailySearches))
		w.Header().Set("X-Quota-Used", fmt.Sprint(used))
		if used >= pl.DailySearches {
			writeError(w, r, apperr.New(apperr.TooMany, "Gunluk arama hakki doldu (%d/%d). Hak her gun 00:00'da yenilenir.", used, pl.DailySearches))
			return
		}
	}
	job, err := s.deps.Jobs.Submit(r.Context(), clientIDFrom(r.Context()), req)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if idn != nil && !idn.IsAdmin && idn.ClientID > 0 && !job.Deduplicated {
		_ = s.db.IncClientDaily(r.Context(), idn.ClientID, "searches", 1)
		_ = s.db.IncClientDaily(r.Context(), idn.ClientID, "api_jobs", 1)
	}
	status := 202
	if job.Deduplicated {
		status = 200
	}
	w.Header().Set("Location", "/api/v1/jobs/"+job.RequestID)
	writeJSON(w, status, map[string]any{
		"requestId": job.RequestID, "status": job.Status, "deduplicated": job.Deduplicated,
		"pollAfterMs": 1000, "statusUrl": "/api/v1/jobs/" + job.RequestID,
		"expiresInSeconds": int(s.cfg.JobResultTTL.Seconds()),
	})
}

// GET /api/v1/jobs/{id}[?wait=20]  wait: en fazla 30 sn bekleyip sonucu doner (uzun bekleme).
func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	if s.deps.Jobs == nil {
		writeError(w, r, apperr.New(apperr.Unavailable, "Asenkron is kuyrugu bu ortamda etkin degil (Redis gerekir)"))
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	wait := time.Duration(atoiDefault(r.URL.Query().Get("wait"), 0)) * time.Second
	if wait > 30*time.Second {
		wait = 30 * time.Second
	}
	job, err := s.deps.Jobs.Get(r.Context(), id, wait)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if cid := clientIDFrom(r.Context()); cid != "" && job.ClientID != "" && job.ClientID != cid {
		writeError(w, r, apperr.NotFoundf("Is bulunamadi: %s", id))
		return
	}
	if !job.Finished() {
		w.Header().Set("Retry-After", "1")
	}
	out := *job
	out.IdentityJSON = nil // ic kimlik bilgisi musteriye donmez
	writeJSON(w, 200, &out)
}

// GET /api/v1/jobs/stats (admin)
func (s *Server) jobStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.deps.Jobs.Stats())
}
