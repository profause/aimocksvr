package auth

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/api"
	"github.com/profause/aimocksvr/internal/config"
)

func newTestApp(cfg *config.Config) *fiber.App {
	logger := zerolog.Nop()
	h := NewHandler(cfg, newTestService(cfg.Auth.APIKeys, cfg.Auth.WorkspaceTokens, cfg.Auth.JWTSecret, cfg.Auth.Enabled), &logger)

	app := fiber.New()
	app.Use(h.Middleware())
	app.Get("/health", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	apiGroup := app.Group("/api/v1")
	h.Register(apiGroup)
	app.Get("/private", func(c fiber.Ctx) error {
		id, ok := c.Locals(IdentityKey).(Identity)
		if !ok {
			return api.Fail(c, fiber.StatusUnauthorized, api.CodeUnauthorized, "no identity")
		}
		return api.OK(c, id)
	})
	return app
}

func testCfg(enabled bool) *config.Config {
	return &config.Config{Auth: config.Auth{
		Enabled:         enabled,
		JWTSecret:       "secret",
		JWTIssuer:       "mocksvr",
		JWTAudience:     "mocksvr",
		JWTTTL:          "1h",
		APIKeys:         "dev:sk_test_123",
		WorkspaceTokens: "acme:tok_acme_456",
	}}
}

func TestMiddlewareDisabledAllowsEverything(t *testing.T) {
	app := newTestApp(testCfg(false))

	// With auth disabled the middleware never blocks; /health is served and
	// non-control-plane paths pass through untouched.
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/health", fiber.StatusOK},
		{"POST", "/api/v1/auth/token", fiber.StatusBadRequest}, // no body
	} {
		resp, err := app.Test(httptest.NewRequest(tc.method, tc.path, nil))
		if err != nil {
			t.Fatalf("%s %s failed: %v", tc.method, tc.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("%s %s: expected %d, got %d", tc.method, tc.path, tc.want, resp.StatusCode)
		}
	}
}

func TestMiddlewareProtectsControlPlane(t *testing.T) {
	app := newTestApp(testCfg(true))

	// /health stays public.
	resp, err := app.Test(httptest.NewRequest("GET", "/health", nil))
	if err != nil {
		t.Fatalf("health failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected health 200, got %d", resp.StatusCode)
	}

	// Control plane without credentials -> 401.
	resp, err = app.Test(httptest.NewRequest("GET", "/api/v1/auth/whoami", nil))
	if err != nil {
		t.Fatalf("whoami failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 without credentials, got %d", resp.StatusCode)
	}

	// Control plane with an invalid credential -> 401.
	req := httptest.NewRequest("GET", "/api/v1/auth/whoami", nil)
	req.Header.Set("X-API-Key", "sk_bad")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("whoami failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 with bad key, got %d", resp.StatusCode)
	}

	// Control plane with a valid API key -> 200 and identity present.
	req = httptest.NewRequest("GET", "/api/v1/auth/whoami", nil)
	req.Header.Set("X-API-Key", "sk_test_123")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("whoami failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 with valid key, got %d (body %q)", resp.StatusCode, body)
	}
	var got struct {
		Data Identity `json:"data"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode whoami: %v", err)
	}
	if got.Data.Kind != KindAPIKey || got.Data.Name != "dev" {
		t.Errorf("unexpected identity: %+v", got.Data)
	}
}

func TestMiddlewarePassesMockPathsThrough(t *testing.T) {
	app := newTestApp(testCfg(true))

	// Non-control-plane paths without credentials are not blocked here; the
	// dynamic router enforces mock endpoint privacy.
	resp, err := app.Test(httptest.NewRequest("GET", "/anything", nil))
	if err != nil {
		t.Fatalf("mock path failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == fiber.StatusUnauthorized {
		t.Errorf("middleware must not 401 mock paths directly")
	}
}

func TestTokenMintsJWT(t *testing.T) {
	app := newTestApp(testCfg(true))

	req := httptest.NewRequest("POST", "/api/v1/auth/token", strings.NewReader(`{"api_key":"sk_test_123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("token request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d (body %q)", resp.StatusCode, body)
	}
	var got struct {
		Data struct {
			Token string `json:"token"`
			Kind  string `json:"kind"`
			Name  string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if got.Data.Token == "" || got.Data.Kind != KindAPIKey || got.Data.Name != "dev" {
		t.Errorf("unexpected token response: %+v", got.Data)
	}

	// The minted JWT authenticates on the control plane.
	req = httptest.NewRequest("GET", "/api/v1/auth/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+got.Data.Token)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("whoami with JWT failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 with minted JWT, got %d (body %q)", resp.StatusCode, body)
	}
}

func TestTokenRejectsInvalidCredentials(t *testing.T) {
	app := newTestApp(testCfg(true))

	req := httptest.NewRequest("POST", "/api/v1/auth/token", strings.NewReader(`{"api_key":"sk_bad"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("token request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for bad key, got %d", resp.StatusCode)
	}

	req = httptest.NewRequest("POST", "/api/v1/auth/token", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("token request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400 when credential missing, got %d", resp.StatusCode)
	}
}

func TestTokenWithWorkspaceToken(t *testing.T) {
	app := newTestApp(testCfg(true))

	req := httptest.NewRequest("POST", "/api/v1/auth/token", strings.NewReader(`{"workspace_token":"tok_acme_456"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("token request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d (body %q)", resp.StatusCode, body)
	}
	var got struct {
		Data struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if got.Data.Kind != KindWorkspaceToken || got.Data.Name != "acme" {
		t.Errorf("unexpected identity: %+v", got.Data)
	}
}

func TestExtractToken(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(extractToken(c))
	})

	for _, tc := range []struct {
		header string
		value  string
		want   string
	}{
		{fiber.HeaderAuthorization, "Bearer sk_test", "sk_test"},
		{fiber.HeaderAuthorization, "sk_no_bearer", ""},
		{"X-API-Key", "sk_x", "sk_x"},
		{"X-Workspace-Token", "tok_x", "tok_x"},
	} {
		req := httptest.NewRequest("GET", "/", nil)
		if tc.value != "" {
			req.Header.Set(tc.header, tc.value)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != tc.want {
			t.Errorf("%s=%q: expected %q, got %q", tc.header, tc.value, tc.want, body)
		}
	}
}
