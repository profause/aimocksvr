package generator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/ai"
	"github.com/profause/aimocksvr/internal/validator"
)

// SchemaLoader resolves the JSON Schema that governs an endpoint's responses.
// The active schema is the one stored on the endpoint's latest version. The
// account owning the endpoint is passed so lookups stay scoped per account.
type SchemaLoader interface {
	LoadSchema(ctx context.Context, accountID, endpointID uuid.UUID) (string, error)
}

// errNoValidResponse reports that the AI produced no schema-conforming output
// within the allowed retries.
var errNoValidResponse = errors.New("no schema-valid response after retries")

const maxGenerationAttempts = 2

// aiGenerator produces responses through the AI provider and validates them
// against the endpoint's stored schema. It degrades to the deterministic
// fallback generator whenever AI is disabled, unreachable, or keeps producing
// output that fails validation, so mock requests keep working without a model
// backend.
type aiGenerator struct {
	provider ai.AIProvider
	schemas  SchemaLoader
	validate validator.Validator
	fallback Generator
	logger   *zerolog.Logger
}

// NewAI creates a Generator that runs the Phase 7 pipeline: load schema,
// generate, validate, and retry once on validation failure. When the AI
// provider is disabled, unreachable, or keeps failing validation, generation
// is handed to fallback (typically the schema-driven faker generator, with
// the deterministic stub as its own fallback).
func NewAI(provider ai.AIProvider, schemas SchemaLoader, v validator.Validator, fallback Generator, logger *zerolog.Logger) Generator {
	return &aiGenerator{
		provider: provider,
		schemas:  schemas,
		validate: v,
		fallback: fallback,
		logger:   logger,
	}
}

func (g *aiGenerator) Generate(ctx context.Context, req *Request) (*Response, error) {
	if req.Endpoint == nil {
		return g.fallback.Generate(ctx, req)
	}

	schema, err := g.schemas.LoadSchema(ctx, req.Endpoint.AccountID, req.Endpoint.ID)
	if err != nil {
		g.logger.Warn().Err(err).Str("endpoint_id", req.Endpoint.ID.String()).Msg("failed to load schema, using fallback generator")
		return g.fallback.Generate(ctx, req)
	}

	resp, err := g.generateWithRetry(ctx, req, schema)
	if err != nil {
		if errors.Is(err, errNoValidResponse) {
			g.logger.Warn().Str("endpoint_id", req.Endpoint.ID.String()).Msg("ai output failed schema validation, using fallback generator")
		} else {
			g.logger.Warn().Err(err).Str("endpoint_id", req.Endpoint.ID.String()).Msg("ai generation failed, using fallback generator")
		}
		return g.fallback.Generate(ctx, req)
	}

	return &Response{Status: resp.Status, Body: resp.Body}, nil
}

// generateWithRetry asks the provider for a response and validates it against
// the schema, retrying once when validation fails. Without a schema there is
// nothing to validate against, so the first response is returned as-is.
func (g *aiGenerator) generateWithRetry(ctx context.Context, req *Request, schema string) (*ai.Response, error) {
	// Phase 8: substitute request variables ({{path.id}}, {{body.email}}, …)
	// into the endpoint prompt before it reaches the provider.
	prompt := RenderVariables(req.Endpoint.Prompt, req)
	build := func() ai.ResponseRequest {
		return ai.ResponseRequest{
			Endpoint:   *req.Endpoint,
			Prompt:     prompt,
			Schema:     schema,
			PathParams: req.PathParams,
			Query:      req.Query,
			Headers:    req.Headers,
			Body:       req.Body,
		}
	}

	var lastErr error
	for attempt := 1; attempt <= maxGenerationAttempts; attempt++ {
		start := time.Now()
		resp, err := g.provider.GenerateResponse(ctx, build())
		g.logger.Debug().
			Int("attempt", attempt).
			Dur("latency", time.Since(start)).
			Str("endpoint_id", req.Endpoint.ID.String()).
			Msg("ai response generated")
		if err != nil {
			return nil, fmt.Errorf("generate response: %w", err)
		}
		if resp == nil {
			return nil, errors.New("ai provider returned no response")
		}
		if schema == "" {
			return resp, nil
		}
		if err := g.validate.ValidateResponse([]byte(schema), resp.Body); err == nil {
			return resp, nil
		} else {
			lastErr = err
			g.logger.Warn().
				Int("attempt", attempt).
				Err(err).
				Str("endpoint_id", req.Endpoint.ID.String()).
				Msg("generated response failed schema validation")
		}
	}
	return nil, fmt.Errorf("%w: %v", errNoValidResponse, lastErr)
}
