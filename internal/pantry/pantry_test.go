package pantry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      Pantry
		wantError string
	}{
		{
			name: "normalizes values and defaults state",
			input: `version: 1
items:
  - name: " Garlic Powder "
    category: " spices "
    state: HAVE
  - name: mustard
    state: low
    note: " running low "
`,
			want: Pantry{
				Version: 1,
				Items: []Item{
					{Name: "Garlic Powder", Category: "spices", State: StateHave},
					{Name: "mustard", State: StateLow, Note: "running low"},
				},
			},
		},
		{
			name:  "version omitted",
			input: "items:\n  - name: salt\n",
			want:  Pantry{Version: CurrentVersion, Items: []Item{{Name: "salt", State: StateHave}}},
		},
		{
			name:      "empty document",
			input:     "   \n",
			wantError: "pantry document is empty",
		},
		{
			name:      "unsupported version",
			input:     "version: 2\nitems: []\n",
			wantError: "unsupported pantry version 2",
		},
		{
			name:      "unsupported state",
			input:     "items:\n  - name: salt\n    state: maybe\n",
			wantError: "unsupported state",
		},
		{
			name:      "missing name",
			input:     "items:\n  - state: have\n",
			wantError: "name is required",
		},
		{
			name:      "duplicate names are rejected",
			input:     "items:\n  - name: salt\n  - name: SALT\n",
			wantError: "duplicate name",
		},
		{
			name:      "unknown field is rejected",
			input:     "items:\n  - name: salt\n    availablity: yes\n",
			wantError: "field availablity not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.input))
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("Parse() error = %v", err)
				}
				if got.Version != tt.want.Version || len(got.Items) != len(tt.want.Items) {
					t.Fatalf("Parse() = %+v, want %+v", got, tt.want)
				}
				for i := range tt.want.Items {
					if got.Items[i] != tt.want.Items[i] {
						t.Errorf("item %d = %+v, want %+v", i, got.Items[i], tt.want.Items[i])
					}
				}
				return
			}

			if err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("Parse() error = %q, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "pantry.yaml")
	if err := os.WriteFile(filename, []byte("version: 1\nitems:\n  - name: pepper\n    state: have\n"), 0o600); err != nil {
		t.Fatalf("write pantry file: %v", err)
	}

	got, err := LoadFile(" " + filename + " ")
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Name != "pepper" || got.Items[0].State != StateHave {
		t.Fatalf("loaded pantry = %+v, want pepper/have", got)
	}
}

func TestLoadFileValidation(t *testing.T) {
	if _, err := LoadFile("   "); err == nil || !strings.Contains(err.Error(), "file path is required") {
		t.Errorf("LoadFile(blank) error = %v, want required path error", err)
	}
	if _, err := LoadFile(filepath.Join(t.TempDir(), "missing.yaml")); err == nil || !strings.Contains(err.Error(), "load pantry file") {
		t.Errorf("LoadFile(missing) error = %v, want wrapped file error", err)
	}
}
