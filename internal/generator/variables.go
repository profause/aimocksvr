package generator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// RenderVariables replaces {{name}} placeholders in a prompt with values taken
// from the request. Supported namespaces:
//
//	{{path.id}}         path parameter
//	{{query.country}}   query parameter
//	{{body.email}}      field of the JSON request body (dot paths are supported)
//	{{headers.auth}}    request header (case-insensitive)
//	{{cookies.session}} request cookie
//
// Placeholder names are case-insensitive. Missing values render as an empty
// string, and malformed placeholders are left untouched.
func RenderVariables(prompt string, req *Request) string {
	if req == nil {
		return prompt
	}
	vars := extractVariables(req)

	var out strings.Builder
	for {
		start := strings.Index(prompt, "{{")
		if start < 0 {
			out.WriteString(prompt)
			break
		}
		out.WriteString(prompt[:start])

		rest := prompt[start+2:]
		end := strings.Index(rest, "}}")
		if end < 0 {
			out.WriteString(prompt[start:])
			break
		}
		name := strings.ToLower(strings.TrimSpace(rest[:end]))
		out.WriteString(vars[name])
		prompt = rest[end+2:]
	}
	return out.String()
}

// extractVariables builds the flat name→value map for a request.
func extractVariables(req *Request) map[string]string {
	vars := map[string]string{}

	for name, value := range req.PathParams {
		vars["path."+strings.ToLower(name)] = value
	}
	for name, value := range req.Query {
		vars["query."+strings.ToLower(name)] = value
	}
	for name, values := range req.Headers {
		vars["headers."+strings.ToLower(name)] = strings.Join(values, ",")
	}
	for _, c := range parseCookies(req.Headers) {
		vars["cookies."+strings.ToLower(c.Name)] = c.Value
	}
	for name, value := range bodyVars(req.Body) {
		vars["body."+name] = value
	}

	return vars
}

// parseCookies reads the Cookie header of the request headers. It is parsed
// through http.Request so the standard cookie semantics apply.
func parseCookies(headers http.Header) []*http.Cookie {
	if len(headers) == 0 {
		return nil
	}
	r := &http.Request{Header: headers}
	return r.Cookies()
}

// bodyVars flattens a JSON request body into dot-path names (body.user.name).
// Non-JSON bodies yield no variables.
func bodyVars(body []byte) map[string]string {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil
	}

	vars := map[string]string{}
	collectBodyVars("", doc, vars)
	return vars
}

func collectBodyVars(prefix string, value any, out map[string]string) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			name := strings.ToLower(key)
			if prefix != "" {
				name = prefix + "." + name
			}
			collectBodyVars(name, item, out)
		}
	case []any:
		if prefix != "" {
			if data, err := json.Marshal(v); err == nil {
				out[prefix] = string(data)
			}
		}
	default:
		if prefix != "" {
			out[prefix] = scalarString(value)
		}
	}
}

// scalarString renders a decoded JSON scalar as a plain string. Strings are
// used verbatim, numbers and booleans are formatted, and null becomes empty.
func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
