package jmespath

import "testing"

func TestValidateDateString(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid regular date", value: "2026-03-05", valid: true},
		{name: "valid leap day", value: "2024-02-29", valid: true},
		{name: "invalid non leap day", value: "2025-02-29", valid: false},
		{name: "invalid april thirty first", value: "2026-04-31", valid: false},
		{name: "invalid month", value: "2026-13-01", valid: false},
		{name: "invalid short month", value: "2026-3-01", valid: false},
		{name: "invalid text", value: "draft", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDateString(tt.value)
			if tt.valid && err != nil {
				t.Fatalf("validateDateString(%q) returned unexpected error: %v", tt.value, err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("validateDateString(%q) expected error", tt.value)
			}
		})
	}
}
