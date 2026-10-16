package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
)

//go:embed openapi.json
var openAPIRaw []byte

func (s *Server) openAPISpec(w http.ResponseWriter, _ *http.Request) {
	var spec map[string]any
	if err := json.Unmarshal(openAPIRaw, &spec); err != nil {
		writeJSON(w, 500, map[string]any{"error": "openapi.json okunamadi"})
		return
	}
	spec["servers"] = []map[string]string{
		{"url": fmt.Sprintf("http://localhost:%d", s.cfg.Port), "description": "Local"},
		{"url": "https://hotel.alpkun.com", "description": "Production"},
	}
	writeJSON(w, 200, spec)
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="tr">
<head>
  <meta charset="utf-8" />
  <title>Hotel Price Bot API</title>
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.17.14/swagger-ui.min.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.17.14/swagger-ui-bundle.min.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/docs/openapi.json", dom_id: "#swagger-ui", persistAuthorization: true });
  </script>
</body>
</html>`

func (s *Server) swaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerHTML))
}
