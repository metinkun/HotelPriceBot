package touristica

// Kalici sekme havuzu (Faz 3 / adim 1).
//
// Eski akis her sorguda yeni sekme acip Cloudflare challenge'i gecer, sayfayi render eder ve
// fiyat metni belirene kadar 1 sn'lik adimlarla bekler (~15 sn, ~200 MB).
// Yeni akis: az sayida sekme surekli acik tutulur. Ayni otel sayfasi acik ise navigasyon yapilmaz;
// sadece tarih/kisi doldurulup "Oda Ara" tetiklenir ve sayfanin kendi jQuery ajaxSuccess olaylari
// uzerinden `/odalar` (ve paket icin `/ucak-ve-odalar`) yanitinin gelmesi beklenir. Oturum/cerezler
// (cf_clearance, ASP.NET_SessionId) sekmede yasar; belli bir yastan sonra sayfa yenilenir.

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"pricebot/internal/browser"
)

type tab struct {
	ctx      context.Context
	cancel   context.CancelFunc
	url      string
	openedAt time.Time
	uses     int
	hooked   bool // jQuery ajaxSuccess kancasi kuruldu (sayac tabanli bekleme mumkun)
}

func (t *tab) alive() bool { return t != nil && t.ctx != nil && t.ctx.Err() == nil }

func (t *tab) close() {
	if t != nil && t.cancel != nil {
		t.cancel()
	}
	if t != nil {
		t.ctx, t.cancel, t.url, t.uses, t.hooked = nil, nil, "", 0, false
	}
}

type pool struct {
	bm     *browser.Manager
	slots  chan *tab
	maxAge time.Duration

	mu    sync.Mutex
	stats poolStats
}

type poolStats struct {
	Reused    atomic.Int64
	Navigated atomic.Int64
	Recreated atomic.Int64
	Failures  atomic.Int64
}

func newPool(bm *browser.Manager, size int, maxAge time.Duration) *pool {
	if size <= 0 {
		size = 1
	}
	if maxAge <= 0 {
		maxAge = 20 * time.Minute
	}
	p := &pool{bm: bm, slots: make(chan *tab, size), maxAge: maxAge}
	for i := 0; i < size; i++ {
		p.slots <- &tab{}
	}
	return p
}

// acquire bos bir sekme slotu alir (ctx iptal olursa hata).
func (p *pool) acquire(ctx context.Context) (*tab, error) {
	select {
	case t := <-p.slots:
		return t, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// release slotu geri verir; broken ise sekme kapatilir (bir sonraki kullanimda yeniden acilir).
func (p *pool) release(t *tab, broken bool) {
	if broken {
		p.stats.Failures.Add(1)
		t.close()
	}
	p.slots <- t
}

// Sayfaya kurulan jQuery kancasi: ajaxSuccess ile hangi uc ne zaman tamamlandi sayilir.
const jsInstallHook = `(function(){
  if (!window.jQuery) return "nojq";
  if (window.__pb && window.__pb.hooked) return "ok";
  window.__pb = { hooked: true, seq: 0, odalar: 0, pkg: 0, last: "" };
  jQuery(document).ajaxSuccess(function(e, xhr, s){
    var u = (s && s.url) || "";
    window.__pb.seq++; window.__pb.last = u;
    if (/\/odalar(\?|$)/i.test(u)) window.__pb.odalar = window.__pb.seq;
    if (/ucak-ve-odalar/i.test(u)) window.__pb.pkg = window.__pb.seq;
  });
  return "installed";
})()`

// maxTabUses: bu kadar sorgudan sonra sekme kapatilip yeniden acilir (Chromium bellek birikimine karsi;
// 2 GB sunucuda olculen: ~6 sorgu sonrasi konteyner ~1 GB). Yeni sekme acmak ~1 sn ek maliyettir.
const maxTabUses = 10

// ensure sekmenin canli, dogru URL'de ve taze olmasini saglar; gerekirse acar/yeniler.
func (p *pool) ensure(t *tab, detailURL string) error {
	if t.alive() && t.uses >= maxTabUses {
		t.close()
	}
	needNav := !t.alive() || t.url != detailURL || time.Since(t.openedAt) > p.maxAge
	if !t.alive() {
		t.close()
		ctx, cancel, err := p.bm.NewPersistentPage(12 * time.Hour)
		if err != nil {
			return fmt.Errorf("kalici sekme acilamadi: %w", err)
		}
		t.ctx, t.cancel = ctx, cancel
		p.stats.Recreated.Add(1)
	}
	if needNav {
		opCtx, cancel := context.WithTimeout(t.ctx, 90*time.Second)
		defer cancel()
		if err := browser.Navigate(opCtx, detailURL, 800*time.Millisecond); err != nil {
			return fmt.Errorf("Touristica sayfasi acilamadi: %w", err)
		}
		_ = browser.Eval(opCtx, jsAcceptCookies, nil)
		t.url, t.openedAt, t.uses = detailURL, time.Now(), 0
		p.stats.Navigated.Add(1)
	} else {
		p.stats.Reused.Add(1)
	}
	t.uses++
	var hook string
	opCtx, cancel := context.WithTimeout(t.ctx, 10*time.Second)
	defer cancel()
	if err := browser.Eval(opCtx, jsInstallHook, &hook); err != nil {
		return fmt.Errorf("ajax kancasi kurulamadi: %w", err)
	}
	t.hooked = hook != "nojq"
	if !t.hooked {
		slog.Warn("touristica: sayfada jQuery yok, metin tabanli bekleme kullanilacak")
	}
	return nil
}

// Onceki aramadan kalan fiyat bloklarini isaretler; okuma sirasinda bunlar atlanir.
// Site oda listesini yeniden render ettiginde yeni dugumler isaretsiz gelir.
const jsMarkStale = `(function(){
  var re = /Gecelik Toplam Fiyat|Otel \+ Uçak \+ Transfer Dahil/i;
  var cands = Array.from(document.querySelectorAll("*")).filter(function(el){
    var t = (el.innerText || "").replace(/\s+/g, " ").trim();
    return t.length <= 400 && re.test(t);
  });
  // Yalnizca EN ICTEKI eslesen elemanlari isaretle: ust kapsayicilar yeniden render edildiginde
  // yeni kartlar isaretsiz kalsin (kapsayici isaretlenirse yeni icerik de bayat sayilirdi).
  var n = 0;
  cands.forEach(function(el){
    var inner = cands.some(function(o){ return o !== el && el.contains(o); });
    if (!inner) { el.setAttribute("data-pb-stale", "1"); n++; }
  });
  return n;
})()`

// counter __pb sayaclarini okur.
func counter(ctx context.Context, field string) int {
	var n int
	_ = browser.Eval(ctx, fmt.Sprintf(`(window.__pb && window.__pb.%s) || 0`, field), &n)
	return n
}

// waitCounter sayac artana kadar bekler; fallbackJS true dondugunde de biter.
func waitCounter(ctx context.Context, field string, before int, fallbackJS string, max time.Duration) bool {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if counter(ctx, field) > before {
			return true
		}
		if fallbackJS != "" {
			var ok bool
			if err := browser.Eval(ctx, fallbackJS, &ok); err == nil && ok {
				return true
			}
		}
		if err := browser.Sleep(ctx, 250*time.Millisecond); err != nil {
			return false
		}
	}
	return false
}

// Stats havuz istatistikleri (provider health icin).
func (p *pool) Stats() map[string]int64 {
	return map[string]int64{
		"reused": p.stats.Reused.Load(), "navigated": p.stats.Navigated.Load(),
		"recreated": p.stats.Recreated.Load(), "failures": p.stats.Failures.Load(),
	}
}
