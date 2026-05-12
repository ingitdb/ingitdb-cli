package commands

import (
	"testing"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
)

func TestEvalWhere(t *testing.T) {
	t.Parallel()

	record := map[string]any{
		"name":       "Ireland",
		"population": float64(5000000),
		"active":     true,
		"continent":  "Europe",
	}
	key := "ie"

	tests := []struct {
		name string
		cond sqlflags.Condition
		want bool
	}{
		// Loose equal: type coercion allowed
		{name: "loose eq string match", cond: sqlflags.Condition{Field: "name", Op: sqlflags.OpLooseEq, Value: "Ireland"}, want: true},
		{name: "loose eq string mismatch", cond: sqlflags.Condition{Field: "name", Op: sqlflags.OpLooseEq, Value: "France"}, want: false},
		{name: "loose eq numeric match", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpLooseEq, Value: float64(5000000)}, want: true},
		{name: "loose eq numeric as string coerces", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpLooseEq, Value: "5000000"}, want: true},

		// Strict equal: types must match
		{name: "strict eq same type", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpStrictEq, Value: float64(5000000)}, want: true},
		{name: "strict eq different type", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpStrictEq, Value: "5000000"}, want: false},
		{name: "strict eq bool match", cond: sqlflags.Condition{Field: "active", Op: sqlflags.OpStrictEq, Value: true}, want: true},
		{name: "strict eq bool vs string", cond: sqlflags.Condition{Field: "active", Op: sqlflags.OpStrictEq, Value: "true"}, want: false},

		// Not equal (loose / strict)
		{name: "loose neq match", cond: sqlflags.Condition{Field: "name", Op: sqlflags.OpLooseNeq, Value: "France"}, want: true},
		{name: "loose neq mismatch", cond: sqlflags.Condition{Field: "name", Op: sqlflags.OpLooseNeq, Value: "Ireland"}, want: false},
		{name: "strict neq same type same val", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpStrictNeq, Value: float64(5000000)}, want: false},
		{name: "strict neq different type counts as not equal", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpStrictNeq, Value: "5000000"}, want: true},

		// Ordering operators
		{name: "gt true", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpGt, Value: float64(1000000)}, want: true},
		{name: "gt false", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpGt, Value: float64(10000000)}, want: false},
		{name: "lt true", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpLt, Value: float64(10000000)}, want: true},
		{name: "gte equal counts", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpGte, Value: float64(5000000)}, want: true},
		{name: "lte equal counts", cond: sqlflags.Condition{Field: "population", Op: sqlflags.OpLte, Value: float64(5000000)}, want: true},

		// $id pseudo-field
		{name: "pseudo id strict match", cond: sqlflags.Condition{Field: "$id", Op: sqlflags.OpStrictEq, Value: "ie"}, want: true},
		{name: "pseudo id strict mismatch", cond: sqlflags.Condition{Field: "$id", Op: sqlflags.OpStrictEq, Value: "us"}, want: false},
		{name: "pseudo id loose match", cond: sqlflags.Condition{Field: "$id", Op: sqlflags.OpLooseEq, Value: "ie"}, want: true},

		// Missing field
		{name: "missing field eq", cond: sqlflags.Condition{Field: "unknown", Op: sqlflags.OpLooseEq, Value: "x"}, want: false},
		{name: "missing field neq", cond: sqlflags.Condition{Field: "unknown", Op: sqlflags.OpLooseNeq, Value: "x"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := evalWhere(record, key, tt.cond)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}

func TestEvalWhere_AllConditionsAND(t *testing.T) {
	t.Parallel()
	record := map[string]any{"a": float64(5), "b": "hello"}
	conds := []sqlflags.Condition{
		{Field: "a", Op: sqlflags.OpGt, Value: float64(1)},
		{Field: "b", Op: sqlflags.OpLooseEq, Value: "hello"},
	}
	got, err := evalAllWhere(record, "k", conds)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !got {
		t.Errorf("expected AND-true")
	}
	conds = append(conds, sqlflags.Condition{Field: "a", Op: sqlflags.OpStrictEq, Value: "5"})
	got, err = evalAllWhere(record, "k", conds)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got {
		t.Errorf("expected AND-false after adding strict-type-mismatch")
	}
}
