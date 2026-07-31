package validator

import (
	"strings"
	"testing"
)

const userSchema = `{"type":"object","required":["id","name"],"properties":{"id":{"type":"integer"},"name":{"type":"string"}}}`

func TestValidateSchemaAcceptsValidSchema(t *testing.T) {
	if err := New().ValidateSchema([]byte(userSchema)); err != nil {
		t.Fatalf("expected valid schema to pass, got %v", err)
	}
}

func TestValidateSchemaRejectsInvalidSchema(t *testing.T) {
	for _, schema := range []string{
		"not json",
		`{"type": "object", "properties": "bogus"}`,
		`{"type": "unknown-type"}`,
	} {
		if err := New().ValidateSchema([]byte(schema)); err == nil {
			t.Errorf("expected invalid schema %q to be rejected", schema)
		}
	}
}

func TestValidateResponsePasses(t *testing.T) {
	v := New()

	if err := v.ValidateResponse([]byte(userSchema), []byte(`{"id": 1, "name": "ada"}`)); err != nil {
		t.Fatalf("expected matching response to pass, got %v", err)
	}
}

func TestValidateResponseRejectsMissingRequired(t *testing.T) {
	v := New()

	err := v.ValidateResponse([]byte(userSchema), []byte(`{"id": 1}`))
	if err == nil {
		t.Fatal("expected missing required field to fail")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("expected error to mention the missing field, got %v", err)
	}
}

func TestValidateResponseRejectsWrongType(t *testing.T) {
	v := New()

	if err := v.ValidateResponse([]byte(userSchema), []byte(`{"id": "nope", "name": "ada"}`)); err == nil {
		t.Fatal("expected wrong type to fail")
	}
}

func TestValidateResponseRejectsInvalidDocument(t *testing.T) {
	v := New()

	if err := v.ValidateResponse([]byte(userSchema), []byte(`{not json`)); err == nil {
		t.Fatal("expected malformed response to fail")
	}
}

func TestValidateResponseRejectsInvalidSchema(t *testing.T) {
	v := New()

	if err := v.ValidateResponse([]byte(`not a schema`), []byte(`{}`)); err == nil {
		t.Fatal("expected invalid schema to fail")
	}
}

func TestValidateRequestPasses(t *testing.T) {
	v := New()

	if err := v.ValidateRequest([]byte(userSchema), []byte(`{"id": 1, "name": "ada"}`)); err != nil {
		t.Fatalf("expected matching request to pass, got %v", err)
	}
}

func TestValidateRequestRejectsMissingRequired(t *testing.T) {
	v := New()

	err := v.ValidateRequest([]byte(userSchema), []byte(`{"name": "ada"}`))
	if err == nil {
		t.Fatal("expected missing required field to fail")
	}
	if !strings.Contains(err.Error(), "id") {
		t.Errorf("expected error to mention the missing field, got %v", err)
	}
}

func TestValidateRequestRejectsMalformedBody(t *testing.T) {
	v := New()

	if err := v.ValidateRequest([]byte(userSchema), []byte(`{not json`)); err == nil {
		t.Fatal("expected malformed request to fail")
	}
}
