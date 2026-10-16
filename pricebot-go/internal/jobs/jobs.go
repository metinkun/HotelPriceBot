// Package jobs asenkron sorgu isleri: istek -> requestId; durum/sonuc Redis'te, calisma in-process worker havuzunda.
package jobs

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"pricebot/internal/apperr"
	"pricebot/internal/auth"
	"pricebot/internal/cache"
	"pricebot/internal/services"
	"pricebot/internal/types"
)

type Type string

const (
	TypePrices   Type = "prices"
	TypePackages Type = "packages"
)

type Status string

const (
	StatusQueued  Status = "queued"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

// Request istemcinin gonderdigi is tanimi.
type Request struct {
	Type        Type     `json:"type"`
	HotelID     string   `json:"hotelId"`
	CheckIn     string   `json:"checkIn"`
	CheckOut    string   `json:"checkOut"`
	Adults      int      `json:"adults"`
	Children    int      `json:"children"`
	ChildAges   []int    `json:"childAges,omitempty"`
	Providers   []string `json:"providers,omitempty"`
	AirportCode string   `json:"airportCode,omitempty"`
	Fresh       bool     `json:"fresh,omitempty"`
}

// Job dis dunyaya gosterilen is durumu.
type Job struct {
	RequestID    string            `json:"requestId"`
	Type         Type              `json:"type"`
	Status       Status            `json:"status"`
	ClientID     string            `json:"clientId,omitempty"`
	IdentityJSON *auth.Identity    `json:"identity,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	StartedAt    *time.Time        `json:"startedAt,omitempty"`
	FinishedAt   *time.Time        `json:"finishedAt,omitempty"`
	Request      Request           `json:"request"`
	Progress     map[string]string `json:"progress"` // provider -> pending|running|done|error
	Result       json.RawMessage   `json:"result,omitempty"`
	Error        string            `json:"error,omitempty"`
	Deduplicated bool              `json:"deduplicated,omitempty"`
}

func (j *Job) Finished() bool { return j.Status == StatusDone || j.Status == StatusFailed }

// Manager is kuyrugu ve calistirici.
type Manager struct {
	rdb       *redis.Client
	prices    *services.PriceService
	packages  *services.PackageService
	ttl       time.Duration
	queue     chan string
	queued    atomic.Int64
	running   atomic.Int64
	maxQueued int
	wg        sync.WaitGroup
	stop      chan struct{}
	stopped   atomic.Bool
}

const (
	keyPrefix = "job:"
	keyActive = "jobs:active"
	keyDedup  = "jobdedup:"
)

// New Redis zorunlu; yoksa nil doner (jobs API 503 verir).
func New(c *cache.Cache, prices *services.PriceService, packages *services.PackageService, ttl time.Duration, maxQueued, workers int) *Manager {
	if !c.Enabled() {
		return nil
	}
	if maxQueued <= 0 {
		maxQueued = 200
	}
	if workers <= 0 {
		workers = 16
	}
	m := &Manager{
		rdb: c.Redis(), prices: prices, packages: packages, ttl: ttl,
		queue: make(chan string, maxQueued), maxQueued: maxQueued, stop: make(chan struct{}),
	}
	m.recoverInterrupted()
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go m.worker()
	}
	return m
}

// Stop yeni is kabulunu durdurur ve calisanlarin bitmesini bekler.
func (m *Manager) Stop() {
	if m == nil {
		return
	}
	m.stopped.Store(true)
	close(m.stop)
	m.wg.Wait()
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func dedupKey(r Request) string {
	ages := append([]int{}, r.ChildAges...)
	sort.Ints(ages)
	provs := append([]string{}, r.Providers...)
	sort.Strings(provs)
	raw := fmt.Sprintf("%s|%s|%s|%s|%d|%d|%v|%v|%s|%v", r.Type, r.HotelID, r.CheckIn, r.CheckOut, r.Adults, r.Children, ages, provs, strings.ToUpper(r.AirportCode), r.Fresh)
	sum := sha256.Sum256([]byte(raw))
	return keyDedup + hex.EncodeToString(sum[:16])
}

// Stats kuyruk durumu.
type Stats struct {
	Queued    int64 `json:"queued"`
	Running   int64 `json:"running"`
	MaxQueued int   `json:"maxQueued"`
}

func (m *Manager) Stats() Stats {
	if m == nil {
		return Stats{}
	}
	return Stats{Queued: m.queued.Load(), Running: m.running.Load(), MaxQueued: m.maxQueued}
}

// Submit isi kaydeder ve kuyruga alir. Ayni istek zaten kuyrukta/calisiyorsa onu doner.
func (m *Manager) Submit(ctx context.Context, clientID string, req Request) (*Job, error) {
	identity := auth.From(ctx)
	if m == nil {
		return nil, apperr.New(apperr.Internal, "Job kuyrugu kullanilamiyor (Redis yok)")
	}
	if m.stopped.Load() {
		return nil, apperr.New(apperr.Internal, "Sunucu kapaniyor, is kabul edilmiyor")
	}
	// Tekillestirme: ayni istek bitmemis bir is ile eslesirse onu don
	dk := dedupKey(req)
	if existingID, err := m.rdb.Get(ctx, dk).Result(); err == nil && existingID != "" {
		if j, err := m.Get(ctx, existingID); err == nil && !j.Finished() {
			cp := *j
			cp.Deduplicated = true
			return &cp, nil
		}
	}
	if m.queued.Load() >= int64(m.maxQueued) {
		return nil, apperr.New(apperr.TooMany, "Kuyruk dolu (%d). Lutfen biraz sonra tekrar deneyin.", m.maxQueued)
	}

	job := &Job{
		RequestID: newID(), Type: req.Type, Status: StatusQueued, ClientID: clientID,
		CreatedAt: time.Now().UTC(), Request: req, Progress: map[string]string{}, IdentityJSON: identity,
	}
	if err := m.save(ctx, job); err != nil {
		return nil, err
	}
	_ = m.rdb.SAdd(ctx, keyActive, job.RequestID).Err()
	_ = m.rdb.Set(ctx, dk, job.RequestID, 10*time.Minute).Err()

	m.queued.Add(1)
	select {
	case m.queue <- job.RequestID:
	default:
		m.queued.Add(-1)
		job.Status = StatusFailed
		job.Error = "Kuyruk dolu"
		_ = m.save(ctx, job)
		return nil, apperr.New(apperr.TooMany, "Kuyruk dolu (%d). Lutfen biraz sonra tekrar deneyin.", m.maxQueued)
	}
	return job, nil
}

// Get is durumunu okur; wait > 0 ise bitmesini en fazla wait kadar bekler.
func (m *Manager) Get(ctx context.Context, id string, wait ...time.Duration) (*Job, error) {
	if m == nil {
		return nil, apperr.New(apperr.Internal, "Job kuyrugu kullanilamiyor (Redis yok)")
	}
	var deadline time.Time
	if len(wait) > 0 && wait[0] > 0 {
		deadline = time.Now().Add(wait[0])
	}
	for {
		j, err := m.load(ctx, id)
		if err != nil {
			return nil, err
		}
		if j.Finished() || deadline.IsZero() || time.Now().After(deadline) {
			return j, nil
		}
		select {
		case <-ctx.Done():
			return j, nil
		case <-time.After(400 * time.Millisecond):
		}
	}
}

func (m *Manager) save(ctx context.Context, j *Job) error {
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	return m.rdb.Set(ctx, keyPrefix+j.RequestID, b, m.ttl).Err()
}

func (m *Manager) load(ctx context.Context, id string) (*Job, error) {
	b, err := m.rdb.Get(ctx, keyPrefix+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, apperr.NotFoundf("Is bulunamadi: %s (suresi dolmus olabilir)", id)
	}
	if err != nil {
		return nil, err
	}
	var j Job
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

// recoverInterrupted onceki calismadan yarim kalan isleri failed olarak isaretler.
func (m *Manager) recoverInterrupted() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ids, err := m.rdb.SMembers(ctx, keyActive).Result()
	if err != nil {
		return
	}
	n := 0
	for _, id := range ids {
		j, err := m.load(ctx, id)
		if err != nil {
			_ = m.rdb.SRem(ctx, keyActive, id).Err()
			continue
		}
		if !j.Finished() {
			now := time.Now().UTC()
			j.Status = StatusFailed
			j.Error = "Sunucu yeniden baslatildi; is yarim kaldi. Lutfen tekrar gonderin."
			j.FinishedAt = &now
			_ = m.save(ctx, j)
			n++
		}
		_ = m.rdb.SRem(ctx, keyActive, id).Err()
	}
	if n > 0 {
		slog.Warn("yarim kalan isler failed olarak isaretlendi", "count", n)
	}
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.stop:
			return
		case id := <-m.queue:
			m.queued.Add(-1)
			m.running.Add(1)
			m.run(id)
			m.running.Add(-1)
		}
	}
}

// run isi calistirir: her provider ayri goroutine, her bitiste kismi sonuc Redis'e yazilir.
func (m *Manager) run(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	j, err := m.load(ctx, id)
	if err != nil {
		return
	}
	if j.IdentityJSON != nil {
		ctx = auth.WithIdentity(ctx, j.IdentityJSON) // kredi dusumu dogru musteriye yazilsin
	}
	now := time.Now().UTC()
	j.Status = StatusRunning
	j.StartedAt = &now
	_ = m.save(ctx, j)

	finish := func(status Status, errMsg string) {
		t := time.Now().UTC()
		j.Status = status
		j.FinishedAt = &t
		j.Error = errMsg
		_ = m.save(ctx, j)
		_ = m.rdb.SRem(ctx, keyActive, id).Err()
		_ = m.rdb.Del(ctx, dedupKey(j.Request)).Err()
	}

	var mu sync.Mutex
	switch j.Type {
	case TypePrices:
		q := types.PriceQuery{HotelID: j.Request.HotelID, CheckIn: j.Request.CheckIn, CheckOut: j.Request.CheckOut, Adults: j.Request.Adults, Children: j.Request.Children, ChildAges: j.Request.ChildAges}
		ms, err := m.prices.MappingsFor(ctx, q.HotelID, j.Request.Providers)
		if err != nil {
			finish(StatusFailed, err.Error())
			return
		}
		agg := &services.MultiPriceResult{HotelID: q.HotelID, CheckIn: q.CheckIn, CheckOut: q.CheckOut, Results: []services.ProviderPriceResult{}}
		for _, mp := range ms {
			j.Progress[mp.Provider] = "pending"
		}
		_ = m.save(ctx, j)
		var wg sync.WaitGroup
		for i := range ms {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				name := ms[i].Provider
				mu.Lock()
				j.Progress[name] = "running"
				_ = m.save(ctx, j)
				mu.Unlock()
				r, err := m.prices.PriceForMapping(ctx, &ms[i], q, j.Request.Fresh)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					j.Progress[name] = "error"
					agg.Errors = append(agg.Errors, services.ProviderError{Provider: name, Error: err.Error()})
				} else {
					j.Progress[name] = "done"
					agg.Results = append(agg.Results, *r)
				}
				j.Result, _ = json.Marshal(agg)
				_ = m.save(ctx, j)
			}(i)
		}
		wg.Wait()
		finish(StatusDone, "")

	case TypePackages:
		q := types.PackageQuery{HotelID: j.Request.HotelID, AirportCode: strings.ToUpper(j.Request.AirportCode), CheckIn: j.Request.CheckIn, CheckOut: j.Request.CheckOut, Adults: j.Request.Adults, Children: j.Request.Children, ChildAges: j.Request.ChildAges}
		ms, notSupported, notMapped, err := m.packages.MappingsForPackages(ctx, q.HotelID, j.Request.Providers)
		if err != nil {
			finish(StatusFailed, err.Error())
			return
		}
		agg := &services.MultiPackageResult{HotelID: q.HotelID, Results: []services.ProviderPackageResult{}, NotSupported: notSupported, NotMapped: notMapped}
		for _, mp := range ms {
			j.Progress[mp.Provider] = "pending"
		}
		j.Result, _ = json.Marshal(agg)
		_ = m.save(ctx, j)
		var wg sync.WaitGroup
		for i := range ms {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				name := ms[i].Provider
				mu.Lock()
				j.Progress[name] = "running"
				_ = m.save(ctx, j)
				mu.Unlock()
				r, err := m.packages.PackageForMapping(ctx, &ms[i], q, j.Request.Fresh)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					j.Progress[name] = "error"
					agg.Errors = append(agg.Errors, services.ProviderError{Provider: name, Error: err.Error()})
				} else {
					j.Progress[name] = "done"
					agg.Results = append(agg.Results, *r)
				}
				j.Result, _ = json.Marshal(agg)
				_ = m.save(ctx, j)
			}(i)
		}
		wg.Wait()
		finish(StatusDone, "")

	default:
		finish(StatusFailed, "Bilinmeyen is tipi: "+string(j.Type))
	}
}
