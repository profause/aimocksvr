package account

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/profause/aimocksvr/internal/auth"
	"github.com/profause/aimocksvr/internal/models"
)

var (
	// ErrValidation is returned for malformed registration input.
	ErrValidation = errors.New("invalid registration")
	// ErrUnauthorized is returned on any login failure. The message is generic
	// so callers cannot tell whether the email or the password was wrong.
	ErrUnauthorized = errors.New("invalid credentials")
)

// Service implements account registration and login and mints account-bound
// JWTs through the auth Service.
type Service struct {
	repo Repository
	auth *auth.Service
}

// NewService builds an account Service. The email address is canonicalized by
// the repository on both write and read, so the service may pass strings
// through untouched.
func NewService(repo Repository, authSvc *auth.Service) *Service {
	return &Service{repo: repo, auth: authSvc}
}

// Register validates the credentials, stores a bcrypt hash and returns the new
// account with an account-bound JWT.
func (s *Service) Register(ctx context.Context, email, password string) (*models.Account, string, error) {
	if err := validateCredentials(email, password); err != nil {
		return nil, "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	a := &models.Account{ID: uuid.New(), Email: email, PasswordHash: string(hash)}
	if err := s.repo.Create(ctx, a); err != nil {
		if errors.Is(err, ErrConflict) {
			return nil, "", ErrConflict
		}
		return nil, "", fmt.Errorf("create account: %w", err)
	}

	token, err := s.auth.MintAccountJWT(a.ID, a.Email)
	if err != nil {
		return nil, "", fmt.Errorf("mint account jwt: %w", err)
	}
	return a, token, nil
}

// dummyPasswordHash is compared against on the unauthorized path so unknown
// emails take roughly as long as a real bcrypt check, closing the login timing
// side channel that would otherwise reveal whether an email is registered.
var dummyPasswordHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

// Login verifies the password against the stored bcrypt hash and returns the
// account with a fresh account-bound JWT. Every failure returns the same
// ErrUnauthorized to avoid revealing whether the email is registered.
func (s *Service) Login(ctx context.Context, email, password string) (*models.Account, string, error) {
	a, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Equalize latency with the wrong-password path.
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
			return nil, "", ErrUnauthorized
		}
		return nil, "", fmt.Errorf("find account: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)) != nil {
		return nil, "", ErrUnauthorized
	}

	token, err := s.auth.MintAccountJWT(a.ID, a.Email)
	if err != nil {
		return nil, "", fmt.Errorf("mint account jwt: %w", err)
	}
	return a, token, nil
}

// validateCredentials enforces the registration rules: a parseable email and a
// password between 8 and 72 bytes (72 is bcrypt's input limit).
func validateCredentials(email, password string) error {
	if !validEmail(strings.TrimSpace(email)) {
		return fmt.Errorf("%w: invalid email", ErrValidation)
	}
	if len(password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrValidation)
	}
	if len(password) > 72 {
		return fmt.Errorf("%w: password must be at most 72 bytes", ErrValidation)
	}
	return nil
}

// validEmail reports whether email is a single parseable address with no
// display-name decoration.
func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}
