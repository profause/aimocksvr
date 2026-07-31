package importer

import (
	"context"

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
// an endpoint. Operations whose method+path already exist are skipped. The
// parsed document is validated before anything is written.
func (s *Service) Import(ctx context.Context, data []byte) (Result, error) {
	specs, err := Parse(data)
	if err != nil {
		return Result{}, err
	}

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

	res, err := s.endpoints.Import(ctx, items)
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
