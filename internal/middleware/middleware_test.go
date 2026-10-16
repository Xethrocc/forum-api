package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"forum-api/internal/auth"
	"forum-api/internal/models"
	"forum-api/config"
)

// mockHandler is a simple HTTP handler for testing
type mockHandler struct{}

func (h *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// setupAuthConfig sets up a test JWT secret
func setupAuthConfig() {
	cfg := &config.Config{
		JWTSecret: "test-secret-key-for-testing-only",
	}
	config.SetForTesting(cfg)
}

// cleanup resets the global state between tests
func cleanup() {
	resetRateLimiter()
}

// generateTestToken creates a JWT token for a test user
func generateTestToken(user *models.User) string {
	token, err := auth.GenerateToken(*user, "test-secret-key-for-testing-only")
	if err != nil {
		panic(err)
	}
	return token
}

func TestRateLimitWithUserID(t *testing.T) {
	setupAuthConfig()
	cleanup()

	// Create test user
	testUser := &models.User{
		ID:       123,
		Username: "testuser",
		Role:     "user",
	}

	token := generateTestToken(testUser)

	maxRequests := 5
	limiter := RateLimit(maxRequests)
	handler := limiter(&mockHandler{})

	// Make requests with JWT token
	for i := 0; i < maxRequests; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		// Simulate JWTAuth middleware
		req = req.WithContext(context.WithValue(req.Context(), "user", testUser))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected status 200, got %d", i+1, w.Code)
		}

		// Check rate limit headers
		limit := w.Header().Get("X-RateLimit-Limit")
		if limit != fmt.Sprintf("%d", maxRequests) {
			t.Errorf("Request %d: expected X-RateLimit-Limit %d, got %s", i+1, maxRequests, limit)
		}

		remaining := w.Header().Get("X-RateLimit-Remaining")
		expectedRemaining := fmt.Sprintf("%d", maxRequests-i-1)
		if remaining != expectedRemaining {
			t.Errorf("Request %d: expected X-RateLimit-Remaining %s, got %s", i+1, expectedRemaining, remaining)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = req.WithContext(context.WithValue(req.Context(), "user", testUser))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	if w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("Expected X-RateLimit-Remaining to be 0, got %s", w.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestRateLimitAdminExemption(t *testing.T) {
	setupAuthConfig()
	cleanup()

	// Create admin user
	adminUser := &models.User{
		ID:       456,
		Username: "admin",
		Role:     "admin",
	}

	token := generateTestToken(adminUser)

	limiter := RateLimit(3)
	handler := limiter(&mockHandler{})

	// Admin should not be rate limited even after many requests
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		// Simulate JWTAuth middleware
		req = req.WithContext(context.WithValue(req.Context(), "user", adminUser))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Admin request %d: expected status 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimitUnauthenticatedFallback(t *testing.T) {
	setupAuthConfig()
	cleanup()

	maxRequests := 3
	limiter := RateLimit(maxRequests)
	handler := limiter(&mockHandler{})

	// Make requests without authentication (should use IP fallback)
	for i := 0; i < maxRequests; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}

func TestRateLimitDifferentUsers(t *testing.T) {
	setupAuthConfig()
	cleanup()

	// Create two different users
	user1 := &models.User{
		ID:       111,
		Username: "user1",
		Role:     "user",
	}
	user2 := &models.User{
		ID:       222,
		Username: "user2",
		Role:     "user",
	}

	token1 := generateTestToken(user1)
	token2 := generateTestToken(user2)

	maxRequests := 2
	limiter := RateLimit(maxRequests)
	handler := limiter(&mockHandler{})

	// Exhaust user1's quota
	for i := 0; i < maxRequests; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		// Simulate JWTAuth middleware
		req = req.WithContext(context.WithValue(req.Context(), "user", user1))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("User1 request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// User1 should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token1)
	req = req.WithContext(context.WithValue(req.Context(), "user", user1))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("User1: expected status 429, got %d", w.Code)
	}

	// User2 should still work
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token2)
	req = req.WithContext(context.WithValue(req.Context(), "user", user2))
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("User2: expected status 200, got %d", w.Code)
	}
}

func TestRateLimitCounterReset(t *testing.T) {
	setupAuthConfig()
	cleanup()

	testUser := &models.User{
		ID:       789,
		Username: "resetuser",
		Role:     "user",
	}

	token := generateTestToken(testUser)

	maxRequests := 2
	limiter := RateLimit(maxRequests)
	handler := limiter(&mockHandler{})

	// Use up quota
	for i := 0; i < maxRequests; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		// Simulate JWTAuth middleware
		req = req.WithContext(context.WithValue(req.Context(), "user", testUser))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = req.WithContext(context.WithValue(req.Context(), "user", testUser))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	// Skip sleep test - counter reset happens after 1 minute
	// This test would take too long for CI
}
