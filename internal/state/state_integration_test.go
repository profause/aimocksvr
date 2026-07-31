package state

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/database"
)

// newTestStore connects to PostgreSQL. The test is skipped when
// MOCKSVR_TEST_DATABASE_URL is not set, so the suite runs without a database.
func newTestStore(t *testing.T) Store {
	t.Helper()

	url := os.Getenv("MOCKSVR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("MOCKSVR_TEST_DATABASE_URL not set; skipping store integration test")
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

	return NewStore(db)
}

func TestStoreCRUD(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Create(ctx, "/users", "1", map[string]any{"id": 1, "name": "Ada"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	data, found, err := store.Get(ctx, "/users", "1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatalf("expected resource to be found")
	}
	if data["name"] != "Ada" {
		t.Errorf("unexpected data: %+v", data)
	}

	if err := store.Update(ctx, "/users", "1", map[string]any{"id": 1, "name": "Bob"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	data, _, _ = store.Get(ctx, "/users", "1")
	if data["name"] != "Bob" {
		t.Errorf("expected updated name, got %+v", data)
	}

	ok, err := store.Delete(ctx, "/users", "1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !ok {
		t.Errorf("expected delete to report the resource existed")
	}
	if _, found, _ := store.Get(ctx, "/users", "1"); found {
		t.Errorf("expected resource to be gone")
	}
}

func TestStoreCollectionsAreIndependent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Create(ctx, "/users", "1", map[string]any{"name": "Ada"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, found, _ := store.Get(ctx, "/orders", "1"); found {
		t.Errorf("expected /orders to be independent of /users")
	}
}

func TestStoreConflict(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Create(ctx, "/users", "1", map[string]any{"name": "Ada"}); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	if err := store.Create(ctx, "/users", "1", map[string]any{"name": "Bob"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestStoreNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if _, found, err := store.Get(ctx, "/users", "nope"); err != nil || found {
		t.Fatalf("expected not found (found=%v err=%v)", found, err)
	}
	if err := store.Update(ctx, "/users", "nope", map[string]any{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on update, got %v", err)
	}
	if ok, err := store.Delete(ctx, "/users", "nope"); err != nil || ok {
		t.Fatalf("expected not found delete (ok=%v err=%v)", ok, err)
	}
}
