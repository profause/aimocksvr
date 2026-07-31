package ai

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/config"
)

// providerDefaults returns the default base URL, model, and whether an API key
// is required for a provider.
func providerDefaults(t ProviderType) (baseURL, model string, keyRequired bool) {
	switch t {
	case ProviderOpenAI:
		return "https://api.openai.com/v1", "gpt-4o-mini", true
	case ProviderOpenRouter:
		return "https://openrouter.ai/api/v1", "openai/gpt-4o-mini", true
	case ProviderOllama:
		return "http://localhost:11434/v1", "llama3.2", false
	}
	return "", "", false
}

// New builds the AIProvider selected by configuration. An empty or unknown
// provider yields a Noop so the server runs without a model backend.
func New(cfg *config.Config, logger *zerolog.Logger) AIProvider {
	provider := ProviderType(strings.ToLower(strings.TrimSpace(cfg.AI.Provider)))
	if provider == "" {
		logger.Info().Msg("ai provider disabled")
		return Noop{}
	}

	baseURL, model, keyRequired := providerDefaults(provider)
	if baseURL == "" {
		logger.Warn().Str("provider", string(provider)).Msg("unknown ai provider, disabling ai")
		return Noop{}
	}
	if cfg.AI.BaseURL != "" {
		baseURL = cfg.AI.BaseURL
	}
	if cfg.AI.Model != "" {
		model = cfg.AI.Model
	}
	if keyRequired && cfg.AI.APIKey == "" {
		logger.Warn().Str("provider", string(provider)).Msg("ai provider requires an api key, disabling ai")
		return Noop{}
	}

	timeout := 60 * time.Second
	if cfg.AI.Timeout != "" {
		parsed, err := time.ParseDuration(cfg.AI.Timeout)
		if err != nil {
			logger.Warn().Err(err).Msg("invalid ai timeout, using default")
		} else {
			timeout = parsed
		}
	}

	logger.Info().Str("provider", string(provider)).Str("base_url", baseURL).Str("model", model).Msg("ai provider enabled")
	return &openAIClient{cfg: clientConfig{
		BaseURL:   baseURL,
		APIKey:    cfg.AI.APIKey,
		Model:     model,
		Provider:  provider,
		MaxTokens: 2048,
		HTTP:      &http.Client{Timeout: timeout},
	}}
}
