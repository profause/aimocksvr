package account

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/database"
	"github.com/profause/aimocksvr/internal/models"
)

// newTestDB connects to PostgreSQL. The test is skipped when
// MOCKSVR_TEST_DATABASE_URL is not set, so the suite runs without a database.
func newTestDB(t *testing.T) Repository {
	t.Helper()

	url := os.Getenv("MOCKSVR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("MOCKSVR_TEST_DATABASE_URL not set; skipping repository integration test")
	}

	cfg := &config.Config{Database: config.Database{URL: url}}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		"DROP TABLE IF EXISTS accounts, mock_resources, request_history, endpoint_versions, endpoints, schema_migrations CASCADE"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	if err := database.Migrate(url); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return NewRepository(db)
}

func TestAccountRepositoryCreateFind(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	a := &models.Account{
		ID:           uuid.New(),
		Email:        "  User@Example.COM ",
		PasswordHash: "$2a$10$unittesthash000000000000000000000000000000000000000",
	}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if a.Email != "user@example.com" {
		t.Errorf("Create did not canonicalize email, stored %q", a.Email)
	}
	if a.CreatedAt.IsZero() {
		t.Error("Create did not populate CreatedAt")
	}

	found, err := repo.FindByEmail(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("FindByEmail failed: %v", err)
	}
	if found.ID != a.ID || found.Email != "user@example.com" {
		t.Errorf("unexpected account: %+v", found)
	}
	if found.PasswordHash != a.PasswordHash {
		t.Errorf("password hash did not round-trip: %q != %q", found.PasswordHash, a.PasswordHash)
	}

	found, err = repo.FindByEmail(ctx, "User@Example.com")
	if err != nil {
		t.Fatalf("FindByEmail (mixed case) failed: %v", err)
	}
	if found.ID != a.ID {
		t.Errorf("mixed-case lookup returned wrong account: %v", found.ID)
	}

	if _, err := repo.FindByEmail(ctx, "nobody@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown email, got %v", err)
	}
}

func TestAccountRepositoryConflict(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	hash := "$2a$10$unittesthash000000000000000000000000000000000000000"
	if err := repo.Create(ctx, &models.Account{
		ID:           uuid.New(),
		Email:        "user@example.com",
		PasswordHash: hash,
	}); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	// Same email in different case must collide after normalization.
	if err := repo.Create(ctx, &models.Account{
		ID:           uuid.New(),
		Email:        "USER@example.com",
		PasswordHash: hash,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate email, got %v", err)
	}
}
