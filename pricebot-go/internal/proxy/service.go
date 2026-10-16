// Package proxy Webshare.io proxy listesini DB'ye senkronlar ve round-robin dagitir.
package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"pricebot/internal/config"
	"pricebot/internal/db"
	"pricebot/internal/httpx"
)

type cached struct {
	host     string
	port     int
	username string
	password string
}

type Service struct {
	cfg *config.Config
	db  *db.DB

	mu      sync.Mutex
	proxies []cached
	index   int

	stop chan struct{}
	wg   sync.WaitGroup
}

func New(cfg *config.Config, d *db.DB) *Service {
	return &Service{cfg: cfg, db: d, stop: make(chan struct{})}
}

// ---- Webshare API tipleri ----

type webshareProxy struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	ProxyAddress     string `json:"proxy_address"`
	Port             int    `json:"port"`
	Valid            bool   `json:"valid"`
	LastVerification string `json:"last_verification"`
	CountryCode      string `json:"country_code"`
}

type webshareList struct {
	Count    int             `json:"count"`
	Next     *string         `json:"next"`
	Previous *string         `json:"previous"`
	Results  []webshareProxy `json:"results"`
}

// Sync Webshare -> DB -> bellek.
func (s *Service) Sync(ctx context.Context) (int, error) {
	if s.cfg.WebshareAPIKey == "" {
		log.Println("[ProxyService] WEBSHARE_API_KEY ayarlanmamis, sync atlaniyor")
		return 0, nil
	}

	var all []webshareProxy
	for page := 1; ; page++ {
		u := fmt.Sprintf("https://proxy.webshare.io/api/v2/proxy/list/?mode=direct&page=%d&page_size=100", page)
		resp, err := httpx.Do(ctx, http.MethodGet, u, httpx.Options{
			Headers: map[string]string{"Authorization": "Token " + s.cfg.WebshareAPIKey},
			Timeout: 15 * time.Second,
			NoProxy: true,
		})
		if err != nil {
			return 0, err
		}
		var lst webshareList
		if err := json.Unmarshal(resp.Body, &lst); err != nil {
			return 0, fmt.Errorf("webshare yaniti parse edilemedi: %w", err)
		}
		all = append(all, lst.Results...)
		if lst.Next == nil {
			break
		}
	}

	ups := make([]db.ProxyUpsert, 0, len(all))
	for _, p := range all {
		var cc *string
		if p.CountryCode != "" {
			c := p.CountryCode
			cc = &c
		}
		var lv *time.Time
		if p.LastVerification != "" {
			if t, err := time.Parse(time.RFC3339Nano, p.LastVerification); err == nil {
				lv = &t
			} else if t, err := time.Parse(time.RFC3339, p.LastVerification); err == nil {
				lv = &t
			}
		}
		ups = append(ups, db.ProxyUpsert{
			WebshareID: p.ID, ProxyAddress: p.ProxyAddress, Port: p.Port,
			Username: p.Username, Password: p.Password, CountryCode: cc,
			Valid: p.Valid, LastVerification: lv,
		})
	}
	if err := s.db.UpsertProxies(ctx, ups); err != nil {
		return 0, err
	}
	if err := s.refreshCache(ctx); err != nil {
		return 0, err
	}
	log.Printf("[ProxyService] %d proxy senkronize edildi", len(all))
	return len(all), nil
}

func (s *Service) refreshCache(ctx context.Context) error {
	list, err := s.db.ValidProxies(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proxies = s.proxies[:0]
	for _, p := range list {
		s.proxies = append(s.proxies, cached{host: p.ProxyAddress, port: p.Port, username: p.Username, password: p.Password})
	}
	if s.index >= len(s.proxies) {
		s.index = 0
	}
	return nil
}

// Next round-robin proxy URL'i doner; proxy yoksa nil (dogrudan baglanti).
func (s *Service) Next() *url.URL {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.proxies) > 0 {
		p := s.proxies[s.index%len(s.proxies)]
		s.index = (s.index + 1) % len(s.proxies)
		return &url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", p.host, p.port), User: url.UserPassword(p.username, p.password)}
	}
	// Statik fallback
	if s.cfg.ProxyHost == "" || s.cfg.ProxyPort == 0 {
		return nil
	}
	u := &url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", s.cfg.ProxyHost, s.cfg.ProxyPort)}
	if s.cfg.ProxyUsername != "" {
		u.User = url.UserPassword(s.cfg.ProxyUsername, s.cfg.ProxyPassword)
	}
	return u
}

// Start baslangic senkronu + periyodik senkron baslatir ve httpx'e proxy secicisini baglar.
func (s *Service) Start(ctx context.Context) {
	httpx.SetProxyFunc(s.Next)

	if _, err := s.Sync(ctx); err != nil {
		log.Printf("[ProxyService] Baslangic senkronizasyonu basarisiz: %v", err)
		if err := s.refreshCache(ctx); err != nil {
			log.Printf("[ProxyService] DB'den proxy yuklenemedi: %v", err)
		}
	} else {
		log.Println("[ProxyService] Baslangic senkronizasyonu tamamlandi")
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		t := time.NewTicker(s.cfg.ProxySyncInterval)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-t.C:
				if _, err := s.Sync(context.Background()); err != nil {
					log.Printf("[ProxyService] Proxy senkronizasyonu basarisiz: %v", err)
				}
			}
		}
	}()
	log.Printf("[ProxyService] Proxy senkronizasyonu her %.0fs ayarlandi", s.cfg.ProxySyncInterval.Seconds())
}

// Stop periyodik senkronu durdurur.
func (s *Service) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	s.wg.Wait()
}
