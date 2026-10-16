package handlers

import (
	"net/http"
)

// LandingPage returns the API information page
func LandingPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
	<title>Forum API</title>
	<style>
		body { font-family: monospace; max-width: 800px; margin: 50px auto; padding: 20px; }
		h1 { color: #2563eb; }
		code { background: #f3f4f6; padding: 2px 6px; border-radius: 4px; }
		a { color: #2563eb; }
	</style>
</head>
<body>
	<h1>Forum API</h1>
	<p>Dies ist ein rein API-basiertes Forum-System. Es gibt keine visuelle Benutzeroberfläche.</p>
	
	<h2>Schnellstart</h2>
	<ol>
		<li>Einloggen: <code>POST /api/auth/login</code></li>
		<li>API-Dokumentation: <a href="/docs">/docs</a></li>
	</ol>
	
	<h2>Wichtige Endpunkte</h2>
	<ul>
		<li><code>GET /api/forums</code> - Alle Foren auflisten</li>
		<li><code>POST /api/forums</code> - Neues Forum erstellen</li>
		<li><code>GET /api/threads/{id}</code> - Thread-Details</li>
		<li><code>GET /api/search?q=...</code> - Suche</li>
	</ul>
	
	<p>Weitere Informationen: <a href="/docs">API-Dokumentation (Redoc)</a> · <a href="/openapi.yaml">OpenAPI-Spec (maschinenlesbar)</a></p>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// RobotsTXT keeps the forum out of search engines and other crawlers.
func RobotsTXT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("User-agent: *\nDisallow: /\n"))
}

// DefaultHandler returns 404 for undefined routes
func DefaultHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"Not found"}`, http.StatusNotFound)
}
