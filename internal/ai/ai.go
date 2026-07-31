// Package ai abstracts the LLM providers that generate mock schemas and
// responses.
//
// OpenAI, OpenRouter, and Ollama all expose OpenAI-compatible chat completion
// endpoints, so a single HTTP client implements all of them; they differ only
// in base URL, API key, and model. The provider is selected through
// configuration, and an empty provider selection yields a Noop that keeps the
// server running without any model backend.
package ai

import (
	"context"
	"net/http"

	"github.com/profause/aimocksvr/internal/models"
)

// ProviderType identifies a supported LLM backend.
type ProviderType string

const (
	// ProviderOpenAI is the hosted OpenAI API.
	ProviderOpenAI ProviderType = "openai"
	// ProviderOllama is a self-hosted Ollama instance.
	ProviderOllama ProviderType = "ollama"
	// ProviderOpenRouter is the OpenRouter multi-model gateway.
	ProviderOpenRouter ProviderType = "openrouter"
)

// AIProvider generates the artifacts the mock server needs: JSON Schemas for
// endpoint responses, response bodies for individual requests, and freeform
// completions used as building blocks.
type AIProvider interface {
	// GenerateSchema asks the model for a JSON Schema describing the shape of
	// an endpoint's response.
	GenerateSchema(ctx context.Context, req SchemaRequest) (*Schema, error)
	// GenerateResponse produces a JSON response body for a single mock
	// request.
	GenerateResponse(ctx context.Context, req ResponseRequest) (*Response, error)
	// GeneratePrompt runs a freeform system/user completion and returns the
	// raw model output.
	GeneratePrompt(ctx context.Context, req PromptRequest) (string, error)
}

// SchemaRequest asks for the JSON Schema of an endpoint's response.
type SchemaRequest struct {
	Endpoint models.Endpoint
	Prompt   string
}

// Schema is a JSON Schema document produced by a model.
type Schema map[string]any

// ResponseRequest describes the inbound request used to generate a response.
// Schema, when non-empty, is embedded in the prompt so the model matches it.
type ResponseRequest struct {
	Endpoint   models.Endpoint
	Prompt     string
	Schema     string
	PathParams map[string]string
	Query      map[string]string
	Headers    http.Header
	Body       []byte
}

// Response is the generated mock response body.
type Response struct {
	Status int
	Body   []byte
}

// PromptRequest is a freeform completion with an optional system prompt.
type PromptRequest struct {
	System string
	User   string
}
