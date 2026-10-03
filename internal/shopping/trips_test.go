package shopping

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
		want      Plan
		wantError string
	}{
		{
			name: "normalizes trips and weekdays",
			input: `version: 1
trips:
  - name: " Run 1 "
    weekdays: [MONDAY, Tuesday]
  - name: Run 2
    weekdays:
      - wednesday
      - thursday
      - friday
`,
			want: Plan{
				Version: 1,
				Trips: []Trip{
					{Name: "Run 1", Weekdays: []Weekday{Monday, Tuesday}},
					{Name: "Run 2", Weekdays: []Weekday{Wednesday, Thursday, Friday}},
				},
			},
		},
		{
			name:  "version omitted",
			input: "trips:\n  - name: Run 1\n    weekdays: [monday]\n",
			want:  Plan{Version: CurrentVersion, Trips: []Trip{{Name: "Run 1", Weekdays: []Weekday{Monday}}}},
		},
		{
			name:      "empty document",
			input:     "  \n",
			wantError: "shopping-trip document is empty",
		},
		{
			name:      "unsupported version",
			input:     "version: 2\ntrips: []\n",
			wantError: "unsupported shopping-trip version 2",
		},
		{
			name:      "no trips",
			input:     "trips: []\n",
			wantError: "at least one shopping trip is required",
		},
		{
			name:      "missing trip name",
			input:     "trips:\n  - weekdays: [monday]\n",
			wantError: "trip 0: name is required",
		},
		{
			name:      "missing weekdays",
			input:     "trips:\n  - name: Run 1\n    weekdays: []\n",
			wantError: "at least one weekday is required",
		},
		{
			name:      "unsupported weekday",
			input:     "trips:\n  - name: Run 1\n    weekdays: [mondayish]\n",
			wantError: "unsupported weekday",
		},
		{
			name:      "duplicate weekday",
			input:     "trips:\n  - name: Run 1\n    weekdays: [monday]\n  - name: Run 2\n    weekdays: [MONDAY]\n",
			wantError: `weekday "monday" is assigned to trips 0 and 1`,
		},
		{
			name:      "duplicate trip name",
			input:     "trips:\n  - name: Run 1\n    weekdays: [monday]\n  - name: run 1\n    weekdays: [tuesday]\n",
			wantError: "duplicate name",
		},
		{
			name:      "unknown field",
			input:     "trips:\n  - name: Run 1\n    weekdys: [monday]\n",
			wantError: "field weekdys not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.input))
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("Parse() error = %v", err)
				}
				if got.Version != tt.want.Version || len(got.Trips) != len(tt.want.Trips) {
					t.Fatalf("Parse() = %+v, want %+v", got, tt.want)
				}
				for i := range tt.want.Trips {
					if got.Trips[i].Name != tt.want.Trips[i].Name || len(got.Trips[i].Weekdays) != len(tt.want.Trips[i].Weekdays) {
						t.Errorf("trip %d = %+v, want %+v", i, got.Trips[i], tt.want.Trips[i])
						continue
					}
					for j := range tt.want.Trips[i].Weekdays {
						if got.Trips[i].Weekdays[j] != tt.want.Trips[i].Weekdays[j] {
							t.Errorf("trip %d weekday %d = %q, want %q", i, j, got.Trips[i].Weekdays[j], tt.want.Trips[i].Weekdays[j])
						}
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

func TestTripForWeekday(t *testing.T) {
	plan := Plan{Trips: []Trip{
		{Name: "Run 1", Weekdays: []Weekday{Monday, Tuesday}},
		{Name: "Run 2", Weekdays: []Weekday{Wednesday, Thursday, Friday}},
	}}

	trip, ok := plan.TripForWeekday(Thursday)
	if !ok || trip.Name != "Run 2" {
		t.Fatalf("TripForWeekday(thursday) = %+v, %t, want Run 2/true", trip, ok)
	}
	if _, ok := plan.TripForWeekday(Saturday); ok {
		t.Fatal("TripForWeekday(saturday) = true, want unassigned day")
	}
}

func TestLoadFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "shopping.yaml")
	if err := os.WriteFile(filename, []byte("version: 1\ntrips:\n  - name: Run 1\n    weekdays: [monday]\n"), 0o600); err != nil {
		t.Fatalf("write shopping file: %v", err)
	}

	got, err := LoadFile(" " + filename + " ")
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if len(got.Trips) != 1 || got.Trips[0].Name != "Run 1" || got.Trips[0].Weekdays[0] != Monday {
		t.Fatalf("loaded plan = %+v, want Run 1/monday", got)
	}
}

func TestLoadFileValidation(t *testing.T) {
	if _, err := LoadFile("   "); err == nil || !strings.Contains(err.Error(), "file path is required") {
		t.Errorf("LoadFile(blank) error = %v, want required path error", err)
	}
	if _, err := LoadFile(filepath.Join(t.TempDir(), "missing.yaml")); err == nil || !strings.Contains(err.Error(), "load shopping trips file") {
		t.Errorf("LoadFile(missing) error = %v, want wrapped file error", err)
	}
}
