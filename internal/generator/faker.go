package generator

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/rs/zerolog"
)

// maxFillDepth caps how deeply a schema is expanded so pathological schemas
// cannot produce unbounded output.
const maxFillDepth = 10

// fakerGenerator realizes the Phase 10 principle "AI defines structure, Faker
// generates values": it fills an endpoint's stored JSON Schema with realistic
// fake data, keyed by property names and JSON Schema formats. It is used as a
// deterministic fallback when the AI provider is unavailable, so mock requests
// still return schema-conforming data without a model backend. When no schema
// is stored it degrades to the wrapped fallback.
type fakerGenerator struct {
	schemas  SchemaLoader
	fallback Generator
	log      *zerolog.Logger
}

// NewFaker creates a Generator that fills stored JSON Schemas with fake
// values, falling back to fallback when the schema is missing or unreadable.
func NewFaker(schemas SchemaLoader, fallback Generator, logger *zerolog.Logger) Generator {
	return &fakerGenerator{schemas: schemas, fallback: fallback, log: logger}
}

func (g *fakerGenerator) Generate(ctx context.Context, req *Request) (*Response, error) {
	if req.Endpoint == nil {
		return g.fallback.Generate(ctx, req)
	}

	schema, err := g.schemas.LoadSchema(ctx, req.Endpoint.ID)
	if err != nil {
		g.log.Warn().Err(err).Str("endpoint_id", req.Endpoint.ID.String()).Msg("failed to load schema, using fallback generator")
		return g.fallback.Generate(ctx, req)
	}
	if strings.TrimSpace(schema) == "" {
		return g.fallback.Generate(ctx, req)
	}

	var doc any
	if err := json.Unmarshal([]byte(schema), &doc); err != nil {
		g.log.Warn().Err(err).Str("endpoint_id", req.Endpoint.ID.String()).Msg("failed to parse schema, using fallback generator")
		return g.fallback.Generate(ctx, req)
	}

	body, err := json.Marshal(fillValue(doc, "", 0))
	if err != nil {
		g.log.Warn().Err(err).Str("endpoint_id", req.Endpoint.ID.String()).Msg("failed to fill schema, using fallback generator")
		return g.fallback.Generate(ctx, req)
	}
	return &Response{Status: http.StatusOK, Body: body}, nil
}

// fillValue turns a JSON Schema fragment into a fake value. key is the
// property name, used to pick a fitting generator when the schema gives no
// strong signal.
func fillValue(schema any, key string, depth int) any {
	if depth >= maxFillDepth {
		return nil
	}

	obj, ok := asObject(schema)
	if !ok {
		return nil
	}
	if merged, changed := composedOf(obj); changed {
		obj = merged
	}

	if constValue, has := obj["const"]; has {
		return constValue
	}
	if enum, has := obj["enum"].([]any); has && len(enum) > 0 {
		return enum[gofakeit.Number(0, len(enum)-1)]
	}

	switch typeOf(obj) {
	case "object":
		properties, _ := obj["properties"].(map[string]any)
		out := make(map[string]any, len(properties))
		for name, prop := range properties {
			out[name] = fillValue(prop, name, depth+1)
		}
		return out
	case "array":
		items, _ := obj["items"]
		n := arrayLength(obj)
		out := make([]any, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, fillValue(items, key, depth+1))
		}
		return out
	case "integer":
		min, max := clampRange(intMin(obj), intMax(obj))
		return gofakeit.Number(min, max)
	case "number":
		min, max := clampFloatRange(floatMin(obj), floatMax(obj))
		return gofakeit.Float64Range(min, max)
	case "boolean":
		return gofakeit.Bool()
	case "null":
		return nil
	default: // string
		return fillString(obj, key)
	}
}

// fillString produces a fake string, preferring a JSON Schema format, then a
// generator hinted by the property name.
func fillString(schema map[string]any, key string) string {
	format, _ := schema["format"].(string)
	switch format {
	case "email":
		return gofakeit.Email()
	case "uuid":
		return gofakeit.UUID()
	case "date", "date-time":
		return formatDate(format)
	case "uri", "url":
		return gofakeit.URL()
	case "hostname":
		return gofakeit.DomainName()
	case "ipv4":
		return gofakeit.IPv4Address()
	case "ipv6":
		return gofakeit.IPv6Address()
	case "phone":
		return gofakeit.Phone()
	}

	if value, matched := hintedString(strings.ToLower(key)); matched {
		return value
	}
	return gofakeit.Word()
}

// hintedString picks a fake generator by property name. The roadmap's
// generators are all covered: person, company, email, phone, address, uuid,
// bank, credit card, date, currency, country.
func hintedString(key string) (string, bool) {
	switch {
	case strings.Contains(key, "email"):
		return gofakeit.Email(), true
	case strings.Contains(key, "credit") || strings.Contains(key, "card") || key == "cc":
		return gofakeit.CreditCardNumber(nil), true
	case strings.Contains(key, "iban") || strings.Contains(key, "bank") || strings.Contains(key, "routing"):
		return gofakeit.AchAccount(), true
	case strings.Contains(key, "currency") || strings.Contains(key, "amount") || strings.Contains(key, "price"):
		return gofakeit.Currency().Short, true
	case strings.Contains(key, "country"):
		return gofakeit.Country(), true
	case strings.Contains(key, "city"):
		return gofakeit.City(), true
	case strings.Contains(key, "state") || strings.Contains(key, "region") || strings.Contains(key, "province"):
		return gofakeit.Address().State, true
	case strings.Contains(key, "street"):
		return gofakeit.Address().Street, true
	case strings.Contains(key, "address") || strings.Contains(key, "location"):
		return gofakeit.Address().Address, true
	case strings.Contains(key, "zip") || strings.Contains(key, "postal") || strings.Contains(key, "postcode"):
		return gofakeit.Address().Zip, true
	case strings.Contains(key, "latitude"):
		return formatFloat(gofakeit.Latitude()), true
	case strings.Contains(key, "longitude"):
		return formatFloat(gofakeit.Longitude()), true
	case strings.Contains(key, "phone") || strings.Contains(key, "mobile") || strings.Contains(key, "cell"):
		return gofakeit.Phone(), true
	case strings.Contains(key, "company") || strings.Contains(key, "organization") || strings.Contains(key, "business") || strings.Contains(key, "employer"):
		return gofakeit.Company(), true
	case strings.Contains(key, "job") || strings.Contains(key, "title") || strings.Contains(key, "position") || strings.Contains(key, "role"):
		if job := gofakeit.Job(); job != nil {
			return job.Title, true
		}
	case strings.Contains(key, "password") || strings.Contains(key, "secret") || strings.Contains(key, "api_key") || strings.Contains(key, "apikey"):
		return gofakeit.Password(true, true, true, true, false, 16), true
	case strings.Contains(key, "first_name") || strings.Contains(key, "firstname") || strings.Contains(key, "given"):
		return gofakeit.FirstName(), true
	case strings.Contains(key, "last_name") || strings.Contains(key, "lastname") || strings.Contains(key, "surname") || strings.Contains(key, "family"):
		return gofakeit.LastName(), true
	case strings.Contains(key, "username") || strings.Contains(key, "handle") || key == "user":
		return gofakeit.Username(), true
	case strings.Contains(key, "gender"):
		return []string{"male", "female"}[gofakeit.Number(0, 1)], true
	case strings.Contains(key, "token") || strings.Contains(key, "key") || strings.Contains(key, "id"):
		return gofakeit.UUID(), true
	case strings.Contains(key, "birth") || strings.Contains(key, "date") || strings.Contains(key, "day") || strings.Contains(key, "year"):
		return formatDate("date"), true
	case strings.Contains(key, "time") || strings.Contains(key, "timestamp") || strings.Contains(key, "created") || strings.Contains(key, "updated"):
		return formatDate("date-time"), true
	case strings.Contains(key, "url") || strings.Contains(key, "website") || strings.Contains(key, "homepage") || strings.Contains(key, "link"):
		return gofakeit.URL(), true
	case strings.Contains(key, "name") || strings.Contains(key, "person"):
		return gofakeit.Name(), true
	}
	return "", false
}

// formatDate renders fake dates for the "date" and "date-time" formats.
func formatDate(format string) string {
	t := gofakeit.Date()
	if format == "date" {
		return t.Format("2006-01-02")
	}
	return t.Format(time.RFC3339)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func asObject(v any) (map[string]any, bool) {
	obj, ok := v.(map[string]any)
	return obj, ok
}

// composedOf flattens composed schemas so the filler can handle them: allOf
// members are merged into a single schema, and oneOf/anyOf pick their first
// member. Imported OpenAPI schemas often reference components through allOf,
// so this keeps such schemas fillable.
func composedOf(schema map[string]any) (map[string]any, bool) {
	if allOf, ok := schema["allOf"].([]any); ok && len(allOf) > 0 {
		merged := make(map[string]any, len(schema)+len(allOf))
		for k, v := range schema {
			if k != "allOf" {
				merged[k] = v
			}
		}
		for _, member := range allOf {
			if m, ok := asObject(member); ok {
				for k, v := range m {
					merged[k] = v
				}
			}
		}
		return merged, true
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		if choices, ok := schema[keyword].([]any); ok && len(choices) > 0 {
			if first, ok := asObject(choices[0]); ok {
				return first, true
			}
		}
	}
	return schema, false
}

func typeOf(schema map[string]any) string {
	if t, ok := schema["type"].(string); ok {
		return t
	}
	if _, has := schema["properties"]; has {
		return "object"
	}
	if _, has := schema["enum"]; has {
		return "string"
	}
	return "string"
}

func arrayLength(schema map[string]any) int {
	length := 1
	if min, has := numOf(schema["minItems"]); has {
		length = int(min)
	}
	if max, has := numOf(schema["maxItems"]); has && int(max) < length {
		length = int(max)
	}
	if length < 0 {
		length = 0
	}
	return length
}

// intMin returns the minimum bound for an integer property. When only a
// maximum is given the default minimum is bounded by it.
func intMin(schema map[string]any) int {
	if v, has := numOf(schema["minimum"]); has {
		return int(v)
	}
	return 1
}

func intMax(schema map[string]any) int {
	if v, has := numOf(schema["maximum"]); has {
		return int(v)
	}
	return 1000
}

func floatMin(schema map[string]any) float64 {
	if v, has := numOf(schema["minimum"]); has {
		return v
	}
	return 0
}

func floatMax(schema map[string]any) float64 {
	if v, has := numOf(schema["maximum"]); has {
		return v
	}
	return 100
}

// clampRange orders a (min, max) pair so faker's random-in-range call is safe
// when a schema's bounds are contradictory.
func clampRange(min, max int) (int, int) {
	if min > max {
		return max, min
	}
	return min, max
}

func clampFloatRange(min, max float64) (float64, float64) {
	if min > max {
		return max, min
	}
	return min, max
}

// numOf reads a JSON Schema numeric keyword (number or integer) and reports
// whether it was present.
func numOf(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
