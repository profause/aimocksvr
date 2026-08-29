package auth

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/api"
	"github.com/profause/aimocksvr/internal/config"
)

// IdentityKey is the fiber Locals key under which the authenticated Identity is
// stored for downstream handlers.
const IdentityKey = "auth.identity"

// Handler wires authentication middleware and the token-minting routes.
type Handler struct {
	cfg    *config.Config
	svc    *Service
	logger *zerolog.Logger
}

// NewHandler creates an auth Handler.
func NewHandler(cfg *config.Config, svc *Service, logger *zerolog.Logger) *Handler {
	return &Handler{cfg: cfg, svc: svc, logger: logger}
}

// Middleware authenticates inbound requests. It never blocks when auth is
// disabled, keeps /health and the token-minting route public, returns 401 for
// the control plane without a valid identity, and stores the identity for
// downstream handlers — the dynamic router enforces mock endpoint privacy via
// each endpoint's public flag.
func (h *Handler) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !h.cfg.Auth.Enabled {
			return c.Next()
		}

		path := c.Path()
		if isPublicPath(path) {
			return c.Next()
		}

		if token := extractToken(c); token != "" {
			if id, err := h.svc.Authenticate(token); err == nil {
				c.Locals(IdentityKey, id)
				return c.Next()
			}
		}

		if strings.HasPrefix(path, "/api/v1") {
			return api.Fail(c, fiber.StatusUnauthorized, api.CodeUnauthorized, "authentication required")
		}
		return c.Next()
	}
}

// isPublicPath reports whether a control-plane route never requires
// credentials. The token-minting route, account registration and login are the
// public entry points of auth.
func isPublicPath(path string) bool {
	switch path {
	case "/health", "/api/v1/auth/token", "/api/v1/auth/register", "/api/v1/auth/login":
		return true
	}
	return false
}

// Register wires the auth routes onto the given router group.
func (h *Handler) Register(r fiber.Router) {
	group := r.Group("/auth")
	group.Post("/token", h.Token)
	group.Get("/whoami", h.Whoami)
}

// Token mints a short-lived JWT from a valid long-lived credential. It is the
// public entry point of auth: clients present an API key or workspace token
// and receive a JWT for subsequent calls.
func (h *Handler) Token(c fiber.Ctx) error {
	var in struct {
		APIKey         string `json:"api_key"`
		WorkspaceToken string `json:"workspace_token"`
	}
	if err := c.Bind().JSON(&in); err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidJSON, "request body must be valid JSON")
	}

	token := in.APIKey
	if token == "" {
		token = in.WorkspaceToken
	}
	if token == "" {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, "api_key or workspace_token is required")
	}

	id, err := h.svc.Authenticate(token)
	if err != nil {
		return api.Fail(c, fiber.StatusUnauthorized, api.CodeUnauthorized, "invalid credentials")
	}

	t, err := h.svc.MintJWT(id)
	if err != nil {
		h.logger.Error().Err(err).Msg("jwt minting failed")
		return api.Fail(c, fiber.StatusInternalServerError, api.CodeInternalError, "failed to mint token")
	}

	return api.OK(c, map[string]any{"token": t, "kind": id.Kind, "name": id.Name})
}

// Whoami reports the authenticated identity, or 401 when none is present. The
// response is built as an explicit map so a zero AccountID on legacy JWTs is
// omitted — uuid.UUID is a [16]byte array, so omitempty cannot hide it.
func (h *Handler) Whoami(c fiber.Ctx) error {
	id, ok := c.Locals(IdentityKey).(Identity)
	if !ok {
		return api.Fail(c, fiber.StatusUnauthorized, api.CodeUnauthorized, "authentication required")
	}
	out := map[string]any{"kind": id.Kind}
	if id.Name != "" {
		out["name"] = id.Name
	}
	if id.AccountID != uuid.Nil {
		out["account_id"] = id.AccountID
	}
	if id.Email != "" {
		out["email"] = id.Email
	}
	return api.OK(c, out)
}

// extractToken reads a credential from the standard auth headers, preferring
// the Authorization bearer header over the dedicated API key and workspace
// token headers.
func extractToken(c fiber.Ctx) string {
	authz := c.Get(fiber.HeaderAuthorization)
	if strings.HasPrefix(authz, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	}
	if key := c.Get("X-API-Key"); key != "" {
		return key
	}
	return c.Get("X-Workspace-Token")
}
