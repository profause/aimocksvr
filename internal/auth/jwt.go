package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// jwtHeader is the fixed JOSE header for the HS256 tokens issued by this
// server. JWTs are intentionally implemented with the standard library only:
// the server signs and validates HMAC-SHA256 tokens against a shared secret,
// which keeps the dependency tree small for a mock tool.
const jwtHeader = `{"alg":"HS256","typ":"JWT"}`

// jwtClaims is the subset of registered and custom JWT claims the server
// signs and validates.
type jwtClaims struct {
	Issuer   string `json:"iss"`
	Audience string `json:"aud"`
	Subject  string `json:"sub"`
	Kind     string `json:"kind,omitempty"`
	Email    string `json:"email,omitempty"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

// signJWT returns a compact HS256 JWT for the given claims.
func signJWT(claims jwtClaims, secret string) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal jwt claims: %w", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString([]byte(jwtHeader)) + "." + base64.RawURLEncoding.EncodeToString(payload)
	sig, err := hmacSHA256(secret, signingInput)
	if err != nil {
		return "", err
	}
	return signingInput + "." + sig, nil
}

// verifyJWT checks the signature, issuer, audience and expiry of a compact
// HS256 JWT.
func verifyJWT(token, secret, expectedIssuer, expectedAudience string, now time.Time) (*jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed jwt")
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("malformed jwt signature")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, errors.New("invalid jwt signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("malformed jwt payload")
	}
	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.New("malformed jwt payload")
	}

	if claims.Issuer != expectedIssuer {
		return nil, fmt.Errorf("unexpected jwt issuer %q", claims.Issuer)
	}
	if claims.Audience != expectedAudience {
		return nil, fmt.Errorf("unexpected jwt audience %q", claims.Audience)
	}
	if claims.Expires > 0 && now.Unix() >= claims.Expires {
		return nil, errors.New("jwt expired")
	}
	return &claims, nil
}

// hmacSHA256 computes the base64url-encoded HMAC-SHA256 signature of input
// under the given secret.
func hmacSHA256(secret, input string) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(input)); err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
