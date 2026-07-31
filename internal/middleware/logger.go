// Package middleware contains HTTP middleware shared across routes.
package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

// RequestLogger logs one line per HTTP request with method, path, status,
// duration and client IP.
func RequestLogger(logger *zerolog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		logger.Info().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", c.Response().StatusCode()).
			Dur("duration", time.Since(start)).
			Str("ip", c.IP()).
			Msg("http request")
		return err
	}
}
