package ingitdb

import (
	"reflect"
	"testing"
)

func TestEvaluateFormula(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		formula string
		fields  map[string]any
		want    any
	}{
		{
			name:    "simple field reference",
			formula: "name",
			fields:  map[string]any{"name": "ada"},
			want:    "ada",
		},
		{
			name:    "arithmetic",
			formula: "a + b * 2",
			fields:  map[string]any{"a": int64(3), "b": int64(4)},
			want:    int64(11),
		},
		// String helpers (REQ:builtin-helpers).
		{
			name:    "string strip",
			formula: "s.strip()",
			fields:  map[string]any{"s": "  hi  "},
			want:    "hi",
		},
		{
			name:    "string lower",
			formula: "s.lower()",
			fields:  map[string]any{"s": "ABC"},
			want:    "abc",
		},
		{
			name:    "string upper",
			formula: "s.upper()",
			fields:  map[string]any{"s": "abc"},
			want:    "ABC",
		},
		{
			name:    "string replace",
			formula: "s.replace('a', 'o')",
			fields:  map[string]any{"s": "banana"},
			want:    "bonono",
		},
		{
			name:    "string split then index",
			formula: "s.split(',')[1]",
			fields:  map[string]any{"s": "a,b,c"},
			want:    "b",
		},
		{
			name:    "string startswith",
			formula: "s.startswith('ab')",
			fields:  map[string]any{"s": "abc"},
			want:    true,
		},
		{
			name:    "string endswith",
			formula: "s.endswith('bc')",
			fields:  map[string]any{"s": "abc"},
			want:    true,
		},
		// Universe functions.
		{
			name:    "len",
			formula: "len(s)",
			fields:  map[string]any{"s": "hello"},
			want:    int64(5),
		},
		{
			name:    "min",
			formula: "min(a, b)",
			fields:  map[string]any{"a": int64(3), "b": int64(1)},
			want:    int64(1),
		},
		{
			name:    "max",
			formula: "max(a, b)",
			fields:  map[string]any{"a": int64(3), "b": int64(1)},
			want:    int64(3),
		},
		// Bare math helpers (REQ:builtin-helpers).
		{
			name:    "abs of negative int",
			formula: "abs(x)",
			fields:  map[string]any{"x": int64(-7)},
			want:    int64(7),
		},
		{
			name:    "abs of positive int",
			formula: "abs(x)",
			fields:  map[string]any{"x": int64(7)},
			want:    int64(7),
		},
		{
			name:    "abs of negative float",
			formula: "abs(x)",
			fields:  map[string]any{"x": -2.5},
			want:    2.5,
		},
		{
			name:    "round float to int",
			formula: "round(x)",
			fields:  map[string]any{"x": 4.4},
			want:    int64(4),
		},
		{
			name:    "round of int is unchanged",
			formula: "round(x)",
			fields:  map[string]any{"x": int64(7)},
			want:    int64(7),
		},
		{
			name:    "floor float to int",
			formula: "floor(x)",
			fields:  map[string]any{"x": 4.9},
			want:    int64(4),
		},
		{
			name:    "floor of int is unchanged",
			formula: "floor(x)",
			fields:  map[string]any{"x": int64(7)},
			want:    int64(7),
		},
		{
			name:    "ceil float to int",
			formula: "ceil(x)",
			fields:  map[string]any{"x": 4.1},
			want:    int64(5),
		},
		{
			name:    "ceil of int is unchanged",
			formula: "ceil(x)",
			fields:  map[string]any{"x": int64(7)},
			want:    int64(7),
		},
		// AC: builtin-string-helper-available.
		// Spec phrases this "when the record is read"; read-path wiring is a
		// later task, so it is verified here at the evaluator level.
		{
			name:    "AC builtin-string-helper-available",
			formula: "first_name.strip().upper()",
			fields:  map[string]any{"first_name": " ada "},
			want:    "ADA",
		},
		// AC: builtin-math-helper-available.
		// Spec phrases this "when the record is read"; verified here at the
		// evaluator level. Integer coercion to the declared column type is a
		// later task, so we assert the numeric result equals 5.
		{
			name:    "AC builtin-math-helper-available",
			formula: "round(score)",
			fields:  map[string]any{"score": 4.6},
			want:    int64(5),
		},
		// Result type conversions.
		{
			name:    "bool result",
			formula: "a == b",
			fields:  map[string]any{"a": int64(1), "b": int64(1)},
			want:    true,
		},
		{
			name:    "float result",
			formula: "x / 2.0",
			fields:  map[string]any{"x": 5.0},
			want:    2.5,
		},
		// None round-trips to Go nil (input nil and None result).
		{
			name:    "nil field round-trips to None then back to nil",
			formula: "x",
			fields:  map[string]any{"x": nil},
			want:    nil,
		},
		// Input type conversions for each supported Go type.
		{
			name:    "input bool",
			formula: "b",
			fields:  map[string]any{"b": true},
			want:    true,
		},
		{
			name:    "input int",
			formula: "x + 1",
			fields:  map[string]any{"x": int(2)},
			want:    int64(3),
		},
		{
			name:    "input int8",
			formula: "x + 1",
			fields:  map[string]any{"x": int8(2)},
			want:    int64(3),
		},
		{
			name:    "input int16",
			formula: "x + 1",
			fields:  map[string]any{"x": int16(2)},
			want:    int64(3),
		},
		{
			name:    "input int32",
			formula: "x + 1",
			fields:  map[string]any{"x": int32(2)},
			want:    int64(3),
		},
		{
			name:    "input int64",
			formula: "x + 1",
			fields:  map[string]any{"x": int64(2)},
			want:    int64(3),
		},
		{
			name:    "input uint",
			formula: "x + 1",
			fields:  map[string]any{"x": uint(2)},
			want:    int64(3),
		},
		{
			name:    "input uint8",
			formula: "x + 1",
			fields:  map[string]any{"x": uint8(2)},
			want:    int64(3),
		},
		{
			name:    "input uint16",
			formula: "x + 1",
			fields:  map[string]any{"x": uint16(2)},
			want:    int64(3),
		},
		{
			name:    "input uint32",
			formula: "x + 1",
			fields:  map[string]any{"x": uint32(2)},
			want:    int64(3),
		},
		{
			name:    "input uint64",
			formula: "x + 1",
			fields:  map[string]any{"x": uint64(2)},
			want:    int64(3),
		},
		{
			name:    "input float32",
			formula: "x + 0.5",
			fields:  map[string]any{"x": float32(2.0)},
			want:    2.5,
		},
		{
			name:    "input float64",
			formula: "x + 0.5",
			fields:  map[string]any{"x": float64(2.0)},
			want:    2.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := EvaluateFormula(tt.formula, tt.fields)
			if err != nil {
				t.Fatalf("EvaluateFormula(%q) returned error: %v", tt.formula, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("EvaluateFormula(%q) = %#v, want %#v", tt.formula, got, tt.want)
			}
		})
	}
}

// TestEvaluateFormulaDeterministic asserts AC: deterministic-evaluation —
// evaluating the same formula and fields twice returns identical output.
func TestEvaluateFormulaDeterministic(t *testing.T) {
	t.Parallel()

	formula := "first_name.strip().upper() + str(round(score))"
	fields := map[string]any{"first_name": " ada ", "score": 4.6}

	first, err := EvaluateFormula(formula, fields)
	if err != nil {
		t.Fatalf("first eval error: %v", err)
	}
	second, err := EvaluateFormula(formula, fields)
	if err != nil {
		t.Fatalf("second eval error: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("non-deterministic: first=%#v second=%#v", first, second)
	}
}

// TestEvaluateFormulaSandbox asserts AC: deterministic-evaluation's sandbox
// clause — no clock, randomness, network, filesystem, or load() is reachable.
func TestEvaluateFormulaSandbox(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		formula string
	}{
		{name: "time module absent", formula: "time.now()"},
		{name: "random module absent", formula: "random.random()"},
		{name: "math module absent (only bare helpers exposed)", formula: "math.pi"},
		{name: "open is absent", formula: "open('/etc/passwd')"},
		{name: "load is not a callable", formula: "load('x', 'y')"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := EvaluateFormula(tt.formula, nil)
			if err == nil {
				t.Fatalf("expected error for %q (IO/non-deterministic access must be absent)", tt.formula)
			}
		})
	}
}

// TestEvaluateFormulaRuntimeError ensures a runtime failure returns an error
// instead of panicking.
func TestEvaluateFormulaRuntimeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		formula string
		fields  map[string]any
	}{
		{name: "unknown identifier", formula: "missing", fields: nil},
		{name: "division by zero", formula: "1 // 0", fields: nil},
		{name: "type error in helper", formula: "round(s)", fields: map[string]any{"s": "x"}},
		{name: "abs on string", formula: "abs(s)", fields: map[string]any{"s": "x"}},
		{name: "abs wrong arg count", formula: "abs()", fields: nil},
		{name: "round wrong arg count", formula: "round()", fields: nil},
		{name: "syntax error", formula: "1 +", fields: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := EvaluateFormula(tt.formula, tt.fields)
			if err == nil {
				t.Fatalf("expected error for %q", tt.formula)
			}
		})
	}
}

// TestEvaluateFormulaUnsupportedFieldType ensures an unsupported Go input type
// returns an error.
func TestEvaluateFormulaUnsupportedFieldType(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"x": []int{1, 2}}
	_, err := EvaluateFormula("x", fields)
	if err == nil {
		t.Fatal("expected error for unsupported field type")
	}
}

// TestEvaluateFormulaUnsupportedResultType ensures a result type with no Go
// mapping returns an error rather than panicking. A list literal result is
// not convertible.
func TestEvaluateFormulaUnsupportedResultType(t *testing.T) {
	t.Parallel()

	_, err := EvaluateFormula("[1, 2, 3]", nil)
	if err == nil {
		t.Fatal("expected error for unsupported result type")
	}
}

// TestEvaluateFormulaIntOverflowResult ensures an integer result that does not
// fit in int64 returns an error rather than silently truncating.
func TestEvaluateFormulaIntOverflowResult(t *testing.T) {
	t.Parallel()

	_, err := EvaluateFormula("x * x", map[string]any{"x": int64(1) << 62})
	if err == nil {
		t.Fatal("expected error for int64 overflow result")
	}
}
