package handlers

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"

	"forum-api/internal/auth"
	"forum-api/internal/repository"
)

// Queries issued by the user repository (see internal/repository/user_repo.go).
const (
	insertUserQuery = "INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id, created_at"

	selectUserByUsernameQuery = "SELECT id, username, password_hash, role, created_at FROM users WHERE username = $1"
)

// newHandlerMockDB installs a sqlmock database as the package level repository
// DB and restores the previous value when the test finishes. It fails the test
// when the handler did not run every expected statement, which is how the
// database side-effects of a request are verified.
func newHandlerMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}

	prev := repository.DB
	repository.DB = db

	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled mock expectations: %v", err)
		}
		db.Close()
		repository.DB = prev
	})

	return mock
}

// argCapture is a sqlmock argument matcher that matches any value and records
// it, so tests can assert on the exact value written to the database (used for
// the bcrypt password hash, which is salted and therefore not predictable).
type argCapture struct {
	value driver.Value
	set   bool
}

func (c *argCapture) Match(v driver.Value) bool {
	c.value = v
	c.set = true
	return true
}

// requireStringArg returns the captured argument as a string.
func (c *argCapture) requireStringArg(t *testing.T) string {
	t.Helper()

	if !c.set {
		t.Fatal("expected argument was never passed to the database")
	}
	s, ok := c.value.(string)
	if !ok {
		t.Fatalf("captured argument = %T (%v), want string", c.value, c.value)
	}
	return s
}

// serveJSON performs a request against the given handler with a JSON body and
// returns the recorded response.
func serveJSON(handler http.HandlerFunc, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

// decodeJSONBody unmarshals the response body into dst.
func decodeJSONBody(t *testing.T, rec *httptest.ResponseRecorder, dst interface{}) {
	t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("failed to decode response body %q: %v", rec.Body.String(), err)
	}
}

// assertContentTypeJSON checks that the response advertises JSON.
func assertContentTypeJSON(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// assertJSONError checks the status code and the {"error": "..."} payload that
// the handlers write through http.Error (which appends a newline).
func assertJSONError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantError string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d (body %q)", rec.Code, wantStatus, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rec.Body.String())), &payload); err != nil {
		t.Fatalf("error body is not JSON: %v (body %q)", err, rec.Body.String())
	}
	if got := payload["error"]; got != wantError {
		t.Errorf("error = %v, want %q", got, wantError)
	}
}

type authUserPayload struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type registerResponse struct {
	Message string          `json:"message"`
	User    authUserPayload `json:"user"`
}

type loginResponse struct {
	Token string          `json:"token"`
	User  authUserPayload `json:"user"`
}

func TestRegister(t *testing.T) {
	t.Run("registers user, inserts row and returns 201 with user payload", func(t *testing.T) {
		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)

		// Database side-effect: exactly one user row must be inserted with the
		// hashed password and the default role.
		hashArg := &argCapture{}
		mock.ExpectQuery(regexp.QuoteMeta(insertUserQuery)).
			WithArgs("alice", hashArg, "user").
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(int64(7), createdAt))

		rec := serveJSON(AuthHandler, http.MethodPost, "/api/auth/enlist",
			`{"username":"alice","password":"secret123"}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
		}
		assertContentTypeJSON(t, rec)

		var resp registerResponse
		decodeJSONBody(t, rec, &resp)

		if resp.Message != "User registered successfully" {
			t.Errorf("message = %q, want %q", resp.Message, "User registered successfully")
		}
		if resp.User.ID != 7 {
			t.Errorf("user.id = %d, want 7", resp.User.ID)
		}
		if resp.User.Username != "alice" {
			t.Errorf("user.username = %q, want %q", resp.User.Username, "alice")
		}
		if resp.User.Role != "user" {
			t.Errorf("user.role = %q, want %q", resp.User.Role, "user")
		}

		// The inserted password must be a bcrypt hash of the submitted
		// password, never the plaintext password itself.
		storedHash := hashArg.requireStringArg(t)
		if storedHash == "secret123" {
			t.Error("plaintext password was written to the database")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte("secret123")); err != nil {
			t.Errorf("stored password hash does not match the password: %v", err)
		}

		// The response must never leak credentials.
		if strings.Contains(rec.Body.String(), "secret123") || strings.Contains(rec.Body.String(), storedHash) {
			t.Errorf("response leaks credentials: %s", rec.Body.String())
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("user row was not inserted as expected: %v", err)
		}
	})

	t.Run("rejects malformed request body", func(t *testing.T) {
		_ = newHandlerMockDB(t) // no database access expected

		rec := serveJSON(AuthHandler, http.MethodPost, "/api/auth/enlist", `{"username":`)

		assertJSONError(t, rec, http.StatusBadRequest, "Invalid request body")
	})

	t.Run("rejects duplicate username", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(insertUserQuery)).
			WithArgs("alice", &argCapture{}, "user").
			WillReturnError(errors.New(`pq: duplicate key value violates unique constraint "users_username_key"`))

		rec := serveJSON(AuthHandler, http.MethodPost, "/api/auth/enlist",
			`{"username":"alice","password":"secret123"}`)

		assertJSONError(t, rec, http.StatusBadRequest, "Username already exists")
	})

	t.Run("rejects unsupported method and unknown path", func(t *testing.T) {
		_ = newHandlerMockDB(t) // no database access expected

		rec := serveJSON(AuthHandler, http.MethodGet, "/api/auth/enlist", "")
		assertJSONError(t, rec, http.StatusMethodNotAllowed, "Method not allowed")

		rec = serveJSON(AuthHandler, http.MethodPost, "/api/auth/unknown", "")
		assertJSONError(t, rec, http.StatusBadRequest, "Invalid endpoint")
	})
}

func TestLogin(t *testing.T) {
	t.Run("authenticates user and returns jwt token", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "integration-test-secret")

		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
		storedHash, err := repository.HashPassword("secret123")
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		mock.ExpectQuery(regexp.QuoteMeta(selectUserByUsernameQuery)).
			WithArgs("alice").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "role", "created_at"}).
				AddRow(int64(7), "alice", storedHash, "user", createdAt))

		rec := serveJSON(AuthHandler, http.MethodPost, "/api/auth/login",
			`{"username":"alice","password":"secret123"}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		assertContentTypeJSON(t, rec)

		var resp loginResponse
		decodeJSONBody(t, rec, &resp)

		if resp.Token == "" {
			t.Fatal("token is empty")
		}
		if resp.User.ID != 7 {
			t.Errorf("user.id = %d, want 7", resp.User.ID)
		}
		if resp.User.Username != "alice" {
			t.Errorf("user.username = %q, want %q", resp.User.Username, "alice")
		}
		if resp.User.Role != "user" {
			t.Errorf("user.role = %q, want %q", resp.User.Role, "user")
		}

		// The token must be a JWT signed with the configured secret that
		// carries the authenticated user.
		claims, err := auth.ValidateToken(resp.Token, "integration-test-secret")
		if err != nil {
			t.Fatalf("returned token is not valid: %v", err)
		}
		if claims.ID != resp.User.ID || claims.Username != resp.User.Username || claims.Role != resp.User.Role {
			t.Errorf("token claims = %+v, want user %+v", claims, resp.User)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("user row was not looked up as expected: %v", err)
		}
	})

	t.Run("rejects wrong password", func(t *testing.T) {
		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
		storedHash, err := repository.HashPassword("some-other-password")
		if err != nil {
			t.Fatalf("HashPassword() error = %v", err)
		}

		mock.ExpectQuery(regexp.QuoteMeta(selectUserByUsernameQuery)).
			WithArgs("alice").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "role", "created_at"}).
				AddRow(int64(7), "alice", storedHash, "user", createdAt))

		rec := serveJSON(AuthHandler, http.MethodPost, "/api/auth/login",
			`{"username":"alice","password":"secret123"}`)

		assertJSONError(t, rec, http.StatusUnauthorized, "Invalid credentials")
	})

	t.Run("rejects unknown user", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectUserByUsernameQuery)).
			WithArgs("nobody").
			WillReturnError(sql.ErrNoRows)

		rec := serveJSON(AuthHandler, http.MethodPost, "/api/auth/login",
			`{"username":"nobody","password":"secret123"}`)

		assertJSONError(t, rec, http.StatusUnauthorized, "User not found")
	})

	t.Run("rejects malformed request body", func(t *testing.T) {
		_ = newHandlerMockDB(t) // no database access expected

		rec := serveJSON(AuthHandler, http.MethodPost, "/api/auth/login", "not json")

		assertJSONError(t, rec, http.StatusBadRequest, "Invalid request body")
	})
}

// TestAuthFlow_RegisterThenLogin exercises the full flow: the row inserted at
// registration carries a password hash that the login request can authenticate
// against afterwards.
func TestAuthFlow_RegisterThenLogin(t *testing.T) {
	t.Setenv("JWT_SECRET", "integration-test-secret")

	mock := newHandlerMockDB(t)
	createdAt := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)

	hashArg := &argCapture{}
	mock.ExpectQuery(regexp.QuoteMeta(insertUserQuery)).
		WithArgs("alice", hashArg, "user").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(7), createdAt))

	registerRec := serveJSON(AuthHandler, http.MethodPost, "/api/auth/enlist",
		`{"username":"alice","password":"secret123"}`)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d (body %q)", registerRec.Code, http.StatusCreated, registerRec.Body.String())
	}

	var registered registerResponse
	decodeJSONBody(t, registerRec, &registered)

	// The login lookup must find the exact hash that registration inserted.
	storedHash := hashArg.requireStringArg(t)
	mock.ExpectQuery(regexp.QuoteMeta(selectUserByUsernameQuery)).
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "role", "created_at"}).
			AddRow(int64(7), "alice", storedHash, "user", createdAt))

	loginRec := serveJSON(AuthHandler, http.MethodPost, "/api/auth/login",
		`{"username":"alice","password":"secret123"}`)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d (body %q)", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}

	var loggedIn loginResponse
	decodeJSONBody(t, loginRec, &loggedIn)

	if loggedIn.Token == "" {
		t.Fatal("login token is empty")
	}
	if loggedIn.User.ID != registered.User.ID {
		t.Errorf("login user.id = %d, want registered id %d", loggedIn.User.ID, registered.User.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("database side-effects missing: %v", err)
	}
}
