package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient starts a mock OpenAI-compatible endpoint and returns a client
// pointed at it.
func newTestClient(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *openAIClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	return &openAIClient{cfg: clientConfig{
		BaseURL:   srv.URL,
		APIKey:    "test-key",
		Model:     "test-model",
		Provider:  ProviderOpenAI,
		MaxTokens: 2048,
		HTTP:      srv.Client(),
	}}
}

func completionResponse(content string) map[string]any {
	return map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{"content": content},
			},
		},
	}
}

func TestGenerateResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected application/json, got %q", got)
		}

		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("expected model test-model, got %q", req.Model)
		}
		if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
			t.Errorf("expected json_object response format, got %+v", req.ResponseFormat)
		}
		if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
			t.Errorf("unexpected messages: %+v", req.Messages)
		}

		json.NewEncoder(w).Encode(completionResponse(`{"id": 42, "name": "ada"}`))
	})

	resp, err := client.GenerateResponse(context.Background(), ResponseRequest{Prompt: "return a user"})
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.Status)
	}
	want := `{"id": 42, "name": "ada"}`
	if string(resp.Body) != want {
		t.Errorf("expected body %q, got %q", want, resp.Body)
	}
}

func TestGenerateResponseRejectsInvalidJSON(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(completionResponse("not json"))
	})

	_, err := client.GenerateResponse(context.Background(), ResponseRequest{Prompt: "p"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGenerateSchema(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(completionResponse(`{"type": "object", "properties": {"id": {"type": "integer"}}}`))
	})

	schema, err := client.GenerateSchema(context.Background(), SchemaRequest{Prompt: "return a user"})
	if err != nil {
		t.Fatalf("GenerateSchema failed: %v", err)
	}
	if (*schema)["type"] != "object" {
		t.Errorf("expected type object, got %v", (*schema)["type"])
	}
}

func TestGenerateSchemaRejectsInvalidJSON(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(completionResponse("nope"))
	})

	_, err := client.GenerateSchema(context.Background(), SchemaRequest{Prompt: "p"})
	if err == nil {
		t.Fatal("expected error for invalid schema")
	}
}

func TestGeneratePromptFreeform(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req chatCompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.ResponseFormat != nil {
			t.Errorf("expected no response_format, got %+v", req.ResponseFormat)
		}
		if len(req.Messages) != 2 || req.Messages[1].Content != "hello" {
			t.Errorf("unexpected messages: %+v", req.Messages)
		}
		json.NewEncoder(w).Encode(completionResponse("hi there"))
	})

	out, err := client.GeneratePrompt(context.Background(), PromptRequest{System: "be terse", User: "hello"})
	if err != nil {
		t.Fatalf("GeneratePrompt failed: %v", err)
	}
	if out != "hi there" {
		t.Errorf("expected %q, got %q", "hi there", out)
	}
}

func TestGenerateResponseErrorStatus(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error": {"message": "rate limited"}}`)
	})

	_, err := client.GenerateResponse(context.Background(), ResponseRequest{Prompt: "p"})
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("429")) {
		t.Errorf("expected status code in error, got %v", err)
	}
}

func TestGenerateResponseEmptyChoices(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	})

	_, err := client.GenerateResponse(context.Background(), ResponseRequest{Prompt: "p"})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}
