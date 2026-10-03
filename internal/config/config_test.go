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
				PantryFile: DefaultPantryFile,
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
		"MEALIE_URL":   "https://mealie.example.test",
		"MEALIE_TOKEN": "secret-token",
		"PANTRY_FILE":  " /etc/homelab-mcp/pantry.yaml ",
	}))
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if got.PantryFile != "/etc/homelab-mcp/pantry.yaml" {
		t.Errorf("PantryFile = %q, want trimmed override", got.PantryFile)
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
