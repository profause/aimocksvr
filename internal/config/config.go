// Package config loads and exposes application configuration.
//
// Configuration is layered: defaults are defined in code, a configs/config.yaml
// file is applied when present, and environment variables prefixed with
// MOCKSVR_ always take precedence (e.g. MOCKSVR_DATABASE_URL).
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	defaultHost = "0.0.0.0"
	defaultPort = 8080
)

// Config is the root configuration for the application.
type Config struct {
	App      App      `mapstructure:"app"`
	Server   Server   `mapstructure:"server"`
	Database Database `mapstructure:"database"`
	Cache    Cache    `mapstructure:"cache"`
	AI       AI       `mapstructure:"ai"`
	Auth     Auth     `mapstructure:"auth"`
	Dashboard Dashboard `mapstructure:"dashboard"`
	Log      Log      `mapstructure:"log"`
}

type App struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type Server struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type Database struct {
	URL string `mapstructure:"url"`
}

type Cache struct {
	Redis Redis `mapstructure:"redis"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	TTL      string `mapstructure:"ttl"`
}

// AI holds the LLM provider settings. An empty Provider disables AI entirely,
// so the server runs without any model backend configured.
type AI struct {
	Provider string `mapstructure:"provider"` // openai | ollama | openrouter
	BaseURL  string `mapstructure:"base_url"`
	APIKey   string `mapstructure:"api_key"`
	Model    string `mapstructure:"model"`
	Timeout  string `mapstructure:"timeout"`
}

type Log struct {
	Level string `mapstructure:"level"`
}

// Auth holds the authentication settings. Auth is disabled by default so the
// server behaves as an open mock. When enabled, the control plane requires a
// valid credential (API key, workspace token or JWT) and mock endpoints can be
// marked non-public. API keys and workspace tokens are configured as
// comma-separated "name:secret" entries; JWTs are shared-secret HS256 tokens
// validated against the configured issuer and audience.
type Auth struct {
	Enabled         bool   `mapstructure:"enabled"`
	JWTSecret       string `mapstructure:"jwt_secret"`
	JWTIssuer       string `mapstructure:"jwt_issuer"`
	JWTAudience     string `mapstructure:"jwt_audience"`
	JWTTTL          string `mapstructure:"jwt_ttl"`
	APIKeys         string `mapstructure:"api_keys"`
	WorkspaceTokens string `mapstructure:"workspace_tokens"`
}

// Dashboard controls whether the embedded React frontend is served.
type Dashboard struct {
	Enabled bool `mapstructure:"enabled"`
}

// Load reads configuration from .env, configs/config.yaml and environment
// variables and returns the resolved configuration.
func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")
	v.AutomaticEnv()
	v.SetEnvPrefix("MOCKSVR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "mocksvr")
	v.SetDefault("app.env", "development")
	v.SetDefault("server.host", defaultHost)
	v.SetDefault("server.port", defaultPort)
	v.SetDefault("cache.redis.addr", "")
	v.SetDefault("cache.redis.password", "")
	v.SetDefault("cache.redis.db", 0)
	v.SetDefault("cache.redis.ttl", "60s")
	v.SetDefault("ai.provider", "")
	v.SetDefault("ai.base_url", "")
	v.SetDefault("ai.api_key", "")
	v.SetDefault("ai.model", "")
	v.SetDefault("ai.timeout", "60s")
	v.SetDefault("auth.enabled", false)
	v.SetDefault("auth.jwt_secret", "")
	v.SetDefault("auth.jwt_issuer", "mocksvr")
	v.SetDefault("auth.jwt_audience", "mocksvr")
	v.SetDefault("auth.jwt_ttl", "1h")
	v.SetDefault("auth.api_keys", "")
	v.SetDefault("auth.workspace_tokens", "")
	v.SetDefault("dashboard.enabled", false)
	v.SetDefault("log.level", "info")
}

// loadDotEnv loads .env into the process environment when present.
// Environment variables explicitly set by the caller are never overridden.
func loadDotEnv() error {
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		return nil
	}
	return godotenv.Load()
}
