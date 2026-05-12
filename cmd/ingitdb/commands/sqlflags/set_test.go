package sqlflags

import (
	"testing"
)

func TestParseSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantFld string
		wantVal any
		wantErr bool
	}{
		{name: "bool true", input: "active=true", wantFld: "active", wantVal: true},
		{name: "bool false", input: "active=false", wantFld: "active", wantVal: false},
		{name: "int", input: "count=42", wantFld: "count", wantVal: 42},
		{name: "float", input: "ratio=3.14", wantFld: "ratio", wantVal: 3.14},
		{name: "string bare", input: "name=Ireland", wantFld: "name", wantVal: "Ireland"},
		{name: "string quoted with comma", input: `greeting="Hello, world"`, wantFld: "greeting", wantVal: "Hello, world"},
		{name: "null", input: "parent=null", wantFld: "parent", wantVal: nil},
		{name: "empty string", input: "tagline=", wantFld: "tagline", wantVal: ""},

		// Rejection cases (req:set-assignment)
		{name: "loose eq rejected", input: "active==true", wantErr: true},
		{name: "strict eq rejected", input: "active===true", wantErr: true},
		{name: "gte rejected", input: "count>=5", wantErr: true},
		{name: "lte rejected", input: "count<=5", wantErr: true},
		{name: "no operator", input: "active", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "missing field", input: "=value", wantErr: true},

		// Operator chars inside value are fine (req:set-assignment example)
		{name: "operator inside value", input: "note=x>=5", wantFld: "note", wantVal: "x>=5"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseSet(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if got.Field != tt.wantFld {
				t.Errorf("field: want %q, got %q", tt.wantFld, got.Field)
			}
			if !equalAny(got.Value, tt.wantVal) {
				t.Errorf("value: want %v (%T), got %v (%T)", tt.wantVal, tt.wantVal, got.Value, got.Value)
			}
		})
	}
}

// equalAny compares two interface values, including nil and basic
// scalar kinds. Used to keep table tests readable.
func equalAny(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a == b
}
