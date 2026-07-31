package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/ai"
	"github.com/profause/aimocksvr/internal/cache"
	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/validator"
)

// ValidationError describes a rejected request input.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

var validMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodOptions: true,
	http.MethodHead:    true,
}

// ImportItem describes an endpoint to create during an import. The schemas
// are stored on the first version as-is; no AI inference is run for imports.
type ImportItem struct {
	Method        string
	Path          string
	Description   string
	Prompt        string
	Schema        string
	RequestSchema string
}

// ImportResult reports how many endpoints were created or skipped.
type ImportResult struct {
	Created   int               `json:"created"`
	Skipped   int               `json:"skipped"`
	Endpoints []models.Endpoint `json:"endpoints"`
}

// Service contains the business logic of the endpoint registry.
type Service interface {
	Create(ctx context.Context, in CreateEndpointParams) (*models.Endpoint, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateEndpointParams) (*models.Endpoint, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*models.Endpoint, error)
	List(ctx context.Context, p ListParams) ([]models.Endpoint, int, error)
	ListVersions(ctx context.Context, endpointID uuid.UUID) ([]models.EndpointVersion, error)
	ListHistory(ctx context.Context, endpointID uuid.UUID) ([]models.RequestHistory, error)
	Import(ctx context.Context, items []ImportItem) (ImportResult, error)
}

type service struct {
	repo     Repository
	cache    cache.Cache
	ai       ai.AIProvider
	validate validator.Validator
	log      *zerolog.Logger
}

// NewService creates an endpoint Service backed by the given Repository.
// The cache is invalidated on mutations so the dynamic router always sees
// fresh endpoints; a Noop cache is used when caching is disabled. The AI
// provider generates a JSON Schema for each version and the validator vets it
// before it is stored; both degrade gracefully when AI is unavailable.
func NewService(repo Repository, c cache.Cache, a ai.AIProvider, v validator.Validator, logger *zerolog.Logger) Service {
	return &service{repo: repo, cache: c, ai: a, validate: v, log: logger}
}

func (s *service) Create(ctx context.Context, in CreateEndpointParams) (*models.Endpoint, error) {
	if err := s.validateCreate(in); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	e := &models.Endpoint{
		ID:            uuid.New(),
		Method:        strings.ToUpper(in.Method),
		Path:          in.Path,
		Description:   in.Description,
		Prompt:        in.Prompt,
		ResponseType:  defaultString(in.ResponseType, models.ResponseTypeJSON),
		Stateful:      in.Stateful,
		Status:        defaultString(in.Status, models.StatusActive),
		RequestSchema: strings.TrimSpace(in.RequestSchema),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	schema := s.generateSchema(ctx, e)

	err := s.repo.WithTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Create(ctx, e); err != nil {
			return err
		}
		v := &models.EndpointVersion{
			ID:         uuid.New(),
			EndpointID: e.ID,
			Prompt:     e.Prompt,
			Schema:     schema,
			Version:    1,
		}
		return s.repo.CreateVersion(ctx, v)
	})
	if err != nil {
		return nil, err
	}

	s.invalidateMethod(e.Method)
	return e, nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, in UpdateEndpointParams) (*models.Endpoint, error) {
	if err := s.validateUpdate(in); err != nil {
		return nil, err
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	oldPrompt := existing.Prompt

	oldMethod := existing.Method

	existing.Method = strings.ToUpper(in.Method)
	existing.Path = in.Path
	existing.Description = in.Description
	existing.Prompt = in.Prompt
	existing.ResponseType = defaultString(in.ResponseType, models.ResponseTypeJSON)
	if in.Stateful != nil {
		existing.Stateful = *in.Stateful
	}
	existing.Status = defaultString(in.Status, models.StatusActive)
	if in.RequestSchema != nil {
		existing.RequestSchema = strings.TrimSpace(*in.RequestSchema)
	}
	existing.UpdatedAt = time.Now().UTC()

	createdVersion := in.Prompt != oldPrompt

	var schema string
	if createdVersion {
		schema = s.generateSchema(ctx, existing)
	}

	err = s.repo.WithTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Update(ctx, existing); err != nil {
			return err
		}
		if createdVersion {
			versions, err := s.repo.ListVersions(ctx, id)
			if err != nil {
				return err
			}
			v := &models.EndpointVersion{
				ID:         uuid.New(),
				EndpointID: id,
				Prompt:     in.Prompt,
				Schema:     schema,
				Version:    nextVersion(versions),
			}
			return s.repo.CreateVersion(ctx, v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.invalidateMethod(oldMethod)
	s.invalidateMethod(existing.Method)
	return existing, nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	e, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateMethod(e.Method)
	return nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*models.Endpoint, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) List(ctx context.Context, p ListParams) ([]models.Endpoint, int, error) {
	normalizePagination(&p)
	return s.repo.List(ctx, p)
}

func (s *service) ListVersions(ctx context.Context, endpointID uuid.UUID) ([]models.EndpointVersion, error) {
	if _, err := s.repo.FindByID(ctx, endpointID); err != nil {
		return nil, err
	}
	return s.repo.ListVersions(ctx, endpointID)
}

func (s *service) ListHistory(ctx context.Context, endpointID uuid.UUID) ([]models.RequestHistory, error) {
	if _, err := s.repo.FindByID(ctx, endpointID); err != nil {
		return nil, err
	}
	return s.repo.ListHistory(ctx, endpointID)
}

// Import creates endpoints in a single transaction from pre-built items.
// Endpoints whose method and path already exist are skipped and reported, so
// re-importing a spec is idempotent. Imported schemas are vetted before being
// stored; a schema that does not compile is dropped with a warning instead of
// failing the whole import, keeping the registry usable without a model
// backend.
func (s *service) Import(ctx context.Context, items []ImportItem) (ImportResult, error) {
	var result ImportResult
	now := time.Now().UTC()

	err := s.repo.WithTx(ctx, func(ctx context.Context) error {
		for _, item := range items {
			e := &models.Endpoint{
				ID:            uuid.New(),
				Method:        strings.ToUpper(item.Method),
				Path:          item.Path,
				Description:   item.Description,
				Prompt:        item.Prompt,
				ResponseType:  models.ResponseTypeJSON,
				Status:        models.StatusActive,
				RequestSchema: item.RequestSchema,
				CreatedAt:     now,
				UpdatedAt:     now,
			}

			schema := ""
			if strings.TrimSpace(item.Schema) != "" {
				if err := s.validate.ValidateSchema([]byte(item.Schema)); err != nil {
					s.log.Warn().Err(err).Str("method", e.Method).Str("path", e.Path).Msg("imported schema failed validation, storing without schema")
				} else {
					schema = item.Schema
				}
			}

			if strings.TrimSpace(e.RequestSchema) != "" {
				if err := s.validate.ValidateSchema([]byte(e.RequestSchema)); err != nil {
					s.log.Warn().Err(err).Str("method", e.Method).Str("path", e.Path).Msg("imported request schema failed validation, storing without request schema")
					e.RequestSchema = ""
				}
			}

			if err := s.repo.Create(ctx, e); err != nil {
				if errors.Is(err, ErrConflict) {
					result.Skipped++
					continue
				}
				return err
			}

			v := &models.EndpointVersion{
				ID:         uuid.New(),
				EndpointID: e.ID,
				Prompt:     e.Prompt,
				Schema:     schema,
				Version:    1,
			}
			if err := s.repo.CreateVersion(ctx, v); err != nil {
				return err
			}
			result.Created++
			result.Endpoints = append(result.Endpoints, *e)
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	methods := make(map[string]struct{}, len(result.Endpoints))
	for _, e := range result.Endpoints {
		methods[e.Method] = struct{}{}
	}
	for m := range methods {
		s.invalidateMethod(m)
	}
	return result, nil
}

func (s *service) validateCreate(in CreateEndpointParams) error {
	if err := validateCommon(in.Method, in.Path, in.Prompt); err != nil {
		return err
	}
	if err := s.validateRequestSchema(in.RequestSchema); err != nil {
		return err
	}
	return validateOptionalFields(in.ResponseType, in.Status)
}

func (s *service) validateUpdate(in UpdateEndpointParams) error {
	if err := validateCommon(in.Method, in.Path, in.Prompt); err != nil {
		return err
	}
	if in.RequestSchema != nil {
		if err := s.validateRequestSchema(*in.RequestSchema); err != nil {
			return err
		}
	}
	return validateOptionalFields(in.ResponseType, in.Status)
}

// validateRequestSchema rejects schemas that do not compile as a JSON Schema.
// Unlike response schemas (which degrade gracefully), an invalid request
// schema is a client contract error and is rejected upfront.
func (s *service) validateRequestSchema(schema string) error {
	if strings.TrimSpace(schema) == "" {
		return nil
	}
	if err := s.validate.ValidateSchema([]byte(schema)); err != nil {
		return &ValidationError{Field: "request_schema", Message: fmt.Sprintf("invalid json schema: %v", err)}
	}
	return nil
}

func validateCommon(method, path, prompt string) error {
	if strings.TrimSpace(method) == "" {
		return &ValidationError{Field: "method", Message: "method is required"}
	}
	if !validMethods[strings.ToUpper(method)] {
		return &ValidationError{Field: "method", Message: fmt.Sprintf("unsupported method %q", method)}
	}
	if strings.TrimSpace(path) == "" {
		return &ValidationError{Field: "path", Message: "path is required"}
	}
	if !strings.HasPrefix(path, "/") {
		return &ValidationError{Field: "path", Message: "path must start with /"}
	}
	if strings.TrimSpace(prompt) == "" {
		return &ValidationError{Field: "prompt", Message: "prompt is required"}
	}
	return nil
}

func validateOptionalFields(responseType, status string) error {
	if responseType != "" && responseType != models.ResponseTypeJSON {
		return &ValidationError{Field: "response_type", Message: fmt.Sprintf("unsupported response_type %q", responseType)}
	}
	if status != "" && status != models.StatusActive && status != models.StatusInactive {
		return &ValidationError{Field: "status", Message: fmt.Sprintf("unsupported status %q", status)}
	}
	return nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// generateSchema asks the AI provider to infer a JSON Schema for the
// endpoint's response shape and returns it as JSON text. It returns an empty
// string when AI is disabled, unreachable, or produced something that does not
// compile as a JSON Schema; failures are logged instead of failing the
// request, keeping the registry usable without a model backend.
func (s *service) generateSchema(ctx context.Context, e *models.Endpoint) string {
	if strings.TrimSpace(e.Prompt) == "" {
		return ""
	}

	schema, err := s.ai.GenerateSchema(ctx, ai.SchemaRequest{Endpoint: *e})
	if err != nil {
		s.log.Warn().Err(err).Str("endpoint_id", e.ID.String()).Msg("schema generation failed")
		return ""
	}
	if schema == nil {
		return ""
	}

	data, err := json.Marshal(schema)
	if err != nil {
		s.log.Warn().Err(err).Str("endpoint_id", e.ID.String()).Msg("schema marshal failed")
		return ""
	}
	if err := s.validate.ValidateSchema(data); err != nil {
		s.log.Warn().Err(err).Str("endpoint_id", e.ID.String()).Msg("generated schema failed validation")
		return ""
	}
	return string(data)
}

// invalidateMethod drops the cached endpoint list for a method. Failures are
// logged; the TTL acts as a fallback, so stale data self-heals.
func (s *service) invalidateMethod(method string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.cache.Delete(ctx, cache.EndpointListKey(method)); err != nil {
		s.log.Warn().Err(err).Str("method", method).Msg("failed to invalidate endpoint cache")
	}
}

func nextVersion(versions []models.EndpointVersion) int {
	highest := 0
	for _, v := range versions {
		if v.Version > highest {
			highest = v.Version
		}
	}
	return highest + 1
}

func normalizePagination(p *ListParams) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = 20
	}
	if p.Limit > 100 {
		p.Limit = 100
	}
}
