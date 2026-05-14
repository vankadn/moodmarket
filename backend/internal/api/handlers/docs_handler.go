package handlers

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed openapi.yaml
var openAPISpec []byte

// DocsHandler serves the Swagger UI at /docs/ and the OpenAPI spec at /docs/openapi.yaml.
// No auth required — registered on the top-level mux.
type DocsHandler struct{}

func NewDocsHandler() *DocsHandler { return &DocsHandler{} }

func (h *DocsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/openapi.yaml") {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(openAPISpec) //nolint:errcheck
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(swaggerUIHTML)) //nolint:errcheck
}

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>InvestIQ API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; }
    .swagger-ui .topbar { background: #1a1a1a; }
    .swagger-ui .topbar .download-url-wrapper { display: none; }
  </style>
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
  SwaggerUIBundle({
    url: "/docs/openapi.yaml",
    dom_id: "#swagger-ui",
    deepLinking: true,
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
    layout: "BaseLayout",
    tryItOutEnabled: true,
    persistAuthorization: true,
  });
</script>
</body>
</html>`
