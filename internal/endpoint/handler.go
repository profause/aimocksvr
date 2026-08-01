package endpoint

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/api"
)

// Handler exposes the endpoint registry over HTTP. It contains no business
// logic; all decisions are delegated to the Service.
type Handler struct {
	svc    Service
	logger *zerolog.Logger
}

// NewHandler creates an endpoint Handler.
func NewHandler(svc Service, logger *zerolog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Register wires the endpoint routes onto the given router group.
func (h *Handler) Register(r fiber.Router) {
	group := r.Group("/endpoints")

	group.Get("/", h.List)
	group.Post("/", h.Create)
	group.Get("/:id", h.Get)
	group.Put("/:id", h.Update)
	group.Delete("/:id", h.Delete)
	group.Get("/:id/versions", h.ListVersions)
	group.Get("/:id/versions/:version/diff", h.DiffVersion)
	group.Post("/:id/versions/:version/rollback", h.Rollback)
	group.Get("/:id/history", h.ListHistory)

	r.Get("/stats", h.Stats)
}

func (h *Handler) Create(c fiber.Ctx) error {
	var in CreateEndpointParams
	if err := c.Bind().JSON(&in); err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidJSON, "request body must be valid JSON")
	}

	e, err := h.svc.Create(c.Context(), in)
	if err != nil {
		return h.fail(c, err)
	}
	return api.Created(c, e)
}

func (h *Handler) Get(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidID, "invalid endpoint id")
	}

	e, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return h.fail(c, err)
	}
	return api.OK(c, e)
}

func (h *Handler) Update(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidID, "invalid endpoint id")
	}

	var in UpdateEndpointParams
	if err := c.Bind().JSON(&in); err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidJSON, "request body must be valid JSON")
	}

	e, err := h.svc.Update(c.Context(), id, in)
	if err != nil {
		return h.fail(c, err)
	}
	return api.OK(c, e)
}

func (h *Handler) Delete(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidID, "invalid endpoint id")
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return h.fail(c, err)
	}
	return api.OK(c, map[string]string{"id": id.String()})
}

func (h *Handler) List(c fiber.Ctx) error {
	p := ListParams{
		Page:  queryInt(c, "page"),
		Limit: queryInt(c, "limit"),
	}

	endpoints, total, err := h.svc.List(c.Context(), p)
	if err != nil {
		return h.fail(c, err)
	}

	return api.OK(c, map[string]any{
		"endpoints": endpoints,
		"page":      p.Page,
		"limit":     p.Limit,
		"total":     total,
	})
}

func (h *Handler) ListVersions(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidID, "invalid endpoint id")
	}

	versions, err := h.svc.ListVersions(c.Context(), id)
	if err != nil {
		return h.fail(c, err)
	}
	return api.OK(c, map[string]any{"versions": versions})
}

func (h *Handler) DiffVersion(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidID, "invalid endpoint id")
	}
	version, err := parseVersion(c.Params("version"))
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, "invalid version")
	}

	changes, err := h.svc.Diff(c.Context(), id, version)
	if err != nil {
		return h.fail(c, err)
	}
	return api.OK(c, map[string]any{"version": version, "changes": changes})
}

func (h *Handler) Rollback(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidID, "invalid endpoint id")
	}
	version, err := parseVersion(c.Params("version"))
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, "invalid version")
	}

	e, err := h.svc.Rollback(c.Context(), id, version)
	if err != nil {
		return h.fail(c, err)
	}
	return api.OK(c, e)
}

func (h *Handler) ListHistory(c fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeInvalidID, "invalid endpoint id")
	}

	history, err := h.svc.ListHistory(c.Context(), id)
	if err != nil {
		return h.fail(c, err)
	}
	return api.OK(c, map[string]any{"history": history})
}

func (h *Handler) Stats(c fiber.Ctx) error {
	stats, err := h.svc.Stats(c.Context())
	if err != nil {
		return h.fail(c, err)
	}
	return api.OK(c, stats)
}

// fail maps service errors to HTTP responses.
func (h *Handler) fail(c fiber.Ctx, err error) error {
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return api.Fail(c, fiber.StatusBadRequest, api.CodeValidationError, validationErr.Error())
	}
	if errors.Is(err, ErrNotFound) {
		return api.Fail(c, fiber.StatusNotFound, api.CodeNotFound, "endpoint not found")
	}
	if errors.Is(err, ErrConflict) {
		return api.Fail(c, fiber.StatusConflict, api.CodeConflict, "an endpoint with this method and path already exists")
	}

	h.logger.Error().Err(err).Str("path", c.Path()).Msg("internal server error")
	return api.Fail(c, fiber.StatusInternalServerError, api.CodeInternalError, "internal server error")
}

func parseID(raw string) (uuid.UUID, error) {
	return uuid.Parse(raw)
}

// parseVersion parses an integer version number from a route parameter.
func parseVersion(raw string) (int, error) {
	return strconv.Atoi(raw)
}

// queryInt parses an optional integer query parameter, returning 0 when it is
// absent or malformed.
func queryInt(c fiber.Ctx, key string) int {
	raw := c.Query(key)
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}
