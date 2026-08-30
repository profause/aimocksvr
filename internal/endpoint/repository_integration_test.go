package endpoint

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

// testEnv bundles the repository and the raw DB so tests can seed accounts
// (endpoints reference accounts via a foreign key).
type testEnv struct {
	repo Repository
	db   *bun.DB
}

// newTestDB connects to PostgreSQL. The test is skipped when
// MOCKSVR_TEST_DATABASE_URL is not set, so the suite runs without a database.
func newTestDB(t *testing.T) *testEnv {
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
		"DROP TABLE IF EXISTS mock_resources, request_history, endpoint_versions, endpoints, accounts, schema_migrations CASCADE"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	if err := database.Migrate(url); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return &testEnv{repo: NewRepository(db), db: db}
}

// newAccount inserts an account and returns its id so tests can own endpoints
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

func TestRepositoryCRUD(t *testing.T) {
	env := newTestDB(t)
	repo := env.repo
	ctx := context.Background()
	owner := env.newAccount(ctx, t)
	foreign := env.newAccount(ctx, t)

	e := &models.Endpoint{
		ID:           uuid.New(),
		AccountID:    owner,
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

	found, err := repo.FindByID(ctx, owner, e.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found.Path != e.Path || found.Method != "GET" {
		t.Errorf("unexpected endpoint: %+v", found)
	}

	if _, err := repo.FindByID(ctx, foreign, e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for foreign owner, got %v", err)
	}

	e.Prompt = "return one user with a company"
	if err := repo.Update(ctx, owner, e); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if err := repo.CreateVersion(ctx, &models.EndpointVersion{
		ID:         uuid.New(),
		EndpointID: e.ID,
		AccountID:  owner,
		Prompt:     e.Prompt,
		Version:    1,
	}); err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	versions, err := repo.ListVersions(ctx, owner, e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0].Version != 1 {
		t.Fatalf("expected one version, got %+v", versions)
	}

	// The version rows are scoped by owner at the repository layer: a foreign
	// account must not see them, even when it knows the endpoint id.
	if foreignVersions, err := repo.ListVersions(ctx, foreign, e.ID); err != nil {
		t.Fatalf("ListVersions for foreign owner failed: %v", err)
	} else if len(foreignVersions) != 0 {
		t.Fatalf("expected no versions for foreign owner, got %+v", foreignVersions)
	}

	if err := repo.Delete(ctx, owner, e.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := repo.FindByID(ctx, owner, e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestRepositoryConflict(t *testing.T) {
	env := newTestDB(t)
	repo := env.repo
	ctx := context.Background()
	owner := env.newAccount(ctx, t)
	other := env.newAccount(ctx, t)

	makeEndpoint := func() *models.Endpoint {
		return &models.Endpoint{
			ID:           uuid.New(),
			AccountID:    owner,
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

	// The same method+path may be owned by a different account.
	foreign := &models.Endpoint{
		ID:           uuid.New(),
		AccountID:    other,
		Method:       "POST",
		Path:         "/users",
		Prompt:       "another account's user create",
		ResponseType: models.ResponseTypeJSON,
		Status:       models.StatusActive,
	}
	if err := repo.Create(ctx, foreign); err != nil {
		t.Fatalf("cross-account Create failed: %v", err)
	}
}

func TestRepositoryPersistsPublicFlag(t *testing.T) {
	env := newTestDB(t)
	repo := env.repo
	ctx := context.Background()
	owner := env.newAccount(ctx, t)

	// An endpoint created without an explicit public flag stays private. This
	// guards the bun insert path: a zero-valued bool tagged as the column
	// default must not silently resolve to the (old) public default at the
	// SQL layer.
	implicit := &models.Endpoint{
		ID:           uuid.New(),
		AccountID:    owner,
		Method:       "GET",
		Path:         "/implicit",
		Prompt:       "private by default",
		ResponseType: models.ResponseTypeJSON,
		Status:       models.StatusActive,
	}
	if err := repo.Create(ctx, implicit); err != nil {
		t.Fatalf("Create implicit failed: %v", err)
	}

	stored, err := repo.FindByID(ctx, owner, implicit.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if stored.Public {
		t.Error("expected public=false to persist, got true (bun DEFAULT override?)")
	}

	active, err := repo.ListActiveByMethod(ctx, "GET")
	if err != nil {
		t.Fatalf("ListActiveByMethod failed: %v", err)
	}
	for _, e := range active {
		if e.ID == implicit.ID && e.Public {
			t.Error("ListActiveByMethod returned public=true for an implicitly private endpoint")
		}
	}

	// An explicit public:true is honored and read back as true.
	explicit := &models.Endpoint{
		ID:           uuid.New(),
		AccountID:    owner,
		Method:       "GET",
		Path:         "/explicit",
		Prompt:       "explicitly public",
		ResponseType: models.ResponseTypeJSON,
		Status:       models.StatusActive,
		Public:       true,
	}
	if err := repo.Create(ctx, explicit); err != nil {
		t.Fatalf("Create explicit failed: %v", err)
	}
	stored, err = repo.FindByID(ctx, owner, explicit.ID)
	if err != nil {
		t.Fatalf("FindByID explicit failed: %v", err)
	}
	if !stored.Public {
		t.Error("expected public=true to persist, got false")
	}
}

func TestRepositoryList(t *testing.T) {
	env := newTestDB(t)
	repo := env.repo
	ctx := context.Background()
	owner := env.newAccount(ctx, t)

	for i := 0; i < 3; i++ {
		e := &models.Endpoint{
			ID:           uuid.New(),
			AccountID:    owner,
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

	// A different account owns this endpoint and must stay isolated.
	foreignOwner := env.newAccount(ctx, t)
	foreign := &models.Endpoint{
		ID:           uuid.New(),
		AccountID:    foreignOwner,
		Method:       "GET",
		Path:         "/foreign/only",
		Prompt:       "return a foreign item",
		ResponseType: models.ResponseTypeJSON,
		Status:       models.StatusActive,
	}
	if err := repo.Create(ctx, foreign); err != nil {
		t.Fatalf("Create foreign failed: %v", err)
	}

	endpoints, total, err := repo.List(ctx, owner, ListParams{Page: 1, Limit: 2})
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
