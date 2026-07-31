package generator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/profause/aimocksvr/internal/models"
)

func TestStaticGenerate(t *testing.T) {
	gen := NewStatic()

	req := &Request{
		Endpoint: &models.Endpoint{
			ID:     uuid.New(),
			Method: "GET",
			Path:   "/users/:id",
			Status: models.StatusActive,
		},
		PathParams: map[string]string{"id": "7"},
		Query:      map[string]string{"expand": "true"},
	}

	resp, err := gen.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if resp.Status != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.Status)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body["endpoint"] != "/users/:id" {
		t.Errorf("unexpected endpoint field: %v", body["endpoint"])
	}
}

func TestStaticGenerateWithoutEndpoint(t *testing.T) {
	gen := NewStatic()
	if _, err := gen.Generate(context.Background(), &Request{}); err == nil {
		t.Fatal("expected error for nil endpoint")
	}
}

func TestStaticGenerateNilMaps(t *testing.T) {
	gen := NewStatic()

	resp, err := gen.Generate(context.Background(), &Request{
		Endpoint: &models.Endpoint{Method: "GET", Path: "/ping", Status: models.StatusActive},
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, ok := body["params"]; !ok {
		t.Error("expected params field to be present")
	}
	if _, ok := body["query"]; !ok {
		t.Error("expected query field to be present")
	}
}
