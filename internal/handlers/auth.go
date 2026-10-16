package handlers

import (
	"encoding/json"
	"net/http"

	"forum-api/internal/auth"
	"forum-api/internal/models"
	"forum-api/internal/repository"
	"forum-api/config"
)

// AuthHandler handles login and registration
func AuthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		if r.URL.Path == "/api/auth/enlist" {
			Register(w, r)
		} else if r.URL.Path == "/api/auth/login" {
			Login(w, r)
		} else {
			http.Error(w, `{"error":"Invalid endpoint"}`, http.StatusBadRequest)
		}
	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// Register creates a new user account
func Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Create bcrypt hash
	passwordHash, err := repository.HashPassword(req.Password)
	if err != nil {
		http.Error(w, `{"error":"Failed to hash password"}`, http.StatusInternalServerError)
		return
	}

	user, err := repository.UserRepository{}.Create(req.Username, passwordHash, "user")
	if err != nil {
		http.Error(w, `{"error":"Username already exists"}`, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "User registered successfully",
		"user": map[string]interface{}{
			"id": user.ID,
			"username": user.Username,
			"role": user.Role,
		},
	})
}

// Login authenticates a user
func Login(w http.ResponseWriter, r *http.Request) {
	var req models.AuthRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	user, err := repository.UserRepository{}.GetByUsername(req.Username)
	if err != nil {
		http.Error(w, `{"error":"User not found"}`, http.StatusUnauthorized)
		return
	}

	// Check password against the stored bcrypt hash
	if !repository.CheckPassword(user.PasswordHash, req.Password) {
		http.Error(w, `{"error":"Invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	// Generate JWT token with 24h expiry
	cfg := config.Load()
	token, err := auth.GenerateToken(*user, cfg.JWTSecret)
	if err != nil {
		http.Error(w, `{"error":"Failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id": user.ID,
			"username": user.Username,
			"role": user.Role,
		},
	})
}

// GetUser retrieves the current authenticated user
func GetUser(w http.ResponseWriter, r *http.Request) {
	// Extract token from Authorization header
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, `{"error":"Authorization header required"}`, http.StatusUnauthorized)
		return
	}

	// Get user from JWT token
	user, err := auth.GetAuthenticatedUser(token)
	if err != nil {
		http.Error(w, `{"error":"Invalid token"}`, http.StatusUnauthorized)
		return
	}

	// Fetch full user details from repository
	fullUser, err := repository.UserRepository{}.GetByID(user.ID)
	if err != nil {
		http.Error(w, `{"error":"User not found"}`, http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": fullUser.ID,
		"username": fullUser.Username,
		"role": fullUser.Role,
		"created_at": fullUser.CreatedAt,
	})
}
