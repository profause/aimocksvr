package ai

import (
	"context"
)

// Noop is an AIProvider that does nothing. It is returned when no provider is
// configured so callers never need nil checks.
type Noop struct{}

func (Noop) GenerateSchema(context.Context, SchemaRequest) (*Schema, error) {
	return nil, nil
}

func (Noop) GenerateResponse(context.Context, ResponseRequest) (*Response, error) {
	return nil, nil
}

func (Noop) GeneratePrompt(context.Context, PromptRequest) (string, error) {
	return "", nil
}
