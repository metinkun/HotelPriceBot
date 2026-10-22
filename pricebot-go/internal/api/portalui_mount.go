package api

import (
	"net/http"

	"pricebot/internal/api/portalui"
)

func (s *Server) mountPortalUI(mux *http.ServeMux) {
	h := portalui.Handler()
	mux.Handle("GET /portal", http.RedirectHandler("/portal/", http.StatusMovedPermanently))
	mux.Handle("GET /portal/", h)
}
