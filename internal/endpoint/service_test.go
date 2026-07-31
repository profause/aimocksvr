package endpoint

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/ai"
	"github.com/profause/aimocksvr/internal/cache"
	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/validator"
)

// fakeRepo is an in-memory Repository used by service unit tests.
type fakeRepo struct {
	endpoints map[uuid.UUID]*models.Endpoint
	versions  []models.EndpointVersion
	history   []models.RequestHistory
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{endpoints: make(map[uuid.UUID]*models.Endpoint)}
}

func (f *fakeRepo) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (f *fakeRepo) Create(ctx context.Context, e *models.Endpoint) error {
	for _, existing := range f.endpoints {
		if existing.Method == e.Method && existing.Path == e.Path {
			return ErrConflict
		}
	}
	f.endpoints[e.ID] = e
	return nil
}

func (f *fakeRepo) Update(ctx context.Context, e *models.Endpoint) error {
	if _, ok := f.endpoints[e.ID]; !ok {
		return ErrNotFound
	}
	f.endpoints[e.ID] = e
	return nil
}

func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := f.endpoints[id]; !ok {
		return ErrNotFound
	}
	delete(f.endpoints, id)
	return nil
}

func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.Endpoint, error) {
	e, ok := f.endpoints[id]
	if !ok {
		return nil, ErrNotFound
	}
	return e, nil
}

func (f *fakeRepo) List(ctx context.Context, p ListParams) ([]models.Endpoint, int, error) {
	all := make([]models.Endpoint, 0, len(f.endpoints))
	for _, e := range f.endpoints {
		all = append(all, *e)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	total := len(all)
	start := (p.Page - 1) * p.Limit
	if start > total {
		start = total
	}
	end := start + p.Limit
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (f *fakeRepo) CreateVersion(ctx context.Context, v *models.EndpointVersion) error {
	f.versions = append(f.versions, *v)
	return nil
}

func (f *fakeRepo) ListVersions(ctx context.Context, endpointID uuid.UUID) ([]models.EndpointVersion, error) {
	var out []models.EndpointVersion
	for _, v := range f.versions {
		if v.EndpointID == endpointID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Version > out[j].Version
	})
	return out, nil
}

func (f *fakeRepo) ListActiveByMethod(ctx context.Context, method string) ([]models.Endpoint, error) {
	var out []models.Endpoint
	for _, e := range f.endpoints {
		if e.Method == method && e.Status == models.StatusActive {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (f *fakeRepo) CreateHistory(ctx context.Context, h *models.RequestHistory) error {
	f.history = append(f.history, *h)
	return nil
}

func (f *fakeRepo) ListHistory(ctx context.Context, endpointID uuid.UUID) ([]models.RequestHistory, error) {
	var out []models.RequestHistory
	for _, h := range f.history {
		if h.EndpointID == endpointID {
			out = append(out, h)
		}
	}
	return out, nil
}

func newTestService() Service {
	return newServiceWithRepo(newFakeRepo())
}

func newServiceWithRepo(repo Repository) Service {
	return newServiceWithAI(repo, ai.Noop{})
}

// newServiceWithAI builds a Service with a configurable AI provider and the
// real JSON Schema validator, so schema generation and vetting are exercised.
func newServiceWithAI(repo Repository, a ai.AIProvider) Service {
	logger := zerolog.Nop()
	return NewService(repo, cache.Noop{}, a, validator.New(), &logger)
}

// fakeAI is a controllable AIProvider for service tests.
type fakeAI struct {
	schemas []*ai.Schema
	err     error
	calls   int
}

func (f *fakeAI) GenerateSchema(_ context.Context, _ ai.SchemaRequest) (*ai.Schema, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.calls <= len(f.schemas) {
		return f.schemas[f.calls-1], nil
	}
	return nil, nil
}

func (f *fakeAI) GenerateResponse(context.Context, ai.ResponseRequest) (*ai.Response, error) {
	return nil, f.err
}

func (f *fakeAI) GeneratePrompt(context.Context, ai.PromptRequest) (string, error) {
	return "", f.err
}

// recordingCache records deleted keys so tests can assert cache invalidation.
type recordingCache struct {
	deleted []string
}

func (r *recordingCache) Get(context.Context, string, any) (bool, error) {
	return false, nil
}

func (r *recordingCache) Set(context.Context, string, any, time.Duration) error {
	return nil
}

func (r *recordingCache) Delete(_ context.Context, keys ...string) error {
	r.deleted = append(r.deleted, keys...)
	return nil
}

func TestServiceCreate(t *testing.T) {
	svc := newTestService()

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "get",
		Path:   "/users",
		Prompt: "return a user",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if e.Method != "GET" {
		t.Errorf("expected method GET, got %q", e.Method)
	}
	if e.ResponseType != models.ResponseTypeJSON {
		t.Errorf("expected response_type %q, got %q", models.ResponseTypeJSON, e.ResponseType)
	}
	if e.Status != models.StatusActive {
		t.Errorf("expected status %q, got %q", models.StatusActive, e.Status)
	}
	if e.ID == uuid.Nil {
		t.Error("expected endpoint to have a generated id")
	}

	versions, err := svc.ListVersions(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ListVersions returned error: %v", err)
	}
	if len(versions) != 1 || versions[0].Version != 1 {
		t.Fatalf("expected a single version 1, got %+v", versions)
	}
}

func TestServiceCreateValidation(t *testing.T) {
	tests := []struct {
		name   string
		params CreateEndpointParams
		field  string
	}{
		{name: "missing method", params: CreateEndpointParams{Path: "/x", Prompt: "p"}, field: "method"},
		{name: "invalid method", params: CreateEndpointParams{Method: "FETCH", Path: "/x", Prompt: "p"}, field: "method"},
		{name: "missing path", params: CreateEndpointParams{Method: "GET", Prompt: "p"}, field: "path"},
		{name: "path without slash", params: CreateEndpointParams{Method: "GET", Path: "users", Prompt: "p"}, field: "path"},
		{name: "missing prompt", params: CreateEndpointParams{Method: "GET", Path: "/users"}, field: "prompt"},
		{name: "invalid response type", params: CreateEndpointParams{Method: "GET", Path: "/users", Prompt: "p", ResponseType: "xml"}, field: "response_type"},
		{name: "invalid status", params: CreateEndpointParams{Method: "GET", Path: "/users", Prompt: "p", Status: "archived"}, field: "status"},
	}

	svc := newTestService()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
			if validationErr.Field != tt.field {
				t.Errorf("expected field %q, got %q", tt.field, validationErr.Field)
			}
		})
	}
}

func TestServiceCreateConflict(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithRepo(repo)

	in := CreateEndpointParams{Method: "POST", Path: "/users", Prompt: "create a user"}
	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	if _, err := svc.Create(context.Background(), in); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestServiceUpdateCreatesVersionOnPromptChange(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithRepo(repo)

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "GET",
		Path:   "/users/:id",
		Prompt: "return one user",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := svc.Update(context.Background(), e.ID, UpdateEndpointParams{
		Method: "GET",
		Path:   "/users/:id",
		Prompt: "return one user with company details",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Prompt != "return one user with company details" {
		t.Errorf("prompt not updated: %q", updated.Prompt)
	}

	versions, err := svc.ListVersions(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions after prompt change, got %d", len(versions))
	}
	if versions[0].Version != 2 || versions[1].Version != 1 {
		t.Errorf("expected versions [2,1], got [%d,%d]", versions[0].Version, versions[1].Version)
	}
}

func TestServiceUpdateWithoutPromptChange(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithRepo(repo)

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "GET",
		Path:   "/users",
		Prompt: "return users",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = svc.Update(context.Background(), e.ID, UpdateEndpointParams{
		Method: "GET",
		Path:   "/users",
		Prompt: "return users",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	versions, err := svc.ListVersions(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(versions))
	}
}

func TestServiceGetNotFound(t *testing.T) {
	svc := newTestService()

	_, err := svc.Get(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestServiceDelete(t *testing.T) {
	svc := newTestService()

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "DELETE",
		Path:   "/users/:id",
		Prompt: "delete a user",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := svc.Delete(context.Background(), e.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if err := svc.Delete(context.Background(), e.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestServiceListPagination(t *testing.T) {
	svc := newTestService()

	for i := 0; i < 5; i++ {
		if _, err := svc.Create(context.Background(), CreateEndpointParams{
			Method: "GET",
			Path:   "/resource/" + string(rune('a'+i)),
			Prompt: "p",
		}); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	endpoints, total, err := svc.List(context.Background(), ListParams{Page: 2, Limit: 2})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(endpoints) != 2 {
		t.Errorf("expected 2 endpoints on page 2, got %d", len(endpoints))
	}
}

func TestServiceCreateInvalidatesCache(t *testing.T) {
	rc := &recordingCache{}
	logger := zerolog.Nop()
	svc := NewService(newFakeRepo(), rc, ai.Noop{}, validator.New(), &logger)

	if _, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "POST",
		Path:   "/users",
		Prompt: "create a user",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	want := cache.EndpointListKey("POST")
	if !slices.Contains(rc.deleted, want) {
		t.Errorf("expected cache key %q to be invalidated, got %v", want, rc.deleted)
	}
}

func TestServiceUpdateInvalidatesOldAndNewMethod(t *testing.T) {
	rc := &recordingCache{}
	logger := zerolog.Nop()
	svc := NewService(newFakeRepo(), rc, ai.Noop{}, validator.New(), &logger)

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "POST",
		Path:   "/users",
		Prompt: "create a user",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	rc.deleted = nil

	if _, err := svc.Update(context.Background(), e.ID, UpdateEndpointParams{
		Method: "PUT",
		Path:   "/users/:id",
		Prompt: "replace a user",
	}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	for _, key := range []string{cache.EndpointListKey("POST"), cache.EndpointListKey("PUT")} {
		if !slices.Contains(rc.deleted, key) {
			t.Errorf("expected cache key %q to be invalidated, got %v", key, rc.deleted)
		}
	}
}

func TestServiceDeleteInvalidatesCache(t *testing.T) {
	rc := &recordingCache{}
	logger := zerolog.Nop()
	svc := NewService(newFakeRepo(), rc, ai.Noop{}, validator.New(), &logger)

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "DELETE",
		Path:   "/users/:id",
		Prompt: "delete a user",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	rc.deleted = nil

	if err := svc.Delete(context.Background(), e.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	want := cache.EndpointListKey("DELETE")
	if !slices.Contains(rc.deleted, want) {
		t.Errorf("expected cache key %q to be invalidated, got %v", want, rc.deleted)
	}
}

func schemaOfType(t string) *ai.Schema {
	return &ai.Schema{"type": t}
}

func TestServiceCreateStoresGeneratedSchema(t *testing.T) {
	a := &fakeAI{schemas: []*ai.Schema{schemaOfType("object")}}
	svc := newServiceWithAI(newFakeRepo(), a)

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "POST",
		Path:   "/users",
		Prompt: "create a user",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	versions, err := svc.ListVersions(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0].Schema != `{"type":"object"}` {
		t.Errorf("expected stored schema, got %q", versions[0].Schema)
	}
	if a.calls != 1 {
		t.Errorf("expected 1 schema generation call, got %d", a.calls)
	}
}

func TestServiceCreateAIErrorKeepsSchemaEmpty(t *testing.T) {
	a := &fakeAI{err: errors.New("ai down")}
	svc := newServiceWithAI(newFakeRepo(), a)

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "POST",
		Path:   "/users",
		Prompt: "create a user",
	})
	if err != nil {
		t.Fatalf("Create should succeed despite AI failure, got %v", err)
	}

	versions, err := svc.ListVersions(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if versions[0].Schema != "" {
		t.Errorf("expected empty schema on AI failure, got %q", versions[0].Schema)
	}
}

func TestServiceCreateRejectsInvalidGeneratedSchema(t *testing.T) {
	a := &fakeAI{schemas: []*ai.Schema{{"type": "not-a-real-type"}}}
	svc := newServiceWithAI(newFakeRepo(), a)

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "POST",
		Path:   "/users",
		Prompt: "create a user",
	})
	if err != nil {
		t.Fatalf("Create should succeed despite invalid schema, got %v", err)
	}

	versions, err := svc.ListVersions(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if versions[0].Schema != "" {
		t.Errorf("expected empty schema when AI output fails validation, got %q", versions[0].Schema)
	}
}

func TestServiceUpdateRegeneratesSchemaOnPromptChange(t *testing.T) {
	a := &fakeAI{schemas: []*ai.Schema{schemaOfType("object"), schemaOfType("array")}}
	svc := newServiceWithAI(newFakeRepo(), a)

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "GET",
		Path:   "/users",
		Prompt: "return a user",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := svc.Update(context.Background(), e.ID, UpdateEndpointParams{
		Method: "GET",
		Path:   "/users",
		Prompt: "return users",
	}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	versions, err := svc.ListVersions(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Version != 2 || versions[0].Schema != `{"type":"array"}` {
		t.Errorf("expected version 2 with regenerated schema, got %+v", versions[0])
	}
	if versions[1].Version != 1 || versions[1].Schema != `{"type":"object"}` {
		t.Errorf("expected version 1 to keep its schema, got %+v", versions[1])
	}
	if a.calls != 2 {
		t.Errorf("expected 2 schema generation calls, got %d", a.calls)
	}
}

func TestServiceUpdateKeepsSchemaWhenPromptUnchanged(t *testing.T) {
	a := &fakeAI{schemas: []*ai.Schema{schemaOfType("object")}}
	svc := newServiceWithAI(newFakeRepo(), a)

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "GET",
		Path:   "/users",
		Prompt: "return a user",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := svc.Update(context.Background(), e.ID, UpdateEndpointParams{
		Method: "GET",
		Path:   "/users",
		Prompt: "return a user",
	}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	versions, err := svc.ListVersions(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version when prompt unchanged, got %d", len(versions))
	}
	if a.calls != 1 {
		t.Errorf("expected no extra schema generation, got %d calls", a.calls)
	}
}

func TestServiceImportCreatesEndpoints(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithRepo(repo)

	res, err := svc.Import(context.Background(), []ImportItem{
		{Method: "GET", Path: "/users", Prompt: "List users.", Schema: `{"type":"object","properties":{"id":{"type":"string","format":"uuid"}}}`},
		{Method: "POST", Path: "/users", Prompt: "Create a user.", Schema: ""},
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if res.Created != 2 || res.Skipped != 0 {
		t.Fatalf("result = %+v, want created=2 skipped=0", res)
	}
	if len(res.Endpoints) != 2 {
		t.Fatalf("expected 2 created endpoints, got %d", len(res.Endpoints))
	}

	e := res.Endpoints[0]
	if e.Method != "GET" || e.Path != "/users" {
		t.Errorf("unexpected endpoint %s %s", e.Method, e.Path)
	}
	if e.ResponseType != models.ResponseTypeJSON || e.Status != models.StatusActive {
		t.Errorf("unexpected defaults: %+v", e)
	}

	versions, err := svc.ListVersions(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0].Version != 1 {
		t.Fatalf("expected a single version 1, got %+v", versions)
	}
	if versions[0].Schema != `{"type":"object","properties":{"id":{"type":"string","format":"uuid"}}}` {
		t.Errorf("expected imported schema stored, got %q", versions[0].Schema)
	}
}

func TestServiceImportSkipsConflicts(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithRepo(repo)

	in := ImportItem{Method: "GET", Path: "/users", Prompt: "List users."}
	if _, err := svc.Import(context.Background(), []ImportItem{in}); err != nil {
		t.Fatalf("first Import failed: %v", err)
	}

	res, err := svc.Import(context.Background(), []ImportItem{in})
	if err != nil {
		t.Fatalf("second Import failed: %v", err)
	}
	if res.Created != 0 || res.Skipped != 1 {
		t.Fatalf("result = %+v, want created=0 skipped=1", res)
	}

	endpoints, _, err := svc.List(context.Background(), ListParams{Page: 1, Limit: 100})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(endpoints) != 1 {
		t.Errorf("expected a single endpoint after re-import, got %d", len(endpoints))
	}
}

func TestServiceImportDropsInvalidSchema(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithRepo(repo)

	res, err := svc.Import(context.Background(), []ImportItem{
		{Method: "GET", Path: "/bad", Prompt: "p.", Schema: `{"type":"not-a-real-type"}`},
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("expected endpoint created despite invalid schema, got %+v", res)
	}

	versions, err := svc.ListVersions(context.Background(), res.Endpoints[0].ID)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if versions[0].Schema != "" {
		t.Errorf("expected empty stored schema for invalid import, got %q", versions[0].Schema)
	}
}

func TestServiceImportInvalidatesCache(t *testing.T) {
	rc := &recordingCache{}
	logger := zerolog.Nop()
	svc := NewService(newFakeRepo(), rc, ai.Noop{}, validator.New(), &logger)

	res, err := svc.Import(context.Background(), []ImportItem{
		{Method: "POST", Path: "/users", Prompt: "Create a user."},
		{Method: "GET", Path: "/users/:id", Prompt: "Get a user."},
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if res.Created != 2 {
		t.Fatalf("expected 2 created, got %+v", res)
	}

	for _, key := range []string{cache.EndpointListKey("POST"), cache.EndpointListKey("GET")} {
		if !slices.Contains(rc.deleted, key) {
			t.Errorf("expected cache key %q to be invalidated, got %v", key, rc.deleted)
		}
	}
}

func TestServiceCreateStoresRequestSchema(t *testing.T) {
	svc := newTestService()

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method:        "POST",
		Path:          "/users",
		Prompt:        "create a user",
		RequestSchema: `{"type":"object","required":["email"],"properties":{"email":{"type":"string","format":"email"}}}`,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !strings.Contains(e.RequestSchema, `"email"`) {
		t.Errorf("expected request schema stored, got %q", e.RequestSchema)
	}
}

func TestServiceCreateRejectsInvalidRequestSchema(t *testing.T) {
	svc := newTestService()

	_, err := svc.Create(context.Background(), CreateEndpointParams{
		Method:        "POST",
		Path:          "/users",
		Prompt:        "create a user",
		RequestSchema: `{"type":"not-a-real-type"}`,
	})
	if err == nil {
		t.Fatal("expected error for invalid request schema")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if validationErr.Field != "request_schema" {
		t.Errorf("expected field request_schema, got %q", validationErr.Field)
	}
}

func TestServiceUpdateRequestSchemaSetClearKeep(t *testing.T) {
	svc := newTestService()

	e, err := svc.Create(context.Background(), CreateEndpointParams{
		Method: "POST",
		Path:   "/users",
		Prompt: "create a user",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	set := `{"type":"object","properties":{"name":{"type":"string"}}}`
	updated, err := svc.Update(context.Background(), e.ID, UpdateEndpointParams{
		Method:        "POST",
		Path:          "/users",
		Prompt:        "create a user",
		RequestSchema: &set,
	})
	if err != nil {
		t.Fatalf("Update set failed: %v", err)
	}
	if updated.RequestSchema != set {
		t.Errorf("expected request schema set, got %q", updated.RequestSchema)
	}

	clear := ""
	if _, err := svc.Update(context.Background(), e.ID, UpdateEndpointParams{
		Method:        "POST",
		Path:          "/users",
		Prompt:        "create a user",
		RequestSchema: &clear,
	}); err != nil {
		t.Fatalf("Update clear failed: %v", err)
	}
	got, err := svc.Get(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.RequestSchema != "" {
		t.Errorf("expected cleared request schema, got %q", got.RequestSchema)
	}

	if _, err := svc.Update(context.Background(), e.ID, UpdateEndpointParams{
		Method: "POST",
		Path:   "/users",
		Prompt: "create a user",
	}); err != nil {
		t.Fatalf("Update without schema failed: %v", err)
	}
	got, err = svc.Get(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.RequestSchema != "" {
		t.Errorf("expected request schema kept empty, got %q", got.RequestSchema)
	}
}

func TestServiceImportStoresRequestSchema(t *testing.T) {
	svc := newTestService()

	res, err := svc.Import(context.Background(), []ImportItem{
		{
			Method:        "POST",
			Path:          "/users",
			Prompt:        "Create a user.",
			RequestSchema: `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`,
		},
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("expected 1 created, got %+v", res)
	}
	if res.Endpoints[0].RequestSchema == "" {
		t.Error("expected imported request schema stored")
	}
}

func TestServiceImportDropsInvalidRequestSchema(t *testing.T) {
	svc := newTestService()

	res, err := svc.Import(context.Background(), []ImportItem{
		{Method: "POST", Path: "/users", Prompt: "Create a user.", RequestSchema: `{"type":"nope"}`},
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("expected endpoint created despite invalid request schema, got %+v", res)
	}
	if res.Endpoints[0].RequestSchema != "" {
		t.Errorf("expected empty stored request schema for invalid import, got %q", res.Endpoints[0].RequestSchema)
	}
}
