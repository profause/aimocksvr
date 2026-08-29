package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/config"
)

func TestJWTVerifyRoundTrip(t *testing.T) {
	token, err := signJWT(jwtClaims{
		Issuer:   "mocksvr",
		Audience: "mocksvr",
		Subject:  "dev",
		Kind:     KindAPIKey,
		IssuedAt: time.Now().UTC().Unix(),
		Expires:  time.Now().UTC().Add(time.Hour).Unix(),
	}, "secret")
	if err != nil {
		t.Fatalf("signJWT failed: %v", err)
	}

	claims, err := verifyJWT(token, "secret", "mocksvr", "mocksvr", time.Now().UTC())
	if err != nil {
		t.Fatalf("verifyJWT failed: %v", err)
	}
	if claims.Subject != "dev" || claims.Kind != KindAPIKey {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestJWTVerifyRejectsTamperedSignature(t *testing.T) {
	token, err := signJWT(jwtClaims{
		Issuer: "mocksvr", Audience: "mocksvr", Subject: "dev",
		IssuedAt: time.Now().Unix(), Expires: time.Now().Add(time.Hour).Unix(),
	}, "secret")
	if err != nil {
		t.Fatalf("signJWT failed: %v", err)
	}

	if _, err := verifyJWT(token+"x", "secret", "mocksvr", "mocksvr", time.Now().UTC()); err == nil {
		t.Error("expected tampered signature to fail")
	}
	if _, err := verifyJWT(token, "wrong-secret", "mocksvr", "mocksvr", time.Now().UTC()); err == nil {
		t.Error("expected wrong secret to fail")
	}
}

func TestJWTVerifyRejectsWrongIssuerAndAudience(t *testing.T) {
	token, err := signJWT(jwtClaims{
		Issuer: "mocksvr", Audience: "mocksvr", Subject: "dev",
		IssuedAt: time.Now().Unix(), Expires: time.Now().Add(time.Hour).Unix(),
	}, "secret")
	if err != nil {
		t.Fatalf("signJWT failed: %v", err)
	}

	if _, err := verifyJWT(token, "secret", "other", "mocksvr", time.Now().UTC()); err == nil {
		t.Error("expected wrong issuer to fail")
	}
	if _, err := verifyJWT(token, "secret", "mocksvr", "other", time.Now().UTC()); err == nil {
		t.Error("expected wrong audience to fail")
	}
}

func TestJWTVerifyRejectsExpired(t *testing.T) {
	token, err := signJWT(jwtClaims{
		Issuer: "mocksvr", Audience: "mocksvr", Subject: "dev",
		IssuedAt: time.Now().Add(-2 * time.Hour).Unix(), Expires: time.Now().Add(-time.Hour).Unix(),
	}, "secret")
	if err != nil {
		t.Fatalf("signJWT failed: %v", err)
	}

	if _, err := verifyJWT(token, "secret", "mocksvr", "mocksvr", time.Now().UTC()); err == nil {
		t.Error("expected expired token to fail")
	}
}

func TestJWTVerifyRejectsMalformed(t *testing.T) {
	for _, token := range []string{"", "no.dots", "a.b", "a.b.c.d", "a..c"} {
		if _, err := verifyJWT(token, "secret", "mocksvr", "mocksvr", time.Now().UTC()); err == nil {
			t.Errorf("expected malformed token %q to fail", token)
		}
	}
}

func TestParseCredentials(t *testing.T) {
	got := parseCredentials("dev:sk_test_123, ci:sk_ci_456, ,bad,bad:,")
	if len(got) != 2 {
		t.Fatalf("expected 2 credentials, got %v", got)
	}
	if got["dev"] != "sk_test_123" || got["ci"] != "sk_ci_456" {
		t.Errorf("unexpected credentials: %v", got)
	}
}

func newTestService(apiKeys, workspaceTokens, jwtSecret string, enabled bool) *Service {
	logger := zerolog.Nop()
	cfg := &config.Config{Auth: config.Auth{
		Enabled:         enabled,
		JWTSecret:       jwtSecret,
		JWTIssuer:       "mocksvr",
		JWTAudience:     "mocksvr",
		JWTTTL:          "1h",
		APIKeys:         apiKeys,
		WorkspaceTokens: workspaceTokens,
	}}
	return NewService(cfg, &logger)
}

func TestServiceAuthenticatesAPIKey(t *testing.T) {
	svc := newTestService("dev:sk_test_123", "", "", true)

	id, err := svc.Authenticate("sk_test_123")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if id.Kind != KindAPIKey || id.Name != "dev" {
		t.Errorf("unexpected identity: %+v", id)
	}
}

func TestServiceAuthenticatesWorkspaceToken(t *testing.T) {
	svc := newTestService("", "acme:tok_acme_456", "", true)

	id, err := svc.Authenticate("tok_acme_456")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if id.Kind != KindWorkspaceToken || id.Name != "acme" {
		t.Errorf("unexpected identity: %+v", id)
	}
}

func TestServiceAuthenticatesJWT(t *testing.T) {
	svc := newTestService("", "", "secret", true)

	token, err := svc.MintJWT(Identity{Kind: KindAPIKey, Name: "dev"})
	if err != nil {
		t.Fatalf("MintJWT failed: %v", err)
	}

	id, err := svc.Authenticate(token)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if id.Kind != KindJWT || id.Name != "dev" {
		t.Errorf("unexpected identity: %+v", id)
	}
}

func TestServiceAuthenticatesAccountJWT(t *testing.T) {
	svc := newTestService("", "", "secret", true)
	acctID := uuid.New()

	token, err := svc.MintAccountJWT(acctID, "user@example.com")
	if err != nil {
		t.Fatalf("MintAccountJWT failed: %v", err)
	}

	id, err := svc.Authenticate(token)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if id.Kind != KindAccount {
		t.Errorf("expected kind %q, got %q", KindAccount, id.Kind)
	}
	if id.AccountID != acctID {
		t.Errorf("expected account id %s, got %s", acctID, id.AccountID)
	}
	if id.Name != "" {
		t.Errorf("expected no name on an account identity, got %q", id.Name)
	}
	if id.Email != "user@example.com" {
		t.Errorf("expected email claim to round-trip, got %q", id.Email)
	}
}

func TestServiceMintJWTRejectsAccountIdentity(t *testing.T) {
	svc := newTestService("", "", "secret", true)

	if _, err := svc.MintJWT(Identity{Kind: KindAccount, AccountID: uuid.New(), Email: "user@example.com"}); err == nil {
		t.Error("expected MintJWT to reject an account identity")
	}
}

func TestServiceLegacyJWTHasNoAccount(t *testing.T) {
	svc := newTestService("dev:sk_test_123", "", "secret", true)

	token, err := svc.MintJWT(Identity{Kind: KindAPIKey, Name: "dev"})
	if err != nil {
		t.Fatalf("MintJWT failed: %v", err)
	}
	id, err := svc.Authenticate(token)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if id.Kind != KindJWT || id.Name != "dev" || id.AccountID != uuid.Nil {
		t.Errorf("legacy JWT identity changed: %+v", id)
	}
}

func TestServiceAccountJWTRequiresUUIDSubject(t *testing.T) {
	svc := newTestService("", "", "secret", true)

	now := time.Now().UTC()
	token, err := signJWT(jwtClaims{
		Issuer:   "mocksvr",
		Audience: "mocksvr",
		Subject:  "not-a-uuid",
		Kind:     KindAccount,
		IssuedAt: now.Unix(),
		Expires:  now.Add(time.Hour).Unix(),
	}, "secret")
	if err != nil {
		t.Fatalf("signJWT failed: %v", err)
	}

	if _, err := svc.Authenticate(token); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for non-UUID account subject, got %v", err)
	}
}

func TestServiceMintAccountJWTWithoutSecret(t *testing.T) {
	svc := newTestService("", "", "", true)

	if _, err := svc.MintAccountJWT(uuid.New(), "user@example.com"); err == nil {
		t.Error("expected MintAccountJWT to fail without a secret")
	}
}

func TestServiceAuthenticateRejectsUnknown(t *testing.T) {
	svc := newTestService("dev:sk_test_123", "acme:tok_acme_456", "secret", true)

	for _, token := range []string{"", "nope", "sk_other", "tok_other"} {
		if _, err := svc.Authenticate(token); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized for %q, got %v", token, err)
		}
	}
}

func TestServiceJWTDisabledWithoutSecret(t *testing.T) {
	svc := newTestService("dev:sk_test_123", "", "", true)

	if _, err := svc.MintJWT(Identity{Kind: KindAPIKey, Name: "dev"}); err == nil {
		t.Error("expected MintJWT to fail without a secret")
	}

	// JWTs cannot be validated without a secret, but API keys still work.
	if _, err := svc.Authenticate("sk_test_123"); err != nil {
		t.Errorf("API key should still authenticate: %v", err)
	}
}

func TestServiceCredentialHashNotStoredPlainText(t *testing.T) {
	svc := newTestService("dev:sk_test_123", "", "", true)

	for _, stored := range svc.apiKeys {
		if stored == "sk_test_123" {
			t.Error("API key secret must not be stored in plain text")
		}
	}
}
