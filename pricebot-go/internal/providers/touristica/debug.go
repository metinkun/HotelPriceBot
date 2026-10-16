package touristica

import (
	"context"
	"fmt"
	"time"

	"pricebot/internal/browser"
)

// DebugPackageFlow gelistirme araci: gercek havuz akisiyla arama + paket sekmesi adimlarini
// calistirir ve her adimda sayfa durumunu kaydeder (cmd/capture touristica-debug).
func (p *Provider) DebugPackageFlow(ctx context.Context, detailURL, checkIn, checkOut string) []string {
	var log []string
	add := func(f string, a ...any) { log = append(log, fmt.Sprintf(f, a...)) }

	snap := func(page context.Context, label string) {
		var s struct {
			URL    string   `json:"url"`
			Title  string   `json:"title"`
			Hooked bool     `json:"hooked"`
			Odalar int      `json:"odalar"`
			Pkg    int      `json:"pkg"`
			Last   string   `json:"last"`
			Has    bool     `json:"has"`
			Fresh  bool     `json:"fresh"`
			Stale  int      `json:"stale"`
			Cards  int      `json:"cards"`
			Tabs   []string `json:"tabs"`
		}
		err := browser.Eval(page, `({
  url: location.href, title: document.title,
  hooked: !!(window.__pb && window.__pb.hooked), odalar: (window.__pb&&window.__pb.odalar)||0, pkg: (window.__pb&&window.__pb.pkg)||0, last: (window.__pb&&window.__pb.last)||"",
  has: `+jsHasPackages+`, fresh: `+jsFreshPackagesReady+`,
  stale: document.querySelectorAll("[data-pb-stale]").length,
  cards: Array.from(document.querySelectorAll("*")).filter(function(el){ var t=(el.innerText||"").replace(/\s+/g," ").trim(); return t.length<=400 && /Otel \+ Uçak \+ Transfer Dahil\s*[\d.,]+\s*TL/i.test(t) && !el.hasAttribute("data-pb-stale"); }).length,
  tabs: Array.from(document.querySelectorAll("a,button,li")).filter(function(e){var t=(e.textContent||"").trim(); return /otel\s*\+\s*uçak/i.test(t)&&t.length<60}).map(function(e){return e.tagName+"."+e.className+"|"+(e.getAttribute("href")||"")+"|"+(e.textContent||"").trim().slice(0,30)})
})`, &s)
		if err != nil {
			add("%s: EVAL HATA %v", label, err)
			return
		}
		add("%s: url=%s title=%q hooked=%v odalar=%d pkg=%d last=%s has=%v fresh=%v stale=%d cards=%d tabs=%v", label, s.URL, s.Title, s.Hooked, s.Odalar, s.Pkg, s.Last, s.Has, s.Fresh, s.Stale, s.Cards, s.Tabs)
	}

	err := p.withTab(ctx, detailURL, func(page context.Context, t *tab) error {
		snap(page, "ensure sonrasi")
		if err := searchOnTab(page, t.hooked, checkIn, checkOut, 2, 0); err != nil {
			add("search HATA: %v", err)
			return err
		}
		snap(page, "arama sonrasi")
		before := counter(page, "pkg")
		found, nav := waitForPackages(page, t.hooked, before, 30*time.Second)
		add("waitForPackages: found=%v navigated=%v", found, nav)
		snap(page, "paket bekleme sonrasi")
		return nil
	})
	if err != nil {
		add("withTab HATA: %v", err)
	}
	return log
}
