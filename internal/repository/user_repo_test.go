package repository

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

const userInsertQuery = "INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id, created_at"

const userSelectByUsernameQuery = "SELECT id, username, password_hash, role, created_at FROM users WHERE username = $1"

func TestUserRepository_Create(t *testing.T) {
	t.Run("returns user with generated id and created_at", func(t *testing.T) {
		mock := newMockDB(t)
		createdAt := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(userInsertQuery)).
			WithArgs("alice", "hash_secret", "user").
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(int64(7), createdAt))

		user, err := UserRepository{}.Create("alice", "hash_secret", "user")
		if err != nil {
			t.Fatalf("Create() error = %v, want nil", err)
		}
		if user == nil {
			t.Fatal("Create() returned nil user")
		}

		if user.ID != 7 {
			t.Errorf("user.ID = %d, want 7", user.ID)
		}
		if user.Username != "alice" {
			t.Errorf("user.Username = %q, want %q", user.Username, "alice")
		}
		if user.Role != "user" {
			t.Errorf("user.Role = %q, want %q", user.Role, "user")
		}
		if !user.CreatedAt.Equal(createdAt) {
			t.Errorf("user.CreatedAt = %v, want %v", user.CreatedAt, createdAt)
		}
	})

	t.Run("passes all arguments to the insert statement", func(t *testing.T) {
		mock := newMockDB(t)
		createdAt := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(userInsertQuery)).
			WithArgs("bob", "hash_pw", "admin").
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(int64(1), createdAt))

		user, err := UserRepository{}.Create("bob", "hash_pw", "admin")
		if err != nil {
			t.Fatalf("Create() error = %v, want nil", err)
		}
		if user.Role != "admin" {
			t.Errorf("user.Role = %q, want %q", user.Role, "admin")
		}
	})

	t.Run("returns error when insert fails", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(userInsertQuery)).
			WithArgs("alice", "hash_secret", "user").
			WillReturnError(errors.New(`pq: duplicate key value violates unique constraint "users_username_key"`))

		user, err := UserRepository{}.Create("alice", "hash_secret", "user")
		if err == nil {
			t.Fatal("Create() error = nil, want error")
		}
		if user != nil {
			t.Errorf("Create() user = %+v, want nil", user)
		}
	})
}

func TestUserRepository_GetByUsername(t *testing.T) {
	t.Run("returns matching user", func(t *testing.T) {
		mock := newMockDB(t)
		createdAt := time.Date(2023, 11, 2, 8, 30, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(userSelectByUsernameQuery)).
			WithArgs("alice").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "role", "created_at"}).
				AddRow(int64(3), "alice", "hash_secret", "user", createdAt))

		user, err := UserRepository{}.GetByUsername("alice")
		if err != nil {
			t.Fatalf("GetByUsername() error = %v, want nil", err)
		}
		if user == nil {
			t.Fatal("GetByUsername() returned nil user")
		}

		if user.ID != 3 {
			t.Errorf("user.ID = %d, want 3", user.ID)
		}
		if user.Username != "alice" {
			t.Errorf("user.Username = %q, want %q", user.Username, "alice")
		}
		if user.PasswordHash != "hash_secret" {
			t.Errorf("user.PasswordHash = %q, want %q", user.PasswordHash, "hash_secret")
		}
		if user.Role != "user" {
			t.Errorf("user.Role = %q, want %q", user.Role, "user")
		}
		if !user.CreatedAt.Equal(createdAt) {
			t.Errorf("user.CreatedAt = %v, want %v", user.CreatedAt, createdAt)
		}
	})

	t.Run("returns sql.ErrNoRows for unknown username", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(userSelectByUsernameQuery)).
			WithArgs("nobody").
			WillReturnError(sql.ErrNoRows)

		user, err := UserRepository{}.GetByUsername("nobody")
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("GetByUsername() error = %v, want sql.ErrNoRows", err)
		}
		if user != nil {
			t.Errorf("GetByUsername() user = %+v, want nil", user)
		}
	})

	t.Run("returns database error", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(userSelectByUsernameQuery)).
			WithArgs("alice").
			WillReturnError(errors.New("connection reset"))

		user, err := UserRepository{}.GetByUsername("alice")
		if err == nil {
			t.Fatal("GetByUsername() error = nil, want error")
		}
		if user != nil {
			t.Errorf("GetByUsername() user = %+v, want nil", user)
		}
	})
}
