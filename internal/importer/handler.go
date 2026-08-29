package importer

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/api"
	"github.com/profause/aimocksvr/internal/auth"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/models"
)

// Handler exposes the importers over HTTP.
type Handler struct {
	svc         *Service
	authEnabled bool
	logger      *zerolog.Logger
}

// NewHandler creates an importer Handler.
func NewHandler(svc *Service, cfg *config.Config, logger *zerolog.Logger) *Handler {
	return &Handler{svc: svc, authEnabled: cfg.Auth.Enabled, logger: logger}
}

// Register wires the import routes onto the given router group.
func (h *Handler) Register(r fiber.Router) {
	r.Post("/imports/openapi", h.ImportOpenAPI)
	r.Post("/imports/postman", h.ImportPostman)
}

// ImportOpenAPI accepts an OpenAPI document as the request body (JSON or
// YAML) or as a multipart file upload in the "file" field, parses it and
// registers every operation as a mock endpoint.
func (h *Handler) ImportOpenAPI(c fiber.Ctx) error {
	owner, err := h.writeOwner(c)
	if err != nil {
		return api.Fail(c, fiber.StatusUnauthorized, api.CodeUnauthorized, "an account is required")
	}

	data, err := readDocument(c)
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, err.Error())
	}

	result, err := h.svc.Import(c.Context(), owner, data)
	if err != nil {
		var perr *ParseError
		if errors.As(err, &perr) {
			return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, perr.Message)
		}
		h.logger.Error().Err(err).Msg("import failed")
		return api.Fail(c, fiber.StatusInternalServerError, api.CodeInternalError, "internal server error")
	}
	return api.Created(c, result)
}

// ImportPostman accepts a Postman Collection as the request body (JSON) or as
// a multipart file upload in the "file" field, parses it and registers every
// request as a mock endpoint.
func (h *Handler) ImportPostman(c fiber.Ctx) error {
	owner, err := h.writeOwner(c)
	if err != nil {
		return api.Fail(c, fiber.StatusUnauthorized, api.CodeUnauthorized, "an account is required")
	}

	data, err := readDocument(c)
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, err.Error())
	}

	result, err := h.svc.ImportPostman(c.Context(), owner, data)
	if err != nil {
		var perr *ParseError
		if errors.As(err, &perr) {
			return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, perr.Message)
		}
		h.logger.Error().Err(err).Msg("postman import failed")
		return api.Fail(c, fiber.StatusInternalServerError, api.CodeInternalError, "internal server error")
	}
	return api.Created(c, result)
}

// writeOwner resolves the account that owns an import. With auth disabled
// every import belongs to the legacy account, preserving the open mock. With
// auth enabled an import requires an authenticated account; a legacy
// API-key/workspace-token identity (AccountID nil) is refused.
func (h *Handler) writeOwner(c fiber.Ctx) (uuid.UUID, error) {
	if !h.authEnabled {
		return models.LegacyAccountID, nil
	}
	id, ok := c.Locals(auth.IdentityKey).(auth.Identity)
	if !ok || id.AccountID == uuid.Nil {
		return uuid.Nil, auth.ErrUnauthorized
	}
	return id.AccountID, nil
}

// readDocument extracts the import document bytes from the request, either as
// a multipart "file" field or the raw body.
func readDocument(c fiber.Ctx) ([]byte, error) {
	if ct := c.Get("Content-Type"); strings.HasPrefix(ct, "multipart/form-data") {
		fh, err := c.FormFile("file")
		if err != nil {
			return nil, errors.New("multipart field 'file' is required")
		}
		f, err := fh.Open()
		if err != nil {
			return nil, errors.New("failed to read uploaded file")
		}
		defer f.Close()
		return io.ReadAll(f)
	}

	data := c.Body()
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("an import document body is required")
	}
	return data, nil
}
