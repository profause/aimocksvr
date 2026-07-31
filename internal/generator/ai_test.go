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

	"github.com/profause/aimocksvr/internal/ai"
	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/validator"
)

const testSchema = `{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}`

// fakeAI is a controllable AIProvider that records the schema it was given.
type fakeAI struct {
	responses []*ai.Response
	err       error
	calls     int
	lastReq   ai.ResponseRequest
}

func (f *fakeAI) GenerateResponse(_ context.Context, req ai.ResponseRequest) (*ai.Response, error) {
	f.calls++
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	if f.calls <= len(f.responses) {
		return f.responses[f.calls-1], nil
	}
	return nil, nil
}

func (f *fakeAI) GenerateSchema(context.Context, ai.SchemaRequest) (*ai.Schema, error) {
	return nil, nil
}

func (f *fakeAI) GeneratePrompt(context.Context, ai.PromptRequest) (string, error) {
	return "", nil
}

type fakeSchemas struct {
	schema string
	err    error
	calls  int
}

func (f *fakeSchemas) LoadSchema(_ context.Context, _ uuid.UUID) (string, error) {
	f.calls++
	return f.schema, f.err
}

func newAITest(t *testing.T, provider ai.AIProvider, schemas SchemaLoader) *aiGenerator {
	t.Helper()
	logger := zerolog.Nop()
	return &aiGenerator{
		provider: provider,
		schemas:  schemas,
		validate: validator.New(),
		fallback: NewFaker(schemas, NewStatic(), &logger),
		logger:   &logger,
	}
}

func newReq() *Request {
	return &Request{
		Endpoint: &models.Endpoint{
			ID:     uuid.New(),
			Method: "GET",
			Path:   "/users/:id",
			Prompt: "return a user",
		},
		PathParams: map[string]string{"id": "42"},
	}
}

func jsonResponse(body string) *ai.Response {
	return &ai.Response{Status: http.StatusOK, Body: []byte(body)}
}

func TestAIGeneratorServesSchemaValidResponse(t *testing.T) {
	provider := &fakeAI{responses: []*ai.Response{jsonResponse(`{"id": 1}`)}}
	schemas := &fakeSchemas{schema: testSchema}
	g := newAITest(t, provider, schemas)

	resp, err := g.Generate(context.Background(), newReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if string(resp.Body) != `{"id": 1}` {
		t.Errorf("expected ai body, got %q", resp.Body)
	}
	if provider.calls != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.calls)
	}
	if provider.lastReq.Schema != testSchema {
		t.Errorf("expected schema to reach the provider, got %q", provider.lastReq.Schema)
	}
	if provider.lastReq.PathParams["id"] != "42" {
		t.Errorf("expected path params to reach the provider, got %+v", provider.lastReq.PathParams)
	}
}

func TestAIGeneratorFallsBackWhenAIDisabled(t *testing.T) {
	provider := &fakeAI{} // Noop-like: returns nil, nil
	schemas := &fakeSchemas{schema: testSchema}
	g := newAITest(t, provider, schemas)

	resp, err := g.Generate(context.Background(), newReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	assertSchemaFilled(t, resp.Body)
}

func TestAIGeneratorFallsBackOnProviderError(t *testing.T) {
	provider := &fakeAI{err: errors.New("provider down")}
	schemas := &fakeSchemas{schema: testSchema}
	g := newAITest(t, provider, schemas)

	resp, err := g.Generate(context.Background(), newReq())
	if err != nil {
		t.Fatalf("Generate should fall back, got %v", err)
	}
	assertSchemaFilled(t, resp.Body)
}

func TestAIGeneratorRetriesOnceOnValidationFailure(t *testing.T) {
	provider := &fakeAI{
		responses: []*ai.Response{
			jsonResponse(`{"id": "not-an-int"}`),
			jsonResponse(`{"id": 7}`),
		},
	}
	schemas := &fakeSchemas{schema: testSchema}
	g := newAITest(t, provider, schemas)

	resp, err := g.Generate(context.Background(), newReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if string(resp.Body) != `{"id": 7}` {
		t.Errorf("expected validated retry body, got %q", resp.Body)
	}
	if provider.calls != 2 {
		t.Errorf("expected 2 provider calls (retry), got %d", provider.calls)
	}
}

func TestAIGeneratorFallsBackAfterRetriesExhausted(t *testing.T) {
	provider := &fakeAI{
		responses: []*ai.Response{
			jsonResponse(`{"id": "bad"}`),
			jsonResponse(`{"id": "still-bad"}`),
		},
	}
	schemas := &fakeSchemas{schema: testSchema}
	g := newAITest(t, provider, schemas)

	resp, err := g.Generate(context.Background(), newReq())
	if err != nil {
		t.Fatalf("Generate should fall back, got %v", err)
	}
	assertSchemaFilled(t, resp.Body)
	if provider.calls != 2 {
		t.Errorf("expected exactly 2 provider calls, got %d", provider.calls)
	}
}

func TestAIGeneratorSkipsValidationWithoutSchema(t *testing.T) {
	body := `{"anything": true}`
	provider := &fakeAI{responses: []*ai.Response{jsonResponse(body)}}
	schemas := &fakeSchemas{schema: ""}
	g := newAITest(t, provider, schemas)

	resp, err := g.Generate(context.Background(), newReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if string(resp.Body) != body {
		t.Errorf("expected raw ai body without schema, got %q", resp.Body)
	}
	if provider.calls != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.calls)
	}
}

func TestAIGeneratorFallsBackOnSchemaLoadError(t *testing.T) {
	provider := &fakeAI{responses: []*ai.Response{jsonResponse(`{"id": 1}`)}}
	schemas := &fakeSchemas{err: errors.New("db down")}
	g := newAITest(t, provider, schemas)

	resp, err := g.Generate(context.Background(), newReq())
	if err != nil {
		t.Fatalf("Generate should fall back, got %v", err)
	}
	if !strings.Contains(string(resp.Body), "mock response generated") {
		t.Errorf("expected static fallback body, got %q", resp.Body)
	}
	if provider.calls != 0 {
		t.Errorf("expected no provider calls when schema load fails, got %d", provider.calls)
	}
}

func TestAIGeneratorPreservesValidJSONShape(t *testing.T) {
	provider := &fakeAI{responses: []*ai.Response{jsonResponse(`{"id": 3, "name": "ada"}`)}}
	schemas := &fakeSchemas{schema: testSchema}
	g := newAITest(t, provider, schemas)

	resp, err := g.Generate(context.Background(), newReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		t.Fatalf("response must be valid JSON: %v", err)
	}
}

func TestAIGeneratorRendersRequestVariablesIntoPrompt(t *testing.T) {
	provider := &fakeAI{responses: []*ai.Response{jsonResponse(`{"id": 42}`)}}
	schemas := &fakeSchemas{schema: testSchema}
	g := newAITest(t, provider, schemas)

	req := newReq()
	req.Endpoint.Prompt = "return user {{path.id}} from {{query.country}}"
	req.Query = map[string]string{"country": "fr"}
	req.Body = []byte(`{"email":"a@b.co"}`)

	if _, err := g.Generate(context.Background(), req); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if got, want := provider.lastReq.Prompt, "return user 42 from fr"; got != want {
		t.Errorf("expected rendered prompt %q, got %q", want, got)
	}
	if provider.lastReq.Body == nil {
		t.Errorf("expected raw body to still reach the provider")
	}
}

// assertSchemaFilled verifies the fallback produced faker data matching the
// test schema (an object with an integer "id").
func assertSchemaFilled(t *testing.T, body []byte) {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("fallback body must be valid JSON, got %q: %v", body, err)
	}
	id, ok := out["id"].(float64)
	if !ok {
		t.Fatalf("expected faker-filled integer id, got %q", body)
	}
	if id < 1 || id > 1000 {
		t.Errorf("expected id in faker range, got %v", id)
	}
}
