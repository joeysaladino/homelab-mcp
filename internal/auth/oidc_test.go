package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestVerifierVerify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	issuer := "https://keycloak.example.test/realms/homelab"
	verifier := newVerifier(oidc.NewVerifier(issuer, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{ClientID: "homelab-mcp"}))

	token := signJWT(t, key, map[string]any{
		"iss":                issuer,
		"sub":                "user-123",
		"aud":                "homelab-mcp",
		"exp":                time.Now().Add(5 * time.Minute).Unix(),
		"iat":                time.Now().Add(-time.Minute).Unix(),
		"scope":              "mcp.read mcp.write mcp.read",
		"scp":                []string{"mcp.audit"},
		"preferred_username": "joey",
		"email":              "joey@example.test",
	})

	info, err := verifier.Verify(context.Background(), token, &http.Request{})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if info.UserID != "user-123" || info.Expiration.Before(time.Now()) {
		t.Fatalf("TokenInfo identity/expiry = %+v, want subject and future expiry", info)
	}
	if strings.Join(info.Scopes, " ") != "mcp.read mcp.write mcp.audit" {
		t.Errorf("Scopes = %v, want deduplicated scope claims", info.Scopes)
	}
	if info.Extra["preferred_username"] != "joey" || info.Extra["email"] != "joey@example.test" {
		t.Errorf("Extra = %+v, want selected identity claims", info.Extra)
	}
}

func TestVerifierRejectsWrongAudience(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	issuer := "https://keycloak.example.test/realms/homelab"
	verifier := newVerifier(oidc.NewVerifier(issuer, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{ClientID: "homelab-mcp"}))
	token := signJWT(t, key, map[string]any{
		"iss": issuer,
		"sub": "user-123",
		"aud": "different-client",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	})

	_, err = verifier.Verify(context.Background(), token, nil)
	if err == nil || !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("Verify() error = %v, want ErrInvalidToken", err)
	}
}

func TestNewOIDCVerifierValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  OIDCConfig
		want string
	}{
		{name: "missing issuer", cfg: OIDCConfig{Audience: "client"}, want: "issuer URL is required"},
		{name: "missing audience", cfg: OIDCConfig{IssuerURL: "https://issuer.example.test", Audience: " "}, want: "audience is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOIDCVerifier(context.Background(), tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewOIDCVerifier() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestNewOIDCVerifierDiscoversAndVerifies(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	var provider *httptest.Server
	provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                                provider.URL,
				"jwks_uri":                              provider.URL + "/jwks",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]string{{
					"kty": "RSA",
					"kid": "test-key",
					"use": "sig",
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()

	verifier, err := NewOIDCVerifier(context.Background(), OIDCConfig{
		IssuerURL: provider.URL,
		Audience:  "homelab-mcp",
	})
	if err != nil {
		t.Fatalf("NewOIDCVerifier() error = %v", err)
	}
	token := signJWT(t, key, map[string]any{
		"iss": provider.URL,
		"sub": "discovered-user",
		"aud": "homelab-mcp",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	})
	info, err := verifier.Verify(context.Background(), token, nil)
	if err != nil {
		t.Fatalf("Verify() after discovery error = %v", err)
	}
	if info.UserID != "discovered-user" {
		t.Errorf("UserID = %q, want discovered-user", info.UserID)
	}
}

func signJWT(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header := encodeJSON(t, map[string]string{"alg": "RS256", "kid": "test-key", "typ": "JWT"})
	payload := encodeJSON(t, claims)
	unsigned := header + "." + payload
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15() error = %v", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encodeJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
