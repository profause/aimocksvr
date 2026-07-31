package importer

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const v3Spec = `{
  "openapi": "3.0.0",
  "info": {"title": "Users API", "version": "1.0.0"},
  "paths": {
    "/users": {
      "get": {
        "summary": "List users",
        "responses": {
          "200": {
            "description": "ok",
            "content": {"application/json": {"schema": {"type": "array", "items": {"$ref": "#/components/schemas/User"}}}}
          }
        }
      },
      "post": {
        "summary": "Create a user",
        "responses": {
          "201": {
            "description": "created",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/User"}}}
          }
        }
      }
    },
    "/users/{id}": {
      "get": {
        "summary": "Get a user by id",
        "description": "Returns a single user with the given identifier.",
        "responses": {
          "200": {
            "description": "ok",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/User"}}}
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "User": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid"},
          "name": {"type": "string"}
        }
      }
    }
  }
}`

const v2Spec = `{
  "swagger": "2.0",
  "info": {"title": "Legacy", "version": "1"},
  "paths": {
    "/pets": {
      "get": {
        "summary": "List pets",
        "responses": {
          "200": {"description": "ok", "schema": {"type": "array", "items": {"$ref": "#/definitions/Pet"}}}
        }
      }
    }
  },
  "definitions": {
    "Pet": {
      "type": "object",
      "properties": {"name": {"type": "string"}}
    }
  }
}`

const v3YAML = `
openapi: 3.0.3
info:
  title: YAML API
  version: "1"
paths:
  /widgets/{widgetId}:
    delete:
      summary: Remove a widget
      responses:
        "204":
          description: removed
`

func TestParseOpenAPIv3JSON(t *testing.T) {
	specs, err := Parse([]byte(v3Spec))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(specs))
	}

	wantPaths := []string{"/users", "/users", "/users/:id"}
	wantMethods := []string{"GET", "POST", "GET"}
	wantPrompts := []string{"List users.", "Create a user.", "Get a user by id. Returns a single user with the given identifier."}
	for i, s := range specs {
		if s.Method != wantMethods[i] {
			t.Errorf("spec[%d] method = %q, want %q", i, s.Method, wantMethods[i])
		}
		if s.Path != wantPaths[i] {
			t.Errorf("spec[%d] path = %q, want %q", i, s.Path, wantPaths[i])
		}
		if s.Prompt != wantPrompts[i] {
			t.Errorf("spec[%d] prompt = %q, want %q", i, s.Prompt, wantPrompts[i])
		}
	}

	for _, s := range specs {
		if s.Schema == "" {
			t.Fatalf("expected schema for %s %s", s.Method, s.Path)
		}
		var doc any
		if err := json.Unmarshal([]byte(s.Schema), &doc); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
	}
}

func TestParseOpenAPIv3YAML(t *testing.T) {
	specs, err := Parse([]byte(v3YAML))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(specs))
	}
	s := specs[0]
	if s.Method != "DELETE" || s.Path != "/widgets/:widgetId" {
		t.Errorf("unexpected endpoint %s %s", s.Method, s.Path)
	}
	if s.Prompt != "Remove a widget." {
		t.Errorf("prompt = %q, want %q", s.Prompt, "Remove a widget.")
	}
	if s.Schema != "" {
		t.Errorf("expected empty schema for 204 response, got %q", s.Schema)
	}
}

func TestParseSwagger2JSON(t *testing.T) {
	specs, err := Parse([]byte(v2Spec))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(specs))
	}
	s := specs[0]
	if s.Method != "GET" || s.Path != "/pets" {
		t.Errorf("unexpected endpoint %s %s", s.Method, s.Path)
	}

	var want, got any
	if err := json.Unmarshal([]byte(`{"items":{"properties":{"name":{"type":"string"}},"type":"object"},"type":"array"}`), &want); err != nil {
		t.Fatalf("decode want: %v", err)
	}
	if err := json.Unmarshal([]byte(s.Schema), &got); err != nil {
		t.Fatalf("schema not valid JSON: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("resolved schema mismatch:\n want %#v\n  got %#v", want, got)
	}
}

func TestParseResolvesNestedRefs(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "paths": {
    "/teams": {
      "get": {
        "responses": {
          "200": {"description": "ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/TeamList"}}}}
        }
      }
    }
  },
  "components": {
    "schemas": {
      "TeamList": {"type": "object", "properties": {"teams": {"type": "array", "items": {"$ref": "#/components/schemas/Team"}}}},
      "Team": {"type": "object", "properties": {"lead": {"$ref": "#/components/schemas/User"}}},
      "User": {"type": "object", "properties": {"name": {"type": "string"}}}
    }
  }
}`
	specs, err := Parse([]byte(spec))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(specs))
	}
	if len(specs[0].Schema) == 0 {
		t.Fatal("expected a resolved schema")
	}
	if strings.Contains(specs[0].Schema, "$ref") {
		t.Errorf("schema still contains $ref: %s", specs[0].Schema)
	}
}

func TestParseComposedSchemaStaysJSON(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "paths": {
    "/pets": {
      "post": {
        "responses": {
          "200": {"description": "ok", "content": {"application/json": {"schema": {"allOf": [{"$ref": "#/components/schemas/Pet"}, {"type": "object", "properties": {"tag": {"type": "string"}}}]}}}}
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Pet": {"type": "object", "properties": {"name": {"type": "string"}}}
    }
  }
}`
	specs, err := Parse([]byte(spec))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(specs) != 1 || specs[0].Schema == "" {
		t.Fatalf("expected schema for composed response, got %+v", specs[0])
	}
	if strings.Contains(specs[0].Schema, "$ref") {
		t.Errorf("schema still contains $ref: %s", specs[0].Schema)
	}
}

func TestParseFallbackPrompt(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "paths": {
    "/widgets": {"get": {"responses": {"200": {"description": "ok"}}}}
  }
}`
	specs, err := Parse([]byte(spec))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(specs))
	}
	if specs[0].Prompt != "handle GET /widgets." {
		t.Errorf("fallback prompt = %q, want %q", specs[0].Prompt, "handle GET /widgets.")
	}
	if specs[0].Schema != "" {
		t.Errorf("expected empty schema, got %q", specs[0].Schema)
	}
}

func TestParseRejectsNonOpenAPI(t *testing.T) {
	_, err := Parse([]byte(`{"hello": "world"}`))
	if err == nil {
		t.Fatal("expected error for non-OpenAPI document")
	}
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Fatalf("expected ParseError, got %T: %v", err, err)
	}
}

func TestParseRejectsEmpty(t *testing.T) {
	if _, err := Parse([]byte("")); err == nil {
		t.Fatal("expected error for empty document")
	}
}

func TestParseRejectsNoPaths(t *testing.T) {
	_, err := Parse([]byte(`{"openapi": "3.0.0", "info": {"title": "t", "version": "1"}}`))
	if err == nil {
		t.Fatal("expected error for document without paths")
	}
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Fatalf("expected ParseError, got %T: %v", err, err)
	}
}

func TestParseRejectsInvalidDocument(t *testing.T) {
	if _, err := Parse([]byte("not: [valid")); err == nil {
		t.Fatal("expected error for malformed document")
	}
}

func TestParseNoSchemaOnNonJSONContent(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "paths": {
    "/file": {"get": {"responses": {"200": {"description": "ok", "content": {"application/octet-stream": {"schema": {"type": "string", "format": "binary"}}}}}}}
  }
}`
	specs, err := Parse([]byte(spec))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(specs))
	}
	if specs[0].Schema != "" {
		t.Errorf("expected no schema for non-JSON content, got %q", specs[0].Schema)
	}
}

const v3RequestSpec = `{
  "openapi": "3.0.0",
  "paths": {
    "/users": {
      "post": {
        "summary": "Create a user",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"$ref": "#/components/schemas/UserInput"}}}
        },
        "responses": {
          "201": {"description": "created", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/User"}}}}
        }
      }
    }
  },
  "components": {
    "schemas": {
      "User": {"type": "object", "properties": {"id": {"type": "string", "format": "uuid"}}},
      "UserInput": {"type": "object", "required": ["email"], "properties": {"email": {"type": "string", "format": "email"}}}
    }
  }
}`

func TestParseExtractsRequestBodySchema(t *testing.T) {
	specs, err := Parse([]byte(v3RequestSpec))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(specs))
	}
	s := specs[0]
	if s.RequestSchema == "" {
		t.Fatal("expected a request schema")
	}
	if strings.Contains(s.RequestSchema, "$ref") {
		t.Errorf("request schema still contains $ref: %s", s.RequestSchema)
	}
	var doc any
	if err := json.Unmarshal([]byte(s.RequestSchema), &doc); err != nil {
		t.Fatalf("request schema is not valid JSON: %v", err)
	}
	obj := doc.(map[string]any)
	if _, ok := obj["required"]; !ok {
		t.Errorf("expected required keyword preserved, got %#v", obj)
	}
	if s.Schema == "" {
		t.Error("expected response schema too")
	}
}

func TestParseNoRequestBodySchema(t *testing.T) {
	specs, err := Parse([]byte(v3Spec))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	for _, s := range specs {
		if s.RequestSchema != "" {
			t.Errorf("expected no request schema for %s %s, got %q", s.Method, s.Path, s.RequestSchema)
		}
	}
}

func TestParseRequestSchemaNonJSONContent(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "paths": {
    "/upload": {
      "post": {
        "requestBody": {"content": {"multipart/form-data": {"schema": {"type": "object", "properties": {"file": {"type": "string"}}}}}},
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`
	specs, err := Parse([]byte(spec))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(specs))
	}
	if specs[0].RequestSchema != "" {
		t.Errorf("expected no request schema for non-JSON content, got %q", specs[0].RequestSchema)
	}
}
