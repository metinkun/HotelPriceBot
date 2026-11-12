package api

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"pricebot/internal/apperr"
)

// Musteriye verilecek entegrasyon paketi: Postman koleksiyonu (gomulu), ortam dosyasi (adres + anahtar dolu),
// rehber (adres dolu), otel listesi (xlsx) ve kisa bir BENIOKU. `python3 postman/musteri.py` dosyalari yeniler.
//
//go:embed clientpkg/SmartRate-API.postman_collection.json clientpkg/API-Entegrasyon-Rehberi.md
var clientPkgFS embed.FS

// apiBaseURL musteri paketindeki taban adres: PUBLIC_API_URL, yoksa istegin kendi adresi.
func (s *Server) apiBaseURL(r *http.Request) string {
	if u := strings.TrimSpace(s.cfg.PublicAPIURL); u != "" {
		return strings.TrimRight(u, "/")
	}
	return strings.TrimSuffix(s.portalBaseURL(r), "/portal/")
}

// GET /api/admin/clients/{id}/package/defaults -> {baseUrl}
func (s *Server) adminClientPackageDefaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"baseUrl": s.apiBaseURL(r), "fromConfig": strings.TrimSpace(s.cfg.PublicAPIURL) != ""})
}

// POST /api/admin/clients/{id}/package {baseUrl?, apiKey?, includeHotels?} -> zip
func (s *Server) adminClientPackage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		badRequest(w, "Gecersiz id")
		return
	}
	c, p, err := s.clientWithPlan(r.Context(), id)
	if err != nil {
		notFoundOr(w, r, err, "Musteri bulunamadi")
		return
	}
	var in struct {
		BaseURL       string `json:"baseUrl"`
		APIKey        string `json:"apiKey"`
		IncludeHotels *bool  `json:"includeHotels"`
	}
	_ = decodeBody(r, &in)
	base := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if base == "" {
		base = s.apiBaseURL(r)
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		writeError(w, r, apperr.BadRequestf("baseUrl http(s):// ile baslamali"))
		return
	}
	key := strings.TrimSpace(in.APIKey)
	if key != "" && !strings.HasPrefix(key, "pb_live_") {
		writeError(w, r, apperr.BadRequestf("apiKey 'pb_live_' ile baslamali (anahtar yalnizca uretildigi anda bilinir)"))
		return
	}
	withHotels := in.IncludeHotels == nil || *in.IncludeHotels

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, body []byte) error {
		f, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return err
		}
		_, err = f.Write(body)
		return err
	}
	// 1) Koleksiyon (oldugu gibi)
	col, _ := clientPkgFS.ReadFile("clientpkg/SmartRate-API.postman_collection.json")
	if err := add("SmartRate-API.postman_collection.json", col); err != nil {
		writeError(w, r, err)
		return
	}
	// 2) Ortam: adres + anahtar dolu
	keyVal := key
	if keyVal == "" {
		keyVal = "pb_live_... (SmartRate tarafından iletilen anahtarı buraya yazın)"
	}
	env := map[string]any{"name": "SmartRate API — " + c.Name, "_postman_variable_scope": "environment", "values": []map[string]any{
		{"key": "baseUrl", "value": base, "enabled": true},
		{"key": "apiKey", "value": keyVal, "enabled": true, "type": "secret"},
		{"key": "requestId", "value": "", "enabled": true},
	}}
	envB, _ := json.MarshalIndent(env, "", "  ")
	if err := add("SmartRate-API.postman_environment.json", envB); err != nil {
		writeError(w, r, err)
		return
	}
	// 3) Rehber: adres dolu
	guide, _ := clientPkgFS.ReadFile("clientpkg/API-Entegrasyon-Rehberi.md")
	if err := add("API-Entegrasyon-Rehberi.md", bytes.ReplaceAll(guide, []byte("{{BASE_URL}}"), []byte(base))); err != nil {
		writeError(w, r, err)
		return
	}
	// 4) Otel listesi
	hotelNote := "Otel listesi pakete eklenmedi; GET /api/v1/hotels ile alinabilir."
	if withHotels {
		ids, labels, linked, err := s.clientHotelIDs(r.Context(), id)
		if err != nil {
			writeError(w, r, err)
			return
		}
		rows := s.hotelRowsFor(r.Context(), ids, labels, linked, s.allowedProviderSet(limitsOf(c, p)))
		sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name) })
		var xb bytes.Buffer
		s.hotelsXLSXTo(&xb, rows, true)
		if err := add("Oteller.xlsx", xb.Bytes()); err != nil {
			writeError(w, r, err)
			return
		}
		hotelNote = fmt.Sprintf("Oteller.xlsx: size tanimli %d otel; 'Otel kimligi (hotelId)' sutunu sorgularda kullanilir.", len(rows))
	}
	// 5) BENIOKU
	readme := fmt.Sprintf(`SmartRate API entegrasyon paketi — %s
Hazirlanma: %s

Icerik
  SmartRate-API.postman_collection.json   Postman koleksiyonu (tum musteri uclari)
  SmartRate-API.postman_environment.json  Postman ortami: baseUrl=%s, apiKey%s
  API-Entegrasyon-Rehberi.md              Uclar, ornekler, hata kodlari, oneriler
  %s

Baslangic
  1. Postman > Import > iki .json dosyasini birlikte birakin.
  2. Sag ustten "SmartRate API — %s" ortamini secin.
  3. "1. Hesap, oteller ve fiyat sorgusu" klasorundeki istekleri sirayla calistirin
     (me -> hotels -> is olustur -> is durumu).
  Swagger: %s/docs
`, c.Name, time.Now().Format("2006-01-02 15:04"), base, map[bool]string{true: " dolu", false: " BOS (ayrica iletilecek)"}[key != ""], hotelNote, c.Name, base)
	if err := add("BENIOKU.txt", []byte(readme)); err != nil {
		writeError(w, r, err)
		return
	}
	if err := zw.Close(); err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="SmartRate-API-%s-%s.zip"`, slugify(c.Name), time.Now().Format("20060102")))
	_, _ = w.Write(buf.Bytes())
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer("ç", "c", "ğ", "g", "ı", "i", "ö", "o", "ş", "s", "ü", "u", " ", "-")
	s = repl.Replace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "musteri"
	}
	return out
}
