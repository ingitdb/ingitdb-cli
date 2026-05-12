package sqlflags

import (
	"fmt"
	"strconv"
	"strings"
)

// Operator identifies a --where comparison.
type Operator int

const (
	OpInvalid Operator = iota
	OpLooseEq
	OpStrictEq
	OpLooseNeq
	OpStrictNeq
	OpGt
	OpLt
	OpGte
	OpLte
)

// Condition is the parsed form of one --where expression.
type Condition struct {
	Field string
	Op    Operator
	Value any
}

// IsStrict reports whether the operator preserves operand types
// (=== or !==).
func (o Operator) IsStrict() bool {
	return o == OpStrictEq || o == OpStrictNeq
}

// operatorTable lists operators longest-first so the parser matches
// "===" before "==" and so on. Order matters.
var operatorTable = []struct {
	literal string
	op      Operator
}{
	{"===", OpStrictEq},
	{"!==", OpStrictNeq},
	{"==", OpLooseEq},
	{"!=", OpLooseNeq},
	{">=", OpGte},
	{"<=", OpLte},
	{">", OpGt},
	{"<", OpLt},
}

// ParseWhere parses one --where expression.
// The bare `=` operator is rejected (spec: req:comparison-operators).
func ParseWhere(s string) (Condition, error) {
	if s == "" {
		return Condition{}, fmt.Errorf("empty --where expression")
	}
	for _, entry := range operatorTable {
		idx := strings.Index(s, entry.literal)
		if idx < 0 {
			continue
		}
		field := strings.TrimSpace(s[:idx])
		rawVal := strings.TrimSpace(s[idx+len(entry.literal):])
		if field == "" {
			return Condition{}, fmt.Errorf("missing field name in %q", s)
		}
		if rawVal == "" {
			return Condition{}, fmt.Errorf("missing value in %q", s)
		}
		val := parseWhereValue(rawVal)
		return Condition{Field: field, Op: entry.op, Value: val}, nil
	}
	if strings.Contains(s, "=") {
		return Condition{}, fmt.Errorf("bare '=' is not a valid --where operator; use '==' for loose equality or '===' for strict equality")
	}
	return Condition{}, fmt.Errorf("no supported operator found in %q (use ==, ===, !=, !==, >=, <=, >, <)", s)
}

// parseWhereValue converts the right-hand side into a typed Go value:
//   - quoted strings stay strings (quotes stripped)
//   - numeric-looking strings (with ASCII commas removed) become float64
//   - everything else stays a plain string
func parseWhereValue(raw string) any {
	if len(raw) >= 2 {
		first, last := raw[0], raw[len(raw)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return raw[1 : len(raw)-1]
		}
	}
	stripped := strings.ReplaceAll(raw, ",", "")
	if f, err := strconv.ParseFloat(stripped, 64); err == nil {
		return f
	}
	return raw
}
