package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/profause/aimocksvr/internal/models"
)

// ErrNotFound is returned when no account exists for the given email.
var ErrNotFound = errors.New("account not found")

// ErrConflict is returned when an account with the same email already exists.
var ErrConflict = errors.New("account already exists")

// mapConflict translates PostgreSQL integrity violations into ErrConflict.
func mapConflict(err error) error {
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) && pgErr.IntegrityViolation() {
		return ErrConflict
	}
	return err
}

// normalizeEmail trims surrounding whitespace and lower-cases the address so
// stored and looked-up emails always share one canonical form.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Repository persists accounts.
type Repository interface {
	// Create inserts a new account. It returns ErrConflict when the email is
	// already taken.
	Create(ctx context.Context, a *models.Account) error
	// FindByEmail returns the account matching email, compared after
	// normalization, or ErrNotFound when no account exists.
	FindByEmail(ctx context.Context, email string) (*models.Account, error)
}

type repository struct {
	db *bun.DB
}

// NewRepository creates a PostgreSQL-backed Repository.
func NewRepository(db *bun.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, a *models.Account) error {
	// Canonicalize before insert so every stored email is lower-case.
	a.Email = normalizeEmail(a.Email)
	if _, err := r.db.NewInsert().Model(a).Returning("*").Exec(ctx); err != nil {
		return fmt.Errorf("insert account: %w", mapConflict(err))
	}
	return nil
}

func (r *repository) FindByEmail(ctx context.Context, email string) (*models.Account, error) {
	a := new(models.Account)
	if err := r.db.NewSelect().Model(a).Where("email = ?", normalizeEmail(email)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find account: %w", err)
	}
	return a, nil
}
