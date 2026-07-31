package importer

import (
	"encoding/json"
	"strings"
	"testing"
)

const postmanCollection = `{
  "info": {"name": "Users API", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
  "item": [
    {
      "name": "Users",
      "item": [
        {
          "name": "List users",
          "request": {
            "method": "GET",
            "url": {"raw": "https://api.example.com/v2/users?page=1", "path": ["v2", "users"]}
          },
          "response": [
            {"name": "ok", "code": 200, "body": "[{\"id\": 1, \"name\": \"Ada\"}]"}
          ]
        },
        {
          "name": "Create user",
          "request": {
            "method": "POST",
            "description": "registers a new user",
            "url": {"raw": "https://api.example.com/v2/users", "path": ["v2", "users"]},
            "body": {"mode": "raw", "raw": "{\"name\":\"Ada\",\"age\":36,\"active\":true}"}
          },
          "response": [
            {"name": "created", "code": 201, "body": "{\"id\": 42, \"name\": \"Ada\"}"}
          ]
        }
      ]
    },
    {
      "name": "Get user by id",
      "request": {
        "method": "GET",
        "url": {"raw": "https://api.example.com/v2/users/{{userId}}", "path": ["v2", "users", ":userId"]}
      },
      "response": [
        {"name": "not found", "code": 404, "body": "{\"error\": \"missing\"}"}
      ]
    }
  ]
}`

func TestParsePostmanFlattensCollection(t *testing.T) {
	specs, err := ParsePostman([]byte(postmanCollection))
	if err != nil {
		t.Fatalf("ParsePostman returned error: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(specs))
	}

	list := specs[0]
	if list.Method != "GET" || list.Path != "/v2/users" {
		t.Errorf("list = %s %s, want GET /v2/users", list.Method, list.Path)
	}
	if list.Prompt != "List users." {
		t.Errorf("prompt = %q, want %q", list.Prompt, "List users.")
	}
	if list.RequestSchema != "" {
		t.Errorf("GET should have no request schema, got %q", list.RequestSchema)
	}
	var listSchema map[string]any
	if err := json.Unmarshal([]byte(list.Schema), &listSchema); err != nil {
		t.Fatalf("list schema should be valid JSON: %v", err)
	}
	if listSchema["type"] != "array" {
		t.Fatalf("list schema type = %v, want array", listSchema["type"])
	}
	itemSchema, _ := listSchema["items"].(map[string]any)
	props, _ := itemSchema["properties"].(map[string]any)
	if _, ok := props["id"]; !ok {
		t.Errorf("array item schema missing id property: %v", itemSchema)
	}
	if _, ok := props["name"]; !ok {
		t.Errorf("array item schema missing name property: %v", itemSchema)
	}

	create := specs[1]
	if create.Method != "POST" || create.Path != "/v2/users" {
		t.Errorf("create = %s %s, want POST /v2/users", create.Method, create.Path)
	}
	if !strings.Contains(create.Prompt, "Create user") || !strings.Contains(create.Prompt, "registers a new user") {
		t.Errorf("prompt = %q, want name and description", create.Prompt)
	}
	var req map[string]any
	if err := json.Unmarshal([]byte(create.RequestSchema), &req); err != nil {
		t.Fatalf("request schema should be valid JSON: %v", err)
	}
	if req["type"] != "object" {
		t.Errorf("request schema type = %v, want object", req["type"])
	}
	reqProps, _ := req["properties"].(map[string]any)
	if _, ok := reqProps["age"]; !ok {
		t.Errorf("request schema missing age: %v", req)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(create.Schema), &resp); err != nil {
		t.Fatalf("response schema should be valid JSON: %v", err)
	}
	if resp["type"] != "object" {
		t.Errorf("response schema type = %v, want object", resp["type"])
	}
}

func TestParsePostmanKeepsColonParamsAndConvertsVars(t *testing.T) {
	specs, err := ParsePostman([]byte(postmanCollection))
	if err != nil {
		t.Fatalf("ParsePostman returned error: %v", err)
	}
	if len(specs) < 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(specs))
	}
	got := specs[2]
	if got.Path != "/v2/users/:userId" {
		t.Errorf("path = %q, want /v2/users/:userId", got.Path)
	}
	if got.Schema == "" {
		t.Errorf("expected a schema from the fallback example, got none")
	}
}

func TestParsePostmanRawPathFallback(t *testing.T) {
	col := `{"info":{"name":"T"},"item":[{"name":"x","request":{"method":"get","url":{"raw":"{{baseUrl}}/items/5?verbose=true#frag"}}}]}`
	specs, err := ParsePostman([]byte(col))
	if err != nil {
		t.Fatalf("ParsePostman returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(specs))
	}
	if specs[0].Path != "/items/5" {
		t.Errorf("path = %q, want /items/5", specs[0].Path)
	}
	if specs[0].Method != "GET" {
		t.Errorf("method = %q, want default GET", specs[0].Method)
	}
}

func TestParsePostmanConvertsVarPath(t *testing.T) {
	col := `{"info":{"name":"T"},"item":[{"name":"x","request":{"method":"get","url":{"raw":"{{baseUrl}}/users/{{id}}","path":["users","{{id}}"]}}}]}`
	specs, err := ParsePostman([]byte(col))
	if err != nil {
		t.Fatalf("ParsePostman returned error: %v", err)
	}
	if specs[0].Path != "/users/:id" {
		t.Errorf("path = %q, want /users/:id", specs[0].Path)
	}
}

func TestParsePostmanSkipsRequestsWithoutPath(t *testing.T) {
	col := `{"info":{"name":"T"},"item":[{"name":"no url","request":{"method":"get"}},{"name":"ok","request":{"method":"get","url":{"path":["ping"]}}}]}`
	specs, err := ParsePostman([]byte(col))
	if err != nil {
		t.Fatalf("ParsePostman returned error: %v", err)
	}
	if len(specs) != 1 || specs[0].Path != "/ping" {
		t.Fatalf("expected only /ping, got %+v", specs)
	}
}

func TestParsePostmanIgnoresNonRawRequestBodies(t *testing.T) {
	col := `{"info":{"name":"T"},"item":[{"name":"form","request":{"method":"post","url":{"path":["login"]},"body":{"mode":"urlencoded","raw":"a=1"}}}]}`
	specs, err := ParsePostman([]byte(col))
	if err != nil {
		t.Fatalf("ParsePostman returned error: %v", err)
	}
	if specs[0].RequestSchema != "" {
		t.Errorf("urlencoded body should yield no request schema, got %q", specs[0].RequestSchema)
	}
}

func TestParsePostmanIgnoresNonJSONBodies(t *testing.T) {
	col := `{"info":{"name":"T"},"item":[{"name":"txt","request":{"method":"post","url":{"path":["echo"]},"body":{"mode":"raw","raw":"hello world"}},"response":[{"name":"text","code":200,"body":"<html>hi</html>"}]}]}`
	specs, err := ParsePostman([]byte(col))
	if err != nil {
		t.Fatalf("ParsePostman returned error: %v", err)
	}
	if specs[0].RequestSchema != "" || specs[0].Schema != "" {
		t.Errorf("non-JSON bodies should yield no schemas, got request=%q response=%q", specs[0].RequestSchema, specs[0].Schema)
	}
}

func TestParsePostmanPrefers2xxExample(t *testing.T) {
	col := `{"info":{"name":"T"},"item":[{"name":"x","request":{"method":"get","url":{"path":["r"]}},"response":[{"name":"err","code":500,"body":"{\"code\":\"boom\"}"},{"name":"ok","code":200,"body":"{\"id\":1}"}]}]}`
	specs, err := ParsePostman([]byte(col))
	if err != nil {
		t.Fatalf("ParsePostman returned error: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(specs[0].Schema), &schema); err != nil {
		t.Fatalf("response schema should be valid JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]any)
	if _, ok := props["id"]; !ok {
		t.Errorf("expected the 2xx example schema (id), got %v", schema)
	}
	if _, ok := props["code"]; ok {
		t.Errorf("non-2xx example should not win, got %v", schema)
	}
}

func TestParsePostmanErrors(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"invalid json", "not: [json"},
		{"no info name", `{"item":[]}`},
		{"no items", `{"info":{"name":"T"}}`},
		{"requests only in folders", `{"info":{"name":"T"},"item":[{"name":"folder","item":[]}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParsePostman([]byte(tc.data)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestInferJSONSchema(t *testing.T) {
	doc := `{"n":1,"f":2.5,"s":"x","b":true,"nil":null,"arr":[{"k":"v"}],"empty":[],"nested":{"deep":{"n":1}}}`
	var v any
	if err := json.Unmarshal([]byte(doc), &v); err != nil {
		t.Fatalf("decode doc: %v", err)
	}
	schema, _ := inferJSONSchema(v).(map[string]any)
	props, _ := schema["properties"].(map[string]any)

	if n, _ := props["n"].(map[string]any); n["type"] != "integer" {
		t.Errorf("n type = %v, want integer", n["type"])
	}
	if f, _ := props["f"].(map[string]any); f["type"] != "number" {
		t.Errorf("f type = %v, want number", f["type"])
	}
	if s, _ := props["s"].(map[string]any); s["type"] != "string" {
		t.Errorf("s type = %v, want string", s["type"])
	}
	if b, _ := props["b"].(map[string]any); b["type"] != "boolean" {
		t.Errorf("b type = %v, want boolean", b["type"])
	}
	if nl, _ := props["nil"].(map[string]any); nl["type"] != "null" {
		t.Errorf("nil type = %v, want null", nl["type"])
	}

	arr, _ := props["arr"].(map[string]any)
	if arr["type"] != "array" {
		t.Errorf("arr type = %v, want array", arr["type"])
	}
	arrItems, _ := arr["items"].(map[string]any)
	if arrItems["type"] != "object" {
		t.Errorf("arr items type = %v, want object", arrItems["type"])
	}

	empty, _ := props["empty"].(map[string]any)
	emptyItems, _ := empty["items"].(map[string]any)
	if len(emptyItems) != 0 {
		t.Errorf("empty array items = %v, want empty schema", emptyItems)
	}

	required, _ := schema["required"].([]string)
	if len(required) != len(props) {
		t.Errorf("required = %v, want all %d keys", required, len(props))
	}
}
