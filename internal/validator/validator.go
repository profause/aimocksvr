// Package validator checks JSON documents against JSON Schemas.
//
// Schemas are compiled once and cached by content, which keeps per-response
// validation cheap. The mock server stores a JSON Schema on every endpoint
// version and uses this package both to vet generated schemas before storing
// them and to validate every generated response against them.
package validator

import (
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator validates JSON documents against JSON Schemas.
type Validator interface {
	// ValidateSchema reports whether schemaJSON is a compilable JSON Schema.
	ValidateSchema(schemaJSON []byte) error
	// ValidateResponse validates responseJSON against schemaJSON.
	ValidateResponse(schemaJSON, responseJSON []byte) error
	// ValidateRequest validates requestJSON against schemaJSON.
	ValidateRequest(schemaJSON, requestJSON []byte) error
}

type validator struct {
	mu      sync.Mutex
	schemas map[string]*jsonschema.Schema
}

// New creates a Validator backed by compiled JSON Schemas.
func New() Validator {
	return &validator{schemas: map[string]*jsonschema.Schema{}}
}

// compile returns the compiled schema for schemaJSON, compiling and caching it
// on first use.
func (v *validator) compile(schemaJSON []byte) (*jsonschema.Schema, error) {
	key := string(schemaJSON)

	v.mu.Lock()
	defer v.mu.Unlock()
	if s, ok := v.schemas[key]; ok {
		return s, nil
	}

	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(schemaJSON)))
	if err != nil {
		return nil, fmt.Errorf("decode json schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	// Assert schema formats (email, uuid, date-time, …) so request and
	// response validation enforce the declared format constraints.
	compiler.AssertFormat()
	if err := compiler.AddResource("schema.json", doc); err != nil {
		return nil, fmt.Errorf("add json schema: %w", err)
	}
	s, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile json schema: %w", err)
	}
	v.schemas[key] = s
	return s, nil
}

func (v *validator) ValidateSchema(schemaJSON []byte) error {
	_, err := v.compile(schemaJSON)
	return err
}

func (v *validator) ValidateResponse(schemaJSON, responseJSON []byte) error {
	return v.validate(schemaJSON, responseJSON, "response")
}

func (v *validator) ValidateRequest(schemaJSON, requestJSON []byte) error {
	return v.validate(schemaJSON, requestJSON, "request")
}

// validate decodes a JSON document and checks it against the compiled schema.
func (v *validator) validate(schemaJSON, documentJSON []byte, kind string) error {
	s, err := v.compile(schemaJSON)
	if err != nil {
		return err
	}

	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(documentJSON)))
	if err != nil {
		return fmt.Errorf("decode %s: %w", kind, err)
	}
	if err := s.Validate(doc); err != nil {
		return fmt.Errorf("validate %s: %s", kind, conciseError(err))
	}
	return nil
}

// conciseError summarizes a jsonschema validation error into a single line,
// dropping the library's "jsonschema validation failed with '<resource>#'"
// preamble and preserving the per-path causes.
func conciseError(err error) string {
	lines := strings.Split(err.Error(), "\n")
	for len(lines) > 0 && strings.HasPrefix(lines[0], "jsonschema validation failed") {
		lines = lines[1:]
	}
	return strings.Join(strings.Fields(strings.Join(lines, " ")), " ")
}
