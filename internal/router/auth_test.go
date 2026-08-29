package router

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/account"
	"github.com/profause/aimocksvr/internal/ai"
	"github.com/profause/aimocksvr/internal/auth"
	"github.com/profause/aimocksvr/internal/cache"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/endpoint"
	"github.com/profause/aimocksvr/internal/generator"
	"github.com/profause/aimocksvr/internal/importer"
	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/validator"
)

// inMemoryAccountRepo satisfies account.Repository without a database for the
// full-router tests, which only exercise the routing surface.
type inMemoryAccountRepo struct {
	accounts map[string]*models.Account
}

func (r *inMemoryAccountRepo) Create(_ context.Context, a *models.Account) error {
	if _, ok := r.accounts[a.Email]; ok {
		return account.ErrConflict
	}
	r.accounts[a.Email] = a
	return nil
}

func (r *inMemoryAccountRepo) FindByEmail(_ context.Context, email string) (*models.Account, error) {
	if a, ok := r.accounts[email]; ok {
		return a, nil
	}
	return nil, account.ErrNotFound
}

func newAccountHandler(logger *zerolog.Logger) *account.Handler {
	authCfg := &config.Config{Auth: config.Auth{
		Enabled:     true,
		JWTSecret:   "test-secret",
		JWTIssuer:   "mocksvr",
		JWTAudience: "mocksvr",
		JWTTTL:      "1h",
	}}
	authSvc := auth.NewService(authCfg, logger)
	repo := &inMemoryAccountRepo{accounts: make(map[string]*models.Account)}
	return account.NewHandler(account.NewService(repo, authSvc), logger)
}

// newAuthApp builds the full router with auth enabled and in-memory
// persistence, mirroring the production wiring.
func newAuthApp(t *testing.T) *fiber.App {
	t.Helper()

	repo := newImportRepo()
	logger := zerolog.Nop()
	esvc := endpoint.NewService(repo, cache.Noop{}, ai.Noop{}, validator.New(), &logger)
	cfg := &config.Config{}
	cfg.App.Name = "aimocksvr-test"
	cfg.Auth.Enabled = true
	cfg.Auth.JWTSecret = "test-secret"
	cfg.Auth.JWTIssuer = "mocksvr"
	cfg.Auth.JWTAudience = "mocksvr"
	cfg.Auth.JWTTTL = "1h"
	cfg.Auth.APIKeys = "dev:sk_test_123"
	cfg.Auth.WorkspaceTokens = "acme:tok_acme_456"

	h := endpoint.NewHandler(esvc, cfg, &logger)
	imp := importer.NewHandler(importer.NewService(esvc, &logger), cfg, &logger)

	gen := generator.NewFaker(importSchemaLoader{repo: repo}, generator.NewStatic(), &logger)
	dyn := NewDynamicHandler(repo, gen, validator.New(), cfg, &logger)
	ah := auth.NewHandler(cfg, auth.NewService(cfg, &logger), &logger)

	return New(cfg, &logger, h, imp, dyn, ah, newAccountHandler(&logger))
}

func TestAuthenticationViaRegistryAPI(t *testing.T) {
	app := newAuthApp(t)

	// /health is always public.
	resp, err := app.Test(httptest.NewRequest("GET", "/health", nil))
	if err != nil {
		t.Fatalf("health failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected /health 200, got %d", resp.StatusCode)
	}

	// The control plane requires a credential.
	resp, err = app.Test(httptest.NewRequest("POST", "/api/v1/endpoints", strings.NewReader(`{
		"method": "get",
		"path": "/secret",
		"prompt": "a private endpoint",
		"public": false
	}`)))
	if err != nil {
		t.Fatalf("create without credentials failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401 without credentials, got %d", resp.StatusCode)
	}

	// Register an account so we get an account-scoped credential; control
	// plane writes are owned by an account, so API keys and workspace tokens
	// (which carry no account id) cannot create endpoints.
	regReq := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(`{
		"email": "creator@example.com",
		"password": "correct-horse-battery-staple"
	}`))
	regReq.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(regReq)
	if err != nil {
		t.Fatalf("account registration failed: %v", err)
	}
	regBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201 on registration, got %d (body %q)", resp.StatusCode, regBody)
	}
	var regEnvelope struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(regBody, &regEnvelope); err != nil {
		t.Fatalf("decode registration response: %v", err)
	}
	accountJWT := regEnvelope.Data.Token
	if accountJWT == "" {
		t.Fatal("expected an account token from registration")
	}

	// Create a private mock endpoint using the account-scoped JWT.
	req := httptest.NewRequest("POST", "/api/v1/endpoints", strings.NewReader(`{
		"method": "get",
		"path": "/secret",
		"prompt": "a private endpoint",
		"public": false
	}`))
	req.Header.Set("Authorization", "Bearer "+accountJWT)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("create with account JWT failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201 with account JWT, got %d", resp.StatusCode)
	}

	// API-key and workspace-token identities carry no account and therefore
	// cannot create endpoints even though they are valid credentials.
	for name, header := range map[string]string{"X-API-Key": "sk_test_123", "X-Workspace-Token": "tok_acme_456"} {
		noreq := httptest.NewRequest("POST", "/api/v1/endpoints", strings.NewReader(`{
			"method": "get",
			"path": "/no-owner",
			"prompt": "cannot be owned",
			"public": false
		}`))
		noreq.Header.Set(header, "v")
		resp, err = app.Test(noreq)
		if err != nil {
			t.Fatalf("create with %s failed: %v", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("expected 401 creating with %s (no account), got %d", name, resp.StatusCode)
		}
	}

	// The private endpoint rejects anonymous requests but accepts every
	// supported credential kind for serving.
	anonymous := httptest.NewRequest("GET", "/secret", nil)
	resp, err = app.Test(anonymous)
	if err != nil {
		t.Fatalf("GET /secret anonymous failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401 on private endpoint, got %d", resp.StatusCode)
	}

	withKey := httptest.NewRequest("GET", "/secret", nil)
	withKey.Header.Set("Authorization", "Bearer sk_test_123")
	if resp, err = app.Test(withKey); err != nil {
		t.Fatalf("GET /secret with key failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 with API key, got %d", resp.StatusCode)
	}

	withWorkspace := httptest.NewRequest("GET", "/secret", nil)
	withWorkspace.Header.Set("X-Workspace-Token", "tok_acme_456")
	if resp, err = app.Test(withWorkspace); err != nil {
		t.Fatalf("GET /secret with workspace token failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 with workspace token, got %d", resp.StatusCode)
	}

	// Mint a JWT from the API key and use it against both the control plane
	// and the private mock endpoint.
	mintReq := httptest.NewRequest("POST", "/api/v1/auth/token", strings.NewReader(`{"api_key":"sk_test_123"}`))
	mintReq.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(mintReq)
	if err != nil {
		t.Fatalf("token minting failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on token minting, got %d (body %q)", resp.StatusCode, body)
	}
	var minted struct {
		Data struct {
			Token string `json:"token"`
			Kind  string `json:"kind"`
			Name  string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &minted); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if minted.Data.Token == "" || minted.Data.Kind != auth.KindAPIKey || minted.Data.Name != "dev" {
		t.Errorf("unexpected minted identity: %+v", minted.Data)
	}

	whoami := httptest.NewRequest("GET", "/api/v1/auth/whoami", nil)
	whoami.Header.Set("Authorization", "Bearer "+minted.Data.Token)
	resp, err = app.Test(whoami)
	if err != nil {
		t.Fatalf("whoami failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on whoami, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), auth.KindJWT) || !strings.Contains(string(body), "dev") {
		t.Errorf("expected JWT identity in whoami, got %q", body)
	}

	withJWT := httptest.NewRequest("GET", "/secret", nil)
	withJWT.Header.Set("Authorization", "Bearer "+minted.Data.Token)
	if resp, err = app.Test(withJWT); err != nil {
		t.Fatalf("GET /secret with JWT failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on private endpoint with JWT, got %d", resp.StatusCode)
	}

	// A public endpoint is served without any credential. public defaults to
	// true when omitted.
	req = httptest.NewRequest("POST", "/api/v1/endpoints", strings.NewReader(`{
		"method": "get",
		"path": "/open",
		"prompt": "a public endpoint"
	}`))
	req.Header.Set("Authorization", "Bearer "+accountJWT)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("create public endpoint failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201 on public endpoint create, got %d", resp.StatusCode)
	}

	anonymous = httptest.NewRequest("GET", "/open", nil)
	resp, err = app.Test(anonymous)
	if err != nil {
		t.Fatalf("GET /open failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on public endpoint, got %d", resp.StatusCode)
	}
}

func TestDynamicHandlerEnforcesEndpointPrivacy(t *testing.T) {
	logger := zerolog.Nop()
	open := newEndpoint("GET", "/open")
	open.Public = true
	private := newEndpoint("GET", "/secret")
	private.Public = false
	store := &fakeStore{
		endpoints: []models.Endpoint{
			open,
			private,
		},
	}

	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	dyn := NewDynamicHandler(store, generator.NewStatic(), validator.New(), cfg, &logger)
	app := fiber.New()
	app.Use(dyn.Serve)

	resp, err := app.Test(httptest.NewRequest("GET", "/open", nil))
	if err != nil {
		t.Fatalf("GET /open failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200 on public endpoint, got %d", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest("GET", "/secret", nil))
	if err != nil {
		t.Fatalf("GET /secret failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 on private endpoint, got %d", resp.StatusCode)
	}

	// With auth disabled the same private endpoint is served openly.
	cfgDisabled := &config.Config{}
	cfgDisabled.Auth.Enabled = false
	dynDisabled := NewDynamicHandler(store, generator.NewStatic(), validator.New(), cfgDisabled, &logger)
	appDisabled := fiber.New()
	appDisabled.Use(dynDisabled.Serve)
	resp, err = appDisabled.Test(httptest.NewRequest("GET", "/secret", nil))
	if err != nil {
		t.Fatalf("GET /secret failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200 on private endpoint when auth disabled, got %d", resp.StatusCode)
	}
}
