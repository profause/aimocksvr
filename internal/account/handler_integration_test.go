package account

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/auth"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/database"
)

type appFixture struct {
	app  *fiber.App
	auth *auth.Service
}

// newTestApp connects to PostgreSQL and builds the control-plane app seeded
// with the real account and auth handlers. The test is skipped when
// MOCKSVR_TEST_DATABASE_URL is not set.
func newTestApp(t *testing.T) *appFixture {
	t.Helper()

	url := os.Getenv("MOCKSVR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("MOCKSVR_TEST_DATABASE_URL not set; skipping account integration test")
	}

	cfg := &config.Config{Database: config.Database{URL: url}}
	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		"DROP TABLE IF EXISTS accounts, mock_resources, request_history, endpoint_versions, endpoints, schema_migrations CASCADE"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	if err := database.Migrate(url); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	cfg = &config.Config{Database: config.Database{URL: url}, Auth: config.Auth{
		Enabled:         true,
		JWTSecret:       "secret",
		JWTIssuer:       "mocksvr",
		JWTAudience:     "mocksvr",
		JWTTTL:          "1h",
		APIKeys:         "dev:sk_test_123",
		WorkspaceTokens: "acme:tok_acme_456",
	}}
	logger := zerolog.Nop()

	authSvc := auth.NewService(cfg, &logger)
	authH := auth.NewHandler(cfg, authSvc, &logger)

	svc := NewService(NewRepository(db), authSvc)
	h := NewHandler(svc, &logger)

	app := fiber.New()
	app.Use(authH.Middleware())
	apiGroup := app.Group("/api/v1")
	authH.Register(apiGroup)
	h.Register(apiGroup)

	return &appFixture{app: app, auth: authSvc}
}

// doReq performs an HTTP call against the in-process app.
func (f *appFixture) doReq(t *testing.T, method, path, body string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.app.Test(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, b
}

func TestAccountHTTPRegister(t *testing.T) {
	f := newTestApp(t)

	// Valid registration returns 201, canonical email and a JWT.
	resp, body := f.doReq(t, "POST", "/api/v1/auth/register", `{"email":"  New@Example.COM  ","password":"password123"}`, nil)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d (body %q)", resp.StatusCode, body)
	}
	var got struct {
		Data struct {
			Account accountDTO `json:"account"`
			Token   string     `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if got.Data.Account.ID == uuid.Nil {
		t.Error("expected a registered account id")
	}
	if got.Data.Account.Email != "new@example.com" {
		t.Errorf("expected canonical email, got %q", got.Data.Account.Email)
	}
	if got.Data.Token == "" {
		t.Error("expected a JWT")
	}

	// Duplicate email -> 409, message never distinguishes the two.
	resp, body = f.doReq(t, "POST", "/api/v1/auth/register", `{"email":"NEW@example.com","password":"password456"}`, nil)
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("expected 409 for duplicate email, got %d (body %q)", resp.StatusCode, body)
	}

	// Bad input -> 400 validation_error.
	for name, payload := range map[string]string{
		"malformed json":   `{`,
		"invalid email":    `{"email":"nope","password":"password123"}`,
		"short password":   `{"email":"x@example.com","password":"short"}`,
		"missing password": `{"email":"x@example.com"}`,
	} {
		resp, body := f.doReq(t, "POST", "/api/v1/auth/register", payload, nil)
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d (body %q)", name, resp.StatusCode, body)
		}
	}
}

func TestAccountHTTPLogin(t *testing.T) {
	f := newTestApp(t)

	// Seed an account through the public register route.
	f.doReq(t, "POST", "/api/v1/auth/register", `{"email":"user@example.com","password":"password123"}`, nil)

	// Correct credentials -> 200 with an account-bound JWT.
	resp, body := f.doReq(t, "POST", "/api/v1/auth/login", `{"email":" user@example.com ","password":"password123"}`, nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d (body %q)", resp.StatusCode, body)
	}
	var got struct {
		Data struct {
			Account accountDTO `json:"account"`
			Token   string     `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if got.Data.Account.Email != "user@example.com" || got.Data.Token == "" {
		t.Errorf("unexpected login result: %+v", got.Data)
	}

	// Wrong password and unknown email both -> 401, indistinguishably generic.
	for name, payload := range map[string]string{
		"wrong password": `{"email":"user@example.com","password":"wrong-password"}`,
		"unknown email":  `{"email":"nobody@example.com","password":"password123"}`,
	} {
		resp, body := f.doReq(t, "POST", "/api/v1/auth/login", payload, nil)
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d (body %q)", name, resp.StatusCode, body)
		}
	}

	// Empty required fields -> 400.
	resp, body = f.doReq(t, "POST", "/api/v1/auth/login", `{"email":"user@example.com"}`, nil)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("missing password: expected 400, got %d (body %q)", resp.StatusCode, body)
	}
}

func TestAccountJWTFlow(t *testing.T) {
	f := newTestApp(t)

	// Register, then prove the returned token authenticates as the account.
	resp, body := f.doReq(t, "POST", "/api/v1/auth/register", `{"email":"flow@example.com","password":"password123"}`, nil)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d (body %q)", resp.StatusCode, body)
	}
	var got struct {
		Data struct {
			Account accountDTO `json:"account"`
			Token   string     `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	id, err := f.auth.Authenticate(got.Data.Token)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if id.Kind != auth.KindAccount {
		t.Errorf("expected kind %q, got %q", auth.KindAccount, id.Kind)
	}
	if id.AccountID != got.Data.Account.ID {
		t.Errorf("token subject %s != registered account %s", id.AccountID, got.Data.Account.ID)
	}

	// The legacy API-key flow still mints a legacy JWT bearing no account.
	resp, body = f.doReq(t, "POST", "/api/v1/auth/token", `{"api_key":"sk_test_123"}`, nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected legacy 200, got %d (body %q)", resp.StatusCode, body)
	}
	var tok struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	id, err = f.auth.Authenticate(tok.Data.Token)
	if err != nil {
		t.Fatalf("Authenticate legacy JWT failed: %v", err)
	}
	if id.Kind != auth.KindJWT || id.Name != "dev" || id.AccountID != uuid.Nil {
		t.Errorf("legacy JWT no longer behaves as before: %+v", id)
	}
}
