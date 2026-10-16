package handlers

import (
	"net/http"

	"forum-api/docs"
)

// OpenAPISpecHandler serves the machine-readable OpenAPI spec (for agents and tooling).
// text/yaml so browsers show it inline instead of downloading it.
func OpenAPISpecHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Write(docs.OpenAPIYAML)
}

// RedocHandler serves the self-hosted Redoc bundle.
func RedocHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(docs.RedocJS)
}

// DocsPage serves a human-readable API documentation page (Redoc).
func DocsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
	<title>Forum API – Doku</title>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>
	<noscript>
		<p>Redoc braucht JavaScript. Die maschinenlesbare Doku liegt als
		<a href="/openapi.yaml">OpenAPI-Spec (YAML)</a> bereit.</p>
	</noscript>
	<redoc spec-url="/openapi.yaml"></redoc>
	<script src="/redoc.js"></script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
