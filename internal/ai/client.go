package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/profause/aimocksvr/internal/prompts"
)

// clientConfig holds the resolved settings for an OpenAI-compatible backend.
type clientConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	HTTP      *http.Client
	Provider  ProviderType
	MaxTokens int
}

// openAIClient talks to any OpenAI-compatible chat completions endpoint.
// OpenAI, OpenRouter, and Ollama all implement this wire format, which is why
// a single implementation serves all three providers.
type openAIClient struct {
	cfg clientConfig
}

// toChatMessages converts the shared prompt templates into the wire format.
func toChatMessages(msgs []prompts.Message) []chatMessage {
	out := make([]chatMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, chatMessage{Role: m.Role, Content: m.Content})
	}
	return out
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// complete calls the chat completions endpoint and returns the model's text
// output. jsonMode asks for a JSON object via response_format; it is ignored
// when the backend does not support it (JSON mode only influences the request
// payload).
func (c *openAIClient) complete(ctx context.Context, messages []chatMessage, jsonMode bool) (string, error) {
	body := chatCompletionRequest{
		Model:       c.cfg.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   c.cfg.MaxTokens,
	}
	if jsonMode {
		body.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encode chat request: %w", err)
	}

	endpoint := strings.TrimSuffix(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.cfg.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat request to %s: %w", c.cfg.Provider, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read chat response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat request to %s failed: status %d: %s", c.cfg.Provider, resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var out chatCompletionResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("decode chat response: %w", err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("chat response from %s was empty", c.cfg.Provider)
	}
	return out.Choices[0].Message.Content, nil
}

func (c *openAIClient) GenerateSchema(ctx context.Context, req SchemaRequest) (*Schema, error) {
	prompt := req.Prompt
	if prompt == "" {
		prompt = req.Endpoint.Prompt
	}

	content, err := c.complete(ctx, toChatMessages(prompts.SchemaRequest(prompt)), true)
	if err != nil {
		return nil, err
	}

	var schema Schema
	if err := json.Unmarshal([]byte(content), &schema); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON schema: %w", err)
	}
	return &schema, nil
}

func (c *openAIClient) GenerateResponse(ctx context.Context, req ResponseRequest) (*Response, error) {
	prompt := req.Prompt
	if prompt == "" {
		prompt = req.Endpoint.Prompt
	}

	content, err := c.complete(ctx, toChatMessages(prompts.ResponseRequest(prompt, req.Schema)), true)
	if err != nil {
		return nil, err
	}
	if !json.Valid([]byte(content)) {
		return nil, fmt.Errorf("model returned invalid JSON: %s", truncate(content, 200))
	}
	return &Response{Status: http.StatusOK, Body: []byte(content)}, nil
}

func (c *openAIClient) GeneratePrompt(ctx context.Context, req PromptRequest) (string, error) {
	messages := make([]chatMessage, 0, 2)
	if req.System != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.System})
	}
	messages = append(messages, chatMessage{Role: "user", Content: req.User})
	return c.complete(ctx, messages, false)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
