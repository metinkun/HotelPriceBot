package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"pricebot/internal/db"
)

// Sites provider (site) ayarlarinin bellek kopyasi: genel acik/kapali, gorunen ad, siralama.
// Admin "Site Yonetimi" ekranindan degistirilir; kapali site tum musteriler icin "bakimda" sayilir.
type Sites struct {
	db   *db.DB
	mu   sync.RWMutex
	m    map[string]db.SiteSetting
	stop chan struct{}
}

func NewSites(d *db.DB) *Sites {
	s := &Sites{db: d, m: map[string]db.SiteSetting{}, stop: make(chan struct{})}
	s.Reload(context.Background())
	return s
}

// Reload ayarlari DB'den yeniden okur.
func (s *Sites) Reload(ctx context.Context) {
	if s.db == nil {
		return
	}
	m, err := s.db.ListSiteSettings(ctx)
	if err != nil {
		slog.Warn("site ayarlari okunamadi", "err", err)
		return
	}
	s.mu.Lock()
	s.m = m
	s.mu.Unlock()
}

// Start 60 sn'de bir yeniler (coklu instance ihtimaline karsi).
func (s *Sites) Start() {
	go func() {
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-t.C:
				s.Reload(context.Background())
			}
		}
	}()
}
func (s *Sites) Stop() { close(s.stop) }

// Enabled site genel olarak acik mi (kayit yoksa acik).
func (s *Sites) Enabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if x, ok := s.m[name]; ok {
		return x.Enabled
	}
	return true
}

// Label admin'in verdigi gorunen ad (yoksa def).
func (s *Sites) Label(name, def string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if x, ok := s.m[name]; ok && x.DisplayName != nil && *x.DisplayName != "" {
		return *x.DisplayName
	}
	return def
}

// Get kayit (yoksa varsayilan).
func (s *Sites) Get(name string) db.SiteSetting {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if x, ok := s.m[name]; ok {
		return x
	}
	return db.SiteSetting{Name: name, Enabled: true, SortOrder: 100}
}

// FilterEnabled listeden kapali siteleri cikarir.
func (s *Sites) FilterEnabled(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if s.Enabled(n) {
			out = append(out, n)
		}
	}
	return out
}
