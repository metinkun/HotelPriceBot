package api

import (
	"net/http"
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
	req.Providers = provs
	req.Fresh = s.freshAllowed(r, req.Fresh)

	job, err := s.deps.Jobs.Submit(r.Context(), clientIDFrom(r.Context()), req)
	if err != nil {
		writeError(w, r, err)
		return
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
	writeJSON(w, 200, job)
}

// GET /api/v1/jobs/stats (admin)
func (s *Server) jobStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.deps.Jobs.Stats())
}
