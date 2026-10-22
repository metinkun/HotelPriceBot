package tatilsepeti

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"

	"pricebot/internal/httpx"
)

func TestParseDetailPageAndToken(t *testing.T) {
	b, err := os.ReadFile("testdata/detail_snippet.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	r, err := parseDetailPage(html)
	if err != nil {
		t.Fatal(err)
	}
	if r.ProviderHotelID != "7741" || r.DestinationCode != "ecnc" || r.HotelName != "Kaya Palazzo Resort" {
		t.Fatalf("beklenmeyen cozum: %+v", r)
	}
	if tok := reForgeryTokenMatch(html); tok != "TOKEN-AAAA,TOKEN-BBBB" {
		t.Fatalf("forgeryToken yanlis: %q", tok)
	}
	if _, err := parseDetailPage("<html><body>parca</body></html>"); err == nil {
		t.Error("hidHotelId olmayan sayfa hata vermeli")
	}
}

func TestLooksBlockedAndJSON(t *testing.T) {
	if !looksBlocked(nil, &httpx.StatusError{Status: 403}) || !looksBlocked(nil, &httpx.StatusError{Status: 503}) {
		t.Error("403/503 engel sayilmali")
	}
	if looksBlocked(nil, &httpx.StatusError{Status: 500, Body: "Internal Server Error"}) {
		t.Error("500 tek basina engel degil")
	}
	if !looksBlocked(nil, &httpx.StatusError{Status: 400, Body: "<title>Just a moment...</title>"}) {
		t.Error("challenge govdesi engel sayilmali")
	}
	h := http.Header{}
	h.Set("content-type", "text/html; charset=utf-8")
	if !looksBlocked(&httpx.Response{Status: 200, Header: h, Body: []byte("<html><title>Just a moment...</title></html>")}, nil) {
		t.Error("200 + challenge HTML engel sayilmali")
	}
	if looksBlocked(&httpx.Response{Status: 200, Header: h, Body: []byte("<html>normal</html>")}, nil) {
		t.Error("normal HTML engel degil")
	}
	if !looksJSON([]byte("\ufeff [{\"a\":1}]")) || !looksJSON([]byte("{}")) || looksJSON([]byte("<html>")) {
		t.Error("looksJSON hatali")
	}
}

func TestCookieHeaderAndNames(t *testing.T) {
	cs := []*network.Cookie{
		{Name: "ASP.NET_SessionId", Value: "abc", Domain: "www.tatilsepeti.com"},
		{Name: "cf_clearance", Value: "xyz", Domain: ".tatilsepeti.com"},
		{Name: "_ga", Value: "1", Domain: ".google.com"},
		nil,
	}
	got := cookieHeader(cs)
	if got != "ASP.NET_SessionId=abc; cf_clearance=xyz" {
		t.Fatalf("beklenmeyen cookie: %q", got)
	}
	names := cookieNames(got)
	if len(names) != 2 || names[0] != "ASP.NET_SessionId" || names[1] != "cf_clearance" {
		t.Fatalf("cerez adlari: %v", names)
	}
}

func TestSessionCacheLifecycle(t *testing.T) {
	// env override once kullanilir, basarisiz olunca otomatik edinime gecilir
	c := newSessionCache("A=1", "tok", time.Minute)
	if c.override == nil || c.override.Source != sourceEnv {
		t.Fatal("override bekleniyordu")
	}
	c.cur = c.override
	c.invalidate(c.override, false)
	if !c.overrideDead || c.cur != nil {
		t.Fatal("override gecersiz sayilmaliydi")
	}
	// suresi dolan oturum yenilenir, env oturumu dolmaz
	old := &session{Cookie: "B=2", At: time.Now().Add(-2 * time.Minute), Source: sourceHTTP}
	if !old.expired(time.Minute) {
		t.Error("eski oturum suresi dolmus olmali")
	}
	if (&session{Source: sourceEnv, At: time.Now().Add(-time.Hour)}).expired(time.Minute) {
		t.Error("env oturumu suresiz")
	}
	// store: override aktifken yazmaz
	c2 := newSessionCache("A=1", "", time.Minute)
	c2.store(&session{Cookie: "C=3", Source: sourceHTTP, At: time.Now()})
	if c2.cur != nil {
		t.Error("override aktifken store yazmamali")
	}
	c3 := newSessionCache("", "", 0)
	c3.store(&session{Cookie: "C=3", Source: sourceHTTP, At: time.Now()})
	if c3.cur == nil || c3.cur.Cookie != "C=3" || c3.maxAge != defaultSessionAge {
		t.Error("store yazmali / varsayilan sure")
	}
	// engel: tarayici yoksa preferBrowser acilmaz, varsa acilir
	c3.invalidate(c3.cur, true)
	if c3.preferBrowser || c3.cur != nil {
		t.Error("tarayici yokken preferBrowser acilmamali")
	}
	if st := c3.Stats(); st["blocked"] != 1 || st["sessionRefresh"] != 1 {
		t.Errorf("stats: %v", st)
	}
}
