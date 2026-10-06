package sqlflags

import (
	"encoding/json"
	"testing"
)

func TestParseWhere_AllOperators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantOp  Operator
		wantFld string
		wantVal any
		wantErr bool
	}{
		{name: "loose equal", input: "name==Alice", wantOp: OpLooseEq, wantFld: "name", wantVal: "Alice"},
		{name: "strict equal", input: "count===42", wantOp: OpStrictEq, wantFld: "count", wantVal: int64(42)},
		{name: "loose not equal", input: "name!=Alice", wantOp: OpLooseNeq, wantFld: "name", wantVal: "Alice"},
		{name: "strict not equal", input: "count!==42", wantOp: OpStrictNeq, wantFld: "count", wantVal: int64(42)},
		{name: "greater than", input: "pop>100", wantOp: OpGt, wantFld: "pop", wantVal: int64(100)},
		{name: "less than", input: "pop<100", wantOp: OpLt, wantFld: "pop", wantVal: int64(100)},
		{name: "greater or equal", input: "pop>=100", wantOp: OpGte, wantFld: "pop", wantVal: int64(100)},
		{name: "less or equal", input: "pop<=100", wantOp: OpLte, wantFld: "pop", wantVal: int64(100)},

		// Bare = rejected (spec: req:comparison-operators)
		{name: "bare = rejected", input: "name=Alice", wantErr: true},

		// Pseudo-field $id
		{name: "pseudo id strict", input: "$id===ie", wantOp: OpStrictEq, wantFld: "$id", wantVal: "ie"},
		{name: "pseudo id loose", input: "$id==ie", wantOp: OpLooseEq, wantFld: "$id", wantVal: "ie"},

		// Comma-stripping for numerics
		{name: "comma in number", input: "pop>1,000,000", wantOp: OpGt, wantFld: "pop", wantVal: int64(1000000)},
		{name: "max signed integer", input: "id==9223372036854775807", wantOp: OpLooseEq, wantFld: "id", wantVal: int64(9223372036854775807)},
		{name: "adjacent wide integer", input: "id==9007199254740993", wantOp: OpLooseEq, wantFld: "id", wantVal: int64(9007199254740993)},
		{name: "beyond int64", input: "amount==9223372036854775809", wantOp: OpLooseEq, wantFld: "amount", wantVal: json.Number("9223372036854775809")},
		{name: "high precision fraction", input: "amount==0.1234567890123456", wantOp: OpLooseEq, wantFld: "amount", wantVal: json.Number("0.1234567890123456")},
		{name: "scientific literal", input: "amount==9.007199254740993e15", wantOp: OpLooseEq, wantFld: "amount", wantVal: json.Number("9.007199254740993e15")},
		{name: "decimal remains float", input: "amount==1.25", wantOp: OpLooseEq, wantFld: "amount", wantVal: float64(1.25)},
		{name: "fraction remains text", input: "amount==1/2", wantOp: OpLooseEq, wantFld: "amount", wantVal: "1/2"},
		{name: "hex remains text", input: "amount==0x10", wantOp: OpLooseEq, wantFld: "amount", wantVal: "0x10"},

		// Malformed inputs
		{name: "missing field", input: "==Alice", wantErr: true},
		{name: "missing value", input: "name==", wantErr: true},
		{name: "no operator", input: "Alice", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseWhere(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Op != tt.wantOp {
				t.Errorf("op: want %v, got %v", tt.wantOp, got.Op)
			}
			if got.Field != tt.wantFld {
				t.Errorf("field: want %q, got %q", tt.wantFld, got.Field)
			}
			if got.Value != tt.wantVal {
				t.Errorf("value: want %v (%T), got %v (%T)", tt.wantVal, tt.wantVal, got.Value, got.Value)
			}
		})
	}
}

func TestOperator_IsStrict(t *testing.T) {
	t.Parallel()
	tests := []struct {
		op   Operator
		want bool
	}{
		{op: OpStrictEq, want: true},
		{op: OpStrictNeq, want: true},
		{op: OpLooseEq, want: false},
		{op: OpLooseNeq, want: false},
		{op: OpGt, want: false},
		{op: OpLt, want: false},
		{op: OpGte, want: false},
		{op: OpLte, want: false},
		{op: OpInvalid, want: false},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			if got := tt.op.IsStrict(); got != tt.want {
				t.Errorf("IsStrict() = %v, want %v for op %v", got, tt.want, tt.op)
			}
		})
	}
}

func TestParseWhere_StrictPreservesStringForQuoted(t *testing.T) {
	t.Parallel()
	// Quoted strings stay strings even when content looks numeric.
	got, err := ParseWhere(`count==="42"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Value != "42" {
		t.Errorf("want string \"42\", got %v (%T)", got.Value, got.Value)
	}
}
