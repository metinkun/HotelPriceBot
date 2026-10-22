package gezinomi

// Oturum / istemci anahtari mekanigi (hibrit: tarayici yalnizca anahtar uretmek icin).
//
// apigezinomi.gezinomi.com her POST'ta iki basligi dogrular:
//   - x-session-id : sunucunun verdigi oturum kimligi. Ilk yanitin `x-session-id` basligindan alinir
//                    (cerez degil, sunucu tarafinda uretilir; gecersiz/eksikse yanitta yenisi doner).
//   - x-client-key : 64 hex. Sitenin Web Worker'i (sifreli WASM `/obfus.enc`) k32(sid, path, dakikaEpoch)
//                    ile uretir; sid + istek yolu ("Hotel/GetHotelRoomPrices") + dakikaya baglidir (±2-3 dk tolerans).
// Anahtar yanlis/eksikse API 200 ile `{"status":"Invalid","message":"Oda bulunamadı"}` doner (yaniltici).
// Anahtar tarayici disinda da gecerli (TLS/cerez bagi yok): bu yuzden kalici, hafif bir sekme
// (`/robots.txt`) acilir, sitenin kendi worker'i o sayfada baslatilir ve her dakika/yol icin anahtar
// `page.evaluate` ile alinir (~1 ms); fiyat istekleri duz HTTP (httpx) ile atilir.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"pricebot/internal/browser"
	"pricebot/internal/httpx"
)

const (
	keyPage      = site + "/robots.txt" // worker icin hafif, ayni origin'li sayfa
	tabMaxAge    = 6 * time.Hour        // sekme bu sureden sonra yenilenir (chunk hash degisimi, bellek)
	bootstrapURL = api + "/Common/GetGezinomiContactNumber"
	sidMinAge    = 2 * time.Minute // "Invalid" yanitta oturum bu yastan buyukse yeniden alinir
)

// jsSetup sayfada sitenin anahtar worker'ini bulur ve baslatir; window.__gz.key(sid, path) tanimlar.
// Worker chunk URL'i (hash'li) once ipucu ile denenir, olmazsa ana sayfa HTML'i + webpack runtime'dan cozulur.
const jsSetup = `async function (hint) {
  if (window.__gz && window.__gz.ready) return "ok";
  const text = async function (u) { const r = await fetch(u, { cache: "no-store" }); if (!r.ok) throw new Error("fetch " + r.status + " " + u); return await r.text(); };
  let workerUrl = hint || "";
  if (workerUrl) { try { const r = await fetch(workerUrl, { method: "HEAD", cache: "no-store" }); if (!r.ok) workerUrl = ""; } catch (e) { workerUrl = ""; } }
  if (!workerUrl) {
    const html = await text("/");
    const srcs = Array.from(new Set(Array.from(html.matchAll(/<script[^>]+src="([^"]*\/_next\/static\/chunks\/[^"]+\.js)"/g)).map(m => m[1])));
    const runtime = srcs.find(s => /\/webpack-[a-f0-9]+\.js/.test(s));
    if (!runtime) throw new Error("webpack runtime bulunamadi");
    const bodies = await Promise.all(srcs.map(s => text(s).catch(() => "")));
    let chunkId = "";
    for (const b of bodies) { const m = b.match(/new Worker\([a-zA-Z_$]+\.tu\(new URL\([a-zA-Z_$]+\.p\+[a-zA-Z_$]+\.u\((\d+)\)/); if (m) { chunkId = m[1]; break; } }
    if (!chunkId) throw new Error("worker chunk id bulunamadi");
    const rt = bodies[srcs.indexOf(runtime)];
    const hm = rt.match(new RegExp("[^0-9]" + chunkId + ":\"([a-f0-9]+)\""));
    if (!hm) throw new Error("worker chunk hash bulunamadi");
    workerUrl = location.origin + "/_next/static/chunks/" + chunkId + "." + hm[1] + ".js";
  }
  const w = new Worker(workerUrl);
  const pending = new Map();
  let seq = 0;
  w.addEventListener("message", e => { const p = pending.get(e.data.id); if (p) { pending.delete(e.data.id); e.data.ok ? p.resolve(e.data.data) : p.reject(new Error(e.data.error)); } });
  w.addEventListener("error", e => { for (const [k, p] of pending) { p.reject(new Error(e.message)); pending.delete(k); } });
  const call = msg => new Promise((res, rej) => { const i = ++seq; pending.set(i, { resolve: res, reject: rej }); w.postMessage({ ...msg, id: i }); });
  await call({ type: "init", baseUrl: location.origin });
  window.__gz = { ready: true, workerUrl, key: (sid, path) => call({ type: "k32Hex", sid, path }) };
  return "ok";
}`

type keyEntry struct {
	minute int64
	sid    string
	key    string
}

// signer kalici sekme + worker ile x-client-key uretir, x-session-id'yi saklar.
type signer struct {
	bm *browser.Manager

	mu        sync.Mutex // sekme ve anahtar onbellegi
	ctx       context.Context
	cancel    context.CancelFunc
	openedAt  time.Time
	workerURL string
	keys      map[string]keyEntry // path -> son anahtar

	sidMu sync.Mutex
	sid   string
	sidAt time.Time
}

func newSigner(bm *browser.Manager) *signer {
	return &signer{bm: bm, keys: map[string]keyEntry{}}
}

func jsStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// evalAwait promise donduren JS ifadesini calistirip sonucunu bekler.
func evalAwait(ctx context.Context, js string, out any) error {
	return chromedp.Run(ctx, chromedp.Evaluate(js, out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true).WithReturnByValue(true)
	}))
}

func (s *signer) closeTabLocked() {
	if s.cancel != nil {
		s.cancel()
	}
	s.ctx, s.cancel = nil, nil
	s.keys = map[string]keyEntry{}
}

// ensureTabLocked sekmenin canli ve worker'in hazir olmasini saglar (mu tutulmus olmali).
func (s *signer) ensureTabLocked() error {
	if s.ctx != nil && s.ctx.Err() == nil && time.Since(s.openedAt) < tabMaxAge {
		return nil
	}
	s.closeTabLocked()
	if s.bm == nil {
		return fmt.Errorf("Gezinomi icin tarayici yoneticisi yok (istemci anahtari uretilemez)")
	}
	ctx, cancel, err := s.bm.NewPersistentPage(12 * time.Hour)
	if err != nil {
		return fmt.Errorf("Gezinomi anahtar sekmesi acilamadi: %w", err)
	}
	op, done := context.WithTimeout(ctx, 60*time.Second)
	defer done()
	if err := browser.Navigate(op, keyPage, 0); err != nil {
		cancel()
		return fmt.Errorf("Gezinomi sayfasi acilamadi: %w", err)
	}
	var res string
	if err := evalAwait(op, fmt.Sprintf("(%s)(%s)", jsSetup, jsStr(s.workerURL)), &res); err != nil || res != "ok" {
		cancel()
		return fmt.Errorf("Gezinomi anahtar worker'i kurulamadi: %v", err)
	}
	var wu string
	if err := browser.Eval(op, `window.__gz.workerUrl`, &wu); err == nil && wu != "" {
		s.workerURL = wu
	}
	s.ctx, s.cancel, s.openedAt = ctx, cancel, time.Now()
	slog.Info("gezinomi: anahtar sekmesi hazir", "worker", s.workerURL)
	return nil
}

// clientKey sid + path icin gecerli dakikanin x-client-key degerini doner (dakika basina bir hesap).
func (s *signer) clientKey(sid, path string) (string, error) {
	minute := time.Now().Unix() / 60
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.keys[path]; ok && e.minute == minute && e.sid == sid {
		return e.key, nil
	}
	js := fmt.Sprintf(`window.__gz.key(%s, %s)`, jsStr(sid), jsStr(path))
	var key string
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if err = s.ensureTabLocked(); err != nil {
			return "", err
		}
		op, done := context.WithTimeout(s.ctx, 15*time.Second)
		err = evalAwait(op, js, &key)
		done()
		if err == nil && len(key) == 64 {
			s.keys[path] = keyEntry{minute: minute, sid: sid, key: key}
			return key, nil
		}
		s.closeTabLocked() // sekme bozuk; bir kez yeniden kur
	}
	return "", fmt.Errorf("Gezinomi istemci anahtari uretilemedi: %v", err)
}

func (s *signer) session() (string, time.Time) {
	s.sidMu.Lock()
	defer s.sidMu.Unlock()
	return s.sid, s.sidAt
}

func (s *signer) setSession(sid string) {
	s.sidMu.Lock()
	defer s.sidMu.Unlock()
	if sid != "" && sid != s.sid {
		s.sid, s.sidAt = sid, time.Now()
	}
}

// bootstrap anahtarsiz ucuz bir cagri ile sunucudan yeni oturum kimligi alir.
func (s *signer) bootstrap(ctx context.Context) (string, error) {
	resp, err := httpx.Post(ctx, bootstrapURL, []byte(`{"contactTypeId":1}`), apiHeaders, 20*time.Second)
	if err != nil {
		return "", err
	}
	sid := resp.Header.Get("X-Session-Id")
	if sid == "" {
		return "", fmt.Errorf("Gezinomi oturum kimligi (x-session-id) alinamadi")
	}
	s.setSession(sid)
	return sid, nil
}

// apiEnvelope yalnizca durum alanlari (gecerlilik kontrolu icin).
type apiEnvelope struct {
	Data    json.RawMessage `json:"data"`
	Status  string          `json:"status"`
	Message string          `json:"message"`
}

// isInvalid yanit "Invalid"/veri yok mu? (anahtar hatasi ile gercek "oda yok" ayni gorunur)
func isInvalid(body []byte) bool {
	var e apiEnvelope
	if err := json.Unmarshal(body, &e); err != nil {
		return true
	}
	return e.Status != "Success" || len(e.Data) == 0 || string(e.Data) == "null"
}

// post API cagrisi: oturum + anahtar basliklarini ekler. Yanit "Invalid" ise ve sunucu yeni oturum
// verdiyse (veya oturum eskiyse yeniden alinir) bir kez tekrar dener.
func (s *signer) post(ctx context.Context, path string, body []byte, timeout time.Duration) (*httpx.Response, error) {
	sid, sidAt := s.session()
	if sid == "" {
		var err error
		if sid, err = s.bootstrap(ctx); err != nil {
			return nil, err
		}
		sidAt = time.Now()
	}
	var resp *httpx.Response
	for attempt := 0; attempt < 2; attempt++ {
		key, err := s.clientKey(sid, path)
		if err != nil {
			return nil, err
		}
		hdr := httpx.Merge(apiHeaders, map[string]string{"x-session-id": sid, "x-client-key": key})
		resp, err = httpx.Post(ctx, api+"/"+path, body, hdr, timeout)
		if err != nil {
			return nil, err
		}
		newSid := resp.Header.Get("X-Session-Id")
		s.setSession(newSid)
		if attempt > 0 || !isInvalid(resp.Body) {
			return resp, nil
		}
		// Invalid: oturum degistiyse yenisiyle, eskiyse yeniden alip tekrar dene
		switch {
		case newSid != "" && newSid != sid:
			sid = newSid
		case time.Since(sidAt) > sidMinAge:
			if sid, err = s.bootstrap(ctx); err != nil {
				return nil, err
			}
		default:
			return resp, nil
		}
		slog.Debug("gezinomi: oturum yenilendi, tekrar deneniyor", "path", path)
	}
	return resp, nil
}

// Stats saglik bilgisi (sekme acik mi, oturum var mi).
func (s *signer) Stats() map[string]any {
	s.mu.Lock()
	alive := s.ctx != nil && s.ctx.Err() == nil
	wu := s.workerURL
	s.mu.Unlock()
	sid, at := s.session()
	return map[string]any{"tabAlive": alive, "workerUrl": wu, "hasSession": sid != "", "sessionAgeSec": int(time.Since(at).Seconds())}
}
