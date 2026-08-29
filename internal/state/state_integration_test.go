package state

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/database"
	"github.com/profause/aimocksvr/internal/models"
)

// testEnv bundles the store and the raw DB so tests can seed accounts
// (mock resources reference accounts via a foreign key).
type testEnv struct {
	store Store
	db    *bun.DB
}

// newTestStore connects to PostgreSQL. The test is skipped when
// MOCKSVR_TEST_DATABASE_URL is not set, so the suite runs without a database.
func newTestStore(t *testing.T) *testEnv {
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
		"DROP TABLE IF EXISTS mock_resources, request_history, endpoint_versions, endpoints, accounts, schema_migrations CASCADE"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	if err := database.Migrate(url); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return &testEnv{store: NewStore(db), db: db}
}

// newAccount inserts an account and returns its id so tests can own resources
// and exercise cross-account isolation.
func (e *testEnv) newAccount(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := e.db.ExecContext(ctx,
		"INSERT INTO accounts (id, email, password_hash) VALUES (?, ?, ?)",
		id, id.String()+"@example.com", "$2a$10$dummy"); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return id
}

func TestStoreCRUD(t *testing.T) {
	store := newTestStore(t).store
	ctx := context.Background()

	if err := store.Create(ctx, models.LegacyAccountID, "/users", "1", map[string]any{"id": 1, "name": "Ada"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	data, found, err := store.Get(ctx, models.LegacyAccountID, "/users", "1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatalf("expected resource to be found")
	}
	if data["name"] != "Ada" {
		t.Errorf("unexpected data: %+v", data)
	}

	if err := store.Update(ctx, models.LegacyAccountID, "/users", "1", map[string]any{"id": 1, "name": "Bob"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	data, _, _ = store.Get(ctx, models.LegacyAccountID, "/users", "1")
	if data["name"] != "Bob" {
		t.Errorf("expected updated name, got %+v", data)
	}

	ok, err := store.Delete(ctx, models.LegacyAccountID, "/users", "1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !ok {
		t.Errorf("expected delete to report the resource existed")
	}
	if _, found, _ := store.Get(ctx, models.LegacyAccountID, "/users", "1"); found {
		t.Errorf("expected resource to be gone")
	}
}

func TestStoreCollectionsAreIndependent(t *testing.T) {
	store := newTestStore(t).store
	ctx := context.Background()

	if err := store.Create(ctx, models.LegacyAccountID, "/users", "1", map[string]any{"name": "Ada"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, found, _ := store.Get(ctx, models.LegacyAccountID, "/orders", "1"); found {
		t.Errorf("expected /orders to be independent of /users")
	}
}

func TestStoreConflict(t *testing.T) {
	store := newTestStore(t).store
	ctx := context.Background()

	if err := store.Create(ctx, models.LegacyAccountID, "/users", "1", map[string]any{"name": "Ada"}); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	if err := store.Create(ctx, models.LegacyAccountID, "/users", "1", map[string]any{"name": "Bob"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestStoreNotFound(t *testing.T) {
	store := newTestStore(t).store
	ctx := context.Background()

	if _, found, err := store.Get(ctx, models.LegacyAccountID, "/users", "nope"); err != nil || found {
		t.Fatalf("expected not found (found=%v err=%v)", found, err)
	}
	if err := store.Update(ctx, models.LegacyAccountID, "/users", "nope", map[string]any{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on update, got %v", err)
	}
	if ok, err := store.Delete(ctx, models.LegacyAccountID, "/users", "nope"); err != nil || ok {
		t.Fatalf("expected not found delete (ok=%v err=%v)", ok, err)
	}
}

func TestStoreCrossAccountIsolation(t *testing.T) {
	env := newTestStore(t)
	ctx := context.Background()

	accountA := env.newAccount(ctx, t)
	accountB := env.newAccount(ctx, t)

	// Account A creates a resource in the shared /users collection.
	if err := env.store.Create(ctx, accountA, "/users", "1", map[string]any{"id": "1", "name": "Ada"}); err != nil {
		t.Fatalf("Create as A failed: %v", err)
	}

	// Account B cannot see A's resource...
	if _, found, err := env.store.Get(ctx, accountB, "/users", "1"); err != nil {
		t.Fatalf("Get as B failed: %v", err)
	} else if found {
		t.Fatalf("expected resource owned by A to be invisible to B")
	}
	// ...cannot update it...
	if err := env.store.Update(ctx, accountB, "/users", "1", map[string]any{"id": "1", "name": "Bob"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound updating A's resource as B, got %v", err)
	}
	// ...and cannot delete it.
	if ok, err := env.store.Delete(ctx, accountB, "/users", "1"); err != nil {
		t.Fatalf("Delete as B failed: %v", err)
	} else if ok {
		t.Fatalf("expected B's delete of A's resource to report not found")
	}

	// The same collection and resource id are free for B: per-account
	// namespacing means A's row does not collide with B's.
	if err := env.store.Create(ctx, accountB, "/users", "1", map[string]any{"id": "1", "name": "Grace"}); err != nil {
		t.Fatalf("Create as B in the shared namespace failed: %v", err)
	}
	data, found, err := env.store.Get(ctx, accountB, "/users", "1")
	if err != nil || !found {
		t.Fatalf("expected B's resource to be readable (found=%v err=%v)", found, err)
	}
	if data["name"] != "Grace" {
		t.Errorf("unexpected B data: %+v", data)
	}

	// Each account still sees its own copy untouched.
	if data, found, _ := env.store.Get(ctx, accountA, "/users", "1"); !found || data["name"] != "Ada" {
		t.Errorf("expected A's copy to be untouched, found=%v data=%+v", found, data)
	}
}
