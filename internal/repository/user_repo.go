package repository

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"forum-api/internal/models"
)

// bcryptCost is the bcrypt work factor used for password hashing.
const bcryptCost = 10

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(password string) (string, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(passwordHash), nil
}

// CheckPassword reports whether the plaintext password matches the stored
// bcrypt hash.
func CheckPassword(passwordHash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
}

type UserRepository struct{}

func (ur UserRepository) Create(username, passwordHash, role string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := DB.QueryRowContext(ctx,
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id, created_at",
		username, passwordHash, role,
	).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	user.Username = username
	user.Role = role

	return &user, nil
}

func (ur UserRepository) GetByUsername(username string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := DB.QueryRowContext(ctx,
		"SELECT id, username, password_hash, role, created_at FROM users WHERE username = $1",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (ur UserRepository) GetByID(id int64) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User
	err := DB.QueryRowContext(ctx,
		"SELECT id, username, role, created_at FROM users WHERE id = $1",
		id,
	).Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (ur UserRepository) Suspend(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := DB.ExecContext(ctx,
		"UPDATE users SET role = 'suspended' WHERE id = $1",
		id,
	)
	return err
}

func (ur UserRepository) GetAll() ([]models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DB.QueryContext(ctx,
		"SELECT id, username, role, created_at FROM users",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
