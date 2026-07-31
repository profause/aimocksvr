package config

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// NewLogger builds a zerolog logger from the configuration. Pretty console
// output is used in development, structured JSON otherwise.
func NewLogger(cfg *Config) *zerolog.Logger {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Log.Level))
	if err != nil {
		level = zerolog.InfoLevel
	}

	var writer io.Writer = os.Stdout
	if cfg.App.Env == "development" {
		writer = zerolog.ConsoleWriter{Out: os.Stdout}
	}

	logger := zerolog.New(writer).
		Level(level).
		With().
		Timestamp().
		Str("service", cfg.App.Name).
		Str("env", cfg.App.Env).
		Logger()

	return &logger
}
