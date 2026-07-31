package endpoint

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
		"DROP TABLE IF EXISTS mock_resources, request_history, endpoint_versions, endpoints, schema_migrations CASCADE"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	if err := database.Migrate(url); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return NewRepository(db)
}

func TestRepositoryCRUD(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	e := &models.Endpoint{
		ID:           uuid.New(),
		Method:       "GET",
		Path:         "/users/:id",
		Description:  "fetch one user",
		Prompt:       "return one user",
		ResponseType: models.ResponseTypeJSON,
		Status:       models.StatusActive,
	}
	if err := repo.Create(ctx, e); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	found, err := repo.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found.Path != e.Path || found.Method != "GET" {
		t.Errorf("unexpected endpoint: %+v", found)
	}

	e.Prompt = "return one user with a company"
	if err := repo.Update(ctx, e); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if err := repo.CreateVersion(ctx, &models.EndpointVersion{
		ID:         uuid.New(),
		EndpointID: e.ID,
		Prompt:     e.Prompt,
		Version:    1,
	}); err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	versions, err := repo.ListVersions(ctx, e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0].Version != 1 {
		t.Fatalf("expected one version, got %+v", versions)
	}

	if err := repo.Delete(ctx, e.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := repo.FindByID(ctx, e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestRepositoryConflict(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	makeEndpoint := func() *models.Endpoint {
		return &models.Endpoint{
			ID:           uuid.New(),
			Method:       "POST",
			Path:         "/users",
			Prompt:       "create a user",
			ResponseType: models.ResponseTypeJSON,
			Status:       models.StatusActive,
		}
	}

	if err := repo.Create(ctx, makeEndpoint()); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	if err := repo.Create(ctx, makeEndpoint()); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestRepositoryList(t *testing.T) {
	repo := newTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		e := &models.Endpoint{
			ID:           uuid.New(),
			Method:       "GET",
			Path:         "/items/" + string(rune('a'+i)),
			Prompt:       "return an item",
			ResponseType: models.ResponseTypeJSON,
			Status:       models.StatusActive,
		}
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	endpoints, total, err := repo.List(ctx, ListParams{Page: 1, Limit: 2})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(endpoints))
	}
}
