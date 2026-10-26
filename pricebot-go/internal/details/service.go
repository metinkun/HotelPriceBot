package details

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"pricebot/internal/apperr"
	"pricebot/internal/db"
	"pricebot/internal/limiter"
	"pricebot/internal/model"
	"pricebot/internal/provider"
	"pricebot/internal/providers"
)

// Service statik detay cekme, saklama, gunluk kontrol ve bildirim.
type Service struct {
	db  *db.DB
	reg *providers.Registry
	lim *limiter.Limiter

	runHour int            // gunluk kontrol saati (Europe/Istanbul)
	loc     *time.Location // zaman dilimi
	stop    chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	running bool
	last    *RunReport
}

type RunReport struct {
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Total      int       `json:"total"`
	Changed    int       `json:"changed"`
	Unchanged  int       `json:"unchanged"`
	Failed     int       `json:"failed"`
	Errors     []string  `json:"errors,omitempty"`
}

func New(d *db.DB, reg *providers.Registry, lim *limiter.Limiter, runHour int) *Service {
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		loc = time.FixedZone("TRT", 3*3600)
	}
	if runHour < 0 || runHour > 23 {
		runHour = 4
	}
	return &Service{db: d, reg: reg, lim: lim, runHour: runHour, loc: loc, stop: make(chan struct{})}
}

// Supports provider statik detay destekliyor mu?
func (s *Service) Supports(providerName string) bool {
	p, err := s.reg.Get(providerName)
	if err != nil {
		return false
	}
	_, ok := p.(provider.DetailProvider)
	return ok
}

// SupportedProviders detay destekleyen provider adlari.
func (s *Service) SupportedProviders() []string {
	out := []string{}
	for _, p := range s.reg.All() {
		if _, ok := p.(provider.DetailProvider); ok {
			out = append(out, p.Name())
		}
	}
	return out
}

// Refresh bir otel+acente icin detayi ceker, onceki snapshot ile karsilastirir, degisikligi kaydeder ve bildirir.
// Donus: satir, degisti mi, hata.
func (s *Service) Refresh(ctx context.Context, m *model.HotelMapping) (*db.HotelDetailRow, bool, error) {
	p, err := s.reg.Get(m.Provider)
	if err != nil {
		return nil, false, err
	}
	dp, ok := p.(provider.DetailProvider)
	if !ok {
		return nil, false, apperr.BadRequestf("'%s' statik detay desteklemiyor", m.Provider)
	}
	release, err := s.lim.Acquire(ctx, m.Provider)
	if err != nil {
		return nil, false, err
	}
	fctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	d, err := safeGetDetails(fctx, dp, m)
	release(err, err == nil)
	if err != nil {
		s.db.SetDetailError(ctx, m.InternalHotelID, m.Provider, err.Error())
		return nil, false, err
	}
	d.Provider, d.ProviderHotelID = m.Provider, m.ProviderHotelID
	if d.SourceURL == "" {
		d.SourceURL = m.MetaString("sourceUrl")
	}
	d.FetchedAt = time.Now().UTC()
	hash := Hash(d)

	prev, perr := s.db.GetDetail(ctx, m.InternalHotelID, m.Provider)
	first := perr != nil || prev.Version == 0
	if !first && prev.ContentHash == hash {
		row, err := s.db.SaveDetail(ctx, m.InternalHotelID, m.Provider, hash, d, false)
		return row, false, err
	}
	row, err := s.db.SaveDetail(ctx, m.InternalHotelID, m.Provider, hash, d, true)
	if err != nil {
		return nil, false, err
	}
	if first {
		return row, false, nil // ilk snapshot: degisiklik degil
	}
	var old provider.HotelDetails
	if err := json.Unmarshal(prev.Snapshot, &old); err != nil {
		return row, true, nil
	}
	changes := Diff(&old, d)
	summary := Summary(changes)
	if _, err := s.db.AddDetailChange(ctx, m.InternalHotelID, m.Provider, row.Version, changes, summary); err != nil {
		slog.Warn("detay degisikligi yazilamadi", "err", err)
	}
	s.notify(ctx, m, d.Name, summary, len(changes))
	return row, true, nil
}

// notify admin'e ve oteli listesinde tutan musterilere acente bazli bildirim yazar.
func (s *Service) notify(ctx context.Context, m *model.HotelMapping, hotelName, summary string, n int) {
	if hotelName == "" && m.HotelName != nil {
		hotelName = *m.HotelName
	}
	title := fmt.Sprintf("%s: %s otel bilgileri degisti", m.Provider, hotelName)
	payload := map[string]any{"changes": n}
	_ = s.db.AddNotification(ctx, nil, "detail_change", m.Provider, m.InternalHotelID, title, summary, payload)
	clients, _ := s.db.ClientsForHotel(ctx, m.InternalHotelID)
	for _, cid := range clients {
		c := cid
		_ = s.db.AddNotification(ctx, &c, "detail_change", m.Provider, m.InternalHotelID, title, summary, payload)
	}
}

// RefreshHotel bir otelin tum acentelerini yeniler (paralel, provider sinirlariyla).
func (s *Service) RefreshHotel(ctx context.Context, hotelID string) ([]db.HotelDetailRow, []string) {
	ms, err := s.db.ListMappings(ctx, db.MappingFilter{InternalHotelID: hotelID, OrderBy: "provider ASC"})
	if err != nil {
		return nil, []string{err.Error()}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	rows := []db.HotelDetailRow{}
	errs := []string{}
	for i := range ms {
		if !s.Supports(ms[i].Provider) || !ms[i].IsActive {
			continue
		}
		wg.Add(1)
		go func(m *model.HotelMapping) {
			defer wg.Done()
			row, _, err := s.Refresh(ctx, m)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, m.Provider+": "+err.Error())
				return
			}
			rows = append(rows, *row)
		}(&ms[i])
	}
	wg.Wait()
	return rows, errs
}

// RunAll tum aktif eslestirmeleri kontrol eder (gunluk is).
func (s *Service) RunAll(ctx context.Context) *RunReport {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return s.last
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	rep := &RunReport{StartedAt: time.Now()}
	active := true
	ms, err := s.db.ListMappings(ctx, db.MappingFilter{Active: &active, OrderBy: "provider ASC"})
	if err != nil {
		rep.Errors = append(rep.Errors, err.Error())
		rep.FinishedAt = time.Now()
		return rep
	}
	// Provider basina sira ile (limiter zaten eszamanliligi sinirlar); provider'lar arasinda paralel
	byProv := map[string][]*model.HotelMapping{}
	for i := range ms {
		if s.Supports(ms[i].Provider) {
			byProv[ms[i].Provider] = append(byProv[ms[i].Provider], &ms[i])
		}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for prov, list := range byProv {
		wg.Add(1)
		go func(prov string, list []*model.HotelMapping) {
			defer wg.Done()
			for _, m := range list {
				select {
				case <-s.stop:
					return
				default:
				}
				_, changed, err := s.Refresh(ctx, m)
				mu.Lock()
				rep.Total++
				switch {
				case err != nil:
					rep.Failed++
					if len(rep.Errors) < 50 {
						rep.Errors = append(rep.Errors, fmt.Sprintf("%s/%s: %v", prov, m.InternalHotelID, err))
					}
				case changed:
					rep.Changed++
				default:
					rep.Unchanged++
				}
				mu.Unlock()
				time.Sleep(300 * time.Millisecond) // site nezaketi
			}
		}(prov, list)
	}
	wg.Wait()
	rep.FinishedAt = time.Now()
	s.mu.Lock()
	s.last = rep
	s.mu.Unlock()
	slog.Info("statik detay gunluk kontrol tamamlandi", "total", rep.Total, "changed", rep.Changed, "failed", rep.Failed, "sure", rep.FinishedAt.Sub(rep.StartedAt).Round(time.Second))
	return rep
}

func (s *Service) LastReport() *RunReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

// Start her gun runHour:00 (Istanbul) RunAll calistirir.
func (s *Service) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			now := time.Now().In(s.loc)
			next := time.Date(now.Year(), now.Month(), now.Day(), s.runHour, 0, 0, 0, s.loc)
			if !next.After(now) {
				next = next.AddDate(0, 0, 1)
			}
			select {
			case <-s.stop:
				return
			case <-time.After(time.Until(next)):
				s.RunAll(context.Background())
			}
		}
	}()
}

func (s *Service) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	s.wg.Wait()
}

// ---- Goruntuleme ----

// View bir goruntuleyici icin otel+acente detayi: snapshot, son degisiklik, vurgulanacak path'ler.
type View struct {
	Provider       string           `json:"provider"`
	Version        int              `json:"version"`
	FetchedAt      time.Time        `json:"fetchedAt"`
	ChangedAt      *time.Time       `json:"changedAt"`
	Error          *string          `json:"error"`
	Snapshot       json.RawMessage  `json:"snapshot"`
	LastChange     *db.DetailChange `json:"lastChange,omitempty"`
	Highlight      bool             `json:"highlight"`                // bu kullanici bu surumu ilk kez goruyor ve degisiklik var
	HighlightPaths []string         `json:"highlightPaths,omitempty"` // vurgulanacak alan path'leri
}

// GetView snapshot'i doner; kullanici bu surumu ilk kez goruyorsa degisen alanlar vurgulanir ve surum "gorulmus" yazilir.
func (s *Service) GetView(ctx context.Context, viewerKind string, viewerID int, hotelID, providerName string, markSeen bool) (*View, error) {
	row, err := s.db.GetDetail(ctx, hotelID, providerName)
	if err != nil {
		return nil, apperr.NotFoundf("Bu acente icin detay henuz cekilmedi: %s", providerName)
	}
	v := &View{Provider: providerName, Version: row.Version, FetchedAt: row.FetchedAt, ChangedAt: row.ChangedAt, Error: row.Error, Snapshot: row.Snapshot}
	if ch, err := s.db.LatestDetailChange(ctx, hotelID, providerName); err == nil {
		v.LastChange = ch
		seen := s.db.SeenVersion(ctx, viewerKind, viewerID, hotelID, providerName)
		if seen > 0 && seen < row.Version && ch.Version > seen {
			v.Highlight = true
			var changes []Change
			_ = json.Unmarshal(ch.Diff, &changes)
			for _, c := range changes {
				v.HighlightPaths = append(v.HighlightPaths, c.Path)
			}
		}
	}
	if markSeen {
		_ = s.db.MarkSeen(ctx, viewerKind, viewerID, hotelID, providerName, row.Version)
	}
	return v, nil
}

// safeGetDetails provider panigini hataya cevirir (tek bir sitenin hatasi sureci dusurmesin).
func safeGetDetails(ctx context.Context, dp provider.DetailProvider, m *model.HotelMapping) (d *provider.HotelDetails, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			d, err = nil, fmt.Errorf("provider panic: %v", rec)
		}
	}()
	return dp.GetDetails(ctx, m)
}
