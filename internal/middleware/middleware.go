package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"forum-api/internal/auth"
	"forum-api/internal/models"
	"forum-api/config"
)

// NoIndex tells search engines to keep this site out of their index.
// The forum is meant to be found through the API spec, not through search.
func NoIndex(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex")
		next.ServeHTTP(w, r)
	})
}

// Logging logs all requests
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		// Log: path, method, duration
		_ = start
	})
}

// CORS enables cross-origin requests
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitState maintains global rate limiting state
var (
	requests = make(map[string][]time.Time)
	mu sync.Mutex
)

// resetRateLimiter clears all rate limit counters (for testing)
func resetRateLimiter() {
	mu.Lock()
	defer mu.Unlock()
	requests = make(map[string][]time.Time)
}

// RateLimit implements rate limiting based on user ID (falls back to IP for unauthenticated users)
func RateLimit(maxRequests int) func(http.Handler) http.Handler {
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			now := time.Now()
			
			// Extract user ID from JWT context (set by JWTAuth)
			userID := ""
			if user, ok := r.Context().Value("user").(*models.User); ok && user != nil {
				userID = fmt.Sprintf("%d", user.ID)
				// Skip rate limiting for Admin users
				if user.Role == "admin" {
					mu.Unlock()
					next.ServeHTTP(w, r)
					return
				}
			} else {
				// Fall back to IP address for unauthenticated users
				userID = r.RemoteAddr
			}
			
			// Clean old requests (older than 1 minute)
			validRequests := []time.Time{}
			for _, req := range requests[userID] {
				if now.Sub(req) <= time.Minute {
					validRequests = append(validRequests, req)
				}
			}
			requests[userID] = validRequests
			
			remaining := maxRequests - len(requests[userID])
			
			if len(requests[userID]) >= maxRequests {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
				w.Header().Set("X-RateLimit-Remaining", "0")
				http.Error(w, `{"error":"Rate limit exceeded"}`, http.StatusTooManyRequests)
				mu.Unlock()
				return
			}
			
			requests[userID] = append(requests[userID], now)
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining-1))
			mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

// JWTAuth validates JWT tokens and injects user into request context
func JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, `{"error":"Authorization required"}`, http.StatusUnauthorized)
			return
		}
		// Standard-Header ist "Bearer <token>"; nur das Token weiterreichen
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		cfg := config.Load()
		user, err := auth.ValidateToken(token, cfg.JWTSecret)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusUnauthorized)
			return
		}

		// Store user in context for handler access
		r = r.WithContext(context.WithValue(r.Context(), "user", user))
		next.ServeHTTP(w, r)
	})
}

// RequireRole checks JWT claims against allowed roles and returns 403 if user lacks permission
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context (set by JWTAuth)
			user, ok := r.Context().Value("user").(*models.User)
			if !ok || user == nil {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Check if user role is in allowed roles
			roleAllowed := false
			for _, role := range roles {
				if user.Role == role {
					roleAllowed = true
					break
				}
			}

			if !roleAllowed {
				http.Error(w, `{"error":"Forbidden"}`, http.StatusForbidden)
				return
			}

		next.ServeHTTP(w, r)
		})
	}
}
