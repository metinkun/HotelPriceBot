// Package portal musteri tipi 2 (web portal) is mantigi: aramalar, izleme, planli aramalar.
package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/db"
	"pricebot/internal/jobs"
	"pricebot/internal/providers"
	"pricebot/internal/services"
)

// SearchRequest portal arama istegi (anlik veya planli). Tarih iki modda verilebilir:
// sabit (checkIn/checkOut) veya goreli (daysAhead + nights; planli aramalar icin).
type SearchRequest struct {
	Kind        string   `json:"kind"` // prices | packages
	HotelIDs    []string `json:"hotelIds"`
	Providers   []string `json:"providers,omitempty"`
	CheckIn     string   `json:"checkIn,omitempty"`
	CheckOut    string   `json:"checkOut,omitempty"`
	DaysAhead   int      `json:"daysAhead,omitempty"`
	Nights      int      `json:"nights,omitempty"`
	Adults      int      `json:"adults"`
	Children    int      `json:"children,omitempty"`
	ChildAges   []int    `json:"childAges,omitempty"`
	AirportCode string   `json:"airportCode,omitempty"`
}

// Resolve goreli tarihleri verilen ana gore sabitler.
func (r SearchRequest) Resolve(now time.Time) (checkIn, checkOut string) {
	if r.CheckIn != "" && r.CheckOut != "" {
		return r.CheckIn, r.CheckOut
	}
	days := r.DaysAhead
	if days <= 0 {
		days = 30
	}
	nights := r.Nights
	if nights <= 0 {
		nights = 3
	}
	ci := now.AddDate(0, 0, days)
	return ci.Format("2006-01-02"), ci.AddDate(0, 0, nights).Format("2006-01-02")
}

// HotelResult bir otelin arama sonucu (job sonucu + ozet).
type HotelResult struct {
	HotelID   string          `json:"hotelId"`
	HotelName string          `json:"hotelName,omitempty"`
	JobID     string          `json:"jobId,omitempty"`
	Status    string          `json:"status"`
	Error     string          `json:"error,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

// SearchResult aramanin toplam sonucu.
type SearchResult struct {
	CheckIn  string        `json:"checkIn"`
	CheckOut string        `json:"checkOut"`
	Hotels   []HotelResult `json:"hotels"`
	Done     int           `json:"done"`
	Total    int           `json:"total"`
}

// Service arama calistirma (job'lar uzerinden), izleme, planli aramalar.
type Service struct {
	db     *db.DB
	jobs   *jobs.Manager
	reg    *providers.Registry
	hotels *services.HotelService
	sites  *services.Sites

	mu       sync.Mutex
	watching map[int64]bool
	stop     chan struct{}
	wg       sync.WaitGroup
}

func New(d *db.DB, jm *jobs.Manager, reg *providers.Registry, hs *services.HotelService, sites *services.Sites) *Service {
	return &Service{db: d, jobs: jm, reg: reg, hotels: hs, sites: sites, watching: map[int64]bool{}, stop: make(chan struct{})}
}

func (p *Service) siteOpen(name string) bool { return p.sites == nil || p.sites.Enabled(name) }

// EffectiveProviders musterinin kullanabilecegi provider'lar: plan izinleri ∩ etkin siteler.
func (p *Service) EffectiveProviders(ctx context.Context, clientID int, planAllowed []string) ([]string, error) {
	s, err := p.db.GetClientSettings(ctx, clientID)
	if err != nil {
		return nil, err
	}
	all := p.reg.Names()
	allowed := map[string]bool{}
	for _, n := range all {
		allowed[n] = true
	}
	if len(planAllowed) > 0 {
		allowed = map[string]bool{}
		for _, n := range planAllowed {
			allowed[n] = true
		}
	}
	if len(s.EnabledProviders) > 0 {
		en := map[string]bool{}
		for _, n := range s.EnabledProviders {
			en[n] = true
		}
		for n := range allowed {
			if !en[n] {
				delete(allowed, n)
			}
		}
	}
	out := []string{}
	for _, n := range all { // kayit sirasini koru
		if allowed[n] && p.siteOpen(n) {
			out = append(out, n)
		}
	}
	return out, nil
}

// RunSearch bir arama kaydi acar, her otel icin job olusturur ve arka planda sonucu izler.
func (p *Service) RunSearch(ctx context.Context, id *auth.Identity, userID, scheduleID *int, req SearchRequest, effective []string) (*db.Search, error) {
	if p.jobs == nil {
		return nil, apperr.New(apperr.Unavailable, "Is kuyrugu etkin degil (Redis gerekli)")
	}
	if len(req.HotelIDs) == 0 {
		return nil, apperr.BadRequestf("En az bir otel secilmeli")
	}
	if len(req.HotelIDs) > 20 {
		return nil, apperr.BadRequestf("Tek aramada en fazla 20 otel")
	}
	if req.Kind == "" {
		req.Kind = "prices"
	}
	if req.Kind != "prices" && req.Kind != "packages" {
		return nil, apperr.BadRequestf("kind 'prices' veya 'packages' olmali")
	}
	if req.Adults <= 0 {
		req.Adults = 2
	}
	// Provider daraltma: istenenler etkin listede olmali; bos ise etkinlerin tamami
	provs := req.Providers
	if len(provs) == 0 {
		provs = effective
	} else {
		eff := map[string]bool{}
		for _, n := range effective {
			eff[n] = true
		}
		for _, n := range provs {
			if !eff[n] {
				return nil, apperr.New(apperr.Forbidden, "'%s' sitesi bu hesap icin etkin degil", n)
			}
		}
	}
	req.Providers = provs
	// Oteller musteriye ait olmali
	for _, h := range req.HotelIDs {
		ok, err := p.db.ClientOwnsHotel(ctx, id.ClientID, h)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, apperr.New(apperr.Forbidden, "Otel bu hesaba ait degil: %s", h)
		}
	}
	ci, co := req.Resolve(time.Now())
	inactive, _ := p.db.InactiveHotels(ctx, req.HotelIDs)
	s, err := p.db.CreateSearch(ctx, id.ClientID, userID, scheduleID, req.Kind, req)
	if err != nil {
		return nil, err
	}

	res := &SearchResult{CheckIn: ci, CheckOut: co, Total: len(req.HotelIDs)}
	jctx := auth.WithIdentity(context.Background(), id)
	for _, h := range req.HotelIDs {
		jr := jobs.Request{Type: jobs.Type(req.Kind), HotelID: h, CheckIn: ci, CheckOut: co, Adults: req.Adults, Children: req.Children, ChildAges: req.ChildAges, Providers: provs, AirportCode: strings.ToUpper(req.AirportCode)}
		hr := HotelResult{HotelID: h, Status: "queued"}
		if hv, err := p.hotels.Get(ctx, h); err == nil && hv.HotelName != nil {
			hr.HotelName = *hv.HotelName
		}
		if inactive[h] {
			hr.Status, hr.Error = "failed", "Otel pasif durumda; arama yapilmadi"
			res.Done++
			res.Hotels = append(res.Hotels, hr)
			continue
		}
		j, err := p.jobs.Submit(jctx, fmt.Sprint(id.ClientID), jr)
		if err != nil {
			hr.Status, hr.Error = "failed", err.Error()
			res.Done++
		} else {
			hr.JobID = j.RequestID
		}
		res.Hotels = append(res.Hotels, hr)
	}
	status := "running"
	if res.Done == res.Total {
		status = "done"
	}
	_ = p.db.UpdateSearch(ctx, s.ID, status, res, nil, status == "done")
	s.Status = status
	b, _ := json.Marshal(res)
	s.Result = b
	if status != "done" {
		p.watch(s.ID, res)
	}
	return s, nil
}

// watch job'lari izleyip arama sonucunu gunceller (kismi sonuclar da yazilir).
func (p *Service) watch(searchID int64, res *SearchResult) {
	p.mu.Lock()
	if p.watching[searchID] {
		p.mu.Unlock()
		return
	}
	p.watching[searchID] = true
	p.mu.Unlock()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			p.mu.Lock()
			delete(p.watching, searchID)
			p.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		for {
			allDone := true
			changed := false
			for i := range res.Hotels {
				h := &res.Hotels[i]
				if h.Status == "done" || h.Status == "failed" || h.JobID == "" {
					continue
				}
				j, err := p.jobs.Get(ctx, h.JobID)
				if err != nil {
					h.Status, h.Error = "failed", err.Error()
					changed = true
					continue
				}
				if len(j.Result) > 0 && string(j.Result) != string(h.Result) {
					h.Result = j.Result
					changed = true
				}
				switch j.Status {
				case jobs.StatusDone:
					h.Status = "done"
					changed = true
				case jobs.StatusFailed:
					h.Status, h.Error = "failed", j.Error
					changed = true
				default:
					h.Status = string(j.Status)
					allDone = false
				}
			}
			res.Done = 0
			for _, h := range res.Hotels {
				if h.Status == "done" || h.Status == "failed" {
					res.Done++
				}
			}
			if allDone {
				_ = p.db.UpdateSearch(ctx, searchID, "done", res, nil, true)
				return
			}
			if changed {
				_ = p.db.UpdateSearch(ctx, searchID, "running", res, nil, false)
			}
			select {
			case <-p.stop:
				return
			case <-ctx.Done():
				msg := "zaman asimi"
				_ = p.db.UpdateSearch(context.Background(), searchID, "failed", res, &msg, true)
				return
			case <-time.After(1500 * time.Millisecond):
			}
		}
	}()
}

// Start acilista yarim kalan aramalari failed yapar ve zamanlayiciyi baslatir.
func (p *Service) Start() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if ids, err := p.db.UnfinishedSearches(ctx); err == nil && len(ids) > 0 {
		_ = p.db.FailSearches(ctx, ids, "Sunucu yeniden baslatildi; arama yarim kaldi")
		slog.Warn("yarim kalan portal aramalari failed yapildi", "count", len(ids))
	}
	p.wg.Add(1)
	go p.schedulerLoop()
}

func (p *Service) Stop() {
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	p.wg.Wait()
}

// ---- Planli aramalar ----

// NextRun verilen hafta gunleri ve saat (HH:MM, tz) icin bir sonraki calisma zamanini hesaplar.
func NextRun(weekdays []int16, runTime, tz string, after time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("TRT", 3*3600)
	}
	var hh, mm int
	if _, err := fmt.Sscanf(runTime, "%d:%d", &hh, &mm); err != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return time.Time{}, apperr.BadRequestf("runTime HH:MM olmali (ornek 14:00)")
	}
	days := map[int]bool{}
	for _, d := range weekdays {
		if d >= 0 && d <= 6 {
			days[int(d)] = true
		}
	}
	now := after.In(loc)
	for i := 0; i < 8; i++ {
		day := now.AddDate(0, 0, i)
		cand := time.Date(day.Year(), day.Month(), day.Day(), hh, mm, 0, 0, loc)
		if !cand.After(now) {
			continue
		}
		if len(days) == 0 || days[int(cand.Weekday())] {
			return cand.UTC(), nil
		}
	}
	return time.Time{}, apperr.BadRequestf("gecerli bir calisma gunu bulunamadi")
}

func (p *Service) schedulerLoop() {
	defer p.wg.Done()
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			p.runDue()
		}
	}
}

func (p *Service) runDue() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	due, err := p.db.DueSchedules(ctx, time.Now())
	if err != nil {
		slog.Warn("planli aramalar okunamadi", "err", err)
		return
	}
	for _, s := range due {
		now := time.Now()
		next, err := NextRun(s.Weekdays, s.RunTime, s.Timezone, now)
		if err != nil {
			next = now.Add(24 * time.Hour)
		}
		_ = p.db.MarkScheduleRun(ctx, s.ID, now, next)

		var req SearchRequest
		if err := json.Unmarshal(s.Request, &req); err != nil {
			slog.Warn("plan istegi bozuk", "schedule", s.ID, "err", err)
			continue
		}
		req.Kind = s.Kind
		// Musteri plani, limitler ve etkin siteler
		id := &auth.Identity{ClientID: s.ClientID, PlanCode: "portal"}
		c, err := p.db.GetClient(ctx, s.ClientID)
		if err != nil || !c.IsActive {
			continue
		}
		if c.DemoExpiresAt != nil && now.After(*c.DemoExpiresAt) {
			continue // deneme suresi dolmus
		}
		var plan *db.Plan
		var planAllowed []string
		if c.PlanID != nil {
			if pl, err := p.db.GetPlan(ctx, *c.PlanID); err == nil {
				plan = pl
				planAllowed = pl.AllowedProviders
				id.PlanCode = pl.Code
			}
		}
		lim := db.EffectiveLimits(c, plan)
		sid := s.ID
		if lim.DailyScheduledRuns > 0 && p.db.ClientDailyToday(ctx, s.ClientID).ScheduledRuns >= lim.DailyScheduledRuns {
			msg := fmt.Sprintf("Gunluk planli gorev limiti doldu (%d); bu calisma atlandi", lim.DailyScheduledRuns)
			if sr, err := p.db.CreateSearch(ctx, s.ClientID, s.UserID, &sid, s.Kind, req); err == nil {
				_ = p.db.UpdateSearch(ctx, sr.ID, "failed", nil, &msg, true)
			}
			slog.Info("planli arama limit nedeniyle atlandi", "schedule", s.ID, "client", s.ClientID)
			continue
		}
		eff, err := p.EffectiveProviders(ctx, s.ClientID, planAllowed)
		if err != nil {
			continue
		}
		if _, err := p.RunSearch(ctx, id, s.UserID, &sid, req, eff); err != nil {
			slog.Warn("planli arama calistirilamadi", "schedule", s.ID, "err", err)
			continue
		}
		_ = p.db.IncClientDaily(ctx, s.ClientID, "scheduled_runs", 1)
		slog.Info("planli arama calistirildi", "schedule", s.ID, "name", s.Name, "next", next)
	}
}

// ProviderInfo portal icin site listesi.
type ProviderInfo struct {
	Name             string `json:"name"`
	Label            string `json:"label"`
	Domain           string `json:"domain"`
	Enabled          bool   `json:"enabled"`
	SupportsPackages bool   `json:"supportsPackages"`
	RequiresBrowser  bool   `json:"requiresBrowser"`
	Maintenance      bool   `json:"maintenance"` // admin tarafindan genel olarak kapatilmis
}

var providerMeta = map[string][2]string{
	"etstur": {"Etstur", "etstur.com"}, "tatilsepeti": {"TatilSepeti", "tatilsepeti.com"}, "jolly": {"Jolly Tur", "jollytur.com"},
	"tatilbudur": {"Tatilbudur", "tatilbudur.com"}, "touristica": {"Touristica", "touristica.com.tr"}, "setur": {"Setur", "setur.com.tr"},
	"gezinomi": {"Gezinomi", "gezinomi.com"}, "tatildukkani": {"Tatil Dükkanı", "tatildukkani.com"}, "tatilsitesi": {"Tatilsitesi", "tatilsitesi.com"},
	"prontotour": {"Prontotour", "prontotour.com"}, "tatilcikus": {"Tatil Çıkış", "tatilcikus.com"}, "enuygun": {"Enuygun", "enuygun.com"},
	"mngturizm": {"MNG Turizm", "mngturizm.com"},
}

// ProviderList tum provider'lari etkinlik bilgisiyle listeler.
func (p *Service) ProviderList(effective []string) []ProviderInfo {
	eff := map[string]bool{}
	for _, n := range effective {
		eff[n] = true
	}
	out := []ProviderInfo{}
	for _, pr := range p.reg.All() {
		meta, ok := providerMeta[pr.Name()]
		label, domain := pr.Name(), pr.Name()+".com"
		if ok {
			label, domain = meta[0], meta[1]
		}
		caps := pr.Capabilities()
		if p.sites != nil {
			label = p.sites.Label(pr.Name(), label)
		}
		out = append(out, ProviderInfo{Name: pr.Name(), Label: label, Domain: domain, Enabled: eff[pr.Name()] && p.siteOpen(pr.Name()), SupportsPackages: caps.SupportsPackages, RequiresBrowser: caps.RequiresBrowser, Maintenance: !p.siteOpen(pr.Name())})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Enabled && !out[j].Enabled })
	return out
}
