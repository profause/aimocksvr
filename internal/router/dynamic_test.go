package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/ai"
	"github.com/profause/aimocksvr/internal/api"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/generator"
	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/validator"
)

type fakeStore struct {
	endpoints []models.Endpoint
	history   []models.RequestHistory
	calls     int
}

func (f *fakeStore) ListActiveByMethod(_ context.Context, method string) ([]models.Endpoint, error) {
	f.calls++
	var out []models.Endpoint
	for _, e := range f.endpoints {
		if e.Method == method {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeStore) CreateHistory(_ context.Context, h *models.RequestHistory) error {
	f.history = append(f.history, *h)
	return nil
}

type fakeGenerator struct {
	response *generator.Response
	err      error
}

func (g *fakeGenerator) Generate(_ context.Context, _ *generator.Request) (*generator.Response, error) {
	return g.response, g.err
}

func newDynamicApp(t *testing.T, store EndpointStore) *fiber.App {
	t.Helper()
	logger := zerolog.Nop()
	dyn := NewDynamicHandler(store, generator.NewStatic(), validator.New(), &config.Config{}, &logger)

	app := fiber.New()
	app.Use(dyn.Serve)
	return app
}

// rtAI is a minimal AI provider for router integration tests.
type rtAI struct {
	body string
}

func (a *rtAI) GenerateResponse(_ context.Context, _ ai.ResponseRequest) (*ai.Response, error) {
	return &ai.Response{Status: 200, Body: []byte(a.body)}, nil
}

func (a *rtAI) GenerateSchema(context.Context, ai.SchemaRequest) (*ai.Schema, error) {
	return nil, nil
}

func (a *rtAI) GeneratePrompt(context.Context, ai.PromptRequest) (string, error) {
	return "", nil
}

type rtSchemas struct {
	schema string
}

func (s rtSchemas) LoadSchema(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return s.schema, nil
}

func newEndpoint(method, path string) models.Endpoint {
	return models.Endpoint{
		ID:           uuid.New(),
		Method:       method,
		Path:         path,
		Prompt:       "mock endpoint",
		ResponseType: models.ResponseTypeJSON,
		Status:       models.StatusActive,
	}
}

func TestDynamicHandlerValidatesRequestBody(t *testing.T) {
	schema := `{"type":"object","required":["email"],"properties":{"email":{"type":"string","format":"email"}}}`
	store := &fakeStore{
		endpoints: []models.Endpoint{
			func() models.Endpoint {
				e := newEndpoint("POST", "/users")
				e.RequestSchema = schema
				return e
			}(),
		},
	}
	app := newDynamicApp(t, store)

	// A conforming body passes through to generation.
	resp, err := app.Test(httptest.NewRequest("POST", "/users", strings.NewReader(`{"email":"a@b.co"}`)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for valid body, got %d", resp.StatusCode)
	}

	// A non-conforming body is rejected before generation.
	resp, err = app.Test(httptest.NewRequest("POST", "/users", strings.NewReader(`{"email":"not-an-email"}`)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if code, _ := body["error"].(map[string]any)["code"].(string); code != api.CodeValidationError {
		t.Errorf("expected code %q, got %v", api.CodeValidationError, code)
	}

	// A missing body is rejected with a clear message.
	resp, err = app.Test(httptest.NewRequest("POST", "/users", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for missing body, got %d", resp.StatusCode)
	}
}

func TestDynamicHandlerWithoutRequestSchemaSkipsValidation(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{newEndpoint("POST", "/users")},
	}
	app := newDynamicApp(t, store)

	resp, err := app.Test(httptest.NewRequest("POST", "/users", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 without request schema, got %d", resp.StatusCode)
	}
}

func TestDynamicHandlerServesEndpoint(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{
			newEndpoint("GET", "/users/:id"),
		},
	}
	app := newDynamicApp(t, store)

	resp, err := app.Test(httptest.NewRequest("GET", "/users/42?active=true", nil))
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["endpoint"] != "/users/:id" {
		t.Errorf("unexpected endpoint field: %v", body["endpoint"])
	}

	params, _ := body["params"].(map[string]any)
	if params["id"] != "42" {
		t.Errorf("expected param id=42, got %v", body["params"])
	}

	query, _ := body["query"].(map[string]any)
	if query["active"] != "true" {
		t.Errorf("expected query active=true, got %v", body["query"])
	}
}

func TestDynamicHandlerNotFound(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{
			newEndpoint("GET", "/users/:id"),
		},
	}
	app := newDynamicApp(t, store)

	resp, err := app.Test(httptest.NewRequest("DELETE", "/users/42", nil))
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["success"] != false {
		t.Errorf("expected failure envelope, got %v", body)
	}
}

func TestDynamicHandlerRecordsHistory(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{
			newEndpoint("POST", "/users"),
		},
	}
	app := newDynamicApp(t, store)

	req := httptest.NewRequest("POST", "/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	defer resp.Body.Close()

	if len(store.history) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(store.history))
	}
	if store.history[0].EndpointID != store.endpoints[0].ID {
		t.Errorf("history endpoint id mismatch")
	}
}

func TestDynamicHandlerGeneratorError(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{
			newEndpoint("GET", "/users"),
		},
	}
	logger := zerolog.Nop()
	dyn := NewDynamicHandler(store, &fakeGenerator{err: context.DeadlineExceeded}, validator.New(), &config.Config{}, &logger)

	app := fiber.New()
	app.Use(dyn.Serve)

	resp, err := app.Test(httptest.NewRequest("GET", "/users", nil))
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestDynamicHandlerServesAIGeneratedSchemaValidResponse(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{
			newEndpoint("GET", "/users/:id"),
		},
	}
	logger := zerolog.Nop()
	aiGen := generator.NewAI(
		&rtAI{body: `{"id": 42}`},
		rtSchemas{schema: `{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}`},
		validator.New(),
		generator.NewStatic(),
		&logger,
	)
	dyn := NewDynamicHandler(store, aiGen, validator.New(), &config.Config{}, &logger)

	app := fiber.New()
	app.Use(dyn.Serve)

	resp, err := app.Test(httptest.NewRequest("GET", "/users/42", nil))
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"id": 42}` {
		t.Errorf("expected ai-generated body, got %q", body)
	}
	if len(store.history) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(store.history))
	}
	if store.history[0].Response != `{"id": 42}` {
		t.Errorf("history should record the generated response, got %q", store.history[0].Response)
	}
}

// stateStore is an in-memory state.Store for router integration tests.
type stateStore struct {
	data map[string]map[string]map[string]any
}

func (s *stateStore) Create(_ context.Context, _ uuid.UUID, collection, resourceID string, data map[string]any) error {
	if s.data == nil {
		s.data = map[string]map[string]map[string]any{}
	}
	if s.data[collection] == nil {
		s.data[collection] = map[string]map[string]any{}
	}
	s.data[collection][resourceID] = data
	return nil
}

func (s *stateStore) Get(_ context.Context, _ uuid.UUID, collection, resourceID string) (map[string]any, bool, error) {
	data, ok := s.data[collection][resourceID]
	return data, ok, nil
}

func (s *stateStore) Update(_ context.Context, _ uuid.UUID, collection, resourceID string, data map[string]any) error {
	s.data[collection][resourceID] = data
	return nil
}

func (s *stateStore) Delete(_ context.Context, _ uuid.UUID, collection, resourceID string) (bool, error) {
	if _, ok := s.data[collection][resourceID]; !ok {
		return false, nil
	}
	delete(s.data[collection], resourceID)
	return true, nil
}

func TestDynamicHandlerServesStatefulResourceFlow(t *testing.T) {
	post := newEndpoint("POST", "/users")
	post.Stateful = true
	item := []models.Endpoint{}
	for _, method := range []string{"GET", "PUT", "PATCH", "DELETE"} {
		e := newEndpoint(method, "/users/:id")
		e.Stateful = true
		item = append(item, e)
	}

	store := &fakeStore{endpoints: append([]models.Endpoint{post}, item...)}
	logger := zerolog.Nop()
	aiGen := generator.NewAI(
		&rtAI{body: `{"id": 42, "name": "Ada"}`},
		rtSchemas{schema: ""},
		validator.New(),
		generator.NewStatic(),
		&logger,
	)
	dyn := NewDynamicHandler(store, generator.NewStateful(&stateStore{}, aiGen, &logger), validator.New(), &config.Config{}, &logger)

	app := fiber.New()
	app.Use(dyn.Serve)

	// POST creates the resource.
	resp, err := app.Test(httptest.NewRequest("POST", "/users", nil))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d (body %q)", resp.StatusCode, body)
	}
	var created map[string]any
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("create body must be JSON: %v", err)
	}
	if created["id"] == nil {
		t.Fatalf("expected id in created resource, got %q", body)
	}

	// GET returns the same object.
	resp, err = app.Test(httptest.NewRequest("GET", "/users/42", nil))
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), `"name":"Ada"`) {
		t.Errorf("expected stored resource, got %q", body)
	}

	// PUT replaces it.
	resp, err = app.Test(httptest.NewRequest("PUT", "/users/42", bytes.NewBufferString(`{"name":"Bob"}`)))
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), `"name":"Bob"`) {
		t.Errorf("expected replaced resource, got %q", body)
	}

	// DELETE removes it.
	resp, err = app.Test(httptest.NewRequest("DELETE", "/users/42", nil))
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// GET now 404s.
	resp, err = app.Test(httptest.NewRequest("GET", "/users/42", nil))
	if err != nil {
		t.Fatalf("GET after delete failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
	}
}
