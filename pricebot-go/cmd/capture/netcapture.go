package main

// Faz 3 arastirma araci: headless Chrome ile bir otel sayfasini acar, tarih doldurup
// "Oda Ara" (veya verilen tiklama metnini) tetikler ve bu sirada yapilan XHR/fetch
// isteklerini (URL, method, govde, yanit durumu, set-cookie/cf isaretleri) JSON olarak kaydeder.
// Amac: Touristica'nin fiyatlari hangi arka plan istegiyle cektigini bulup tarayicisiz
// (cf_clearance + TLS taklidi) tekrar oynatilabilirligini degerlendirmek; Etstur paket
// API'sinin yeni istek formatini gormek.
//
// Kullanim: capture net <url> <checkIn yyyy-MM-dd> <checkOut> <outFile.json> [tiklaMetni]

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"pricebot/internal/browser"
	"pricebot/internal/config"
)

type capturedReq struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers,omitempty"`
	PostData string            `json:"postData,omitempty"`
	Status   int64             `json:"status,omitempty"`
	MimeType string            `json:"mimeType,omitempty"`
	RespHdr  map[string]string `json:"respHeaders,omitempty"`
	Phase    string            `json:"phase"` // load | afterSearch | afterClick
	At       time.Time         `json:"at"`
}

func captureNet() {
	if len(os.Args) < 6 {
		fmt.Println("kullanim: capture net <url> <checkIn> <checkOut> <out.json> [tiklaMetni]")
		os.Exit(2)
	}
	u, ci, co, out := os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	clickText := ""
	if len(os.Args) > 6 {
		clickText = os.Args[6]
	}

	cfg := config.Load()
	bm := browser.New(cfg)
	defer bm.Close()

	page, cancel, err := bm.NewPage(context.Background(), 4*time.Minute)
	must(err)
	defer cancel()

	var mu sync.Mutex
	reqs := map[string]*capturedReq{}
	order := []string{}
	phase := "load"

	interesting := func(t network.ResourceType) bool {
		return t == network.ResourceTypeXHR || t == network.ResourceTypeFetch || t == network.ResourceTypeDocument
	}

	chromedp.ListenTarget(page, func(ev any) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if !interesting(e.Type) {
				return
			}
			mu.Lock()
			r := &capturedReq{ID: string(e.RequestID), Type: string(e.Type), Method: e.Request.Method, URL: e.Request.URL, Phase: phase, At: time.Now(), Headers: map[string]string{}}
			for k, v := range e.Request.Headers {
				r.Headers[k] = fmt.Sprint(v)
			}
			if e.Request.HasPostData && len(e.Request.PostDataEntries) > 0 {
				for _, pe := range e.Request.PostDataEntries {
					r.PostData += pe.Bytes
				}
			}
			reqs[r.ID] = r
			order = append(order, r.ID)
			mu.Unlock()
		case *network.EventResponseReceived:
			mu.Lock()
			if r, ok := reqs[string(e.RequestID)]; ok {
				r.Status = e.Response.Status
				r.MimeType = e.Response.MimeType
				r.RespHdr = map[string]string{}
				for k, v := range e.Response.Headers {
					lk := strings.ToLower(k)
					if lk == "set-cookie" || strings.HasPrefix(lk, "cf-") || lk == "server" || lk == "content-type" {
						r.RespHdr[k] = fmt.Sprint(v)
					}
				}
			}
			mu.Unlock()
		}
	})

	must(chromedp.Run(page, network.Enable()))
	must(browser.Navigate(page, u, 3*time.Second))

	// Touristica tarzi: cerez banner'i + tarih doldur + Oda Ara
	_ = browser.Eval(page, `(function(){var b=Array.from(document.querySelectorAll("button,a")).find(function(x){return /tümünü kabul|kabul et/i.test(x.textContent||"")}); if(b) b.click(); return true})()`, nil)
	_ = browser.Sleep(page, time.Second)
	mu.Lock()
	phase = "afterSearch"
	mu.Unlock()
	p := strings.Split(ci, "-")
	q := strings.Split(co, "-")
	fill := fmt.Sprintf(`(function(){
	  var set=function(sel,val){var el=document.querySelector(sel); if(el){el.value=val; el.setAttribute("data-value",val); el.dispatchEvent(new Event("change",{bubbles:true}));}};
	  set("#txtCheckInDate", %q); set("#txtCheckOutDate", %q);
	  var btn=Array.from(document.querySelectorAll("button,a,input")).find(function(b){return /oda ara/i.test(b.textContent||b.value||"")});
	  if(btn) btn.click(); return !!btn; })()`, p[2]+"."+p[1]+"."+p[0], q[2]+"."+q[1]+"."+q[0])
	var clicked bool
	_ = browser.Eval(page, fill, &clicked)
	fmt.Println("arama tetiklendi:", clicked)
	_ = browser.Sleep(page, 15*time.Second)

	if clickText != "" {
		mu.Lock()
		phase = "afterClick"
		mu.Unlock()
		js := fmt.Sprintf(`(function(){var re=new RegExp(%q,"i"); var el=Array.from(document.querySelectorAll("a,button,li,span,div")).find(function(e){var t=(e.textContent||"").trim(); return re.test(t)&&t.length<60}); if(el){el.click(); return true} return false})()`, clickText)
		var ok bool
		_ = browser.Eval(page, js, &ok)
		fmt.Println("tiklama:", clickText, ok)
		_ = browser.Sleep(page, 15*time.Second)
	}

	// Sayfa cerezleri (cf_clearance var mi?)
	var cookies []*network.Cookie
	_ = chromedp.Run(page, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = network.GetCookies().Do(ctx)
		return err
	}))
	cookieNames := []string{}
	for _, c := range cookies {
		cookieNames = append(cookieNames, c.Name)
	}

	mu.Lock()
	list := make([]*capturedReq, 0, len(order))
	for _, id := range order {
		list = append(list, reqs[id])
	}
	mu.Unlock()

	report := map[string]any{"url": u, "checkIn": ci, "checkOut": co, "cookies": cookieNames, "requests": list}
	b, _ := json.MarshalIndent(report, "", " ")
	must(os.WriteFile(out, b, 0o644))
	fmt.Printf("%d istek kaydedildi -> %s\n", len(list), out)
	for _, r := range list {
		if r.Type == "Document" {
			continue
		}
		fmt.Printf("[%s] %s %s -> %d %s (body %d b)\n", r.Phase, r.Method, trunc(r.URL, 110), r.Status, r.MimeType, len(r.PostData))
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
