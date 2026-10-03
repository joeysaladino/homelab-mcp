// Package shopping contains household shopping-workflow configuration and
// models shared by future grocery-planning tools.
package shopping

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// CurrentVersion is the current shopping-trip document schema version.
const CurrentVersion = 1

// Weekday is a normalized weekday name used by the shopping-trip file.
type Weekday string

const (
	Monday    Weekday = "monday"
	Tuesday   Weekday = "tuesday"
	Wednesday Weekday = "wednesday"
	Thursday  Weekday = "thursday"
	Friday    Weekday = "friday"
	Saturday  Weekday = "saturday"
	Sunday    Weekday = "sunday"
)

// Valid reports whether the weekday is supported.
func (d Weekday) Valid() bool {
	switch d {
	case Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday:
		return true
	default:
		return false
	}
}

// ParseWeekday normalizes a full weekday name from YAML or another caller.
func ParseWeekday(value string) (Weekday, error) {
	day := Weekday(strings.ToLower(strings.TrimSpace(value)))
	if !day.Valid() {
		return "", fmt.Errorf("unsupported weekday %q", value)
	}
	return day, nil
}

// Trip is one named grocery run and the weekdays whose meals belong to it.
type Trip struct {
	Name     string    `yaml:"name"`
	Weekdays []Weekday `yaml:"weekdays"`
}

// Plan is the versioned, human-editable shopping-trip document.
type Plan struct {
	Version int    `yaml:"version"`
	Trips   []Trip `yaml:"trips"`
}

// LoadFile reads and validates a shopping-trip YAML document from disk.
func LoadFile(path string) (Plan, error) {
	filename := strings.TrimSpace(path)
	if filename == "" {
		return Plan{}, fmt.Errorf("load shopping trips: file path is required")
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return Plan{}, fmt.Errorf("load shopping trips file %q: %w", filename, err)
	}

	loaded, err := Parse(data)
	if err != nil {
		return Plan{}, fmt.Errorf("load shopping trips file %q: %w", filename, err)
	}
	return loaded, nil
}

// Parse decodes and validates one shopping-trip YAML document.
func Parse(data []byte) (Plan, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Plan{}, fmt.Errorf("shopping-trip document is empty")
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var result Plan
	if err := decoder.Decode(&result); err != nil {
		return Plan{}, fmt.Errorf("decode YAML: %w", err)
	}
	if result.Version == 0 {
		result.Version = CurrentVersion
	}
	if result.Version != CurrentVersion {
		return Plan{}, fmt.Errorf("unsupported shopping-trip version %d", result.Version)
	}
	if len(result.Trips) == 0 {
		return Plan{}, fmt.Errorf("at least one shopping trip is required")
	}

	seenNames := make(map[string]int, len(result.Trips))
	seenDays := make(map[Weekday]int)
	for tripIndex := range result.Trips {
		trip := &result.Trips[tripIndex]
		trip.Name = strings.TrimSpace(trip.Name)
		if trip.Name == "" {
			return Plan{}, fmt.Errorf("trip %d: name is required", tripIndex)
		}
		nameKey := strings.ToLower(trip.Name)
		if previous, ok := seenNames[nameKey]; ok {
			return Plan{}, fmt.Errorf("trip %d: duplicate name also used by trip %d", tripIndex, previous)
		}
		seenNames[nameKey] = tripIndex

		if len(trip.Weekdays) == 0 {
			return Plan{}, fmt.Errorf("trip %d %q: at least one weekday is required", tripIndex, trip.Name)
		}
		for dayIndex := range trip.Weekdays {
			day, err := ParseWeekday(string(trip.Weekdays[dayIndex]))
			if err != nil {
				return Plan{}, fmt.Errorf("trip %d %q weekday %d: %w", tripIndex, trip.Name, dayIndex, err)
			}
			trip.Weekdays[dayIndex] = day
			if previous, ok := seenDays[day]; ok {
				return Plan{}, fmt.Errorf("weekday %q is assigned to trips %d and %d", day, previous, tripIndex)
			}
			seenDays[day] = tripIndex
		}
	}

	return result, nil
}

// TripForWeekday returns the configured trip for a weekday. Unassigned days
// are valid and return false so callers can decide how to handle them.
func (p Plan) TripForWeekday(day Weekday) (Trip, bool) {
	for _, trip := range p.Trips {
		for _, assignedDay := range trip.Weekdays {
			if assignedDay == day {
				return trip, true
			}
		}
	}
	return Trip{}, false
}
