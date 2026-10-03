package config

import (
	"strings"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		want      Config
		wantError string
	}{
		{
			name: "valid configuration",
			env: map[string]string{
				"MEALIE_URL":   " https://mealie.example.test/// ",
				"MEALIE_TOKEN": " secret-token ",
			},
			want: Config{
				Mealie: MealieConfig{
					BaseURL: "https://mealie.example.test",
					token:   "secret-token",
				},
				PantryFile:   DefaultPantryFile,
				ShoppingFile: DefaultShoppingFile,
				Transport:    DefaultTransport,
				HTTPAddr:     DefaultHTTPAddr,
				AuthMode:     AuthNone,
			},
		},
		{
			name: "missing URL",
			env: map[string]string{
				"MEALIE_TOKEN": "secret-token",
			},
			wantError: "MEALIE_URL is required",
		},
		{
			name: "blank token",
			env: map[string]string{
				"MEALIE_URL":   "https://mealie.example.test",
				"MEALIE_TOKEN": "   ",
			},
			wantError: "MEALIE_TOKEN is required",
		},
		{
			name: "relative URL",
			env: map[string]string{
				"MEALIE_URL":   "mealie.example.test",
				"MEALIE_TOKEN": "secret-token",
			},
			wantError: "scheme must be http or https",
		},
		{
			name: "URL with query",
			env: map[string]string{
				"MEALIE_URL":   "https://mealie.example.test?foo=bar",
				"MEALIE_TOKEN": "secret-token",
			},
			wantError: "query and fragment are not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadFromEnv(mapLookup(tt.env))

			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("LoadFromEnv() error = %v", err)
				}
				if got.Mealie.BaseURL != tt.want.Mealie.BaseURL {
					t.Errorf("BaseURL = %q, want %q", got.Mealie.BaseURL, tt.want.Mealie.BaseURL)
				}
				if got.Mealie.Token() != tt.want.Mealie.Token() {
					t.Errorf("Token() = %q, want %q", got.Mealie.Token(), tt.want.Mealie.Token())
				}
				if got.PantryFile != tt.want.PantryFile {
					t.Errorf("PantryFile = %q, want %q", got.PantryFile, tt.want.PantryFile)
				}
				if got.ShoppingFile != tt.want.ShoppingFile {
					t.Errorf("ShoppingFile = %q, want %q", got.ShoppingFile, tt.want.ShoppingFile)
				}
				if got.Transport != tt.want.Transport {
					t.Errorf("Transport = %q, want %q", got.Transport, tt.want.Transport)
				}
				if got.HTTPAddr != tt.want.HTTPAddr {
					t.Errorf("HTTPAddr = %q, want %q", got.HTTPAddr, tt.want.HTTPAddr)
				}
				if got.AuthMode != tt.want.AuthMode {
					t.Errorf("AuthMode = %q, want %q", got.AuthMode, tt.want.AuthMode)
				}
				return
			}

			if err == nil {
				t.Fatal("LoadFromEnv() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("LoadFromEnv() error = %q, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestLoadFromEnvPantryFileOverride(t *testing.T) {
	got, err := LoadFromEnv(mapLookup(map[string]string{
		"MEALIE_URL":    "https://mealie.example.test",
		"MEALIE_TOKEN":  "secret-token",
		"PANTRY_FILE":   " /etc/homelab-mcp/pantry.yaml ",
		"SHOPPING_FILE": " /etc/homelab-mcp/shopping.yaml ",
	}))
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if got.PantryFile != "/etc/homelab-mcp/pantry.yaml" {
		t.Errorf("PantryFile = %q, want trimmed override", got.PantryFile)
	}
	if got.ShoppingFile != "/etc/homelab-mcp/shopping.yaml" {
		t.Errorf("ShoppingFile = %q, want trimmed override", got.ShoppingFile)
	}
}

func TestLoadFromEnvHTTPTransport(t *testing.T) {
	got, err := LoadFromEnv(mapLookup(map[string]string{
		"MEALIE_URL":                "https://mealie.example.test",
		"MEALIE_TOKEN":              "secret-token",
		"MCP_TRANSPORT":             " HTTP ",
		"MCP_HTTP_ADDR":             " 127.0.0.1:9090 ",
		"MCP_AUTH_MODE":             "oidc",
		"OIDC_ISSUER_URL":           "https://sso.example.test/realms/home",
		"OIDC_AUDIENCE":             "homelab-mcp",
		"OIDC_REQUIRED_SCOPES":      "mcp.read mcp.write",
		"MCP_PUBLIC_URL":            "https://mcp.example.test/mcp",
		"MCP_RESOURCE_METADATA_URL": "https://mcp.example.test/.well-known/oauth-protected-resource",
	}))
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if got.Transport != TransportHTTP {
		t.Errorf("Transport = %q, want %q", got.Transport, TransportHTTP)
	}
	if got.HTTPAddr != "127.0.0.1:9090" {
		t.Errorf("HTTPAddr = %q, want trimmed override", got.HTTPAddr)
	}
	if got.AuthMode != AuthOIDC || got.OIDCIssuerURL != "https://sso.example.test/realms/home" || got.OIDCAudience != "homelab-mcp" {
		t.Errorf("OIDC config = mode %q issuer %q audience %q", got.AuthMode, got.OIDCIssuerURL, got.OIDCAudience)
	}
	if len(got.OIDCScopes) != 2 || got.OIDCScopes[0] != "mcp.read" || got.OIDCScopes[1] != "mcp.write" {
		t.Errorf("OIDCScopes = %+v, want two configured scopes", got.OIDCScopes)
	}
	if got.PublicURL != "https://mcp.example.test/mcp" || got.MetadataURL != "https://mcp.example.test/.well-known/oauth-protected-resource" {
		t.Errorf("resource URLs = %q/%q, want trimmed URLs", got.PublicURL, got.MetadataURL)
	}
}

func TestLoadFromEnvHTTPRequiresExplicitNoAuthOrOIDC(t *testing.T) {
	_, err := LoadFromEnv(mapLookup(map[string]string{
		"MEALIE_URL":    "https://mealie.example.test",
		"MEALIE_TOKEN":  "secret-token",
		"MCP_TRANSPORT": "http",
	}))
	if err == nil || !strings.Contains(err.Error(), "OIDC_ISSUER_URL is required") {
		t.Fatalf("LoadFromEnv() error = %v, want fail-closed OIDC configuration error", err)
	}

	got, err := LoadFromEnv(mapLookup(map[string]string{
		"MEALIE_URL":    "https://mealie.example.test",
		"MEALIE_TOKEN":  "secret-token",
		"MCP_TRANSPORT": "http",
		"MCP_AUTH_MODE": "none",
	}))
	if err != nil {
		t.Fatalf("LoadFromEnv() explicit no-auth error = %v", err)
	}
	if got.AuthMode != AuthNone {
		t.Errorf("AuthMode = %q, want %q", got.AuthMode, AuthNone)
	}
}

func TestLoadFromEnvInvalidTransport(t *testing.T) {
	_, err := LoadFromEnv(mapLookup(map[string]string{
		"MEALIE_URL":    "https://mealie.example.test",
		"MEALIE_TOKEN":  "secret-token",
		"MCP_TRANSPORT": "websocket",
	}))
	if err == nil || !strings.Contains(err.Error(), "MCP_TRANSPORT must be") {
		t.Fatalf("LoadFromEnv() error = %v, want invalid transport error", err)
	}
}

func TestLoadFromEnvNilLookup(t *testing.T) {
	_, err := LoadFromEnv(nil)
	if err == nil {
		t.Fatal("LoadFromEnv(nil) error = nil, want error")
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
