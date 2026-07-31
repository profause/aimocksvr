package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/ai"
	"github.com/profause/aimocksvr/internal/auth"
	"github.com/profause/aimocksvr/internal/cache"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/endpoint"
	"github.com/profause/aimocksvr/internal/generator"
	"github.com/profause/aimocksvr/internal/importer"
	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/validator"
)

// importRepo is an in-memory store that doubles as the endpoint Repository
// (so imports persist) and the dynamic router's EndpointStore (so imported
// endpoints are served). It mirrors how the real server wires the two.
type importRepo struct {
	endpoints map[uuid.UUID]*models.Endpoint
	versions  []models.EndpointVersion
	history   []models.RequestHistory
}

func newImportRepo() *importRepo {
	return &importRepo{endpoints: make(map[uuid.UUID]*models.Endpoint)}
}

func (f *importRepo) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (f *importRepo) Create(_ context.Context, e *models.Endpoint) error {
	for _, existing := range f.endpoints {
		if existing.Method == e.Method && existing.Path == e.Path {
			return endpoint.ErrConflict
		}
	}
	f.endpoints[e.ID] = e
	return nil
}

func (f *importRepo) Update(ctx context.Context, e *models.Endpoint) error {
	if _, ok := f.endpoints[e.ID]; !ok {
		return endpoint.ErrNotFound
	}
	f.endpoints[e.ID] = e
	return nil
}

func (f *importRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.endpoints[id]; !ok {
		return endpoint.ErrNotFound
	}
	delete(f.endpoints, id)
	return nil
}

func (f *importRepo) FindByID(_ context.Context, id uuid.UUID) (*models.Endpoint, error) {
	e, ok := f.endpoints[id]
	if !ok {
		return nil, endpoint.ErrNotFound
	}
	return e, nil
}

func (f *importRepo) List(_ context.Context, p endpoint.ListParams) ([]models.Endpoint, int, error) {
	all := make([]models.Endpoint, 0, len(f.endpoints))
	for _, e := range f.endpoints {
		all = append(all, *e)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	return all, len(all), nil
}

func (f *importRepo) ListActiveByMethod(_ context.Context, method string) ([]models.Endpoint, error) {
	var out []models.Endpoint
	for _, e := range f.endpoints {
		if e.Method == method && e.Status == models.StatusActive {
			out = append(out, *e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (f *importRepo) CreateVersion(_ context.Context, v *models.EndpointVersion) error {
	f.versions = append(f.versions, *v)
	return nil
}

func (f *importRepo) ListVersions(_ context.Context, endpointID uuid.UUID) ([]models.EndpointVersion, error) {
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

func (f *importRepo) ListHistory(_ context.Context, endpointID uuid.UUID) ([]models.RequestHistory, error) {
	var out []models.RequestHistory
	for _, h := range f.history {
		if h.EndpointID == endpointID {
			out = append(out, h)
		}
	}
	return out, nil
}

func (f *importRepo) CreateHistory(_ context.Context, h *models.RequestHistory) error {
	f.history = append(f.history, *h)
	return nil
}

// importSchemaLoader adapts the repo to the generator's SchemaLoader, like the
// endpointSchemaLoader in cmd/server.
type importSchemaLoader struct {
	repo *importRepo
}

func (s importSchemaLoader) LoadSchema(ctx context.Context, endpointID uuid.UUID) (string, error) {
	versions, err := s.repo.ListVersions(ctx, endpointID)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", nil
	}
	return versions[0].Schema, nil
}

// newImportApp builds the full router with in-memory persistence so the
// whole import -> serve pipeline runs in-process.
func newImportApp(t *testing.T) *fiber.App {
	t.Helper()

	repo := newImportRepo()
	logger := zerolog.Nop()
	esvc := endpoint.NewService(repo, cache.Noop{}, ai.Noop{}, validator.New(), &logger)
	h := endpoint.NewHandler(esvc, &logger)
	imp := importer.NewHandler(importer.NewService(esvc, &logger), &logger)

	cfg := &config.Config{}
	cfg.App.Name = "aimocksvr-test"

	gen := generator.NewFaker(importSchemaLoader{repo: repo}, generator.NewStatic(), &logger)
	dyn := NewDynamicHandler(repo, gen, validator.New(), cfg, &logger)
	ah := auth.NewHandler(cfg, auth.NewService(cfg, &logger), &logger)

	return New(cfg, &logger, h, imp, dyn, ah)
}

const importSpec = `{
  "openapi": "3.0.0",
  "info": {"title": "Catalog", "version": "1.0.0"},
  "paths": {
    "/catalog/items": {
      "get": {
        "summary": "List catalog items",
        "responses": {
          "200": {
            "description": "ok",
            "content": {"application/json": {"schema": {"type": "array", "items": {"$ref": "#/components/schemas/Item"}}}}
          }
        }
      },
      "post": {
        "summary": "Create a catalog item",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ItemInput"}}}
        },
        "responses": {
          "201": {
            "description": "created",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Item"}}}
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Item": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "name": {"type": "string"},
          "price": {"type": "number"}
        }
      },
      "ItemInput": {
        "type": "object",
        "required": ["name", "price"],
        "properties": {
          "name": {"type": "string"},
          "price": {"type": "number"}
        }
      }
    }
  }
}`

func TestImportOpenAPIThenServe(t *testing.T) {
	app := newImportApp(t)

	req := httptest.NewRequest("POST", "/api/v1/imports/openapi", bytes.NewReader([]byte(importSpec)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("import request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d (body %q)", resp.StatusCode, body)
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Parsed  int `json:"parsed"`
			Created int `json:"created"`
			Skipped int `json:"skipped"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if !envelope.Success || envelope.Data.Parsed != 2 || envelope.Data.Created != 2 {
		t.Fatalf("unexpected import result: %+v", envelope)
	}

	// GET the imported collection: the faker fills the imported schema.
	resp, err = app.Test(httptest.NewRequest("GET", "/catalog/items", nil))
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d (body %q)", resp.StatusCode, body)
	}
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("expected JSON array body, got %q", body)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 faked item, got %d", len(items))
	}
	if id, _ := items[0]["id"].(string); id == "" {
		t.Errorf("expected a faked uuid id, got %v", items[0]["id"])
	}
	if name, _ := items[0]["name"].(string); name == "" {
		t.Errorf("expected a faked name, got %v", items[0]["name"])
	}

	// POST serves the same imported endpoint and enforces its request schema.
	resp, err = app.Test(httptest.NewRequest("POST", "/catalog/items", bytes.NewBufferString(`{"name":"chair","price":9.99}`)))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for valid request body, got %d", resp.StatusCode)
	}

	// Missing required field -> 400 from the imported request schema.
	resp, err = app.Test(httptest.NewRequest("POST", "/catalog/items", bytes.NewBufferString(`{"name":"chair"}`)))
	if err != nil {
		t.Fatalf("POST invalid failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for invalid request body, got %d", resp.StatusCode)
	}

	// Empty body -> 400.
	resp, err = app.Test(httptest.NewRequest("POST", "/catalog/items", nil))
	if err != nil {
		t.Fatalf("POST empty failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for empty request body, got %d", resp.StatusCode)
	}

	// Re-importing the same spec is idempotent: both operations are skipped.
	resp, err = app.Test(httptest.NewRequest("POST", "/api/v1/imports/openapi", bytes.NewReader([]byte(importSpec))))
	if err != nil {
		t.Fatalf("second import failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode second import: %v", err)
	}
	if envelope.Data.Created != 0 || envelope.Data.Skipped != 2 {
		t.Fatalf("expected created=0 skipped=2, got %+v", envelope.Data)
	}
}

func TestImportOpenAPIRejectsInvalidSpec(t *testing.T) {
	app := newImportApp(t)

	req := httptest.NewRequest("POST", "/api/v1/imports/openapi", bytes.NewBufferString("not a spec"))
	req.Header.Set("Content-Type", "application/yaml")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

const postmanImportCollection = `{
  "info": {"name": "Catalog", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
  "item": [
    {
      "name": "Items",
      "item": [
        {
          "name": "List items",
          "request": {
            "method": "GET",
            "url": {"raw": "https://catalog.example.com/api/items", "path": ["api", "items"]}
          },
          "response": [
            {"name": "ok", "code": 200, "body": "[{\"id\": \"9f8b\", \"name\": \"chair\"}]"}
          ]
        },
        {
          "name": "Create item",
          "request": {
            "method": "POST",
            "description": "adds an item to the catalog",
            "url": {"raw": "https://catalog.example.com/api/items", "path": ["api", "items"]},
            "body": {"mode": "raw", "raw": "{\"name\":\"chair\",\"price\":9.99}"}
          },
          "response": [
            {"name": "created", "code": 201, "body": "{\"id\": \"9f8b\", \"name\": \"chair\", \"price\": 9.99}"}
          ]
        },
        {
          "name": "Get item",
          "request": {
            "method": "GET",
            "url": {"raw": "https://catalog.example.com/api/items/{{id}}", "path": ["api", "items", ":id"]}
          },
          "response": [
            {"name": "ok", "code": 200, "body": "{\"id\": \"9f8b\", \"name\": \"chair\", \"price\": 9.99}"}
          ]
        }
      ]
    }
  ]
}`

func TestImportPostmanThenServe(t *testing.T) {
	app := newImportApp(t)

	req := httptest.NewRequest("POST", "/api/v1/imports/postman", bytes.NewReader([]byte(postmanImportCollection)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("import request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d (body %q)", resp.StatusCode, body)
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Parsed  int `json:"parsed"`
			Created int `json:"created"`
			Skipped int `json:"skipped"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if !envelope.Success || envelope.Data.Parsed != 3 || envelope.Data.Created != 3 {
		t.Fatalf("unexpected import result: %+v", envelope)
	}

	// GET the collection endpoint: the faker fills the inferred schema.
	resp, err = app.Test(httptest.NewRequest("GET", "/api/items", nil))
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d (body %q)", resp.StatusCode, body)
	}
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("expected JSON array body, got %q", body)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 faked item, got %d", len(items))
	}
	if name, _ := items[0]["name"].(string); name == "" {
		t.Errorf("expected a faked name, got %v", items[0]["name"])
	}

	// Path params from {{var}}/":id" resolve.
	resp, err = app.Test(httptest.NewRequest("GET", "/api/items/42", nil))
	if err != nil {
		t.Fatalf("GET by id failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for /api/items/42, got %d", resp.StatusCode)
	}

	// POST enforces the inferred request schema.
	resp, err = app.Test(httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(`{"name":"table","price":129.0}`)))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for valid request body, got %d", resp.StatusCode)
	}

	// Missing inferred required field -> 400.
	resp, err = app.Test(httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(`{"name":"table"}`)))
	if err != nil {
		t.Fatalf("POST invalid failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for invalid request body, got %d", resp.StatusCode)
	}

	// Re-importing the same collection is idempotent.
	resp, err = app.Test(httptest.NewRequest("POST", "/api/v1/imports/postman", bytes.NewReader([]byte(postmanImportCollection))))
	if err != nil {
		t.Fatalf("second import failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode second import: %v", err)
	}
	if envelope.Data.Created != 0 || envelope.Data.Skipped != 3 {
		t.Fatalf("expected created=0 skipped=3, got %+v", envelope.Data)
	}
}

func TestImportPostmanRejectsInvalidCollection(t *testing.T) {
	app := newImportApp(t)

	req := httptest.NewRequest("POST", "/api/v1/imports/postman", bytes.NewBufferString(`{"info":{"name":"T"}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
