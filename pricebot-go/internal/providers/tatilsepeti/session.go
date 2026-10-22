package tatilsepeti

// Otomatik oturum (hibrit).
//
// TatilSepeti'nin fiyat/paket uclari ASP.NET MVC JSON aksiyonlaridir. Sitenin JS'i her XHR'a
// sayfadaki `#forgeryToken` degerini `VerificationToken` basligi olarak ekler ve tarayici
// ASP.NET_SessionId + Cloudflare (cf_clearance) cerezlerini tasir. Olcumlerimizde (Ekim 2026)
// uclar cerezsiz ve token'siz de JSON donuyor; ancak site Cloudflare bot yonetimi arkasinda
// (sayfa yuklenirken /cdn-cgi/challenge-platform/.../jsd cagrisi, cf_clearance cerezi), bu yuzden
// sunucu/proxy IP'lerinde engel gorulebilir. Bu dosya oturumu once duz HTTP ile (detay sayfasi
// GET -> Set-Cookie + forgeryToken), o da engellenirse headless Chrome ile (sayfayi ac, challenge
// gecsin, cerezleri ve token'i oku, sekmeyi kapat) edinir ve ~20 dk onbellekler.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"pricebot/internal/browser"
	"pricebot/internal/httpx"
	"pricebot/internal/provider"
)

const (
	homeURL           = "https://www.tatilsepeti.com/"
	defaultSessionAge = 20 * time.Minute
)

// sessionSource oturumun nereden geldigi (log/health icin).
type sessionSource string

const (
	sourceEnv     sessionSource = "env"
	sourceHTTP    sessionSource = "http"
	sourceBrowser sessionSource = "browser"
)

type session struct {
	Cookie string
	Token  string
	At     time.Time
	Source sessionSource
}

func (s *session) expired(maxAge time.Duration) bool {
	return s == nil || (s.Source != sourceEnv && time.Since(s.At) > maxAge)
}

// sessionCache tek bir paylasilan oturumu tutar; cerezler otele bagli degildir.
type sessionCache struct {
	mu            sync.Mutex
	cur           *session
	override      *session // env'den gelen cookie/token (varsa once denenir)
	overrideDead  bool     // override ile istek basarisiz oldu -> otomatik edinime gec
	preferBrowser bool     // duz HTTP engellendi -> bir sonraki edinimde tarayici
	maxAge        time.Duration
	bm            *browser.Manager
	forceBrowser  bool

	stats sessionStats
}

type sessionStats struct {
	HTTP     atomic.Int64 // duz HTTP ile edinilen oturum
	Browser  atomic.Int64 // tarayici ile edinilen oturum
	Refresh  atomic.Int64 // gecersiz sayilip yenilenen oturum
	Blocked  atomic.Int64 // engel (challenge/403/JSON olmayan yanit) sayisi
	Failures atomic.Int64 // oturum edinilemedi
}

func newSessionCache(cookie, token string, maxAge time.Duration) *sessionCache {
	if maxAge <= 0 {
		maxAge = defaultSessionAge
	}
	c := &sessionCache{maxAge: maxAge}
	if cookie != "" || token != "" {
		c.override = &session{Cookie: cookie, Token: token, At: time.Now(), Source: sourceEnv}
	}
	return c
}

// errBlocked: Cloudflare challenge / WAF engeli (duz HTTP ile asilamaz).
var errBlocked = errors.New("tatilsepeti: Cloudflare/WAF engeli")

var (
	reForgeryToken = regexp.MustCompile(`id="forgeryToken"[^>]*value="([^"]+)"`)
	reChallenge    = regexp.MustCompile(`(?i)just a moment|cf-browser-verification|challenge-platform/h/[a-z]/orchestrate|cf_chl_opt|attention required|access denied`)
)

// looksBlocked yanit/hata bir bot engeline mi isaret ediyor?
func looksBlocked(resp *httpx.Response, err error) bool {
	var se *httpx.StatusError
	if errors.As(err, &se) {
		if se.Status == 403 || se.Status == 429 || se.Status == 503 {
			return true
		}
		return reChallenge.MatchString(se.Body)
	}
	if resp == nil {
		return false
	}
	ct := strings.ToLower(resp.Header.Get("content-type"))
	if strings.Contains(ct, "text/html") && reChallenge.MatchString(string(resp.Body)) {
		return true
	}
	return false
}

// looksJSON govde JSON mu (BOM/bosluk toleransli)?
func looksJSON(body []byte) bool {
	s := strings.TrimLeft(strings.TrimPrefix(string(body), "\ufeff"), " \t\r\n")
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

// get mevcut (taze) oturumu doner; yoksa edinir. pageURL: cerez/token icin acilacak sayfa
// (otel detay sayfasi tercih edilir; bos ise ana sayfa).
func (c *sessionCache) get(ctx context.Context, pageURL string) (*session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cur != nil && !c.cur.expired(c.maxAge) {
		return c.cur, nil
	}
	if c.override != nil && !c.overrideDead {
		c.cur = c.override
		return c.cur, nil
	}
	s, err := c.acquire(ctx, pageURL)
	if err != nil {
		c.stats.Failures.Add(1)
		return nil, err
	}
	c.cur = s
	return s, nil
}

// store disaridan (orn. URL cozumleme sirasinda cekilen detay sayfasindan) elde edilen oturumu
// kaydeder; env override aktifken dokunulmaz.
func (c *sessionCache) store(s *session) {
	if s == nil || (s.Cookie == "" && s.Token == "") {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if (c.override != nil && !c.overrideDead) || c.forceBrowser {
		return
	}
	c.cur = s
	c.stats.HTTP.Add(1)
}

// invalidate s ile yapilan istek basarisiz oldugunda cagrilir; blocked true ise bir sonraki
// edinim (tarayici varsa) tarayici ile yapilir.
func (c *sessionCache) invalidate(s *session, blocked bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s != nil && s.Source == sourceEnv {
		c.overrideDead = true
		slog.Warn("tatilsepeti: TATILSEPETI_COOKIE/TOKEN ile istek basarisiz, otomatik oturuma geciliyor")
	}
	if c.cur == s {
		c.cur = nil
		c.stats.Refresh.Add(1)
	}
	if blocked {
		c.stats.Blocked.Add(1)
		if c.bm != nil {
			c.preferBrowser = true
		}
	}
}

func (c *sessionCache) acquire(ctx context.Context, pageURL string) (*session, error) {
	if pageURL == "" {
		pageURL = homeURL
	}
	useBrowser := c.bm != nil && (c.forceBrowser || c.preferBrowser)
	if !useBrowser {
		s, err := acquireHTTP(ctx, pageURL)
		if err == nil {
			c.stats.HTTP.Add(1)
			slog.Info("tatilsepeti: oturum duz HTTP ile alindi", "cookies", cookieNames(s.Cookie), "token", s.Token != "")
			return s, nil
		}
		if c.bm == nil || !errors.Is(err, errBlocked) {
			return nil, err
		}
		slog.Warn("tatilsepeti: duz HTTP engellendi, tarayici ile oturum aliniyor", "err", err)
		c.stats.Blocked.Add(1)
	}
	s, err := acquireBrowser(ctx, c.bm, pageURL)
	if err != nil {
		return nil, err
	}
	c.stats.Browser.Add(1)
	c.preferBrowser = false
	slog.Info("tatilsepeti: oturum tarayici ile alindi", "cookies", cookieNames(s.Cookie), "token", s.Token != "")
	return s, nil
}

// acquireHTTP detay sayfasini tarayici-gezinme basliklariyla GET eder; Set-Cookie + forgeryToken.
// Not: x-requested-with/origin gibi XHR basliklari gonderilirse site eksik (parca) HTML doner.
func acquireHTTP(ctx context.Context, pageURL string) (*session, error) {
	resp, err := httpx.Do(ctx, http.MethodGet, pageURL, httpx.Options{Headers: pageHeaders, Timeout: 25 * time.Second})
	if looksBlocked(resp, err) {
		return nil, fmt.Errorf("%w (GET %s: %v)", errBlocked, pageURL, err)
	}
	if err != nil {
		return nil, err
	}
	html := resp.Text()
	token := provider.FirstMatch(reForgeryToken, html)
	cookie := resp.SetCookies()
	if token == "" && cookie == "" {
		// Ne token ne cerez: sayfa beklenen sablon degil (buyuk ihtimalle ara sayfa/engel).
		return nil, fmt.Errorf("%w (sayfada forgeryToken/cerez yok)", errBlocked)
	}
	return &session{Cookie: cookie, Token: token, At: time.Now(), Source: sourceHTTP}, nil
}

// acquireBrowser sayfayi headless Chrome'da acar (Cloudflare challenge'i gecer), cerezleri ve
// #forgeryToken degerini okur, sekmeyi kapatir. Tarayici yalnizca bu kisa an icin kullanilir.
func acquireBrowser(ctx context.Context, bm *browser.Manager, pageURL string) (*session, error) {
	if bm == nil {
		return nil, errors.New("tatilsepeti: tarayici yok (browser.Manager verilmedi)")
	}
	page, cancel, err := bm.NewPage(ctx, 90*time.Second)
	if err != nil {
		return nil, fmt.Errorf("tatilsepeti: sekme acilamadi: %w", err)
	}
	defer cancel()

	if err := browser.Navigate(page, pageURL, 1500*time.Millisecond); err != nil {
		return nil, fmt.Errorf("tatilsepeti: sayfa acilamadi: %w", err)
	}
	var token string
	_ = browser.Eval(page, `(function(){var e=document.getElementById("forgeryToken"); return e ? (e.value||"") : ""})()`, &token)

	var cookies []*network.Cookie
	if err := chromedp.Run(page, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = network.GetCookies().Do(ctx)
		return err
	})); err != nil {
		return nil, fmt.Errorf("tatilsepeti: cerezler okunamadi: %w", err)
	}
	cookie := cookieHeader(cookies)
	if cookie == "" && token == "" {
		return nil, errors.New("tatilsepeti: tarayici oturumu bos (cerez/token okunamadi)")
	}
	return &session{Cookie: cookie, Token: token, At: time.Now(), Source: sourceBrowser}, nil
}

// cookieHeader tatilsepeti.com alanina ait cerezleri "a=1; b=2" formatina cevirir.
func cookieHeader(cookies []*network.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c == nil || !strings.Contains(strings.ToLower(c.Domain), "tatilsepeti.com") {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// cookieNames log icin yalnizca cerez adlari (degerler loglanmaz).
func cookieNames(cookie string) []string {
	if cookie == "" {
		return nil
	}
	out := []string{}
	for _, p := range strings.Split(cookie, ";") {
		if name, _, ok := strings.Cut(strings.TrimSpace(p), "="); ok {
			out = append(out, name)
		}
	}
	return out
}

// Stats oturum sayaclari (provider health icin).
func (c *sessionCache) Stats() map[string]int64 {
	c.mu.Lock()
	src := ""
	if c.cur != nil {
		src = string(c.cur.Source)
	}
	c.mu.Unlock()
	m := map[string]int64{
		"sessionHttp": c.stats.HTTP.Load(), "sessionBrowser": c.stats.Browser.Load(),
		"sessionRefresh": c.stats.Refresh.Load(), "blocked": c.stats.Blocked.Load(),
		"sessionFailures": c.stats.Failures.Load(),
	}
	switch src {
	case string(sourceEnv):
		m["sessionSource"] = 1
	case string(sourceHTTP):
		m["sessionSource"] = 2
	case string(sourceBrowser):
		m["sessionSource"] = 3
	}
	return m
}
