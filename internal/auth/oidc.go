// Package auth contains authentication adapters used by the network gateway.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// OIDCConfig identifies the Keycloak realm and the audience accepted by the
// gateway. The issuer's discovery document supplies the JWKS endpoint.
type OIDCConfig struct {
	IssuerURL string
	Audience  string
}

// Verifier validates bearer JWTs signed by the configured OIDC issuer.
type Verifier struct {
	verifier *oidc.IDTokenVerifier
}

// NewOIDCVerifier discovers the issuer's metadata and constructs a verifier
// that validates issuer, signature, expiry, and audience. Discovery happens at
// startup so a misconfigured or unreachable Keycloak realm fails closed before
// the HTTP listener accepts requests.
func NewOIDCVerifier(ctx context.Context, cfg OIDCConfig) (*Verifier, error) {
	if ctx == nil {
		return nil, fmt.Errorf("create OIDC verifier: context is nil")
	}
	issuerURL := strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	if issuerURL == "" {
		return nil, fmt.Errorf("create OIDC verifier: issuer URL is required")
	}
	audience := strings.TrimSpace(cfg.Audience)
	if audience == "" {
		return nil, fmt.Errorf("create OIDC verifier: audience is required")
	}

	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("create OIDC verifier: discover issuer %q: %w", issuerURL, err)
	}
	return newVerifier(provider.Verifier(&oidc.Config{ClientID: audience})), nil
}

func newVerifier(verifier *oidc.IDTokenVerifier) *Verifier {
	if verifier == nil {
		panic("create OIDC verifier: nil token verifier")
	}
	return &Verifier{verifier: verifier}
}

// Verify implements the MCP SDK bearer-token verifier contract. Keycloak's
// access-token scope claim is copied into TokenInfo so the SDK can enforce
// required scopes, while selected identity claims remain available for future
// authorization and audit logging.
func (v *Verifier) Verify(ctx context.Context, rawToken string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	if v == nil || v.verifier == nil {
		return nil, fmt.Errorf("%w: verifier is not initialized", mcpauth.ErrInvalidToken)
	}
	token, err := v.verifier.Verify(ctx, strings.TrimSpace(rawToken))
	if err != nil {
		return nil, fmt.Errorf("%w: verify bearer token: %v", mcpauth.ErrInvalidToken, err)
	}

	var claims struct {
		Scope             string   `json:"scope"`
		Scopes            []string `json:"scp"`
		PreferredUsername string   `json:"preferred_username"`
		Email             string   `json:"email"`
		Name              string   `json:"name"`
	}
	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("%w: read bearer claims: %v", mcpauth.ErrInvalidToken, err)
	}

	info := &mcpauth.TokenInfo{
		Scopes:     uniqueStrings(append(strings.Fields(claims.Scope), claims.Scopes...)),
		Expiration: token.Expiry,
		UserID:     token.Subject,
		Extra:      make(map[string]any),
	}
	if claims.PreferredUsername != "" {
		info.Extra["preferred_username"] = claims.PreferredUsername
	}
	if claims.Email != "" {
		info.Extra["email"] = claims.Email
	}
	if claims.Name != "" {
		info.Extra["name"] = claims.Name
	}
	return info, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
