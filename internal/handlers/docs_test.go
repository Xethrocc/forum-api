package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"forum-api/docs"
)

func TestOpenAPISpecHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/openapi.yaml", nil)
	w := httptest.NewRecorder()
	OpenAPISpecHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/yaml") {
		t.Errorf("expected text/yaml, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "openapi: 3.0.3") {
		t.Error("Spec enthält kein openapi-Versionsfeld")
	}
	if !strings.Contains(body, "bearerAuth") {
		t.Error("Spec ohne bearerAuth")
	}

	req = httptest.NewRequest("POST", "/openapi.yaml", nil)
	w = httptest.NewRecorder()
	OpenAPISpecHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: expected 405, got %d", w.Code)
	}
}

func TestDocsPage(t *testing.T) {
	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	DocsPage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "/openapi.yaml") {
		t.Error("Docs-Seite verlinkt die Spec nicht")
	}
	if !strings.Contains(w.Body.String(), "/redoc.js") {
		t.Error("Docs-Seite lädt Redoc nicht lokal")
	}
	if strings.Contains(w.Body.String(), "cdn.redoc.ly") {
		t.Error("Docs-Seite hängt noch am CDN")
	}
}

func TestRobotsTXT(t *testing.T) {
	req := httptest.NewRequest("GET", "/robots.txt", nil)
	w := httptest.NewRecorder()
	RobotsTXT(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "User-agent: *\nDisallow: /\n" {
		t.Errorf("robots.txt body = %q", body)
	}
}

func TestRedocHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/redoc.js", nil)
	w := httptest.NewRecorder()
	RedocHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("expected application/javascript, got %q", ct)
	}
	if len(docs.RedocJS) < 100000 {
		t.Errorf("Redoc-Bundle zu klein: %d Bytes", len(docs.RedocJS))
	}
}
