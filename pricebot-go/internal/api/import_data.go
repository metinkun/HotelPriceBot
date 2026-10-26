package api

import (
	_ "embed"
	"net/http"
)

//go:embed trnc_hotels.json
var trncHotels []byte

// GET /api/admin/hotels/import/trnc — Excel'den gelen 78 KKTC oteli (data/hotels-trnc.json kopyasi).
func (s *Server) adminImportTRNC(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(trncHotels)
}
