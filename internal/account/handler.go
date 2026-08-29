package account

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/api"
	"github.com/profause/aimocksvr/internal/models"
)

// Handler exposes account registration and login over HTTP. It contains no
// business logic; all decisions are delegated to the Service.
type Handler struct {
	svc    *Service
	logger *zerolog.Logger
}

// NewHandler creates an account Handler.
func NewHandler(svc *Service, logger *zerolog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Register wires the account routes onto the given router group.
func (h *Handler) Register(r fiber.Router) {
	group := r.Group("/auth")
	group.Post("/register", h.register)
	group.Post("/login", h.login)
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type accountDTO struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

func (h *Handler) register(c fiber.Ctx) error {
	var in credentials
	if err := c.Bind().JSON(&in); err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidJSON, "request body must be valid JSON")
	}

	acc, token, err := h.svc.Register(c.Context(), in.Email, in.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrValidation):
			return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, "validation failed")
		case errors.Is(err, ErrConflict):
			return api.Fail(c, fiber.StatusConflict, api.CodeConflict, "an account with this email already exists")
		default:
			h.logger.Error().Err(err).Msg("account registration failed")
			return api.Fail(c, fiber.StatusInternalServerError, api.CodeInternalError, "registration failed")
		}
	}
	return api.Created(c, authResult(acc, token))
}

func (h *Handler) login(c fiber.Ctx) error {
	var in credentials
	if err := c.Bind().JSON(&in); err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidJSON, "request body must be valid JSON")
	}
	if strings.TrimSpace(in.Email) == "" || in.Password == "" {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, "email and password are required")
	}

	acc, token, err := h.svc.Login(c.Context(), in.Email, in.Password)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return api.Fail(c, fiber.StatusUnauthorized, api.CodeUnauthorized, "invalid credentials")
		}
		h.logger.Error().Err(err).Msg("account login failed")
		return api.Fail(c, fiber.StatusInternalServerError, api.CodeInternalError, "login failed")
	}
	return api.OK(c, authResult(acc, token))
}

func authResult(a *models.Account, token string) map[string]any {
	return map[string]any{
		"account": accountDTO{ID: a.ID, Email: a.Email},
		"token":   token,
	}
}
