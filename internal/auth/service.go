// Package auth provides credential-based authentication for the mock server.
//
// Three credential kinds are supported: long-lived API keys, workspace-scoped
// tokens, and shared-secret HS256 JWTs. API keys and workspace tokens are
// configured via environment (MOCKSVR_AUTH_API_KEYS, MOCKSVR_AUTH_WORKSPACE_TOKENS)
// and stored only as SHA-256 hashes; JWTs are validated against the configured
// issuer and audience. When auth is disabled the server behaves as an open
// mock, which is the default.
package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/config"
)

const (
	KindAPIKey         = "api_key"
	KindWorkspaceToken = "workspace_token"
	KindJWT            = "jwt"
)

// Identity describes an authenticated caller.
type Identity struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// ErrUnauthorized is returned when a credential does not match anything the
// server knows.
var ErrUnauthorized = errors.New("unauthorized")

// Service authenticates credentials and mints JWTs.
type Service struct {
	cfg       *config.Config
	apiKeys   map[string]string
	workspace map[string]string
	jwtSecret string
	log       *zerolog.Logger
}

// NewService builds the auth Service from configuration. Credential secrets
// are hashed immediately and never kept in plain text.
func NewService(cfg *config.Config, logger *zerolog.Logger) *Service {
	s := &Service{
		cfg:       cfg,
		apiKeys:   make(map[string]string),
		workspace: make(map[string]string),
		jwtSecret: cfg.Auth.JWTSecret,
		log:       logger,
	}

	for name, secret := range parseCredentials(cfg.Auth.APIKeys) {
		s.apiKeys[hashToken(secret)] = name
	}
	for name, secret := range parseCredentials(cfg.Auth.WorkspaceTokens) {
		s.workspace[hashToken(secret)] = name
	}

	if !cfg.Auth.Enabled {
		logger.Info().Msg("authentication disabled; all endpoints are open")
	} else {
		if cfg.Auth.JWTSecret == "" {
			logger.Warn().Msg("auth enabled but jwt_secret is empty; JWT validation and minting disabled")
		}
		if len(s.apiKeys) == 0 && len(s.workspace) == 0 {
			logger.Warn().Msg("auth enabled but no api_keys or workspace_tokens configured")
		}
	}
	return s
}

// Authenticate resolves a raw credential against the API key store, the
// workspace token store and finally the JWT validator, in that order.
func (s *Service) Authenticate(token string) (Identity, error) {
	if strings.TrimSpace(token) == "" {
		return Identity{}, ErrUnauthorized
	}

	if name, ok := s.apiKeys[hashToken(token)]; ok {
		return Identity{Kind: KindAPIKey, Name: name}, nil
	}
	if name, ok := s.workspace[hashToken(token)]; ok {
		return Identity{Kind: KindWorkspaceToken, Name: name}, nil
	}
	if s.jwtSecret != "" {
		if claims, err := verifyJWT(token, s.jwtSecret, s.cfg.Auth.JWTIssuer, s.cfg.Auth.JWTAudience, time.Now().UTC()); err == nil {
			return Identity{Kind: KindJWT, Name: claims.Subject}, nil
		}
	}
	return Identity{}, ErrUnauthorized
}

// MintJWT signs a short-lived JWT for the given identity. It fails when no
// JWT secret is configured.
func (s *Service) MintJWT(id Identity) (string, error) {
	if s.jwtSecret == "" {
		return "", errors.New("jwt secret not configured")
	}
	ttl, err := time.ParseDuration(s.cfg.Auth.JWTTTL)
	if err != nil {
		return "", fmt.Errorf("parse jwt_ttl %q: %w", s.cfg.Auth.JWTTTL, err)
	}

	now := time.Now().UTC()
	return signJWT(jwtClaims{
		Issuer:   s.cfg.Auth.JWTIssuer,
		Audience: s.cfg.Auth.JWTAudience,
		Subject:  id.Name,
		Kind:     id.Kind,
		IssuedAt: now.Unix(),
		Expires:  now.Add(ttl).Unix(),
	}, s.jwtSecret)
}

// parseCredentials splits a comma-separated "name:secret" list into a map.
// Malformed entries are skipped so a single bad value cannot disable auth.
func parseCredentials(raw string) map[string]string {
	out := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		idx := strings.Index(entry, ":")
		if idx <= 0 || idx == len(entry)-1 {
			continue
		}
		name := strings.TrimSpace(entry[:idx])
		secret := strings.TrimSpace(entry[idx+1:])
		if name != "" && secret != "" {
			out[name] = secret
		}
	}
	return out
}

// hashToken returns the lowercase hex SHA-256 of a credential secret.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
