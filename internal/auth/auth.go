package auth

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"forum-api/internal/models"
	"forum-api/config"
)

// bcryptCost is the bcrypt work factor used for password hashing.
const bcryptCost = 10

// In-memory user store (replace with database in production)
var (
	users    = make(map[string]models.User)
	mu       sync.RWMutex
)

// Register creates a new user
func Register(username, password string) (*models.User, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := users[username]; exists {
		return nil, errors.New("username already exists")
	}

	// Store bcrypt password hash
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		ID:           int64(len(users) + 1),
		Username:     username,
		PasswordHash: string(passwordHash),
		Role:         "user",
	}

	users[username] = *user
	return user, nil
}

// Login authenticates a user and returns JWT token
func Login(username, password string) (*models.AuthResponse, error) {
	mu.RLock()
	user, exists := users[username]
	mu.RUnlock()

	// Verify password against the stored bcrypt hash
	if !exists || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, errors.New("invalid credentials")
	}

	// Generate JWT token with 24h expiry
	cfg := config.Load()
	token, err := GenerateToken(user, cfg.JWTSecret)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// GetAuthenticatedUser extracts user from JWT token in Authorization header
func GetAuthenticatedUser(authHeader string) (*models.User, error) {
	if authHeader == "" {
		return nil, errors.New("missing authorization header")
	}

	// Expect "Bearer <token>"
	var tokenString string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenString = authHeader[7:]
	} else {
		tokenString = authHeader
	}

	cfg := config.Load()
	return ValidateToken(tokenString, cfg.JWTSecret)
}

// Logout is a placeholder for token invalidation
// In production, implement token blacklisting with Redis or database
func Logout(token string) {
	// Token-based logout requires blacklisting mechanism
	// For now, tokens expire naturally after 24h
	_ = token
}

// GenerateToken creates a new JWT token for the given user
func GenerateToken(user models.User, secret string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken validates a JWT token and returns the user info
func ValidateToken(tokenString, secret string) (*models.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return &models.User{
		ID:       int64(claims["user_id"].(float64)),
		Username: claims["username"].(string),
		Role:     claims["role"].(string),
	}, nil
}
