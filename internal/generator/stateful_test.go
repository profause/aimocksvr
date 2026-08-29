package generator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/state"
)

type fakeStore struct {
	data map[string]map[string]map[string]any
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: map[string]map[string]map[string]any{}}
}

func (f *fakeStore) Create(_ context.Context, accountID uuid.UUID, collection, resourceID string, data map[string]any) error {
	if _, exists := f.data[collection][resourceID]; exists {
		return state.ErrConflict
	}
	if f.data[collection] == nil {
		f.data[collection] = map[string]map[string]any{}
	}
	f.data[collection][resourceID] = data
	return nil
}

func (f *fakeStore) Get(_ context.Context, accountID uuid.UUID, collection, resourceID string) (map[string]any, bool, error) {
	data, exists := f.data[collection][resourceID]
	return data, exists, nil
}

func (f *fakeStore) Update(_ context.Context, accountID uuid.UUID, collection, resourceID string, data map[string]any) error {
	if _, exists := f.data[collection][resourceID]; !exists {
		return state.ErrNotFound
	}
	f.data[collection][resourceID] = data
	return nil
}

func (f *fakeStore) Delete(_ context.Context, accountID uuid.UUID, collection, resourceID string) (bool, error) {
	if _, exists := f.data[collection][resourceID]; !exists {
		return false, nil
	}
	delete(f.data[collection], resourceID)
	return true, nil
}

type fakeInner struct {
	body    string
	status  int
	calls   int
	lastReq *Request
}

func (f *fakeInner) Generate(_ context.Context, req *Request) (*Response, error) {
	f.calls++
	f.lastReq = req
	return &Response{Status: f.status, Body: []byte(f.body)}, nil
}

func newStatefulTest(t *testing.T, store state.Store, inner Generator) *statefulGenerator {
	t.Helper()
	logger := zerolog.Nop()
	return &statefulGenerator{store: store, inner: inner, log: &logger}
}

func statefulReq(method, path string, params map[string]string, body string) *Request {
	return &Request{
		Endpoint: &models.Endpoint{
			ID:       uuid.New(),
			Method:   method,
			Path:     path,
			Prompt:   "create or fetch a user",
			Stateful: true,
		},
		PathParams: params,
		Body:       []byte(body),
	}
}

func TestStatefulGeneratorDelegatesNonStateful(t *testing.T) {
	store := newFakeStore()
	inner := &fakeInner{body: `{"ok":true}`, status: http.StatusOK}
	g := newStatefulTest(t, store, inner)

	req := statefulReq("GET", "/users/:id", map[string]string{"id": "1"}, "")
	req.Endpoint.Stateful = false

	resp, err := g.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if inner.calls != 1 {
		t.Fatalf("expected inner to be called, got %d calls", inner.calls)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Errorf("expected inner body, got %q", resp.Body)
	}
}

func TestStatefulCreateAssignsGeneratedID(t *testing.T) {
	store := newFakeStore()
	inner := &fakeInner{body: `{"name":"Ada"}`, status: http.StatusOK}
	g := newStatefulTest(t, store, inner)

	resp, err := g.Generate(context.Background(), statefulReq("POST", "/users", nil, ""))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.Status)
	}

	var data map[string]any
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		t.Fatalf("response must be JSON: %v", err)
	}
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatalf("expected generated id, got %q", data["id"])
	}
	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("expected uuid id, got %q", id)
	}

	stored, found, err := store.Get(context.Background(), uuid.Nil, "/users", id)
	if err != nil || !found {
		t.Fatalf("expected resource stored (found=%v err=%v)", found, err)
	}
	if stored["name"] != "Ada" {
		t.Errorf("expected stored name, got %+v", stored)
	}
}

func TestStatefulCreateUsesBodyID(t *testing.T) {
	store := newFakeStore()
	inner := &fakeInner{body: `{"id":5,"name":"Ada"}`, status: http.StatusOK}
	g := newStatefulTest(t, store, inner)

	resp, err := g.Generate(context.Background(), statefulReq("POST", "/users", nil, ""))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.Status)
	}
	if _, found, _ := store.Get(context.Background(), uuid.Nil, "/users", "5"); !found {
		t.Errorf("expected resource stored under id 5")
	}
	if !strings.Contains(string(resp.Body), `"id":5`) {
		t.Errorf("expected body id 5 in response, got %s", resp.Body)
	}
}

func TestStatefulCreateWithPathID(t *testing.T) {
	store := newFakeStore()
	inner := &fakeInner{body: `{"name":"Ada"}`, status: http.StatusOK}
	g := newStatefulTest(t, store, inner)

	resp, err := g.Generate(context.Background(), statefulReq("POST", "/users/:id", map[string]string{"id": "7"}, ""))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	stored, found, _ := store.Get(context.Background(), uuid.Nil, "/users", "7")
	if !found {
		t.Fatalf("expected resource stored under path id 7")
	}
	if stored["id"] != "7" {
		t.Errorf("expected id from path to override body, got %v", stored["id"])
	}
	if resp.Status != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.Status)
	}
}

func TestStatefulCreateConflict(t *testing.T) {
	store := newFakeStore()
	inner := &fakeInner{body: `{"id":"dup"}`, status: http.StatusOK}
	g := newStatefulTest(t, store, inner)

	if _, err := g.Generate(context.Background(), statefulReq("POST", "/users", nil, "")); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	resp, err := g.Generate(context.Background(), statefulReq("POST", "/users", nil, ""))
	if err != nil {
		t.Fatalf("second create failed: %v", err)
	}
	if resp.Status != http.StatusConflict {
		t.Errorf("expected 409, got %d", resp.Status)
	}
}

func TestStatefulGetReturnsStoredResource(t *testing.T) {
	store := newFakeStore()
	if err := store.Create(context.Background(), uuid.Nil, "/users", "1", map[string]any{"id": 1, "name": "Ada"}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	inner := &fakeInner{}
	g := newStatefulTest(t, store, inner)

	resp, err := g.Generate(context.Background(), statefulReq("GET", "/users/:id", map[string]string{"id": "1"}, ""))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.Status)
	}
	if !strings.Contains(string(resp.Body), `"name":"Ada"`) {
		t.Errorf("expected stored resource in body, got %s", resp.Body)
	}
	if inner.calls != 0 {
		t.Errorf("expected no inner call for a stored resource, got %d", inner.calls)
	}
}

func TestStatefulGetNotFound(t *testing.T) {
	g := newStatefulTest(t, newFakeStore(), &fakeInner{})
	resp, err := g.Generate(context.Background(), statefulReq("GET", "/users/:id", map[string]string{"id": "nope"}, ""))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.Status)
	}
	if !strings.Contains(string(resp.Body), "not found") {
		t.Errorf("expected not found message, got %s", resp.Body)
	}
}

func TestStatefulPutReplacesResource(t *testing.T) {
	store := newFakeStore()
	if err := store.Create(context.Background(), uuid.Nil, "/users", "1", map[string]any{"id": 1, "name": "Ada"}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	g := newStatefulTest(t, store, &fakeInner{})

	resp, err := g.Generate(context.Background(), statefulReq("PUT", "/users/:id", map[string]string{"id": "1"}, `{"name":"Bob","age":30}`))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.Status)
	}
	stored, found, _ := store.Get(context.Background(), uuid.Nil, "/users", "1")
	if !found {
		t.Fatalf("expected resource to still exist")
	}
	if stored["name"] != "Bob" {
		t.Errorf("expected replaced name, got %+v", stored)
	}
	if stored["id"] != 1 {
		t.Errorf("expected existing id type to be preserved, got %v (%T)", stored["id"], stored["id"])
	}
	if _, has := stored["age"]; !has {
		t.Errorf("expected age in stored resource, got %+v", stored)
	}
}

func TestStatefulPutOverridesBodyIDWithPath(t *testing.T) {
	store := newFakeStore()
	if err := store.Create(context.Background(), uuid.Nil, "/users", "1", map[string]any{"id": 1}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	g := newStatefulTest(t, store, &fakeInner{})

	resp, err := g.Generate(context.Background(), statefulReq("PUT", "/users/:id", map[string]string{"id": "1"}, `{"id": 99, "name":"Bob"}`))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.Status)
	}
	stored, _, _ := store.Get(context.Background(), uuid.Nil, "/users", "1")
	if stored["id"] != "1" {
		t.Errorf("expected path id to win, got %v", stored["id"])
	}
}

func TestStatefulPutNotFound(t *testing.T) {
	g := newStatefulTest(t, newFakeStore(), &fakeInner{})
	resp, err := g.Generate(context.Background(), statefulReq("PUT", "/users/:id", map[string]string{"id": "9"}, `{"name":"X"}`))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.Status)
	}
}

func TestStatefulPatchMergesDeeply(t *testing.T) {
	store := newFakeStore()
	if err := store.Create(context.Background(), uuid.Nil, "/users", "1", map[string]any{
		"id":   1,
		"name": "Ada",
		"addr": map[string]any{"city": "Lisbon", "zip": "1000"},
	}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	g := newStatefulTest(t, store, &fakeInner{})

	resp, err := g.Generate(context.Background(), statefulReq("PATCH", "/users/:id", map[string]string{"id": "1"}, `{"name":"Ada Lovelace","addr":{"zip":"1200"}}`))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.Status)
	}
	stored, _, _ := store.Get(context.Background(), uuid.Nil, "/users", "1")
	if stored["name"] != "Ada Lovelace" {
		t.Errorf("expected patched name, got %v", stored["name"])
	}
	addr := stored["addr"].(map[string]any)
	if addr["city"] != "Lisbon" || addr["zip"] != "1200" {
		t.Errorf("expected deep merge, got %+v", addr)
	}
}

func TestStatefulPatchNotFound(t *testing.T) {
	g := newStatefulTest(t, newFakeStore(), &fakeInner{})
	resp, err := g.Generate(context.Background(), statefulReq("PATCH", "/users/:id", map[string]string{"id": "9"}, `{"name":"X"}`))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.Status)
	}
}

func TestStatefulDeleteRemovesResource(t *testing.T) {
	store := newFakeStore()
	if err := store.Create(context.Background(), uuid.Nil, "/users", "1", map[string]any{"id": 1}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	g := newStatefulTest(t, store, &fakeInner{})

	resp, err := g.Generate(context.Background(), statefulReq("DELETE", "/users/:id", map[string]string{"id": "1"}, ""))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.Status)
	}
	if len(resp.Body) != 0 {
		t.Errorf("expected empty body, got %q", resp.Body)
	}
	if _, found, _ := store.Get(context.Background(), uuid.Nil, "/users", "1"); found {
		t.Errorf("expected resource to be removed")
	}
}

func TestStatefulDeleteNotFound(t *testing.T) {
	g := newStatefulTest(t, newFakeStore(), &fakeInner{})
	resp, err := g.Generate(context.Background(), statefulReq("DELETE", "/users/:id", map[string]string{"id": "9"}, ""))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.Status)
	}
}

func TestStatefulPutNonObjectBody(t *testing.T) {
	store := newFakeStore()
	if err := store.Create(context.Background(), uuid.Nil, "/users", "1", map[string]any{"id": 1}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	g := newStatefulTest(t, store, &fakeInner{})

	resp, err := g.Generate(context.Background(), statefulReq("PUT", "/users/:id", map[string]string{"id": "1"}, `"just a string"`))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Status)
	}
}

func TestStatefulGetWithoutPathParamDelegates(t *testing.T) {
	store := newFakeStore()
	inner := &fakeInner{body: `{"ok":true}`, status: http.StatusOK}
	g := newStatefulTest(t, store, inner)

	resp, err := g.Generate(context.Background(), statefulReq("GET", "/users", nil, ""))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if inner.calls != 1 {
		t.Errorf("expected delegation to inner, got %d calls", inner.calls)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Errorf("expected inner body, got %q", resp.Body)
	}
}

func TestStatefulGeneratorPropagatesStoreErrors(t *testing.T) {
	store := &errorStore{}
	g := newStatefulTest(t, store, &fakeInner{body: `{"name":"Ada"}`})

	_, err := g.Generate(context.Background(), statefulReq("POST", "/users", nil, ""))
	if err == nil {
		t.Fatalf("expected store error to propagate")
	}
}

type errorStore struct{}

func (e *errorStore) Create(context.Context, uuid.UUID, string, string, map[string]any) error {
	return errors.New("store down")
}
func (e *errorStore) Get(context.Context, uuid.UUID, string, string) (map[string]any, bool, error) {
	return nil, false, errors.New("store down")
}
func (e *errorStore) Update(context.Context, uuid.UUID, string, string, map[string]any) error {
	return errors.New("store down")
}
func (e *errorStore) Delete(context.Context, uuid.UUID, string, string) (bool, error) {
	return false, errors.New("store down")
}
