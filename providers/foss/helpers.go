package foss

import (
	"fmt"

	"github.com/gecko-iac/gecko/internal/core"
)

// compareField compares a single field between current and desired inputs,
// returning any detected changes.
func compareField(field string, current, desired core.Inputs) []core.FieldChange {
	dv, desiredHas := desired[field]
	cv, currentHas := current[field]

	if desiredHas && !currentHas {
		return []core.FieldChange{{Field: field, Kind: core.ChangeAdd, NewValue: dv}}
	}
	if !desiredHas && currentHas {
		return []core.FieldChange{{Field: field, Kind: core.ChangeDelete, OldValue: cv}}
	}
	if desiredHas && currentHas && fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", dv) {
		return []core.FieldChange{{Field: field, Kind: core.ChangeUpdate, OldValue: cv, NewValue: dv}}
	}
	return nil
}

// toInt converts an interface{} value to an int, handling common numeric types.
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

// getStringInputFallback returns the string value for a key from inputs,
// or a fallback if the key is missing or empty.
func getStringInputFallback(inputs core.Inputs, key, fallback string) string {
	if v, ok := inputs[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
