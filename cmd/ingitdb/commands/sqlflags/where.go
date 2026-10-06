package sqlflags

// specscore: feature/shared-cli-flags

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
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

var decimalNumberPattern = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?$`)

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
//   - integral numbers (with ASCII commas removed) become int64 when in range
//   - high-precision numeric text becomes json.Number
//   - other numeric-looking strings become float64
//   - everything else stays a plain string
func parseWhereValue(raw string) any {
	if len(raw) >= 2 {
		first, last := raw[0], raw[len(raw)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return raw[1 : len(raw)-1]
		}
	}
	stripped := strings.ReplaceAll(raw, ",", "")
	// Parse integers before floats so no valid int64 literal loses low bits.
	if n, err := strconv.ParseInt(stripped, 10, 64); err == nil {
		return n
	}
	if decimalNumberPattern.MatchString(stripped) {
		if _, ok := new(big.Rat).SetString(stripped); ok && needsExactNumber(stripped) {
			return json.Number(stripped)
		}
	}
	if f, err := strconv.ParseFloat(stripped, 64); err == nil {
		return f
	}
	return raw
}

// needsExactNumber identifies numeric text whose integer magnitude or decimal
// precision cannot safely be represented by the historical float64 parser.
func needsExactNumber(s string) bool {
	if strings.ContainsAny(s, "eE") {
		return true
	}
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return true // ParseInt failed, but big.Rat accepted an integer.
	}
	if len(s)-dot-1 > 15 {
		return true
	}
	digits := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			digits++
		}
	}
	return digits > 15
}
