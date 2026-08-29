package importer

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/endpoint"
	"github.com/profause/aimocksvr/internal/models"
)

// Service imports OpenAPI documents into the endpoint registry.
type Service struct {
	endpoints endpoint.Service
	log       *zerolog.Logger
}

// NewService creates an importer Service backed by the endpoint registry.
func NewService(endpoints endpoint.Service, logger *zerolog.Logger) *Service {
	return &Service{endpoints: endpoints, log: logger}
}

// Result describes a completed import.
type Result struct {
	Parsed    int               `json:"parsed"`
	Created   int               `json:"created"`
	Skipped   int               `json:"skipped"`
	Endpoints []models.Endpoint `json:"endpoints"`
}

// Import parses data as an OpenAPI document and registers every operation as
// an endpoint owned by owner. Operations whose method+path already exist for
// that owner are skipped. The parsed document is validated before anything is
// written.
func (s *Service) Import(ctx context.Context, owner uuid.UUID, data []byte) (Result, error) {
	specs, err := Parse(data)
	if err != nil {
		return Result{}, err
	}
	return s.importSpecs(ctx, owner, specs)
}

// ImportPostman parses data as a Postman Collection and registers every
// request as an endpoint owned by owner, reusing the same idempotent write
// path as Import.
func (s *Service) ImportPostman(ctx context.Context, owner uuid.UUID, data []byte) (Result, error) {
	specs, err := ParsePostman(data)
	if err != nil {
		return Result{}, err
	}
	return s.importSpecs(ctx, owner, specs)
}

// importSpecs converts parsed endpoints into import items, writes them to the
// registry and reports the outcome.
func (s *Service) importSpecs(ctx context.Context, owner uuid.UUID, specs []EndpointSpec) (Result, error) {
	items := make([]endpoint.ImportItem, 0, len(specs))
	for _, spec := range specs {
		items = append(items, endpoint.ImportItem{
			Method:        spec.Method,
			Path:          spec.Path,
			Description:   spec.Description,
			Prompt:        spec.Prompt,
			Schema:        spec.Schema,
			RequestSchema: spec.RequestSchema,
		})
	}

	res, err := s.endpoints.Import(ctx, owner, items)
	if err != nil {
		s.log.Error().Err(err).Msg("import failed")
		return Result{}, err
	}

	return Result{
		Parsed:    len(specs),
		Created:   res.Created,
		Skipped:   res.Skipped,
		Endpoints: res.Endpoints,
	}, nil
}
