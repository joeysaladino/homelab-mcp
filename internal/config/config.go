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

// Config contains configuration for the application.
type Config struct {
	Mealie     MealieConfig
	PantryFile string
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

	return Config{
		Mealie: MealieConfig{
			BaseURL: baseURL,
			token:   strings.TrimSpace(rawToken),
		},
		PantryFile: pantryFile,
	}, nil
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
