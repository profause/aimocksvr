// Package router assembles the Fiber application and registers all routes.
package router

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/api"
	"github.com/profause/aimocksvr/internal/auth"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/endpoint"
	"github.com/profause/aimocksvr/internal/importer"
	"github.com/profause/aimocksvr/internal/middleware"
	"github.com/profause/aimocksvr/internal/web"
)

// New builds and configures the Fiber application with all routes registered.
func New(cfg *config.Config, logger *zerolog.Logger, h *endpoint.Handler, imp *importer.Handler, dyn *DynamicHandler, ah *auth.Handler) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ErrorHandler: errorHandler(logger),
	})

	app.Use(middleware.RequestLogger(logger))
	app.Use(ah.Middleware())

	apiGroup := app.Group("/api/v1")
	h.Register(apiGroup)
	imp.Register(apiGroup)
	ah.Register(apiGroup)

	registerHealth(app)
	registerDynamic(app, dyn)

	if cfg.Dashboard.Enabled {
		web.Register(app)
	}

	return app
}

// registerHealth exposes GET /health used by load balancers and orchestrators.
func registerHealth(app *fiber.App) {
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
}

// registerDynamic installs the mock endpoint resolver as a catch-all. It runs
// only when no control-plane route matched, so mock endpoints live at any path
// outside /api/v1 and /health.
func registerDynamic(app *fiber.App, dyn *DynamicHandler) {
	app.Use(dyn.Serve)
}

// errorHandler converts unhandled errors into the consistent error envelope.
func errorHandler(logger *zerolog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			code := api.CodeInternalError
			if fiberErr.Code >= 400 && fiberErr.Code < 500 {
				code = api.CodeValidationError
			}
			return api.Fail(c, fiberErr.Code, code, fiberErr.Message)
		}

		logger.Error().Err(err).Str("path", c.Path()).Msg("unhandled error")
		return api.Fail(c, fiber.StatusInternalServerError, api.CodeInternalError, "internal server error")
	}
}
