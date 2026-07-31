package importer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/endpoint"
	"github.com/profause/aimocksvr/internal/models"
)

// fakeEndpointService captures import calls and returns a canned result.
type fakeEndpointService struct {
	items  []endpoint.ImportItem
	result endpoint.ImportResult
	err    error
}

func (f *fakeEndpointService) Import(_ context.Context, items []endpoint.ImportItem) (endpoint.ImportResult, error) {
	f.items = items
	return f.result, f.err
}

func (f *fakeEndpointService) Create(context.Context, endpoint.CreateEndpointParams) (*models.Endpoint, error) {
	return nil, nil
}

func (f *fakeEndpointService) Update(context.Context, uuid.UUID, endpoint.UpdateEndpointParams) (*models.Endpoint, error) {
	return nil, nil
}

func (f *fakeEndpointService) Delete(context.Context, uuid.UUID) error { return nil }

func (f *fakeEndpointService) Get(context.Context, uuid.UUID) (*models.Endpoint, error) {
	return nil, nil
}

func (f *fakeEndpointService) List(context.Context, endpoint.ListParams) ([]models.Endpoint, int, error) {
	return nil, 0, nil
}

func (f *fakeEndpointService) ListVersions(context.Context, uuid.UUID) ([]models.EndpointVersion, error) {
	return nil, nil
}

func (f *fakeEndpointService) ListHistory(context.Context, uuid.UUID) ([]models.RequestHistory, error) {
	return nil, nil
}

func newTestService(f *fakeEndpointService) *Service {
	logger := zerolog.Nop()
	return NewService(f, &logger)
}

func TestServiceImportMapsSpecsToItems(t *testing.T) {
	es := &fakeEndpointService{result: endpoint.ImportResult{Created: 3}}
	svc := newTestService(es)

	res, err := svc.Import(context.Background(), []byte(v3Spec))
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if res.Parsed != 3 || res.Created != 3 {
		t.Errorf("result = %+v, want parsed=3 created=3", res)
	}
	if len(es.items) != 3 {
		t.Fatalf("expected 3 import items, got %d", len(es.items))
	}

	var listUsers *endpoint.ImportItem
	for i := range es.items {
		item := &es.items[i]
		if item.Method == "GET" && item.Path == "/users" {
			listUsers = item
		}
	}
	if listUsers == nil {
		t.Fatalf("expected a GET /users item, got %+v", es.items)
	}
	if listUsers.Prompt != "List users." {
		t.Errorf("prompt = %q, want %q", listUsers.Prompt, "List users.")
	}
	if !strings.Contains(listUsers.Schema, `"uuid"`) {
		t.Errorf("GET /users should carry the resolved schema, got %q", listUsers.Schema)
	}
}

func TestServiceImportSkipsEmptyDoc(t *testing.T) {
	es := &fakeEndpointService{}
	svc := newTestService(es)

	_, err := svc.Import(context.Background(), []byte(""))
	if err == nil {
		t.Fatal("expected error for empty document")
	}
	if len(es.items) != 0 {
		t.Errorf("expected no import items, got %d", len(es.items))
	}
}

func TestServiceImportPropagatesEndpointError(t *testing.T) {
	boom := errors.New("db down")
	es := &fakeEndpointService{err: boom}
	svc := newTestService(es)

	_, err := svc.Import(context.Background(), []byte(v3Spec))
	if !errors.Is(err, boom) {
		t.Fatalf("expected stored error, got %v", err)
	}
}

func TestServiceImportPostman(t *testing.T) {
	es := &fakeEndpointService{result: endpoint.ImportResult{Created: 2}}
	svc := newTestService(es)

	res, err := svc.ImportPostman(context.Background(), []byte(postmanCollection))
	if err != nil {
		t.Fatalf("ImportPostman returned error: %v", err)
	}
	if res.Parsed != 3 || res.Created != 2 {
		t.Errorf("result = %+v, want parsed=3 created=2", res)
	}
	if len(es.items) != 3 {
		t.Fatalf("expected 3 import items, got %d", len(es.items))
	}
	create := es.items[1]
	if create.Method != "POST" || create.Path != "/v2/users" {
		t.Errorf("second item = %s %s, want POST /v2/users", create.Method, create.Path)
	}
	if create.RequestSchema == "" {
		t.Errorf("POST item should carry an inferred request schema")
	}
	if create.Schema == "" {
		t.Errorf("POST item should carry an inferred response schema")
	}
}
