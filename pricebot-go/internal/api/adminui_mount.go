package api

import (
	"net/http"

	"pricebot/internal/api/adminui"
)

func (s *Server) mountAdminUI(mux *http.ServeMux) {
	h := adminui.Handler()
	mux.Handle("GET /admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently))
	mux.Handle("GET /admin/", h)
}
