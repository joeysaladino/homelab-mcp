// Package config loads and validates homelab-mcp configuration.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// DefaultPantryFile is the repository-relative pantry path used when
// PANTRY_FILE is not set.
const DefaultPantryFile = "config/pantry.yaml"

// DefaultShoppingFile is the repository-relative shopping-trip path used when
// SHOPPING_FILE is not set.
const DefaultShoppingFile = "config/shopping.yaml"

// Transport selects how the MCP server accepts client connections.
type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportHTTP  Transport = "http"
)

// AuthMode selects the authentication policy for network transport.
type AuthMode string

const (
	AuthNone AuthMode = "none"
	AuthOIDC AuthMode = "oidc"
)

const (
	// DefaultTransport keeps local development compatible with MCP clients
	// that launch the server as a subprocess.
	DefaultTransport = TransportStdio
	// DefaultHTTPAddr is suitable for a container behind a reverse proxy.
	DefaultHTTPAddr = ":8080"
)

// Config contains configuration for the application.
type Config struct {
	Mealie        MealieConfig
	PantryFile    string
	ShoppingFile  string
	Transport     Transport
	HTTPAddr      string
	AuthMode      AuthMode
	OIDCIssuerURL string
	OIDCAudience  string
	OIDCScopes    []string
	PublicURL     string
	MetadataURL   string
}

// MealieConfig contains the connection details for Mealie.
//
// The token is intentionally kept private so it is not included if this
// configuration is later marshaled with encoding/json.
type MealieConfig struct {
	BaseURL string
	token   string
}

// Token returns the server-side token used for Mealie API requests.
func (c MealieConfig) Token() string {
	return c.token
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return LoadFromEnv(os.LookupEnv)
}

// LoadFromEnv is the testable form of Load. Keeping environment lookup at the
// boundary means the rest of the application can work with typed values.
func LoadFromEnv(lookup func(string) (string, bool)) (Config, error) {
	if lookup == nil {
		return Config{}, fmt.Errorf("load config: environment lookup is nil")
	}

	rawURL, ok := lookup("MEALIE_URL")
	if !ok || strings.TrimSpace(rawURL) == "" {
		return Config{}, fmt.Errorf("load config: MEALIE_URL is required")
	}

	baseURL, err := normalizeBaseURL(rawURL)
	if err != nil {
		return Config{}, fmt.Errorf("load config: MEALIE_URL: %w", err)
	}

	rawToken, ok := lookup("MEALIE_TOKEN")
	if !ok || strings.TrimSpace(rawToken) == "" {
		return Config{}, fmt.Errorf("load config: MEALIE_TOKEN is required")
	}

	pantryFile := DefaultPantryFile
	if rawPantryFile, ok := lookup("PANTRY_FILE"); ok && strings.TrimSpace(rawPantryFile) != "" {
		pantryFile = strings.TrimSpace(rawPantryFile)
	}
	shoppingFile := DefaultShoppingFile
	if rawShoppingFile, ok := lookup("SHOPPING_FILE"); ok && strings.TrimSpace(rawShoppingFile) != "" {
		shoppingFile = strings.TrimSpace(rawShoppingFile)
	}

	transport := DefaultTransport
	if rawTransport, ok := lookup("MCP_TRANSPORT"); ok && strings.TrimSpace(rawTransport) != "" {
		transport = Transport(strings.ToLower(strings.TrimSpace(rawTransport)))
	}
	if transport != TransportStdio && transport != TransportHTTP {
		return Config{}, fmt.Errorf("load config: MCP_TRANSPORT must be %q or %q", TransportStdio, TransportHTTP)
	}

	httpAddr := DefaultHTTPAddr
	if rawHTTPAddr, ok := lookup("MCP_HTTP_ADDR"); ok && strings.TrimSpace(rawHTTPAddr) != "" {
		httpAddr = strings.TrimSpace(rawHTTPAddr)
	}

	authMode := AuthNone
	rawAuthMode, authModeSet := lookup("MCP_AUTH_MODE")
	if authModeSet && strings.TrimSpace(rawAuthMode) != "" {
		authMode = AuthMode(strings.ToLower(strings.TrimSpace(rawAuthMode)))
	} else if transport == TransportHTTP {
		// Network transport fails closed unless local development explicitly opts
		// out. Stdio has no network listener and therefore needs no auth mode.
		authMode = AuthOIDC
	}
	if authMode != AuthNone && authMode != AuthOIDC {
		return Config{}, fmt.Errorf("load config: MCP_AUTH_MODE must be %q or %q", AuthNone, AuthOIDC)
	}

	var issuerURL, audience, publicURL, metadataURL string
	var oidcScopes []string
	if authMode == AuthOIDC {
		var err error
		issuerURL, err = requiredConfiguredURL(lookup, "OIDC_ISSUER_URL")
		if err != nil {
			return Config{}, err
		}
		audience, err = requiredConfiguredValue(lookup, "OIDC_AUDIENCE")
		if err != nil {
			return Config{}, err
		}
		publicURL, err = requiredConfiguredURL(lookup, "MCP_PUBLIC_URL")
		if err != nil {
			return Config{}, err
		}
		metadataURL, err = requiredConfiguredURL(lookup, "MCP_RESOURCE_METADATA_URL")
		if err != nil {
			return Config{}, err
		}
		if rawScopes, ok := lookup("OIDC_REQUIRED_SCOPES"); ok {
			oidcScopes = strings.Fields(rawScopes)
		}
	}

	return Config{
		Mealie: MealieConfig{
			BaseURL: baseURL,
			token:   strings.TrimSpace(rawToken),
		},
		PantryFile:    pantryFile,
		ShoppingFile:  shoppingFile,
		Transport:     transport,
		HTTPAddr:      httpAddr,
		AuthMode:      authMode,
		OIDCIssuerURL: issuerURL,
		OIDCAudience:  audience,
		OIDCScopes:    oidcScopes,
		PublicURL:     publicURL,
		MetadataURL:   metadataURL,
	}, nil
}

func requiredConfiguredValue(lookup func(string) (string, bool), name string) (string, error) {
	raw, ok := lookup(name)
	if !ok || strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("load config: %s is required when OIDC authentication is enabled", name)
	}
	return strings.TrimSpace(raw), nil
}

func requiredConfiguredURL(lookup func(string) (string, bool), name string) (string, error) {
	raw, err := requiredConfiguredValue(lookup, name)
	if err != nil {
		return "", err
	}
	value, err := normalizeBaseURL(raw)
	if err != nil {
		return "", fmt.Errorf("load config: %s: %w", name, err)
	}
	return value, nil
}

func normalizeBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("host is required")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("query and fragment are not allowed")
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}
