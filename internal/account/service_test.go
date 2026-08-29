package account

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/profause/aimocksvr/internal/auth"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/models"
)

// fakeRepo mirrors the real repository's observable semantics: emails are
// canonicalized on write and compared canonical on read, and duplicate emails
// map to ErrConflict.
type fakeRepo struct {
	accounts map[string]*models.Account
	findErr  error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{accounts: make(map[string]*models.Account)}
}

func (f *fakeRepo) Create(_ context.Context, a *models.Account) error {
	key := normalizeEmail(a.Email)
	if _, ok := f.accounts[key]; ok {
		return ErrConflict
	}
	a.Email = normalizeEmail(a.Email)
	f.accounts[key] = a
	return nil
}

func (f *fakeRepo) FindByEmail(_ context.Context, email string) (*models.Account, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if a, ok := f.accounts[normalizeEmail(email)]; ok {
		return a, nil
	}
	return nil, ErrNotFound
}

func newTestAccountService(repo Repository) *Service {
	logger := zerolog.Nop()
	cfg := &config.Config{Auth: config.Auth{
		Enabled:     true,
		JWTSecret:   "secret",
		JWTIssuer:   "mocksvr",
		JWTAudience: "mocksvr",
		JWTTTL:      "1h",
	}}
	return NewService(repo, auth.NewService(cfg, &logger))
}

func TestRegisterSuccess(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestAccountService(repo)

	a, token, err := svc.Register(context.Background(), "  User@Example.COM ", "password123")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if a.ID == uuid.Nil {
		t.Error("expected a generated account id")
	}
	if a.Email != "user@example.com" {
		t.Errorf("expected canonical email, got %q", a.Email)
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte("password123")) != nil {
		t.Error("stored password hash does not verify")
	}
	if token == "" {
		t.Error("expected a minted token")
	}
}

func TestRegisterValidation(t *testing.T) {
	svc := newTestAccountService(newFakeRepo())

	for _, tc := range []struct {
		name, email, password string
	}{
		{"empty email", "", "password123"},
		{"malformed email", "not-an-email", "password123"},
		{"short password", "user@example.com", "short"},
		{"over-long password", "user@example.com", strings.Repeat("x", 73)},
	} {
		if _, _, err := svc.Register(context.Background(), tc.email, tc.password); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: expected ErrValidation, got %v", tc.name, err)
		}
	}
}

func TestRegisterConflict(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestAccountService(repo)

	if _, _, err := svc.Register(context.Background(), "user@example.com", "password123"); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	// Same email, different case: the repo normalizes, so this is a conflict.
	if _, _, err := svc.Register(context.Background(), "User@Example.com", "password456"); !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestLoginSuccess(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestAccountService(repo)

	if _, _, err := svc.Register(context.Background(), "user@example.com", "password123"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	a, token, err := svc.Login(context.Background(), " user@example.com ", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if a.Email != "user@example.com" || token == "" {
		t.Errorf("unexpected login result: email=%q token=%q", a.Email, token)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestAccountService(repo)

	if _, _, err := svc.Register(context.Background(), "user@example.com", "password123"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if _, _, err := svc.Login(context.Background(), "user@example.com", "wrong-password"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestLoginRejectsUnknownEmail(t *testing.T) {
	svc := newTestAccountService(newFakeRepo())

	if _, _, err := svc.Login(context.Background(), "nobody@example.com", "password123"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected generic ErrUnauthorized, got %v", err)
	}
}

func TestLoginWrapsRepositoryFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.findErr = errors.New("database down")
	svc := newTestAccountService(repo)

	_, _, err := svc.Login(context.Background(), "user@example.com", "password123")
	if err == nil || errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFound) {
		t.Errorf("expected a wrapped repository error, got %v", err)
	}
}
