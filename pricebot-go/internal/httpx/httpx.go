// Package httpx proxy rotasyonlu, axios benzeri (4xx/5xx'te hata donen) HTTP yardimcilari.
package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ProxyFunc her istekte cagrilir; nil donerse dogrudan baglanilir.
type ProxyFunc func() *url.URL

var (
	mu        sync.RWMutex
	proxyFunc ProxyFunc
)

// SetProxyFunc dis istekler icin proxy secicisini ayarlar (proxy servisi tarafindan).
func SetProxyFunc(f ProxyFunc) {
	mu.Lock()
	defer mu.Unlock()
	proxyFunc = f
}

func nextProxy(_ *http.Request) (*url.URL, error) {
	mu.RLock()
	f := proxyFunc
	mu.RUnlock()
	if f == nil {
		return nil, nil
	}
	return f(), nil
}

var transport = &http.Transport{
	Proxy: nextProxy,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   10,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   15 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// Client paylasilan transport'u kullanan, cerez saklamayan istemci.
var Client = &http.Client{Transport: transport}

// Response ham yanit.
type Response struct {
	Status  int
	Header  http.Header
	Body    []byte
	FinalURL string
}

// Text govdeyi string olarak doner.
func (r *Response) Text() string { return string(r.Body) }

// SetCookies yanittaki tum Set-Cookie basliklarini "a=1; b=2" formatina cevirir.
func (r *Response) SetCookies() string {
	vals := r.Header.Values("Set-Cookie")
	if len(vals) == 0 {
		return ""
	}
	parts := make([]string, 0, len(vals))
	for _, c := range vals {
		parts = append(parts, strings.SplitN(c, ";", 2)[0])
	}
	return strings.Join(parts, "; ")
}

// StatusError 4xx/5xx yanitlar icin hata (axios davranisi).
type StatusError struct {
	Status int
	URL    string
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("Request failed with status code %d", e.Status)
}

// Options istek secenekleri.
type Options struct {
	Headers map[string]string
	Timeout time.Duration
	Body    []byte
	// NoProxy true ise proxy kullanilmaz (orn. Webshare API).
	NoProxy bool
}

var directTransport = &http.Transport{
	ForceAttemptHTTP2:   true,
	MaxIdleConns:        20,
	IdleConnTimeout:     60 * time.Second,
	TLSHandshakeTimeout: 15 * time.Second,
}

// Do istegi yapar; 400+ durum kodunda *StatusError doner.
func Do(ctx context.Context, method, rawURL string, opt Options) (*Response, error) {
	if opt.Timeout <= 0 {
		opt.Timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, opt.Timeout)
	defer cancel()

	var body io.Reader
	if opt.Body != nil {
		body = bytes.NewReader(opt.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range opt.Headers {
		if strings.EqualFold(k, "host") {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}

	client := Client
	if opt.NoProxy {
		client = &http.Client{Transport: directTransport}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}

	out := &Response{Status: resp.StatusCode, Header: resp.Header, Body: data}
	if resp.Request != nil && resp.Request.URL != nil {
		out.FinalURL = resp.Request.URL.String()
	}
	if resp.StatusCode >= 400 {
		snippet := string(data)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return out, &StatusError{Status: resp.StatusCode, URL: rawURL, Body: snippet}
	}
	return out, nil
}

// Get kisayolu.
func Get(ctx context.Context, rawURL string, headers map[string]string, timeout time.Duration) (*Response, error) {
	return Do(ctx, http.MethodGet, rawURL, Options{Headers: headers, Timeout: timeout})
}

// Post kisayolu.
func Post(ctx context.Context, rawURL string, body []byte, headers map[string]string, timeout time.Duration) (*Response, error) {
	return Do(ctx, http.MethodPost, rawURL, Options{Headers: headers, Timeout: timeout, Body: body})
}

// Merge birden fazla header haritasini birlestirir (sonrakiler oncekileri ezer).
func Merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// FormValues sirali form-urlencoded govde olusturur (qs.stringify esdegeri; sira korunur).
type FormValues [][2]string

func (f FormValues) Encode() string {
	var b strings.Builder
	for i, kv := range f {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(kv[0]))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(kv[1]))
	}
	return b.String()
}
