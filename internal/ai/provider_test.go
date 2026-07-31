package ai

import (
	"testing"

	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/config"
)

func newLogger() *zerolog.Logger {
	l := zerolog.Nop()
	return &l
}

func TestNewDisabledWhenProviderEmpty(t *testing.T) {
	cfg := &config.Config{}
	if _, ok := New(cfg, newLogger()).(Noop); !ok {
		t.Fatal("expected Noop when provider is empty")
	}
}

func TestNewDisabledOnUnknownProvider(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Provider = "wat"
	if _, ok := New(cfg, newLogger()).(Noop); !ok {
		t.Fatal("expected Noop for unknown provider")
	}
}

func TestNewOpenAISetsDefaultsAndRequiresKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Provider = "openai"
	if _, ok := New(cfg, newLogger()).(Noop); !ok {
		t.Fatal("expected Noop when openai has no api key")
	}

	cfg.AI.APIKey = "sk-test"
	c := New(cfg, newLogger())
	client, ok := c.(*openAIClient)
	if !ok {
		t.Fatalf("expected openAIClient, got %T", c)
	}
	if client.cfg.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("unexpected base url %q", client.cfg.BaseURL)
	}
	if client.cfg.Model != "gpt-4o-mini" {
		t.Errorf("unexpected model %q", client.cfg.Model)
	}
}

func TestNewOpenRouterDefaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Provider = "openrouter"
	cfg.AI.APIKey = "sk-test"
	c := New(cfg, newLogger())
	client := c.(*openAIClient)
	if client.cfg.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("unexpected base url %q", client.cfg.BaseURL)
	}
}

func TestNewOllamaNoKeyRequired(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Provider = "ollama"
	c := New(cfg, newLogger())
	client, ok := c.(*openAIClient)
	if !ok {
		t.Fatalf("expected openAIClient, got %T", c)
	}
	if client.cfg.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("unexpected base url %q", client.cfg.BaseURL)
	}
	if client.cfg.Model != "llama3.2" {
		t.Errorf("unexpected model %q", client.cfg.Model)
	}
}

func TestNewOverrides(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Provider = "ollama"
	cfg.AI.BaseURL = "http://other:1234/v1"
	cfg.AI.Model = "custom-model"
	c := New(cfg, newLogger())
	client := c.(*openAIClient)
	if client.cfg.BaseURL != "http://other:1234/v1" {
		t.Errorf("unexpected base url %q", client.cfg.BaseURL)
	}
	if client.cfg.Model != "custom-model" {
		t.Errorf("unexpected model %q", client.cfg.Model)
	}
}
