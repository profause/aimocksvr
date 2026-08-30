package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

// FieldChange describes a single versioned field that differs between two
// snapshots. From is the earlier snapshot's value and To the later one's.
type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// Service contains the business logic of the endpoint registry. Every method
// is scoped to the owning account: resources owned by another account are
// invisible (reads return ErrNotFound / empty), and Create/Import stamp the
// owner from the caller. owner must be a concrete account id — the caller
// resolves the legacy account when there is no authenticated one.
type Service interface {
	Create(ctx context.Context, owner uuid.UUID, in CreateEndpointParams) (*models.Endpoint, error)
	Update(ctx context.Context, owner, id uuid.UUID, in UpdateEndpointParams) (*models.Endpoint, error)
	Delete(ctx context.Context, owner, id uuid.UUID) error
	Get(ctx context.Context, owner, id uuid.UUID) (*models.Endpoint, error)
	List(ctx context.Context, owner uuid.UUID, p ListParams) ([]models.Endpoint, int, error)
	ListVersions(ctx context.Context, owner, endpointID uuid.UUID) ([]models.EndpointVersion, error)
	ListHistory(ctx context.Context, owner, endpointID uuid.UUID) ([]models.RequestHistory, error)
	Import(ctx context.Context, owner uuid.UUID, items []ImportItem) (ImportResult, error)
	Rollback(ctx context.Context, owner, id uuid.UUID, version int) (*models.Endpoint, error)
	Diff(ctx context.Context, owner, id uuid.UUID, version int) ([]FieldChange, error)
	Stats(ctx context.Context, owner uuid.UUID) (*DashboardStats, error)
}

// DashboardStats holds the aggregated metrics shown on the dashboard.
type DashboardStats struct {
	TotalEndpoints  int     `json:"total_endpoints"`
	ActiveRequests  int     `json:"active_requests"`
	AvgLatency      float64 `json:"avg_latency"`
	ErrorRate       float64 `json:"error_rate"`
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

func (s *service) Create(ctx context.Context, owner uuid.UUID, in CreateEndpointParams) (*models.Endpoint, error) {
	if err := s.validateCreate(in); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	e := &models.Endpoint{
		ID:            uuid.New(),
		AccountID:     owner,
		Method:        strings.ToUpper(in.Method),
		Path:          in.Path,
		Description:   in.Description,
		Prompt:        in.Prompt,
		ResponseType:  defaultString(in.ResponseType, models.ResponseTypeJSON),
		Stateful:      in.Stateful,
		Status:        defaultString(in.Status, models.StatusActive),
		RequestSchema: strings.TrimSpace(in.RequestSchema),
		ErrorSim:      strings.TrimSpace(in.ErrorSim),
		Public:        boolDefault(in.Public, false),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	schema := s.generateSchema(ctx, e)

	err := s.repo.WithTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Create(ctx, e); err != nil {
			return err
		}
		return s.repo.CreateVersion(ctx, snapshotVersion(e, schema, 1))
	})
	if err != nil {
		return nil, err
	}

	s.invalidateMethod(e.Method)
	return e, nil
}

func (s *service) Update(ctx context.Context, owner, id uuid.UUID, in UpdateEndpointParams) (*models.Endpoint, error) {
	if err := s.validateUpdate(in); err != nil {
		return nil, err
	}

	existing, err := s.repo.FindByID(ctx, owner, id)
	if err != nil {
		return nil, err
	}

	oldMethod := existing.Method
	oldPrompt := existing.Prompt

	before := *existing

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
	if in.ErrorSim != nil {
		existing.ErrorSim = strings.TrimSpace(*in.ErrorSim)
	}
	if in.Public != nil {
		existing.Public = *in.Public
	}

	// Every modification produces a new version. A PUT that changes nothing is
	// not a modification, so it is skipped to keep the history meaningful.
	if endpointSnapshotEqual(&before, existing) {
		return existing, nil
	}
	existing.UpdatedAt = time.Now().UTC()

	versions, err := s.repo.ListVersions(ctx, owner, id)
	if err != nil {
		return nil, err
	}

	// The response schema is regenerated only when the prompt changes, since
	// it is derived from the prompt; otherwise the previous schema carries over.
	var schema string
	if in.Prompt != oldPrompt {
		schema = s.generateSchema(ctx, existing)
	} else if len(versions) > 0 {
		schema = versions[0].Schema
	}

	err = s.repo.WithTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Update(ctx, owner, existing); err != nil {
			return err
		}
		return s.repo.CreateVersion(ctx, snapshotVersion(existing, schema, nextVersion(versions)))
	})
	if err != nil {
		return nil, err
	}

	s.invalidateMethod(oldMethod)
	s.invalidateMethod(existing.Method)
	return existing, nil
}

// Rollback restores an endpoint to the state captured by a historical version
// and records the rollback as a new version, so the rollback itself stays in
// the history and can be reverted. Rolling back to the current latest version
// is rejected as a no-op.
func (s *service) Rollback(ctx context.Context, owner, id uuid.UUID, version int) (*models.Endpoint, error) {
	existing, err := s.repo.FindByID(ctx, owner, id)
	if err != nil {
		return nil, err
	}

	versions, err := s.repo.ListVersions(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	target, err := findVersion(versions, version)
	if err != nil {
		return nil, err
	}
	if target.Version >= versions[0].Version {
		return nil, &ValidationError{Field: "version", Message: fmt.Sprintf("version %d is already the latest, nothing to roll back", version)}
	}

	oldMethod := existing.Method

	existing.Method = target.Method
	existing.Path = target.Path
	existing.Description = target.Description
	existing.Prompt = target.Prompt
	existing.ResponseType = defaultString(target.ResponseType, models.ResponseTypeJSON)
	existing.Stateful = target.Stateful
	existing.Status = defaultString(target.Status, models.StatusActive)
	existing.RequestSchema = target.RequestSchema
	existing.ErrorSim = target.ErrorSim
	existing.Public = target.Public
	existing.UpdatedAt = time.Now().UTC()

	err = s.repo.WithTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Update(ctx, owner, existing); err != nil {
			return err
		}
		return s.repo.CreateVersion(ctx, snapshotVersion(existing, target.Schema, nextVersion(versions)))
	})
	if err != nil {
		return nil, err
	}

	s.invalidateMethod(oldMethod)
	s.invalidateMethod(existing.Method)
	return existing, nil
}

// Diff compares the version snapshot with the endpoint's latest version and
// reports which versioned fields changed and how. An empty result means the
// two snapshots are identical.
func (s *service) Diff(ctx context.Context, owner, id uuid.UUID, version int) ([]FieldChange, error) {
	if _, err := s.repo.FindByID(ctx, owner, id); err != nil {
		return nil, err
	}

	versions, err := s.repo.ListVersions(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	target, err := findVersion(versions, version)
	if err != nil {
		return nil, err
	}
	return diffVersions(target, &versions[0]), nil
}

func (s *service) Delete(ctx context.Context, owner, id uuid.UUID) error {
	e, err := s.repo.FindByID(ctx, owner, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, owner, id); err != nil {
		return err
	}
	s.invalidateMethod(e.Method)
	return nil
}

func (s *service) Get(ctx context.Context, owner, id uuid.UUID) (*models.Endpoint, error) {
	return s.repo.FindByID(ctx, owner, id)
}

func (s *service) List(ctx context.Context, owner uuid.UUID, p ListParams) ([]models.Endpoint, int, error) {
	normalizePagination(&p)
	return s.repo.List(ctx, owner, p)
}

func (s *service) ListVersions(ctx context.Context, owner, endpointID uuid.UUID) ([]models.EndpointVersion, error) {
	if _, err := s.repo.FindByID(ctx, owner, endpointID); err != nil {
		return nil, err
	}
	return s.repo.ListVersions(ctx, owner, endpointID)
}

func (s *service) ListHistory(ctx context.Context, owner, endpointID uuid.UUID) ([]models.RequestHistory, error) {
	if _, err := s.repo.FindByID(ctx, owner, endpointID); err != nil {
		return nil, err
	}
	return s.repo.ListHistory(ctx, owner, endpointID)
}

func (s *service) Stats(ctx context.Context, owner uuid.UUID) (*DashboardStats, error) {
	since := time.Now().Add(-24 * time.Hour)

	total, err := s.repo.CountEndpoints(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("stats: count endpoints: %w", err)
	}

	requests, err := s.repo.CountRecentRequests(ctx, owner, since)
	if err != nil {
		return nil, fmt.Errorf("stats: count recent requests: %w", err)
	}

	latency, err := s.repo.AvgLatency(ctx, owner, since)
	if err != nil {
		return nil, fmt.Errorf("stats: avg latency: %w", err)
	}

	errorRate, err := s.repo.ErrorRate(ctx, owner, since)
	if err != nil {
		return nil, fmt.Errorf("stats: error rate: %w", err)
	}

	return &DashboardStats{
		TotalEndpoints: total,
		ActiveRequests: requests,
		AvgLatency:     latency,
		ErrorRate:      errorRate,
	}, nil
}

// Import creates endpoints in a single transaction from pre-built items.
// Endpoints whose method and path already exist are skipped and reported, so
// re-importing a spec is idempotent. Imported schemas are vetted before being
// stored; a schema that does not compile is dropped with a warning instead of
// failing the whole import, keeping the registry usable without a model
// backend.
func (s *service) Import(ctx context.Context, owner uuid.UUID, items []ImportItem) (ImportResult, error) {
	var result ImportResult
	now := time.Now().UTC()

	err := s.repo.WithTx(ctx, func(ctx context.Context) error {
		for _, item := range items {
			e := &models.Endpoint{
				ID:            uuid.New(),
				AccountID:     owner,
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

			if err := s.repo.CreateVersion(ctx, snapshotVersion(e, schema, 1)); err != nil {
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
	if err := validateErrorSim(in.ErrorSim); err != nil {
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
	if in.ErrorSim != nil {
		if err := validateErrorSim(*in.ErrorSim); err != nil {
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

// validateErrorSim rejects error simulation configs that are not valid JSON or
// fall outside the supported ranges. An empty config is always accepted.
func validateErrorSim(config string) error {
	text := strings.TrimSpace(config)
	if text == "" {
		return nil
	}
	sim, err := models.UnmarshalErrorSim(text)
	if err != nil {
		return &ValidationError{Field: "error_sim", Message: fmt.Sprintf("invalid json: %v", err)}
	}
	if sim.Status < 0 || sim.Status != 0 && (sim.Status < 400 || sim.Status > 599) {
		return &ValidationError{Field: "error_sim", Message: fmt.Sprintf("unsupported status %d (want 0 or 400-599)", sim.Status)}
	}
	if sim.FailureRate < 0 || sim.FailureRate > 100 {
		return &ValidationError{Field: "error_sim", Message: fmt.Sprintf("failure_rate must be between 0 and 100, got %d", sim.FailureRate)}
	}
	if sim.LatencyMs < 0 {
		return &ValidationError{Field: "error_sim", Message: "latency_ms cannot be negative"}
	}
	if sim.TimeoutMs < 0 {
		return &ValidationError{Field: "error_sim", Message: "timeout_ms cannot be negative"}
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

// boolDefault returns value when set and fallback otherwise.
func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
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

// snapshotVersion captures the full state of an endpoint at a point in time.
// The response schema is stored separately because it is derived from the
// prompt, not persisted on the endpoint itself.
func snapshotVersion(e *models.Endpoint, schema string, version int) *models.EndpointVersion {
	return &models.EndpointVersion{
		ID:            uuid.New(),
		EndpointID:    e.ID,
		AccountID:     e.AccountID,
		Method:        e.Method,
		Path:          e.Path,
		Description:   e.Description,
		Prompt:        e.Prompt,
		ResponseType:  e.ResponseType,
		Stateful:      e.Stateful,
		Status:        e.Status,
		RequestSchema: e.RequestSchema,
		ErrorSim:      e.ErrorSim,
		Public:        e.Public,
		Schema:        schema,
		Version:       version,
	}
}

// endpointSnapshotEqual reports whether two endpoints carry identical versioned
// state. The identity, timestamps and stored schema are excluded.
func endpointSnapshotEqual(a, b *models.Endpoint) bool {
	return a.Method == b.Method &&
		a.Path == b.Path &&
		a.Description == b.Description &&
		a.Prompt == b.Prompt &&
		a.ResponseType == b.ResponseType &&
		a.Stateful == b.Stateful &&
		a.Status == b.Status &&
		a.RequestSchema == b.RequestSchema &&
		a.ErrorSim == b.ErrorSim &&
		a.Public == b.Public
}

// findVersion returns the snapshot with the requested version number.
func findVersion(versions []models.EndpointVersion, version int) (*models.EndpointVersion, error) {
	for i := range versions {
		if versions[i].Version == version {
			return &versions[i], nil
		}
	}
	return nil, &ValidationError{Field: "version", Message: fmt.Sprintf("version %d not found", version)}
}

// diffVersions compares the earlier snapshot (from) with the later one (to)
// and lists every versioned field that differs.
func diffVersions(from, to *models.EndpointVersion) []FieldChange {
	fields := []struct {
		name string
		a, b string
	}{
		{"method", from.Method, to.Method},
		{"path", from.Path, to.Path},
		{"description", from.Description, to.Description},
		{"prompt", from.Prompt, to.Prompt},
		{"response_type", from.ResponseType, to.ResponseType},
		{"stateful", strconv.FormatBool(from.Stateful), strconv.FormatBool(to.Stateful)},
		{"status", from.Status, to.Status},
		{"request_schema", from.RequestSchema, to.RequestSchema},
		{"error_sim", from.ErrorSim, to.ErrorSim},
		{"public", strconv.FormatBool(from.Public), strconv.FormatBool(to.Public)},
		{"schema", from.Schema, to.Schema},
	}
	var changes []FieldChange
	for _, f := range fields {
		if f.a != f.b {
			changes = append(changes, FieldChange{Field: f.name, From: f.a, To: f.b})
		}
	}
	return changes
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
