package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"forum-api/internal/models"
)

// resetUsers clears the in-memory user store so each test starts from a
// well-defined state.
func resetUsers() {
	mu.Lock()
	defer mu.Unlock()
	users = make(map[string]models.User)
}

func TestRegister(t *testing.T) {
	t.Run("creates user with default role and hashed password", func(t *testing.T) {
		resetUsers()

		user, err := Register("alice", "secret123")
		if err != nil {
			t.Fatalf("Register() error = %v, want nil", err)
		}
		if user == nil {
			t.Fatal("Register() returned nil user")
		}

		if user.ID != 1 {
			t.Errorf("user.ID = %d, want 1", user.ID)
		}
		if user.Username != "alice" {
			t.Errorf("user.Username = %q, want %q", user.Username, "alice")
		}
		if !strings.HasPrefix(user.PasswordHash, "$2") {
			t.Errorf("user.PasswordHash = %q, want a bcrypt hash", user.PasswordHash)
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("secret123")) != nil {
			t.Errorf("user.PasswordHash = %q, want it to verify against password %q", user.PasswordHash, "secret123")
		}
		cost, err := bcrypt.Cost([]byte(user.PasswordHash))
		if err != nil {
			t.Fatalf("bcrypt.Cost() error = %v", err)
		}
		if cost != 10 {
			t.Errorf("bcrypt cost = %d, want 10", cost)
		}
		if user.Role != "user" {
			t.Errorf("user.Role = %q, want %q", user.Role, "user")
		}
	})

	t.Run("assigns incrementing ids", func(t *testing.T) {
		resetUsers()

		first, err := Register("alice", "pw1")
		if err != nil {
			t.Fatalf("Register(alice) error = %v, want nil", err)
		}
		second, err := Register("bob", "pw2")
		if err != nil {
			t.Fatalf("Register(bob) error = %v, want nil", err)
		}

		if first.ID != 1 {
			t.Errorf("first.ID = %d, want 1", first.ID)
		}
		if second.ID != 2 {
			t.Errorf("second.ID = %d, want 2", second.ID)
		}
	})

	t.Run("rejects duplicate username", func(t *testing.T) {
		resetUsers()

		if _, err := Register("alice", "secret123"); err != nil {
			t.Fatalf("Register(alice) error = %v, want nil", err)
		}

		user, err := Register("alice", "other-password")
		if err == nil {
			t.Fatal("Register(duplicate) error = nil, want error")
		}
		if !strings.Contains(err.Error(), "username already exists") {
			t.Errorf("Register(duplicate) error = %q, want it to contain %q", err.Error(), "username already exists")
		}
		if user != nil {
			t.Errorf("Register(duplicate) user = %+v, want nil", user)
		}

		// The original account must stay untouched.
		login, err := Login("alice", "secret123")
		if err != nil {
			t.Fatalf("Login(alice, secret123) error = %v, want nil", err)
		}
		if login.User.Role != "user" {
			t.Errorf("login.User.Role = %q, want %q", login.User.Role, "user")
		}
	})
}

func TestLogin(t *testing.T) {
	t.Run("returns token and user for valid credentials", func(t *testing.T) {
		resetUsers()
		t.Setenv("JWT_SECRET", "test-secret")

		if _, err := Register("alice", "secret123"); err != nil {
			t.Fatalf("Register() error = %v, want nil", err)
		}

		resp, err := Login("alice", "secret123")
		if err != nil {
			t.Fatalf("Login() error = %v, want nil", err)
		}
		if resp == nil {
			t.Fatal("Login() returned nil response")
		}
		if resp.Token == "" {
			t.Error("Login() token is empty, want non-empty token")
		}

		if resp.User.ID != 1 {
			t.Errorf("resp.User.ID = %d, want 1", resp.User.ID)
		}
		if resp.User.Username != "alice" {
			t.Errorf("resp.User.Username = %q, want %q", resp.User.Username, "alice")
		}
		if resp.User.Role != "user" {
			t.Errorf("resp.User.Role = %q, want %q", resp.User.Role, "user")
		}

		// The returned token must be signed with the configured secret and
		// carry the user claims.
		tokenUser, err := ValidateToken(resp.Token, "test-secret")
		if err != nil {
			t.Fatalf("ValidateToken() error = %v, want nil", err)
		}
		if tokenUser.ID != 1 || tokenUser.Username != "alice" || tokenUser.Role != "user" {
			t.Errorf("ValidateToken() user = %+v, want {ID:1 Username:alice Role:user}", tokenUser)
		}
	})

	t.Run("rejects wrong password", func(t *testing.T) {
		resetUsers()

		if _, err := Register("alice", "secret123"); err != nil {
			t.Fatalf("Register() error = %v, want nil", err)
		}

		resp, err := Login("alice", "wrong-password")
		if err == nil {
			t.Fatal("Login(wrong password) error = nil, want error")
		}
		if !strings.Contains(err.Error(), "invalid credentials") {
			t.Errorf("Login(wrong password) error = %q, want it to contain %q", err.Error(), "invalid credentials")
		}
		if resp != nil {
			t.Errorf("Login(wrong password) resp = %+v, want nil", resp)
		}
	})

	t.Run("rejects unknown username", func(t *testing.T) {
		resetUsers()

		resp, err := Login("nobody", "secret123")
		if err == nil {
			t.Fatal("Login(unknown user) error = nil, want error")
		}
		if !strings.Contains(err.Error(), "invalid credentials") {
			t.Errorf("Login(unknown user) error = %q, want it to contain %q", err.Error(), "invalid credentials")
		}
		if resp != nil {
			t.Errorf("Login(unknown user) resp = %+v, want nil", resp)
		}
	})

	t.Run("registered user can log in", func(t *testing.T) {
		resetUsers()
		t.Setenv("JWT_SECRET", "test-secret")

		if _, err := Register("bob", "pw-42"); err != nil {
			t.Fatalf("Register() error = %v, want nil", err)
		}

		resp, err := Login("bob", "pw-42")
		if err != nil {
			t.Fatalf("Login() error = %v, want nil", err)
		}
		if resp.Token == "" {
			t.Error("Login() token is empty, want non-empty token")
		}
		if resp.User.Username != "bob" {
			t.Errorf("resp.User.Username = %q, want %q", resp.User.Username, "bob")
		}
	})
}
