package generator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/validator"
)

func newFakerTest(t *testing.T, schemas SchemaLoader) Generator {
	t.Helper()
	logger := zerolog.Nop()
	return NewFaker(schemas, NewStatic(), &logger)
}

func newSchemaReq() *Request {
	return &Request{
		Endpoint: &models.Endpoint{
			ID:     uuid.New(),
			Method: "GET",
			Path:   "/users/:id",
			Prompt: "return a user",
		},
		PathParams: map[string]string{"id": "1"},
	}
}

func TestFakerGeneratorFillsSchemaWithRealisticValues(t *testing.T) {
	schema := `{
		"type":"object",
		"required":["id","email"],
		"properties":{
			"id":{"type":"string","format":"uuid"},
			"name":{"type":"string"},
			"email":{"type":"string","format":"email"},
			"phone":{"type":"string"},
			"age":{"type":"integer","minimum":18,"maximum":99},
			"score":{"type":"number"},
			"active":{"type":"boolean"},
			"company":{"type":"string"},
			"country":{"type":"string"},
			"created_at":{"type":"string","format":"date-time"}
		}
	}`
	g := newFakerTest(t, &fakeSchemas{schema: schema})

	resp, err := g.Generate(context.Background(), newSchemaReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.Status)
	}

	var out map[string]any
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		t.Fatalf("body must be JSON, got %q: %v", resp.Body, err)
	}

	// The generated body must satisfy the schema.
	if err := validator.New().ValidateResponse([]byte(schema), resp.Body); err != nil {
		t.Fatalf("faker output must validate against the schema: %v (body %q)", err, resp.Body)
	}

	if id, _ := out["id"].(string); !isUUID(id) {
		t.Errorf("expected uuid id, got %q", out["id"])
	}
	if email, _ := out["email"].(string); !strings.Contains(email, "@") {
		t.Errorf("expected email, got %q", out["email"])
	}
	if phone, _ := out["phone"].(string); phone == "" {
		t.Errorf("expected phone, got %q", phone)
	}
	if age, ok := out["age"].(float64); !ok || age < 18 || age > 99 {
		t.Errorf("expected age in [18,99], got %v", out["age"])
	}
	if _, ok := out["score"].(float64); !ok {
		t.Errorf("expected numeric score, got %T", out["score"])
	}
	if _, ok := out["active"].(bool); !ok {
		t.Errorf("expected boolean active, got %T", out["active"])
	}
	if company, _ := out["company"].(string); company == "" {
		t.Errorf("expected company, got %q", company)
	}
	if country, _ := out["country"].(string); country == "" {
		t.Errorf("expected country, got %q", country)
	}
	if created, _ := out["created_at"].(string); !isRFC3339(created) {
		t.Errorf("expected date-time, got %q", created)
	}
}

func TestFakerGeneratorHandlesNestedObjects(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{
			"user":{
				"type":"object",
				"properties":{
					"first_name":{"type":"string"},
					"address":{"type":"object","properties":{"city":{"type":"string"},"zip":{"type":"string"}}}
				}
			}
		}
	}`
	g := newFakerTest(t, &fakeSchemas{schema: schema})

	resp, err := g.Generate(context.Background(), newSchemaReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if err := validator.New().ValidateResponse([]byte(schema), resp.Body); err != nil {
		t.Fatalf("nested output must validate: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		t.Fatalf("body must be JSON: %v", err)
	}
	user := out["user"].(map[string]any)
	if _, ok := user["first_name"].(string); !ok {
		t.Errorf("expected first name, got %v", user["first_name"])
	}
	addr := user["address"].(map[string]any)
	if _, ok := addr["city"].(string); !ok {
		t.Errorf("expected city, got %v", addr["city"])
	}
}

func TestFakerGeneratorFillsArrays(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{
			"tags":{"type":"array","items":{"type":"string"},"minItems":2,"maxItems":2},
			"prices":{"type":"array","items":{"type":"number"}}
		}
	}`
	g := newFakerTest(t, &fakeSchemas{schema: schema})

	resp, err := g.Generate(context.Background(), newSchemaReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if err := validator.New().ValidateResponse([]byte(schema), resp.Body); err != nil {
		t.Fatalf("array output must validate: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		t.Fatalf("body must be JSON: %v", err)
	}
	if tags := out["tags"].([]any); len(tags) != 2 {
		t.Errorf("expected 2 tags (minItems), got %d", len(tags))
	}
	if prices := out["prices"].([]any); len(prices) < 1 {
		t.Errorf("expected at least one price, got %d", len(prices))
	}
}

func TestFakerGeneratorHonorsEnumAndConst(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{
			"status":{"type":"string","enum":["active","inactive"]},
			"kind":{"type":"string","const":"user"}
		}
	}`
	g := newFakerTest(t, &fakeSchemas{schema: schema})

	resp, err := g.Generate(context.Background(), newSchemaReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if err := validator.New().ValidateResponse([]byte(schema), resp.Body); err != nil {
		t.Fatalf("enum/const output must validate: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		t.Fatalf("body must be JSON: %v", err)
	}
	if out["kind"] != "user" {
		t.Errorf("expected const value, got %v", out["kind"])
	}
	status := out["status"].(string)
	if status != "active" && status != "inactive" {
		t.Errorf("expected enum member, got %q", status)
	}
}

func TestFakerGeneratorFallsBackWithoutSchema(t *testing.T) {
	g := newFakerTest(t, &fakeSchemas{schema: ""})

	resp, err := g.Generate(context.Background(), newSchemaReq())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !strings.Contains(string(resp.Body), "mock response generated") {
		t.Errorf("expected static fallback body, got %q", resp.Body)
	}
}

func TestFakerGeneratorFallsBackOnSchemaLoadError(t *testing.T) {
	g := newFakerTest(t, &fakeSchemas{err: errors.New("db down")})

	resp, err := g.Generate(context.Background(), newSchemaReq())
	if err != nil {
		t.Fatalf("Generate should fall back, got %v", err)
	}
	if !strings.Contains(string(resp.Body), "mock response generated") {
		t.Errorf("expected static fallback body, got %q", resp.Body)
	}
}

func TestFakerGeneratorFallsBackOnInvalidSchema(t *testing.T) {
	g := newFakerTest(t, &fakeSchemas{schema: "not a schema"})

	resp, err := g.Generate(context.Background(), newSchemaReq())
	if err != nil {
		t.Fatalf("Generate should fall back, got %v", err)
	}
	if !strings.Contains(string(resp.Body), "mock response generated") {
		t.Errorf("expected static fallback body, got %q", resp.Body)
	}
}

func TestFakerGeneratorCoversRoadmapGenerators(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{
			"person":{"type":"string"},
			"company":{"type":"string"},
			"email":{"type":"string"},
			"phone":{"type":"string"},
			"address":{"type":"string"},
			"uuid":{"type":"string"},
			"bank_account":{"type":"string"},
			"credit_card":{"type":"string"},
			"birth_date":{"type":"string"},
			"currency":{"type":"string"},
			"country":{"type":"string"}
		}
	}`
	g := newFakerTest(t, &fakeSchemas{schema: schema})

	for i := 0; i < 5; i++ {
		resp, err := g.Generate(context.Background(), newSchemaReq())
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(resp.Body, &out); err != nil {
			t.Fatalf("body must be JSON: %v", err)
		}
		for key, value := range out {
			if value == "" || value == nil {
				t.Errorf("expected a value for %s, got %v", key, value)
			}
		}
		if cc, _ := out["credit_card"].(string); len(cc) < 12 {
			t.Errorf("expected a card number, got %q", cc)
		}
		if bank, _ := out["bank_account"].(string); len(bank) < 4 {
			t.Errorf("expected a bank account, got %q", bank)
		}
	}
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func isRFC3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
