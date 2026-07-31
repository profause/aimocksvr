package importer

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// PostmanCollection is a Postman Collection v2.0/v2.1 document. Collections
// are JSON only, so the struct decodes directly with encoding/json.
type PostmanCollection struct {
	Info PostmanInfo    `json:"info"`
	Item []*PostmanItem `json:"item"`
}

// PostmanInfo identifies the collection.
type PostmanInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

// PostmanItem is either a request (Request set) or a folder (Item children).
type PostmanItem struct {
	Name     string            `json:"name"`
	Item     []*PostmanItem    `json:"item"`
	Request  *PostmanRequest   `json:"request"`
	Response []*PostmanExample `json:"response"`
}

// PostmanRequest describes one HTTP request in the collection.
type PostmanRequest struct {
	Method      string       `json:"method"`
	Description string       `json:"description"`
	URL         *PostmanURL  `json:"url"`
	Body        *PostmanBody `json:"body"`
}

// PostmanURL is the request target. Path is either a string or an array of
// segments depending on how the collection was exported.
type PostmanURL struct {
	Raw  string `json:"raw"`
	Path any    `json:"path"`
}

// PostmanBody is the request body. Only "raw" mode carries meaningful content
// for schema inference.
type PostmanBody struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

// PostmanExample is a saved example response attached to a request.
type PostmanExample struct {
	Name   string `json:"name"`
	Code   int    `json:"code"`
	Status string `json:"status"`
	Body   string `json:"body"`
}

// postmanVarRe matches {{variable}} placeholders used in Postman paths and
// host templates.
var postmanVarRe = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

// ParsePostman decodes a Postman Collection JSON document and flattens every
// request into an EndpointSpec. Folders are walked recursively in document
// order. The request URL becomes the router path ({{vars}} become :params),
// the request name and description drive the prompt, and the sample request
// and example response bodies are turned into JSON Schemas. Empty or
// non-collection input is rejected.
func ParsePostman(data []byte) ([]EndpointSpec, error) {
	var col PostmanCollection
	if err := json.Unmarshal(data, &col); err != nil {
		return nil, &ParseError{Message: fmt.Sprintf("invalid Postman collection: %v", err)}
	}
	if strings.TrimSpace(col.Info.Name) == "" {
		return nil, &ParseError{Message: "document is not a Postman collection (missing info.name)"}
	}
	if len(col.Item) == 0 {
		return nil, &ParseError{Message: "collection contains no items"}
	}

	var out []EndpointSpec
	var walk func(items []*PostmanItem)
	walk = func(items []*PostmanItem) {
		for _, it := range items {
			if it == nil {
				continue
			}
			if it.Request != nil {
				if spec, ok := postmanEndpoint(it); ok {
					out = append(out, spec)
				}
			}
			if len(it.Item) > 0 {
				walk(it.Item)
			}
		}
	}
	walk(col.Item)

	if len(out) == 0 {
		return nil, &ParseError{Message: "collection contains no requests"}
	}
	return out, nil
}

// postmanEndpoint converts one collection item into an EndpointSpec. Requests
// without a resolvable path are skipped.
func postmanEndpoint(it *PostmanItem) (EndpointSpec, bool) {
	method := strings.ToUpper(strings.TrimSpace(it.Request.Method))
	if method == "" {
		method = "GET"
	}
	path := postmanPath(it.Request.URL)
	if path == "" {
		return EndpointSpec{}, false
	}
	return EndpointSpec{
		Method:        method,
		Path:          path,
		Description:   strings.TrimSpace(it.Request.Description),
		Prompt:        postmanPrompt(it, method, path),
		Schema:        responseExampleSchema(it.Response),
		RequestSchema: requestExampleSchema(it.Request.Body),
	}, true
}

// postmanPath derives the router path from the request URL. url.path is
// preferred (already split into segments); url.raw falls back and is stripped
// of its scheme and host. Postman {{variable}} segments become :params and
// Postman :param segments are kept as-is.
func postmanPath(u *PostmanURL) string {
	if u == nil {
		return ""
	}
	var raw string
	switch p := u.Path.(type) {
	case string:
		raw = p
	case []any:
		parts := make([]string, 0, len(p))
		for _, seg := range p {
			if s, ok := seg.(string); ok {
				parts = append(parts, s)
			}
		}
		raw = strings.Join(parts, "/")
	}
	if raw == "" {
		raw = u.Raw
	}
	if raw == "" {
		return ""
	}
	raw = stripURLPrefix(raw)
	raw = dropLeadingVariable(raw)
	raw = postmanVarRe.ReplaceAllString(raw, ":$1")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	for strings.HasSuffix(raw, "/") {
		raw = strings.TrimSuffix(raw, "/")
	}
	for strings.Contains(raw, "//") {
		raw = strings.ReplaceAll(raw, "//", "/")
	}
	return raw
}

// stripURLPrefix reduces a full URL to its path, dropping scheme, host, query
// and fragment.
func stripURLPrefix(raw string) string {
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			raw = rest[j:]
		} else {
			return ""
		}
	}
	if i := strings.IndexByte(raw, '?'); i >= 0 {
		raw = raw[:i]
	}
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		raw = raw[:i]
	}
	return raw
}

// dropLeadingVariable removes a leading {{host}} segment from a bare
// variable-prefixed path like "{{baseUrl}}/users/1".
func dropLeadingVariable(raw string) string {
	if !strings.HasPrefix(raw, "{{") {
		return raw
	}
	if i := strings.IndexByte(raw, '/'); i >= 0 {
		if strings.HasSuffix(raw[:i], "}}") {
			return raw[i+1:]
		}
	}
	return raw
}

// postmanPrompt derives the endpoint prompt from the request name and
// description, falling back to a generated sentence.
func postmanPrompt(it *PostmanItem, method, path string) string {
	parts := make([]string, 0, 2)
	if name := strings.TrimSpace(it.Name); name != "" {
		parts = append(parts, name)
	}
	if d := strings.TrimSpace(it.Request.Description); d != "" {
		parts = append(parts, d)
	}
	if len(parts) > 0 {
		return normalizePrompt(strings.Join(parts, ". "))
	}
	return normalizePrompt(fmt.Sprintf("handle %s %s", method, path))
}

// requestExampleSchema infers a JSON Schema from a raw JSON request body, or
// "" when the body is not raw JSON.
func requestExampleSchema(body *PostmanBody) string {
	if body == nil || body.Mode != "raw" {
		return ""
	}
	if !isJSON(body.Raw) {
		return ""
	}
	return marshalSchema(inferJSONSchema(unmarshal(body.Raw)))
}

// responseExampleSchema infers a JSON Schema from the example responses,
// preferring the first 2xx example with a JSON body and otherwise falling
// back to the first JSON example.
func responseExampleSchema(responses []*PostmanExample) string {
	var fallback string
	for _, r := range responses {
		if r == nil {
			continue
		}
		if !isJSON(r.Body) {
			continue
		}
		schema := marshalSchema(inferJSONSchema(unmarshal(r.Body)))
		if r.Code >= 200 && r.Code < 300 {
			return schema
		}
		if fallback == "" {
			fallback = schema
		}
	}
	return fallback
}

// isJSON reports whether text parses as a non-empty JSON value.
func isJSON(text string) bool {
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return false
	}
	return v != nil
}

// unmarshal decodes JSON text into a value; the caller guarantees validity.
func unmarshal(text string) any {
	var v any
	_ = json.Unmarshal([]byte(text), &v)
	return v
}

// inferJSONSchema builds a minimal JSON Schema describing the decoded JSON
// sample. Arrays infer their element type from the first element and objects
// mark every observed key as required, so generated mocks and validated
// requests stay faithful to the recorded example.
func inferJSONSchema(v any) any {
	switch t := v.(type) {
	case nil:
		return map[string]any{"type": "null"}
	case bool:
		return map[string]any{"type": "boolean"}
	case float64:
		if t == math.Trunc(t) {
			return map[string]any{"type": "integer"}
		}
		return map[string]any{"type": "number"}
	case string:
		return map[string]any{"type": "string"}
	case []any:
		items := any(map[string]any{})
		for _, child := range t {
			if child != nil {
				items = inferJSONSchema(child)
				break
			}
		}
		return map[string]any{"type": "array", "items": items}
	case map[string]any:
		props := make(map[string]any, len(t))
		keys := make([]string, 0, len(t))
		for k, child := range t {
			props[k] = inferJSONSchema(child)
			keys = append(keys, k)
		}
		schema := map[string]any{"type": "object", "properties": props}
		if len(keys) > 0 {
			sort.Strings(keys)
			schema["required"] = keys
		}
		return schema
	default:
		return map[string]any{}
	}
}
