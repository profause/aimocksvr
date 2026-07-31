// Package generator produces the HTTP responses served by mock endpoints.
//
// Phase 3 ships a deterministic stub. Phase 4 introduces AI providers and
// Phase 7 the full generation pipeline; the Generator interface keeps the
// dynamic router independent of those implementations.
package generator

import (
	"context"
	"net/http"

	"github.com/profause/aimocksvr/internal/models"
)

// Request describes an inbound request to a mock endpoint.
type Request struct {
	Endpoint   *models.Endpoint
	PathParams map[string]string
	Query      map[string]string
	Headers    http.Header
	Body       []byte
}

// Response is the generated mock response.
type Response struct {
	Status int
	Body   []byte
}

// Generator produces responses for mock endpoints.
type Generator interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
}
