package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/profause/aimocksvr/internal/account"
	"github.com/profause/aimocksvr/internal/ai"
	"github.com/profause/aimocksvr/internal/auth"
	"github.com/profause/aimocksvr/internal/cache"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/database"
	"github.com/profause/aimocksvr/internal/endpoint"
	"github.com/profause/aimocksvr/internal/generator"
	"github.com/profause/aimocksvr/internal/importer"
	"github.com/profause/aimocksvr/internal/router"
	"github.com/profause/aimocksvr/internal/state"
	"github.com/profause/aimocksvr/internal/validator"
)

func main() {
	app := fx.New(
		fx.Provide(
			config.Load,
			config.NewLogger,
			database.Connect,
			endpoint.NewRepository,
			endpoint.NewService,
			endpoint.NewHandler,
			importer.NewService,
			importer.NewHandler,
			auth.NewService,
			auth.NewHandler,
			account.NewRepository,
			account.NewService,
			account.NewHandler,
			provideGenerator,
			endpointSchemaLoader,
			cache.New,
			ai.New,
			state.NewStore,
			validator.New,
			cachedEndpointStore,
			router.NewDynamicHandler,
			router.New,
		),
		fx.Invoke(runServer),
	)

	app.Run()
}

// provideGenerator composes the stateful wrapper around the AI generation
// pipeline. The AI generator validates responses against the endpoint schema;
// when AI is unavailable, the schema-driven faker generator fills the schema
// with realistic fake data, and the deterministic stub is the final fallback.
func provideGenerator(provider ai.AIProvider, schemas generator.SchemaLoader, v validator.Validator, store state.Store, logger *zerolog.Logger) generator.Generator {
	static := generator.NewStatic()
	faker := generator.NewFaker(schemas, static, logger)
	aiGen := generator.NewAI(provider, schemas, v, faker, logger)
	return generator.NewStateful(store, aiGen, logger)
}

// endpointSchemaLoader adapts the endpoint repository to the narrow SchemaLoader
// interface the AI generator needs.
func endpointSchemaLoader(repo endpoint.Repository) generator.SchemaLoader {
	return schemaLoader{repo: repo}
}

type schemaLoader struct {
	repo endpoint.Repository
}

func (s schemaLoader) LoadSchema(ctx context.Context, accountID, endpointID uuid.UUID) (string, error) {
	versions, err := s.repo.ListVersions(ctx, accountID, endpointID)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", nil
	}
	return versions[0].Schema, nil
}

// cachedEndpointStore wraps the endpoint repository with a read-through cache,
// resolving the cache TTL from configuration.
func cachedEndpointStore(repo endpoint.Repository, c cache.Cache, cfg *config.Config, logger *zerolog.Logger) (router.EndpointStore, error) {
	ttl := 60 * time.Second
	if cfg.Cache.Redis.TTL != "" {
		parsed, err := time.ParseDuration(cfg.Cache.Redis.TTL)
		if err != nil {
			return nil, fmt.Errorf("parse cache ttl %q: %w", cfg.Cache.Redis.TTL, err)
		}
		ttl = parsed
	}
	return router.NewCachedEndpointStore(repo, c, ttl, logger), nil
}

type runServerParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Logger    *zerolog.Logger
	DB        *bun.DB
	App       *fiber.App
}

// runServer wires the HTTP server lifecycle: migrations first, then the
// listener, with a clean shutdown on stop.
func runServer(p runServerParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			p.Logger.Info().Msg("applying database migrations")
			if err := database.Migrate(p.Config.Database.URL); err != nil {
				return fmt.Errorf("apply migrations: %w", err)
			}

			addr := net.JoinHostPort(p.Config.Server.Host, strconv.Itoa(p.Config.Server.Port))
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("bind %s: %w", addr, err)
			}

			p.Logger.Info().Str("addr", addr).Msg("http server listening")
			go func() {
				if err := p.App.Listener(ln); err != nil {
					p.Logger.Error().Err(err).Msg("http server error")
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			p.Logger.Info().Msg("shutting down http server")
			if err := p.App.Shutdown(); err != nil {
				p.Logger.Error().Err(err).Msg("http server shutdown failed")
			}
			if err := p.DB.Close(); err != nil {
				p.Logger.Error().Err(err).Msg("database close failed")
			}
			p.Logger.Info().Msg("shutdown complete")
			return nil
		},
	})
}
