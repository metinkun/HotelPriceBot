package main

// Arastirma araci: bir sayfayi acar ve verilen JS dosyasini sayfa baglaminda (async destekli) calistirir,
// sonucu JSON olarak yazar. Site JS'ini incelemek ve ayni oturum/TLS ile fetch denemek icin.
// Kullanim: capture js <url> <script.js> <out.json>

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"pricebot/internal/browser"
	"pricebot/internal/config"
)

func runJS() {
	if len(os.Args) < 5 {
		fmt.Println("kullanim: capture js <url> <script.js> <out.json>")
		os.Exit(2)
	}
	u, scriptPath, out := os.Args[2], os.Args[3], os.Args[4]
	js, err := os.ReadFile(scriptPath)
	must(err)

	cfg := config.Load()
	bm := browser.New(cfg)
	defer bm.Close()

	page, cancel, err := bm.NewPage(context.Background(), 5*time.Minute)
	must(err)
	defer cancel()

	must(browser.Navigate(page, u, 3*time.Second))
	_ = browser.Eval(page, `(function(){var b=Array.from(document.querySelectorAll("button,a")).find(function(x){return /tümünü kabul|kabul et/i.test(x.textContent||"")}); if(b) b.click(); return true})()`, nil)

	var res json.RawMessage
	err = chromedp.Run(page, chromedp.Evaluate(string(js), &res, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true).WithReturnByValue(true)
	}))
	if err != nil {
		fmt.Fprintln(os.Stderr, "JS hatasi:", err)
		res = json.RawMessage(fmt.Sprintf("%q", err.Error()))
	}
	must(os.WriteFile(out, res, 0o644))
	fmt.Printf("sonuc %d bayt -> %s\n", len(res), out)
	if len(res) < 4000 {
		fmt.Println(string(res))
	}
}
