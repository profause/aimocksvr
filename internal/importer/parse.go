package importer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// EndpointSpec is a parsed operation ready to be stored in the registry.
type EndpointSpec struct {
	Method        string
	Path          string
	Description   string
	Prompt        string
	Schema        string
	RequestSchema string
}

// ParseError marks input that is not a valid OpenAPI document.
type ParseError struct {
	Message string
}

func (e *ParseError) Error() string { return e.Message }

// maxRefDepth caps $ref resolution so cyclic schemas cannot loop forever.
const maxRefDepth = 10

var pathParamRe = regexp.MustCompile(`\{([^{}]+)\}`)

// Parse decodes an OpenAPI 2.0 or 3.x document (JSON or YAML) and flattens
// its operations into EndpointSpecs. OpenAPI path templates ({id}) become
// router parameters (:id), prompts come from the operation summary and
// description, and the JSON Schema of the first 2xx response is resolved
// against the document's components. Empty or unversioned input is rejected.
func Parse(data []byte) ([]EndpointSpec, error) {
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("invalid OpenAPI document: %v", err)}
	}
	if spec.OpenAPI == "" && spec.Swagger == "" {
		return nil, &ParseError{Message: "document is not an OpenAPI or Swagger spec"}
	}
	if len(spec.Paths) == 0 {
		return nil, &ParseError{Message: "document contains no paths"}
	}

	schemas := make(map[string]any, len(spec.Definitions))
	for name, s := range spec.Definitions {
		schemas[name] = s
	}
	if spec.Components != nil {
		for name, s := range spec.Components.Schemas {
			schemas[name] = s
		}
	}

	paths := make([]string, 0, len(spec.Paths))
	for p := range spec.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var out []EndpointSpec
	for _, rawPath := range paths {
		item := spec.Paths[rawPath]
		if item == nil {
			continue
		}
		for _, mo := range operationsOf(item) {
			out = append(out, EndpointSpec{
				Method:        strings.ToUpper(mo.Method),
				Path:          toRoutePath(rawPath),
				Description:   strings.TrimSpace(mo.Op.Description),
				Prompt:        buildPrompt(mo.Op, strings.ToUpper(mo.Method), rawPath),
				Schema:        responseSchema(mo.Op.Responses, schemas),
				RequestSchema: requestSchema(mo.Op, schemas),
			})
		}
	}
	return out, nil
}

// toRoutePath converts an OpenAPI template such as /users/{id} into the
// router's colon form /users/:id.
func toRoutePath(raw string) string {
	return pathParamRe.ReplaceAllString(raw, ":$1")
}

// buildPrompt derives the endpoint's natural-language prompt from the
// operation summary, falling back to the operation description and finally to
// a generated sentence. The prompt drives response generation, so a useful
// description produces richer mocks.
func buildPrompt(op *Operation, method, rawPath string) string {
	parts := make([]string, 0, 2)
	if s := strings.TrimSpace(op.Summary); s != "" {
		parts = append(parts, s)
	}
	if d := strings.TrimSpace(op.Description); d != "" {
		parts = append(parts, d)
	}
	if len(parts) > 0 {
		return normalizePrompt(strings.Join(parts, ". "))
	}
	return normalizePrompt(fmt.Sprintf("handle %s %s", method, rawPath))
}

// normalizePrompt trims trailing punctuation and ends the text with a single
// period so prompts read as complete sentences.
func normalizePrompt(text string) string {
	text = strings.TrimSpace(text)
	for len(text) > 0 {
		last := text[len(text)-1]
		if last == '.' || last == '!' || last == '?' {
			text = strings.TrimSpace(text[:len(text)-1])
			continue
		}
		break
	}
	if text == "" {
		return ""
	}
	return text + "."
}

// responseSchema returns the JSON Schema of the first 2xx response as JSON
// text, or "" when the operation declares no schema. 200 and 201 are checked
// before the generic 2XX/default cases.
func responseSchema(responses map[string]*Response, schemas map[string]any) string {
	for _, status := range []string{"200", "201", "2XX", "2xx", "default"} {
		if r, ok := responses[status]; ok {
			if v := schemaValue(r, schemas); v != nil {
				return marshalSchema(v)
			}
		}
	}
	for status, r := range responses {
		if len(status) == 3 && status[0] == '2' {
			if v := schemaValue(r, schemas); v != nil {
				return marshalSchema(v)
			}
		}
	}
	return ""
}

// requestSchema returns the JSON Schema of the operation's request body as
// JSON text, or "" when no JSON request body is declared. Ref resolution and
// content-type preference mirror responseSchema.
func requestSchema(op *Operation, schemas map[string]any) string {
	if op.RequestBody == nil || op.RequestBody.Content == nil {
		return ""
	}
	for _, ct := range []string{"application/json", "application/*", "*/*"} {
		if mt, ok := op.RequestBody.Content[ct]; ok {
			if v := resolvedSchema(mt.Schema, schemas); v != nil {
				return marshalSchema(v)
			}
		}
	}
	for ct, mt := range op.RequestBody.Content {
		if strings.Contains(ct, "json") {
			if v := resolvedSchema(mt.Schema, schemas); v != nil {
				return marshalSchema(v)
			}
		}
	}
	return ""
}

// schemaValue pulls the response schema out of either OpenAPI 3.x content or
// the Swagger 2.0 schema field, resolving $refs against the document's
// schemas.
func schemaValue(r *Response, schemas map[string]any) any {
	if r.Content != nil {
		for _, ct := range []string{"application/json", "application/*", "*/*"} {
			if mt, ok := r.Content[ct]; ok {
				if v := resolvedSchema(mt.Schema, schemas); v != nil {
					return v
				}
			}
		}
		for ct, mt := range r.Content {
			if strings.Contains(ct, "json") {
				if v := resolvedSchema(mt.Schema, schemas); v != nil {
					return v
				}
			}
		}
	}
	return resolvedSchema(r.Schema, schemas)
}

// resolvedSchema resolves a $ref root and reports whether a schema was
// present.
func resolvedSchema(v any, schemas map[string]any) any {
	if v == nil {
		return nil
	}
	return resolveRefs(v, schemas, 0)
}

func marshalSchema(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

// resolveRefs replaces $ref objects with the referenced schema and resolves
// refs nested inside properties, items and composed schemas. Unresolvable
// refs are left in place; the importer stores the result only if it compiles
// as a JSON Schema.
func resolveRefs(v any, schemas map[string]any, depth int) any {
	if depth >= maxRefDepth {
		return v
	}
	if arr, ok := v.([]any); ok {
		for i, child := range arr {
			arr[i] = resolveRefs(child, schemas, depth+1)
		}
		return v
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if ref, ok := obj["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		name = strings.TrimPrefix(name, "#/definitions/")
		if target, found := schemas[name]; found {
			return resolveRefs(target, schemas, depth+1)
		}
		return obj
	}
	for k, child := range obj {
		obj[k] = resolveRefs(child, schemas, depth+1)
	}
	return v
}
