// Package browser chromedp ile paylasilan headless Chrome yonetimi (Puppeteer manager karsiligi).
package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"

	"pricebot/internal/config"
)

const UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

// Sistem Chrome/Chromium yollari (chromedp kendi tarayici indirmez).
var fallbackPaths = []string{
	"/usr/bin/chromium",
	"/usr/bin/chromium-browser",
	"/usr/bin/google-chrome",
	"/usr/bin/google-chrome-stable",
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
}

type Manager struct {
	cfg *config.Config

	tabs      chan struct{} // eszamanli sekme siniri
	pageCount int           // yeniden baslatma sayaci

	mu          sync.Mutex
	allocCtx    context.Context
	allocCancel context.CancelFunc
	browserCtx  context.Context
	browserStop context.CancelFunc
}

func New(cfg *config.Config) *Manager {
	n := cfg.BrowserMaxTabs
	if n <= 0 {
		n = 2
	}
	return &Manager{cfg: cfg, tabs: make(chan struct{}, n)}
}

func (m *Manager) execPath() (string, error) {
	if m.cfg.BrowserExecutablePath != "" {
		return m.cfg.BrowserExecutablePath, nil
	}
	for _, p := range fallbackPaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("Chrome/Chromium bulunamadi; BROWSER_EXECUTABLE_PATH ayarlayin")
}

// browser lazily baslatilan paylasilan tarayici context'ini doner.
func (m *Manager) browser() (context.Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.browserCtx != nil && m.browserCtx.Err() == nil {
		return m.browserCtx, nil
	}
	m.closeLocked()

	path, err := m.execPath()
	if err != nil {
		return nil, err
	}

	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.ExecPath(path),
		chromedp.NoSandbox,
		chromedp.UserAgent(UA),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("lang", "tr-TR"),
		chromedp.WindowSize(1440, 1000),
	)
	if m.cfg.BrowserHeadless {
		opts = append(opts, chromedp.Flag("headless", "new"))
	} else {
		opts = append(opts, chromedp.Flag("headless", false))
	}

	m.allocCtx, m.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	m.browserCtx, m.browserStop = chromedp.NewContext(m.allocCtx)

	// Tarayiciyi hemen baslat. DIKKAT: ilk Run tarayici baglantisini bu context'e
	// baglar; bu yuzden turev (timeout'lu) bir context DEGIL, browserCtx'in kendisi kullanilir.
	if err := chromedp.Run(m.browserCtx); err != nil {
		m.closeLocked()
		return nil, fmt.Errorf("tarayici baslatilamadi: %w", err)
	}
	return m.browserCtx, nil
}

func (m *Manager) closeLocked() {
	if m.browserStop != nil {
		m.browserStop()
	}
	if m.allocCancel != nil {
		m.allocCancel()
	}
	m.browserCtx, m.browserStop, m.allocCtx, m.allocCancel = nil, nil, nil, nil
}

// Close tarayiciyi kapatir.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeLocked()
}

// NewPage yeni sekme (tab) context'i acar; viewport ve UA ayarlanir. cancel sekmeyi kapatir.
// Tarayici baglantisi kopmussa bir kez yeniden baslatip tekrar dener.
func (m *Manager) NewPage(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	// Sekme slotu bekle (bellek korumasi)
	select {
	case m.tabs <- struct{}{}:
	case <-parent.Done():
		return nil, nil, parent.Err()
	}
	release := func() { <-m.tabs }

	// Belli sayida sayfadan sonra tarayiciyi tazele (bellek sizintilarina karsi)
	m.mu.Lock()
	m.pageCount++
	restart := m.cfg.BrowserRestartAfter > 0 && m.pageCount >= m.cfg.BrowserRestartAfter && len(m.tabs) == 1
	if restart {
		m.pageCount = 0
		m.closeLocked()
	}
	m.mu.Unlock()

	ctx, cancel, err := m.newPage(parent, timeout)
	if err != nil {
		m.Close()
		ctx, cancel, err = m.newPage(parent, timeout)
	}
	if err != nil {
		release()
		return nil, nil, err
	}
	return ctx, func() { cancel(); release() }, nil
}

func (m *Manager) newPage(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	b, err := m.browser()
	if err != nil {
		return nil, nil, err
	}
	tabCtx, cancelTab := chromedp.NewContext(b)
	ctx, cancelTimeout := context.WithTimeout(tabCtx, timeout)

	// Ust istek iptal edilirse sekmeyi de kapat
	stop := context.AfterFunc(parent, func() { cancelTab() })

	cancel := func() {
		stop()
		cancelTimeout()
		cancelTab()
	}

	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1440, 1000),
		emulation.SetUserAgentOverride(UA).WithAcceptLanguage("tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7"),
	); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("sekme acilamadi: %w", err)
	}
	return ctx, cancel, nil
}

// Navigate sayfaya gider ve Cloudflare "Just a moment" ekrani gecinceye kadar bekler.
func Navigate(ctx context.Context, url string, settle time.Duration) error {
	if err := chromedp.Run(ctx, chromedp.Navigate(url)); err != nil {
		return err
	}
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		var state struct {
			Ready bool   `json:"ready"`
			Title string `json:"title"`
		}
		err := chromedp.Run(ctx, chromedp.Evaluate(`({ready: document.readyState === "complete" && !!document.body, title: document.title || ""})`, &state))
		if err == nil && state.Ready && !isChallenge(state.Title) {
			break
		}
		if err := sleep(ctx, 500*time.Millisecond); err != nil {
			return err
		}
	}
	return sleep(ctx, settle)
}

func isChallenge(title string) bool {
	t := strings.ToLower(title)
	return strings.Contains(t, "just a moment") || strings.Contains(t, "bir dakika") || strings.Contains(t, "attention required")
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// Sleep context'e duyarli bekleme (disa acik).
func Sleep(ctx context.Context, d time.Duration) error { return sleep(ctx, d) }

// Eval JS ifadesini calistirir; out nil ise sonuc yoksayilir.
func Eval(ctx context.Context, js string, out any) error {
	return chromedp.Run(ctx, chromedp.Evaluate(js, out))
}
