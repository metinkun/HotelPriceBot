// Package adminui derlenmis React admin panelini (ui/dist) gomer ve /admin altinda sunar.
package adminui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler /admin/ altinda SPA sunar: dosya varsa dosya, yoksa index.html (client-side routing).
func Handler() http.Handler {
	sub, _ := fs.Sub(distFS, "dist")
	files := http.FS(sub)
	fileServer := http.FileServer(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/admin")
		if path == "" || path == "/" {
			path = "/index.html"
		}
		// index.html dogrudan yazilir: http.FileServer "/index.html" istegini "./"e yonlendirir (bos govde).
		if strings.HasSuffix(path, "/index.html") {
			serveIndex(w, sub)
			return
		}
		if f, err := sub.Open(strings.TrimPrefix(path, "/")); err == nil {
			f.Close()
			r2 := r.Clone(r.Context())
			r2.URL.Path = path
			if strings.Contains(path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r2)
			return
		}
		// SPA fallback (client-side routing)
		serveIndex(w, sub)
	})
}

func serveIndex(w http.ResponseWriter, sub fs.FS) {
	idx, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		http.Error(w, "panel bulunamadi", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(idx)
}
