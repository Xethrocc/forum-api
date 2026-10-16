package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"forum-api/internal/auth"
	"forum-api/internal/models"
)

// Regression: JWTAuth muss "Bearer <token>" akzeptieren (Standard-Header).
func TestJWTAuthAcceptsBearerPrefix(t *testing.T) {
	t.Setenv("JWT_SECRET", "bearer-test-secret")
	tok, err := auth.GenerateToken(models.User{ID: 1, Username: "u", Role: "user"}, "bearer-test-secret")
	if err != nil {
		t.Fatal(err)
	}

	h := JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Bearer header: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer kaputt")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ungültiges Token: expected 401, got %d", w.Code)
	}
}
