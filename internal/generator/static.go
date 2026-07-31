package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Static is the deterministic response generator used until AI providers
// arrive in Phase 4. It echoes the request context back as JSON so clients can
// exercise the dynamic routing pipeline end to end.
type Static struct{}

// NewStatic creates the deterministic static generator.
func NewStatic() Generator {
	return &Static{}
}

func (g *Static) Generate(_ context.Context, req *Request) (*Response, error) {
	if req.Endpoint == nil {
		return nil, fmt.Errorf("generator: request has no endpoint")
	}

	payload := map[string]any{
		"endpoint": req.Endpoint.Path,
		"method":   req.Endpoint.Method,
		"params":   nonNilMap(req.PathParams),
		"query":    nonNilMap(req.Query),
		"message":  "mock response generated for endpoint " + req.Endpoint.Path,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mock response: %w", err)
	}

	return &Response{Status: http.StatusOK, Body: body}, nil
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
