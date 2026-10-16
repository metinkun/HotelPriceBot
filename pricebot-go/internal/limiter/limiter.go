// Package limiter provider basina eszamanlilik siniri, devre kesici ve saglik istatistikleri.
package limiter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrCircuitOpen provider gecici olarak devre disi.
var ErrCircuitOpen = errors.New("provider gecici olarak devre disi (devre kesici acik)")

// Stats bir provider'in anlik saglik ozeti.
type Stats struct {
	Provider        string     `json:"provider"`
	Concurrency     int        `json:"concurrency"`
	InFlight        int        `json:"inFlight"`
	Total           int64      `json:"total"`
	Success         int64      `json:"success"`
	Failed          int64      `json:"failed"`
	NotFound        int64      `json:"notFound"`
	ConsecutiveFail int        `json:"consecutiveFailures"`
	CircuitOpen     bool       `json:"circuitOpen"`
	CircuitOpenTill *time.Time `json:"circuitOpenUntil,omitempty"`
	AvgLatencyMs    int64      `json:"avgLatencyMs"`
	LastSuccess     *time.Time `json:"lastSuccess,omitempty"`
	LastFailure     *time.Time `json:"lastFailure,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
}

type entry struct {
	sem        chan struct{}
	stats      Stats
	latencySum int64
	openUntil  time.Time
}

type Limiter struct {
	mu            sync.Mutex
	entries       map[string]*entry
	defaultLimit  int
	limits        map[string]int
	failThreshold int
	openFor       time.Duration
}

// ParseLimits "touristica=1,setur=3,default=5" formatini cozer.
func ParseLimits(spec string) (map[string]int, int) {
	limits := map[string]int{}
	def := 5
	for _, part := range strings.Split(spec, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err != nil || n <= 0 {
			continue
		}
		if strings.TrimSpace(kv[0]) == "default" {
			def = n
		} else {
			limits[strings.TrimSpace(kv[0])] = n
		}
	}
	return limits, def
}

func New(limits map[string]int, defaultLimit, failThreshold int, openFor time.Duration) *Limiter {
	if defaultLimit <= 0 {
		defaultLimit = 5
	}
	if failThreshold <= 0 {
		failThreshold = 5
	}
	if openFor <= 0 {
		openFor = 2 * time.Minute
	}
	if limits == nil {
		limits = map[string]int{}
	}
	return &Limiter{entries: map[string]*entry{}, defaultLimit: defaultLimit, limits: limits, failThreshold: failThreshold, openFor: openFor}
}

func (l *Limiter) get(name string) *entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[name]
	if !ok {
		limit := l.defaultLimit
		if v, ok := l.limits[name]; ok {
			limit = v
		}
		e = &entry{sem: make(chan struct{}, limit), stats: Stats{Provider: name, Concurrency: limit}}
		l.entries[name] = e
	}
	return e
}

// Acquire eszamanlilik slotu alir (ctx iptal olursa hata). Devre acik ise ErrCircuitOpen.
// Donen fonksiyon slotu birakir ve sonucu kaydeder: release(err, found).
func (l *Limiter) Acquire(ctx context.Context, name string) (func(err error, found bool), error) {
	e := l.get(name)

	l.mu.Lock()
	open := time.Now().Before(e.openUntil)
	l.mu.Unlock()
	if open {
		return nil, fmt.Errorf("%s: %w", name, ErrCircuitOpen)
	}

	select {
	case e.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	l.mu.Lock()
	e.stats.InFlight++
	l.mu.Unlock()
	start := time.Now()

	return func(err error, found bool) {
		<-e.sem
		l.mu.Lock()
		defer l.mu.Unlock()
		now := time.Now()
		e.stats.InFlight--
		e.stats.Total++
		e.latencySum += now.Sub(start).Milliseconds()
		e.stats.AvgLatencyMs = e.latencySum / e.stats.Total
		if err != nil {
			e.stats.Failed++
			e.stats.ConsecutiveFail++
			e.stats.LastFailure = &now
			e.stats.LastError = err.Error()
			if e.stats.ConsecutiveFail >= l.failThreshold {
				e.openUntil = now.Add(l.openFor)
			}
			return
		}
		e.stats.Success++
		e.stats.ConsecutiveFail = 0
		e.stats.LastSuccess = &now
		e.openUntil = time.Time{}
		if !found {
			e.stats.NotFound++
		}
	}, nil
}

// Snapshot verilen provider'larin istatistiklerini doner.
func (l *Limiter) Snapshot(names []string) []Stats {
	for _, n := range names {
		l.get(n)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Stats, 0, len(names))
	now := time.Now()
	for _, n := range names {
		e := l.entries[n]
		s := e.stats
		if now.Before(e.openUntil) {
			s.CircuitOpen = true
			t := e.openUntil
			s.CircuitOpenTill = &t
		}
		out = append(out, s)
	}
	return out
}

// Reset bir provider'in devre kesicisini kapatir (admin).
func (l *Limiter) Reset(name string) {
	e := l.get(name)
	l.mu.Lock()
	e.openUntil = time.Time{}
	e.stats.ConsecutiveFail = 0
	l.mu.Unlock()
}
