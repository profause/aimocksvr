// Package importer turns OpenAPI documents (Swagger 2.0 or OpenAPI 3.x, in
// JSON or YAML) into registry endpoints. The response schemas extracted from
// the spec are stored on the first version of each endpoint, so imported
// endpoints return schema-conforming data without an AI backend.
package importer

// Spec is a minimal OpenAPI document model covering what the importer reads:
// version markers, the operation map, and the schema components. Because YAML
// is a superset of JSON, both document encodings decode with a single decoder.
type Spec struct {
	OpenAPI     string               `json:"openapi" yaml:"openapi"`
	Swagger     string               `json:"swagger" yaml:"swagger"`
	Paths       map[string]*PathItem `json:"paths" yaml:"paths"`
	Components  *Components          `json:"components" yaml:"components"`
	Definitions map[string]any       `json:"definitions" yaml:"definitions"`
}

// Components holds the reusable schemas of an OpenAPI 3.x document.
type Components struct {
	Schemas map[string]any `json:"schemas" yaml:"schemas"`
}

// PathItem groups the operations available on one path.
type PathItem struct {
	Get     *Operation `json:"get" yaml:"get"`
	Put     *Operation `json:"put" yaml:"put"`
	Post    *Operation `json:"post" yaml:"post"`
	Delete  *Operation `json:"delete" yaml:"delete"`
	Options *Operation `json:"options" yaml:"options"`
	Head    *Operation `json:"head" yaml:"head"`
	Patch   *Operation `json:"patch" yaml:"patch"`
}

// Operation describes one HTTP operation.
type Operation struct {
	Summary     string               `json:"summary" yaml:"summary"`
	Description string               `json:"description" yaml:"description"`
	RequestBody *RequestBody         `json:"requestBody" yaml:"requestBody"`
	Responses   map[string]*Response `json:"responses" yaml:"responses"`
}

// RequestBody carries the request content types of an operation (OpenAPI 3.x).
type RequestBody struct {
	Content map[string]*MediaType `json:"content" yaml:"content"`
}

// Response describes a single status response. Swagger 2.0 puts the schema on
// Response.Schema directly; OpenAPI 3.x nests it under Response.Content.
type Response struct {
	Schema  any                   `json:"schema" yaml:"schema"`
	Content map[string]*MediaType `json:"content" yaml:"content"`
}

// MediaType pairs a content type with its schema (OpenAPI 3.x).
type MediaType struct {
	Schema any `json:"schema" yaml:"schema"`
}

// orderedMethods keeps operation extraction deterministic: paths are sorted
// and operations are emitted in this fixed order.
var orderedMethods = []string{"get", "put", "post", "delete", "options", "head", "patch"}

// methodOperation pairs an HTTP method with its operation.
type methodOperation struct {
	Method string
	Op     *Operation
}

// operationsOf extracts the non-nil operations of a path item in the
// canonical order.
func operationsOf(item *PathItem) []methodOperation {
	ops := make([]methodOperation, 0, len(orderedMethods))
	for _, m := range orderedMethods {
		var op *Operation
		switch m {
		case "get":
			op = item.Get
		case "put":
			op = item.Put
		case "post":
			op = item.Post
		case "delete":
			op = item.Delete
		case "options":
			op = item.Options
		case "head":
			op = item.Head
		case "patch":
			op = item.Patch
		}
		if op != nil {
			ops = append(ops, methodOperation{Method: m, Op: op})
		}
	}
	return ops
}
