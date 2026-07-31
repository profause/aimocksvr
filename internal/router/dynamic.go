package router

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/api"
	"github.com/profause/aimocksvr/internal/auth"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/generator"
	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/validator"
)

// EndpointStore is the slice of the endpoint repository the dynamic router
// needs: resolve active endpoints and record served requests.
type EndpointStore interface {
	ListActiveByMethod(ctx context.Context, method string) ([]models.Endpoint, error)
	CreateHistory(ctx context.Context, h *models.RequestHistory) error
}

// DynamicHandler resolves inbound requests against the endpoint registry and
// serves a generated response. It is registered as a catch-all, so endpoints
// are served from the database with no restart and no static routes.
type DynamicHandler struct {
	store       EndpointStore
	generator   generator.Generator
	validate    validator.Validator
	authEnabled bool
	logger      *zerolog.Logger
}

// NewDynamicHandler creates a dynamic mock endpoint handler.
func NewDynamicHandler(store EndpointStore, gen generator.Generator, v validator.Validator, cfg *config.Config, logger *zerolog.Logger) *DynamicHandler {
	authEnabled := false
	if cfg != nil {
		authEnabled = cfg.Auth.Enabled
	}
	return &DynamicHandler{store: store, generator: gen, validate: v, authEnabled: authEnabled, logger: logger}
}

// Serve resolves the current request to an endpoint and writes the generated
// response. It returns a 404 envelope when no endpoint matches.
func (h *DynamicHandler) Serve(c fiber.Ctx) error {
	ctx := c.Context()
	method := c.Method()
	path := c.Path()

	endpoints, err := h.store.ListActiveByMethod(ctx, method)
	if err != nil {
		h.logger.Error().Err(err).Str("method", method).Msg("failed to resolve mock endpoints")
		return api.Fail(c, fiber.StatusInternalServerError, api.CodeInternalError, "internal server error")
	}

	e, params, ok := bestMatch(endpoints, path)
	if !ok {
		return api.Fail(c, fiber.StatusNotFound, api.CodeNotFound,
			fmt.Sprintf("no mock endpoint matches %s %s", method, path))
	}

	// When auth is enabled, non-public mock endpoints require a valid
	// identity, which the auth middleware stores on the context.
	if h.authEnabled && !e.Public && !hasIdentity(c) {
		return api.Fail(c, fiber.StatusUnauthorized, api.CodeUnauthorized,
			fmt.Sprintf("endpoint %s %s is private and requires authentication", e.Method, e.Path))
	}

	// Endpoints may declare a request schema; a non-conforming body is
	// rejected before any generation, so every generation path (AI, faker,
	// stateful) enforces the request contract uniformly.
	if err := validateRequestBody(h.validate, e, c.Body()); err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, err.Error())
	}

	// Endpoints may declare error simulation. Latency always applies; the
	// configured failure is rolled against failure_rate and short-circuits
	// generation when it triggers.
	sim, err := models.UnmarshalErrorSim(e.ErrorSim)
	if err != nil {
		h.logger.Warn().Err(err).Str("endpoint_id", e.ID.String()).Msg("invalid error_sim config, ignoring")
	}
	if sim != nil {
		if sim.LatencyMs > 0 {
			time.Sleep(time.Duration(sim.LatencyMs) * time.Millisecond)
		}
		if sim.ShouldFail(rand.Intn(100)) {
			return applyFailure(c, sim)
		}
	}

	req := &generator.Request{
		Endpoint:   e,
		PathParams: nonNilParams(params),
		Query:      c.Queries(),
		Headers:    headersMap(c.GetReqHeaders()),
		Body:       c.Body(),
	}

	start := time.Now()
	resp, err := h.generator.Generate(ctx, req)
	if err != nil {
		h.logger.Error().Err(err).Str("endpoint_id", e.ID.String()).Msg("mock response generation failed")
		return api.Fail(c, fiber.StatusInternalServerError, api.CodeInternalError, "internal server error")
	}

	h.recordHistory(ctx, e, c.Body(), resp.Body, time.Since(start))

	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	c.Status(resp.Status)
	return c.Send(resp.Body)
}

// validateRequestBody checks the request body against the endpoint's request
// schema. An empty body is rejected with a clear message, then the document is
// validated. A missing request schema passes everything through.
func validateRequestBody(v validator.Validator, e *models.Endpoint, body []byte) error {
	if strings.TrimSpace(e.RequestSchema) == "" {
		return nil
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return errors.New("request body is required")
	}
	if err := v.ValidateRequest([]byte(e.RequestSchema), body); err != nil {
		return fmt.Errorf("request body does not match the endpoint request schema: %w", err)
	}
	return nil
}

// applyFailure performs the endpoint's simulated failure. Precedence: timeout
// (sleep, then drop the connection), dropped connection, malformed JSON, and
// finally the configured status.
func applyFailure(c fiber.Ctx, sim *models.ErrorSimulation) error {
	switch {
	case sim.TimeoutMs > 0:
		time.Sleep(time.Duration(sim.TimeoutMs) * time.Millisecond)
		return dropConnection(c)
	case sim.DropConnection:
		return dropConnection(c)
	case sim.MalformedJSON:
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		c.Status(fiber.StatusOK)
		return c.Send([]byte(`{"error": "simulated malformed response"`))
	default:
		return api.Fail(c, sim.Status, api.CodeErrorSimulation, "simulated endpoint failure")
	}
}

// dropConnection aborts the response and closes the connection without sending
// a reply, so the client observes an empty/truncated response.
func dropConnection(c fiber.Ctx) error {
	ctx := c.RequestCtx()
	ctx.HijackSetNoResponse(true)
	ctx.Hijack(func(conn net.Conn) { _ = conn.Close() })
	return nil
}

// recordHistory persists a served request. Failures are logged but never fail
// the mock request itself.
func (h *DynamicHandler) recordHistory(ctx context.Context, e *models.Endpoint, reqBody, respBody []byte, latency time.Duration) {
	history := &models.RequestHistory{
		ID:         uuid.New(),
		EndpointID: e.ID,
		Request:    string(reqBody),
		Response:   string(respBody),
		Latency:    latency.Milliseconds(),
	}
	if err := h.store.CreateHistory(ctx, history); err != nil {
		h.logger.Warn().Err(err).Str("endpoint_id", e.ID.String()).Msg("failed to record request history")
	}
}

func nonNilParams(params map[string]string) map[string]string {
	if params == nil {
		return map[string]string{}
	}
	return params
}

// hasIdentity reports whether the auth middleware stored an identity for the
// current request.
func hasIdentity(c fiber.Ctx) bool {
	id, ok := c.Locals(auth.IdentityKey).(auth.Identity)
	return ok && id.Kind != "" && id.Name != ""
}

func headersMap(h map[string][]string) http.Header {
	if h == nil {
		return http.Header{}
	}
	return http.Header(h)
}
