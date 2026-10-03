// Package pantry loads the human-editable pantry context used by grocery
// planning workflows.
package pantry

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// CurrentVersion is the current pantry document schema version.
const CurrentVersion = 1

// InventoryState describes how much confidence the household has that an item
// is available.
type InventoryState string

const (
	StateHave    InventoryState = "have"
	StateLow     InventoryState = "low"
	StateOut     InventoryState = "out"
	StateUnknown InventoryState = "unknown"
)

// Valid reports whether the state is supported by the pantry file format.
func (s InventoryState) Valid() bool {
	switch s {
	case StateHave, StateLow, StateOut, StateUnknown:
		return true
	default:
		return false
	}
}

// Item is one pantry entry. Category and Note are descriptive context for
// future grocery planning; matching should use Name and State.
type Item struct {
	Name     string         `yaml:"name"`
	Category string         `yaml:"category,omitempty"`
	State    InventoryState `yaml:"state"`
	Note     string         `yaml:"note,omitempty"`
}

// Pantry is the versioned, human-editable pantry document.
type Pantry struct {
	Version int    `yaml:"version"`
	Items   []Item `yaml:"items"`
}

// LoadFile reads and validates a pantry YAML document from disk.
func LoadFile(path string) (Pantry, error) {
	filename := strings.TrimSpace(path)
	if filename == "" {
		return Pantry{}, fmt.Errorf("load pantry: file path is required")
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return Pantry{}, fmt.Errorf("load pantry file %q: %w", filename, err)
	}

	loaded, err := Parse(data)
	if err != nil {
		return Pantry{}, fmt.Errorf("load pantry file %q: %w", filename, err)
	}
	return loaded, nil
}

// Parse decodes and validates one pantry YAML document. Empty state values
// default to have so a concise item entry remains possible.
func Parse(data []byte) (Pantry, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Pantry{}, fmt.Errorf("pantry document is empty")
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var result Pantry
	if err := decoder.Decode(&result); err != nil {
		return Pantry{}, fmt.Errorf("decode YAML: %w", err)
	}
	if result.Version == 0 {
		result.Version = CurrentVersion
	}
	if result.Version != CurrentVersion {
		return Pantry{}, fmt.Errorf("unsupported pantry version %d", result.Version)
	}

	if result.Items == nil {
		result.Items = make([]Item, 0)
	}
	seen := make(map[string]int, len(result.Items))
	for i := range result.Items {
		item := &result.Items[i]
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			return Pantry{}, fmt.Errorf("item %d: name is required", i)
		}

		key := strings.ToLower(item.Name)
		if previous, ok := seen[key]; ok {
			return Pantry{}, fmt.Errorf("item %d: duplicate name also used by item %d", i, previous)
		}
		seen[key] = i

		item.Category = strings.TrimSpace(item.Category)
		item.Note = strings.TrimSpace(item.Note)
		item.State = InventoryState(strings.ToLower(strings.TrimSpace(string(item.State))))
		if item.State == "" {
			item.State = StateHave
		}
		if !item.State.Valid() {
			return Pantry{}, fmt.Errorf("item %d %q: unsupported state %q", i, item.Name, item.State)
		}
	}

	return result, nil
}
