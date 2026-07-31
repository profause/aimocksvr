package prompts

import (
	"strings"
	"testing"
)

func TestSystemContainsRequiredConstraints(t *testing.T) {
	system := System()

	for _, constraint := range []string{
		"valid JSON",
		"Markdown",
		"JSON Schema",
		"deterministic",
	} {
		if !strings.Contains(system, constraint) {
			t.Errorf("system prompt missing %q: %q", constraint, system)
		}
	}
}

func TestSystemMentionsNoMarkdown(t *testing.T) {
	if !strings.Contains(System(), "Never return Markdown") {
		t.Error("system prompt must forbid Markdown")
	}
}

func TestSchemaRequest(t *testing.T) {
	msgs := SchemaRequest("return a user")

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != RoleSystem || msgs[0].Content != System() {
		t.Errorf("expected system message, got %+v", msgs[0])
	}
	if msgs[1].Role != RoleUser {
		t.Errorf("expected user role, got %q", msgs[1].Role)
	}
	if !strings.Contains(msgs[1].Content, "JSON Schema") {
		t.Errorf("schema request should ask for a JSON Schema, got %q", msgs[1].Content)
	}
	if !strings.Contains(msgs[1].Content, "return a user") {
		t.Errorf("schema request should embed the endpoint prompt, got %q", msgs[1].Content)
	}
}

func TestResponseRequest(t *testing.T) {
	msgs := ResponseRequest("return users", "")

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != RoleSystem || msgs[0].Content != System() {
		t.Errorf("expected system message, got %+v", msgs[0])
	}
	if msgs[1].Role != RoleUser {
		t.Errorf("expected user role, got %q", msgs[1].Role)
	}
	if !strings.Contains(msgs[1].Content, "JSON response") {
		t.Errorf("response request should ask for a JSON response, got %q", msgs[1].Content)
	}
	if !strings.Contains(msgs[1].Content, "return users") {
		t.Errorf("response request should embed the endpoint prompt, got %q", msgs[1].Content)
	}
	if strings.Contains(msgs[1].Content, "JSON Schema") {
		t.Errorf("response request without a schema must not mention schemas, got %q", msgs[1].Content)
	}
}

func TestResponseRequestEmbedsSchema(t *testing.T) {
	schema := `{"type":"object"}`
	msgs := ResponseRequest("return users", schema)

	if !strings.Contains(msgs[1].Content, schema) {
		t.Errorf("response request should embed the schema, got %q", msgs[1].Content)
	}
	if !strings.Contains(msgs[1].Content, "Match this JSON Schema exactly") {
		t.Errorf("response request should instruct matching the schema, got %q", msgs[1].Content)
	}
}

func TestRequestsUseSystemPromptFirst(t *testing.T) {
	for name, msgs := range map[string][]Message{
		"schema":   SchemaRequest("x"),
		"response": ResponseRequest("x", ""),
	} {
		if len(msgs) == 0 || msgs[0].Role != RoleSystem {
			t.Errorf("%s request must lead with the system prompt", name)
		}
	}
}
