# Shared CLI Flags — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the parsing, applicability-check, and cobra flag-registration machinery for the new SQL-verb CLI redesign. This plan ships the *foundational* shared layer that every verb plan (cli/select, cli/insert, cli/update, cli/delete, cli/drop) will import. No verb command is wired up here.

**Architecture:** A new isolated Go package `cmd/ingitdb/commands/sqlflags/` hosts all new functionality. Old verbs (`read record`, `create record`, `query`, `delete record`, `delete records`, `delete collection`, `delete view`) and their parsers in `query_parser.go` are **not modified** — they keep working until the final cleanup plan. Each parser is a pure function with no global state; each registration helper takes `*cobra.Command` and adds the flag. The package is dependency-free except for `dalgo`, `cobra`, and `gopkg.in/yaml.v3`.

**Tech Stack:** Go 1.21+, `github.com/dal-go/dalgo/dal`, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`.

**Spec:** `spec/features/shared-cli-flags/README.md`

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Create | `cmd/ingitdb/commands/sqlflags/doc.go` | Package documentation |
| Create | `cmd/ingitdb/commands/sqlflags/where.go` | `ParseWhere` + operator types |
| Create | `cmd/ingitdb/commands/sqlflags/where_test.go` | Operator + value-parsing tests |
| Create | `cmd/ingitdb/commands/sqlflags/set.go` | `ParseSet` (`field=value` with YAML inference) |
| Create | `cmd/ingitdb/commands/sqlflags/set_test.go` | Tests for set parsing + value types |
| Create | `cmd/ingitdb/commands/sqlflags/unset.go` | `ParseUnset` (comma-separated field list) |
| Create | `cmd/ingitdb/commands/sqlflags/unset_test.go` | Tests |
| Create | `cmd/ingitdb/commands/sqlflags/min_affected.go` | `ParseMinAffected` (positive int) |
| Create | `cmd/ingitdb/commands/sqlflags/min_affected_test.go` | Tests |
| Create | `cmd/ingitdb/commands/sqlflags/order_by.go` | `ParseOrderBy` (comma-separated, `-` prefix) |
| Create | `cmd/ingitdb/commands/sqlflags/order_by_test.go` | Tests |
| Create | `cmd/ingitdb/commands/sqlflags/fields.go` | `ParseFields` (`*`, `$id`, comma list) |
| Create | `cmd/ingitdb/commands/sqlflags/fields_test.go` | Tests |
| Create | `cmd/ingitdb/commands/sqlflags/mode.go` | `Mode` enum + `ResolveMode` |
| Create | `cmd/ingitdb/commands/sqlflags/mode_test.go` | Tests |
| Create | `cmd/ingitdb/commands/sqlflags/applicability.go` | Per-verb rejection check helpers |
| Create | `cmd/ingitdb/commands/sqlflags/applicability_test.go` | Tests |
| Create | `cmd/ingitdb/commands/sqlflags/register.go` | Cobra flag-registration helpers |
| Create | `cmd/ingitdb/commands/sqlflags/register_test.go` | Tests |

**Out of scope:** No edits to `query_parser.go`, `flags.go`, or any existing `*_record.go` / `*_records.go` / `*_collection.go` / `*_view.go` file. Old commands stay alive.

---

## Task 1 — Package Scaffold and Doc

**Context:** Create the package skeleton with documentation that explains the contract. Every subsequent task adds files to this package.

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/doc.go`

- [ ] **Step 1.1 — Create the package doc file**

Write `cmd/ingitdb/commands/sqlflags/doc.go`:

```go
// Package sqlflags implements the shared CLI flag grammar for the
// SQL-verb commands (select, insert, update, delete, drop).
//
// Each parser is a pure function. Each flag-registration helper takes
// a *cobra.Command and adds the flag with the documented metadata.
// The package is the single source of truth for:
//
//   - --where  comparison operators (==, ===, !=, !==, >=, <=, >, <)
//   - --set    YAML-inferred assignments
//   - --unset  comma-separated field removal list
//   - --id     collection/key targeting (single-record mode)
//   - --from   collection targeting (set mode)
//   - --into   collection targeting (insert only)
//   - --all    full-collection scope guard
//   - --min-affected   positive-integer count threshold
//   - --order-by       comma-separated, '-' prefix for descending
//   - --fields         '*', '$id', or comma-separated projection
//
// Mode resolution (single-record vs set) is handled by ResolveMode.
// Applicability checks (which verb accepts which flag) are handled by
// the Reject* helpers. Authoritative spec:
// spec/features/shared-cli-flags/README.md
package sqlflags
```

- [ ] **Step 1.2 — Verify it compiles**

```bash
go build ./cmd/ingitdb/commands/sqlflags/...
```

Expected: no output, exit code 0.

- [ ] **Step 1.3 — Commit**

```bash
git add cmd/ingitdb/commands/sqlflags/doc.go
git commit -m "$(cat <<'EOF'
feat(sqlflags): scaffold new shared-cli-flags package

Empty package with doc comment describing the contract. Subsequent
commits add the parsers, mode resolver, applicability helpers, and
cobra registration helpers. Verb plans (cli/select, cli/insert,
cli/update, cli/delete, cli/drop) will import from here.

Spec: spec/features/shared-cli-flags/README.md
EOF
)"
```

---

## Task 2 — `ParseWhere` operators and types

**Context:** The `--where` flag accepts eight comparison operators and rejects bare `=`. Strict operators (`===`, `!==`) preserve operand types; loose operators (`==`, `!=`) allow numeric coercion. Numeric values strip ASCII commas; quoted strings preserve them.

Requirements covered: `req:comparison-operators`, `req:loose-equality`, `req:strict-equality`, `req:numeric-comma-stripping`, `req:where-repeatable`, `req:pseudo-id-field`. (`req:strict-equality-yaml-types` is verb-level; the parser produces the raw operand and lets verb implementations resolve schema-aware type checks.)

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/where.go`
- Create: `cmd/ingitdb/commands/sqlflags/where_test.go`

- [ ] **Step 2.1 — Write the failing tests**

Write `cmd/ingitdb/commands/sqlflags/where_test.go`:

```go
package sqlflags

import (
	"testing"
)

func TestParseWhere_AllOperators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantOp   Operator
		wantFld  string
		wantVal  any
		wantErr  bool
	}{
		{name: "loose equal", input: "name==Alice", wantOp: OpLooseEq, wantFld: "name", wantVal: "Alice"},
		{name: "strict equal", input: "count===42", wantOp: OpStrictEq, wantFld: "count", wantVal: float64(42)},
		{name: "loose not equal", input: "name!=Alice", wantOp: OpLooseNeq, wantFld: "name", wantVal: "Alice"},
		{name: "strict not equal", input: "count!==42", wantOp: OpStrictNeq, wantFld: "count", wantVal: float64(42)},
		{name: "greater than", input: "pop>100", wantOp: OpGt, wantFld: "pop", wantVal: float64(100)},
		{name: "less than", input: "pop<100", wantOp: OpLt, wantFld: "pop", wantVal: float64(100)},
		{name: "greater or equal", input: "pop>=100", wantOp: OpGte, wantFld: "pop", wantVal: float64(100)},
		{name: "less or equal", input: "pop<=100", wantOp: OpLte, wantFld: "pop", wantVal: float64(100)},

		// Bare = rejected (spec: req:comparison-operators)
		{name: "bare = rejected", input: "name=Alice", wantErr: true},

		// Pseudo-field $id
		{name: "pseudo id strict", input: "$id===ie", wantOp: OpStrictEq, wantFld: "$id", wantVal: "ie"},
		{name: "pseudo id loose", input: "$id==ie", wantOp: OpLooseEq, wantFld: "$id", wantVal: "ie"},

		// Comma-stripping for numerics
		{name: "comma in number", input: "pop>1,000,000", wantOp: OpGt, wantFld: "pop", wantVal: float64(1000000)},

		// Malformed inputs
		{name: "missing field", input: "==Alice", wantErr: true},
		{name: "missing value", input: "name==", wantErr: true},
		{name: "no operator", input: "Alice", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
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
```

- [ ] **Step 2.2 — Run the tests to confirm they fail**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
```

Expected: FAIL with `undefined: ParseWhere` and `undefined: Operator` etc.

- [ ] **Step 2.3 — Write the parser**

Write `cmd/ingitdb/commands/sqlflags/where.go`:

```go
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
```

- [ ] **Step 2.4 — Run the tests to confirm they pass**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
```

Expected: PASS for all cases.

- [ ] **Step 2.5 — Run the project linter**

```bash
golangci-lint run ./cmd/ingitdb/commands/sqlflags/...
```

Expected: 0 issues.

- [ ] **Step 2.6 — Commit**

```bash
git add cmd/ingitdb/commands/sqlflags/where.go cmd/ingitdb/commands/sqlflags/where_test.go
git commit -m "$(cat <<'EOF'
feat(sqlflags): add ParseWhere with eight operators

Adds the parser for --where expressions with operators ==, ===, !=,
!==, >=, <=, >, <. Bare '=' is rejected with a diagnostic pointing at
'==' (loose) and '===' (strict). Quoted strings preserve their type
under strict comparison; numeric values strip ASCII commas. The $id
pseudo-field is recognised as a valid field name.

Spec:
- shared-cli-flags#req:comparison-operators
- shared-cli-flags#req:loose-equality
- shared-cli-flags#req:strict-equality
- shared-cli-flags#req:numeric-comma-stripping
- shared-cli-flags#req:pseudo-id-field
EOF
)"
```

---

## Task 3 — `ParseSet` (assignment with YAML inference)

**Context:** `--set field=value` uses a single `=`. The right-hand side is parsed with YAML 1.2 scalar inference: `true`/`false` → bool, `42` → int, `3.14` → float, `null` → nil, `Ireland` → string, `"Hello, world"` → quoted string. Anything else with an operator between field and value (`==`, `===`, etc.) is rejected.

Requirements: `req:set-assignment`, `req:set-value-types`.

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/set.go`
- Create: `cmd/ingitdb/commands/sqlflags/set_test.go`

- [ ] **Step 3.1 — Write the failing tests**

Write `cmd/ingitdb/commands/sqlflags/set_test.go`:

```go
package sqlflags

import (
	"testing"
)

func TestParseSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantFld  string
		wantVal  any
		wantErr  bool
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
```

- [ ] **Step 3.2 — Run to confirm failure**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
```

Expected: FAIL with `undefined: ParseSet` and `undefined: Assignment`.

- [ ] **Step 3.3 — Write the parser**

Write `cmd/ingitdb/commands/sqlflags/set.go`:

```go
package sqlflags

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Assignment is one parsed --set expression.
type Assignment struct {
	Field string
	Value any
}

// rejectedSetOperators are the operators that must not appear between
// field and value in a --set expression. Order matters: longer prefixes
// first so we detect "===" before "==".
var rejectedSetOperators = []string{"===", "!==", "==", "!=", ">=", "<=", ">", "<"}

// ParseSet parses one --set expression: `field=value`.
// Comparison operators between field and value are rejected.
func ParseSet(s string) (Assignment, error) {
	if s == "" {
		return Assignment{}, fmt.Errorf("empty --set expression")
	}
	idx := strings.Index(s, "=")
	if idx < 0 {
		return Assignment{}, fmt.Errorf("missing '=' in --set expression %q", s)
	}
	field := strings.TrimSpace(s[:idx])
	if field == "" {
		return Assignment{}, fmt.Errorf("missing field name in --set expression %q", s)
	}
	// Check the substring from field-end up to the '=' for forbidden operators.
	// We only need to look at chars immediately before '=' for "==" / "===" /
	// "!=" / "!==" / ">=" / "<=" / ">" / "<".
	for _, op := range rejectedSetOperators {
		if strings.HasSuffix(field, op[:len(op)-1]) && len(op) > 1 {
			// e.g. field "active=" with input "active==true" leaves field="active="; trimmed differently.
			// More robust: re-derive on the original.
		}
		// Simpler robust check: look in raw substring before '='.
		if strings.Contains(s[:idx]+string(s[idx]), op) && op != "=" {
			// guard: the '=' alone is what we want; reject only if a longer op straddles idx
		}
	}
	// Re-do the rejection check robustly by re-scanning from the start.
	for _, op := range rejectedSetOperators {
		opIdx := strings.Index(s, op)
		if opIdx >= 0 && opIdx <= idx {
			// op appears at or before the '=' we picked; that means the real
			// operator between field and value is `op`, not bare `=`.
			return Assignment{}, fmt.Errorf("--set requires single '='; found %q in %q", op, s)
		}
	}
	rawVal := s[idx+1:]
	val, parseErr := parseYAMLScalar(rawVal)
	if parseErr != nil {
		return Assignment{}, fmt.Errorf("invalid --set value in %q: %w", s, parseErr)
	}
	return Assignment{Field: field, Value: val}, nil
}

// parseYAMLScalar performs YAML 1.2 scalar inference on raw, returning
// the typed Go value. An empty string is returned as "".
func parseYAMLScalar(raw string) (any, error) {
	if raw == "" {
		return "", nil
	}
	var out any
	if err := yaml.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}
```

- [ ] **Step 3.4 — Run tests to confirm pass**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
```

Expected: PASS for all cases.

- [ ] **Step 3.5 — Run linter**

```bash
golangci-lint run ./cmd/ingitdb/commands/sqlflags/...
```

Expected: 0 issues. If the linter complains about dead `for _, op := range rejectedSetOperators { if strings.HasSuffix...` block (it's a leftover during drafting), delete the entire first loop and keep only the second robust one.

- [ ] **Step 3.6 — Simplify the parser**

Replace the body of `ParseSet` with this cleaner version (deleting the duplicate first loop noted above):

```go
func ParseSet(s string) (Assignment, error) {
	if s == "" {
		return Assignment{}, fmt.Errorf("empty --set expression")
	}
	idx := strings.Index(s, "=")
	if idx < 0 {
		return Assignment{}, fmt.Errorf("missing '=' in --set expression %q", s)
	}
	field := strings.TrimSpace(s[:idx])
	if field == "" {
		return Assignment{}, fmt.Errorf("missing field name in --set expression %q", s)
	}
	for _, op := range rejectedSetOperators {
		opIdx := strings.Index(s, op)
		if opIdx >= 0 && opIdx <= idx {
			return Assignment{}, fmt.Errorf("--set requires single '='; found %q in %q", op, s)
		}
	}
	rawVal := s[idx+1:]
	val, parseErr := parseYAMLScalar(rawVal)
	if parseErr != nil {
		return Assignment{}, fmt.Errorf("invalid --set value in %q: %w", s, parseErr)
	}
	return Assignment{Field: field, Value: val}, nil
}
```

Re-run tests + linter to confirm still passing.

- [ ] **Step 3.7 — Commit**

```bash
git add cmd/ingitdb/commands/sqlflags/set.go cmd/ingitdb/commands/sqlflags/set_test.go
git commit -m "$(cat <<'EOF'
feat(sqlflags): add ParseSet with YAML scalar inference

Parses --set 'field=value' assignments. Right-hand side is processed
via YAML 1.2 scalar inference (true/false/int/float/null/string).
Comparison operators between field and value (==, ===, !=, !==, >=,
<=, >, <) are rejected with a diagnostic. Characters inside the value
itself are not subject to the rejection rule.

Spec:
- shared-cli-flags#req:set-assignment
- shared-cli-flags#req:set-value-types
EOF
)"
```

---

## Task 4 — `ParseUnset` (comma-separated field list)

**Context:** `--unset=field1,field2` removes fields. Repeatable; empty entries rejected; names with `=` or whitespace rejected.

Requirements: `req:unset-syntax`, `req:unset-semantics` (semantics are owned by verb implementations; this parser just validates the syntax).

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/unset.go`
- Create: `cmd/ingitdb/commands/sqlflags/unset_test.go`

- [ ] **Step 4.1 — Write the failing tests**

Write `cmd/ingitdb/commands/sqlflags/unset_test.go`:

```go
package sqlflags

import (
	"reflect"
	"testing"
)

func TestParseUnset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{name: "single", input: "active", want: []string{"active"}},
		{name: "multiple", input: "active,note", want: []string{"active", "note"}},
		{name: "with spaces", input: "active, note", want: []string{"active", "note"}},
		{name: "empty input", input: "", wantErr: true},
		{name: "trailing comma", input: "active,", wantErr: true},
		{name: "leading comma", input: ",active", wantErr: true},
		{name: "double comma", input: "a,,b", wantErr: true},
		{name: "name with =", input: "active=true", wantErr: true},
		{name: "name with space inside", input: "active flag", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseUnset(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}
```

- [ ] **Step 4.2 — Run to confirm failure**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
```

Expected: FAIL with `undefined: ParseUnset`.

- [ ] **Step 4.3 — Write the parser**

Write `cmd/ingitdb/commands/sqlflags/unset.go`:

```go
package sqlflags

import (
	"fmt"
	"strings"
)

// ParseUnset parses a comma-separated --unset field list.
// Each field must be non-empty, contain no '=', and contain no
// whitespace inside the name.
func ParseUnset(s string) ([]string, error) {
	if s == "" {
		return nil, fmt.Errorf("empty --unset value")
	}
	parts := strings.Split(s, ",")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("empty field in --unset %q (check for stray commas)", s)
		}
		if strings.Contains(trimmed, "=") {
			return nil, fmt.Errorf("--unset field %q must not contain '='", trimmed)
		}
		if strings.ContainsAny(trimmed, " \t\n") {
			return nil, fmt.Errorf("--unset field %q must not contain whitespace", trimmed)
		}
		fields = append(fields, trimmed)
	}
	return fields, nil
}
```

- [ ] **Step 4.4 — Run tests to confirm pass**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
```

Expected: PASS.

- [ ] **Step 4.5 — Linter + commit**

```bash
golangci-lint run ./cmd/ingitdb/commands/sqlflags/...
git add cmd/ingitdb/commands/sqlflags/unset.go cmd/ingitdb/commands/sqlflags/unset_test.go
git commit -m "$(cat <<'EOF'
feat(sqlflags): add ParseUnset for comma-separated field lists

Parses --unset=field1,field2 into a slice of field names. Empty
entries, embedded '=', and embedded whitespace are all rejected.
Field-vs-record-set-membership and the set/unset mutual-field
exclusion are owned by verb implementations.

Spec: shared-cli-flags#req:unset-syntax
EOF
)"
```

---

## Task 5 — `ParseMinAffected` (positive integer)

**Context:** `--min-affected=N` where N is a positive integer (N ≥ 1). Zero and negative values are rejected; the flag is set-mode-only (mode rejection is enforced elsewhere).

Requirements: `req:min-affected-syntax`.

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/min_affected.go`
- Create: `cmd/ingitdb/commands/sqlflags/min_affected_test.go`

- [ ] **Step 5.1 — Write the failing tests**

```go
package sqlflags

import "testing"

func TestParseMinAffected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "one", input: "1", want: 1},
		{name: "ten", input: "10", want: 10},
		{name: "large", input: "1000000", want: 1000000},
		{name: "zero rejected", input: "0", wantErr: true},
		{name: "negative rejected", input: "-1", wantErr: true},
		{name: "non-numeric rejected", input: "foo", wantErr: true},
		{name: "float rejected", input: "1.5", wantErr: true},
		{name: "empty rejected", input: "", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseMinAffected(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("want %d, got %d", tt.want, got)
			}
		})
	}
}
```

- [ ] **Step 5.2 — Run to confirm failure**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
```

Expected: FAIL with `undefined: ParseMinAffected`.

- [ ] **Step 5.3 — Write the parser**

```go
package sqlflags

import (
	"fmt"
	"strconv"
)

// ParseMinAffected parses --min-affected=N into a positive integer.
// N must be >= 1; zero and negative values are rejected.
func ParseMinAffected(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("--min-affected value is required")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("--min-affected %q is not an integer: %w", s, err)
	}
	if n < 1 {
		return 0, fmt.Errorf("--min-affected must be >= 1, got %d", n)
	}
	return n, nil
}
```

- [ ] **Step 5.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
golangci-lint run ./cmd/ingitdb/commands/sqlflags/...
git add cmd/ingitdb/commands/sqlflags/min_affected.go cmd/ingitdb/commands/sqlflags/min_affected_test.go
git commit -m "$(cat <<'EOF'
feat(sqlflags): add ParseMinAffected positive-integer parser

Parses --min-affected=N as a positive integer (N >= 1). Zero,
negative, non-numeric, and floating-point inputs are all rejected.
The semantic check (count vs threshold) and the set-mode-only
applicability are enforced at the verb level.

Spec: shared-cli-flags#req:min-affected-syntax
EOF
)"
```

---

## Task 6 — `ParseOrderBy` and `ParseFields`

**Context:** `--order-by=name,-population` parses into a list of ascending/descending field orderings. `--fields=*` means all fields, `$id` is the pseudo-field for the record key, and a comma list projects specific fields.

The existing implementations in `cmd/ingitdb/commands/query_parser.go` are correct for the new semantics but are package-local and tied to the old commands. We re-implement here (no DAL types in the result — the verb wraps the result into `dal.OrderExpression` etc.) so `sqlflags` stays decoupled from `dal`.

Requirements: `req:order-by-syntax`, `req:fields-syntax`.

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/order_by.go`
- Create: `cmd/ingitdb/commands/sqlflags/order_by_test.go`
- Create: `cmd/ingitdb/commands/sqlflags/fields.go`
- Create: `cmd/ingitdb/commands/sqlflags/fields_test.go`

- [ ] **Step 6.1 — Write the order-by tests**

```go
package sqlflags

import (
	"reflect"
	"testing"
)

func TestParseOrderBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []OrderTerm
		wantErr bool
	}{
		{name: "empty", input: "", want: nil},
		{name: "single ascending", input: "name", want: []OrderTerm{{Field: "name", Descending: false}}},
		{name: "single descending", input: "-population", want: []OrderTerm{{Field: "population", Descending: true}}},
		{name: "mixed", input: "country,-population,name", want: []OrderTerm{
			{Field: "country", Descending: false},
			{Field: "population", Descending: true},
			{Field: "name", Descending: false},
		}},
		{name: "with spaces", input: "country, -population , name", want: []OrderTerm{
			{Field: "country", Descending: false},
			{Field: "population", Descending: true},
			{Field: "name", Descending: false},
		}},
		{name: "dash only", input: "-", wantErr: true},
		{name: "empty between commas", input: "name,,country", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseOrderBy(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}
```

- [ ] **Step 6.2 — Write the order-by implementation**

```go
package sqlflags

import (
	"fmt"
	"strings"
)

// OrderTerm is one parsed --order-by entry.
type OrderTerm struct {
	Field      string
	Descending bool
}

// ParseOrderBy parses a comma-separated --order-by list. A leading '-'
// indicates descending order for that field. An empty input returns
// nil with no error.
func ParseOrderBy(s string) ([]OrderTerm, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	terms := make([]OrderTerm, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("empty --order-by entry in %q (check for stray commas)", s)
		}
		desc := false
		if strings.HasPrefix(trimmed, "-") {
			desc = true
			trimmed = strings.TrimSpace(trimmed[1:])
			if trimmed == "" {
				return nil, fmt.Errorf("empty field after '-' in --order-by %q", s)
			}
		}
		terms = append(terms, OrderTerm{Field: trimmed, Descending: desc})
	}
	return terms, nil
}
```

- [ ] **Step 6.3 — Write the fields tests**

```go
package sqlflags

import (
	"reflect"
	"testing"
)

func TestParseFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string // nil means "all fields"
	}{
		{name: "star", input: "*", want: nil},
		{name: "empty", input: "", want: nil},
		{name: "single id", input: "$id", want: []string{"$id"}},
		{name: "id and name", input: "$id,name", want: []string{"$id", "name"}},
		{name: "with spaces", input: "$id, name , age", want: []string{"$id", "name", "age"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseFields(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}
```

- [ ] **Step 6.4 — Write the fields implementation**

```go
package sqlflags

import "strings"

// ParseFields parses --fields. Returns nil for "*" or empty (meaning
// "all fields"). Otherwise returns the trimmed comma-separated list,
// preserving order.
func ParseFields(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
```

- [ ] **Step 6.5 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
golangci-lint run ./cmd/ingitdb/commands/sqlflags/...
git add cmd/ingitdb/commands/sqlflags/order_by.go cmd/ingitdb/commands/sqlflags/order_by_test.go cmd/ingitdb/commands/sqlflags/fields.go cmd/ingitdb/commands/sqlflags/fields_test.go
git commit -m "$(cat <<'EOF'
feat(sqlflags): add ParseOrderBy and ParseFields

Parsers for the two projection-shape flags. ParseOrderBy returns a
slice of OrderTerm{Field, Descending}; '-' prefix flips Descending to
true. ParseFields returns nil for '*' or empty (meaning all fields)
and a trimmed comma list otherwise. The DAL-specific conversion
(dal.AscendingField / dal.FieldRef) is done by each verb's adapter.

Spec:
- shared-cli-flags#req:order-by-syntax
- shared-cli-flags#req:fields-syntax
EOF
)"
```

---

## Task 7 — Mode resolver

**Context:** `select`, `update`, and `delete` operate in exactly one mode: single-record (`--id` supplied) or set (`--from` supplied). Supplying both or neither is rejected. The resolver is a single function that takes the two flag values and returns the resolved mode or an error.

Requirements: `req:exactly-one-mode`.

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/mode.go`
- Create: `cmd/ingitdb/commands/sqlflags/mode_test.go`

- [ ] **Step 7.1 — Write the failing tests**

```go
package sqlflags

import "testing"

func TestResolveMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		from    string
		want    Mode
		wantErr bool
	}{
		{name: "id only", id: "countries/ie", from: "", want: ModeID},
		{name: "from only", id: "", from: "countries", want: ModeFrom},
		{name: "neither rejected", id: "", from: "", wantErr: true},
		{name: "both rejected", id: "countries/ie", from: "countries", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveMode(tt.id, tt.from)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}
```

- [ ] **Step 7.2 — Write the implementation**

```go
package sqlflags

import "fmt"

// Mode is the verb operating mode.
type Mode int

const (
	ModeInvalid Mode = iota
	ModeID            // single-record (--id supplied)
	ModeFrom          // set (--from supplied)
)

// ResolveMode returns the operating mode for a verb based on its --id
// and --from flag values. Empty string means "not supplied".
// Supplying both or neither is rejected.
func ResolveMode(idFlag, fromFlag string) (Mode, error) {
	hasID := idFlag != ""
	hasFrom := fromFlag != ""
	switch {
	case hasID && hasFrom:
		return ModeInvalid, fmt.Errorf("--id and --from are mutually exclusive; supply exactly one")
	case hasID:
		return ModeID, nil
	case hasFrom:
		return ModeFrom, nil
	default:
		return ModeInvalid, fmt.Errorf("one of --id or --from is required")
	}
}
```

- [ ] **Step 7.3 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
golangci-lint run ./cmd/ingitdb/commands/sqlflags/...
git add cmd/ingitdb/commands/sqlflags/mode.go cmd/ingitdb/commands/sqlflags/mode_test.go
git commit -m "$(cat <<'EOF'
feat(sqlflags): add ResolveMode for --id vs --from mutual exclusion

Returns ModeID, ModeFrom, or an error. Supplying both or neither is
rejected with a clear diagnostic. Called by select, update, and
delete; insert uses --into (separate flow) and drop uses positional
subcommands (no mode concept).

Spec: shared-cli-flags#req:exactly-one-mode
EOF
)"
```

---

## Task 8 — Applicability check helpers

**Context:** Each verb accepts a subset of shared flags and rejects the rest. Centralising the rejection logic here gives verbs a one-liner instead of a flag-by-flag check.

Requirements: `req:from-flag`, `req:into-flag`, `req:id-flag`, `req:all-flag`, `req:where-requires-set-mode`, `req:set-flag-applies-to-both-modes`, `req:set-unset-mutual-field-exclusion`, `req:min-affected-applicability`, `req:order-by-applicability`, `req:fields-applicability`, `req:unset-applicability`.

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/applicability.go`
- Create: `cmd/ingitdb/commands/sqlflags/applicability_test.go`

- [ ] **Step 8.1 — Write the failing tests**

```go
package sqlflags

import (
	"strings"
	"testing"
)

func TestRejectUnusedFlags_SelectMode(t *testing.T) {
	t.Parallel()
	// In single-record mode, select rejects --where, --order-by, --limit,
	// --min-affected. Test the shared subset here (--limit is verb-local).
	err := RejectSetModeFlags(SetModeFlags{
		WhereSupplied:       true,
		AllSupplied:         false,
		MinAffectedSupplied: false,
	}, ModeID)
	if err == nil {
		t.Fatal("expected error when --where supplied in ModeID")
	}
	if !strings.Contains(err.Error(), "--where") {
		t.Errorf("error should name --where, got: %v", err)
	}
}

func TestRejectSetUnsetSameField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		set      []Assignment
		unset    []string
		wantErr  bool
		errField string
	}{
		{name: "no conflict", set: []Assignment{{Field: "name"}}, unset: []string{"draft"}},
		{name: "conflict", set: []Assignment{{Field: "active"}}, unset: []string{"active"}, wantErr: true, errField: "active"},
		{name: "conflict mid-list", set: []Assignment{{Field: "a"}, {Field: "b"}}, unset: []string{"x", "b"}, wantErr: true, errField: "b"},
		{name: "empty both", set: nil, unset: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := RejectSetUnsetSameField(tt.set, tt.unset)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error")
					return
				}
				if !strings.Contains(err.Error(), tt.errField) {
					t.Errorf("error should name field %q, got: %v", tt.errField, err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRejectAllWithWhere(t *testing.T) {
	t.Parallel()
	// --all and --where are mutually exclusive in set mode.
	err := RejectSetModeFlags(SetModeFlags{
		WhereSupplied: true,
		AllSupplied:   true,
	}, ModeFrom)
	if err == nil {
		t.Fatal("expected error when --all and --where both supplied")
	}
}

func TestRejectSetModeFlagsRequireOneOf(t *testing.T) {
	t.Parallel()
	// In set mode (ModeFrom), neither --where nor --all → rejected.
	err := RejectSetModeFlags(SetModeFlags{
		WhereSupplied: false,
		AllSupplied:   false,
	}, ModeFrom)
	if err == nil {
		t.Fatal("expected error when neither --where nor --all supplied in set mode")
	}
}
```

- [ ] **Step 8.2 — Run to confirm failure**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
```

Expected: FAIL with several undefined symbols.

- [ ] **Step 8.3 — Write the implementation**

```go
package sqlflags

import "fmt"

// SetModeFlags carries the boolean presence of set-mode-only flags for
// applicability checking. Verb-specific flags (--limit, --fields)
// remain the verb's concern; this helper covers only the shared
// shape governed by shared-cli-flags.
type SetModeFlags struct {
	WhereSupplied       bool
	AllSupplied         bool
	MinAffectedSupplied bool
}

// RejectSetModeFlags enforces the cross-flag rules that depend on the
// resolved Mode.
//
// In ModeID (single-record): --where, --all, and --min-affected MUST
// all be absent.
//
// In ModeFrom (set): exactly one of --where or --all MUST be supplied;
// neither and both are rejected. --min-affected is unconstrained at
// this layer (it has its own validation in ParseMinAffected and its
// own applicability rule against ModeID).
func RejectSetModeFlags(f SetModeFlags, mode Mode) error {
	switch mode {
	case ModeID:
		if f.WhereSupplied {
			return fmt.Errorf("--where is invalid with --id (single-record mode); use --from for set queries")
		}
		if f.AllSupplied {
			return fmt.Errorf("--all is invalid with --id (single-record mode)")
		}
		if f.MinAffectedSupplied {
			return fmt.Errorf("--min-affected is invalid with --id (single-record mode)")
		}
		return nil
	case ModeFrom:
		if !f.WhereSupplied && !f.AllSupplied {
			return fmt.Errorf("set mode requires one of --where or --all")
		}
		if f.WhereSupplied && f.AllSupplied {
			return fmt.Errorf("--where and --all are mutually exclusive")
		}
		return nil
	default:
		return fmt.Errorf("invalid mode")
	}
}

// RejectSetUnsetSameField enforces that no field name appears in both
// --set and --unset within the same invocation.
func RejectSetUnsetSameField(sets []Assignment, unsets []string) error {
	if len(sets) == 0 || len(unsets) == 0 {
		return nil
	}
	unsetIndex := make(map[string]struct{}, len(unsets))
	for _, name := range unsets {
		unsetIndex[name] = struct{}{}
	}
	for _, a := range sets {
		if _, conflict := unsetIndex[a.Field]; conflict {
			return fmt.Errorf("field %q appears in both --set and --unset; use one or the other", a.Field)
		}
	}
	return nil
}
```

- [ ] **Step 8.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
golangci-lint run ./cmd/ingitdb/commands/sqlflags/...
git add cmd/ingitdb/commands/sqlflags/applicability.go cmd/ingitdb/commands/sqlflags/applicability_test.go
git commit -m "$(cat <<'EOF'
feat(sqlflags): add applicability check helpers

RejectSetModeFlags enforces the cross-flag rules tied to Mode: --where,
--all, --min-affected are invalid in ModeID; exactly one of --where or
--all is required in ModeFrom; --where and --all are mutually
exclusive. RejectSetUnsetSameField enforces the no-same-field rule
between --set and --unset.

Spec:
- shared-cli-flags#req:where-requires-set-mode
- shared-cli-flags#req:all-flag
- shared-cli-flags#req:set-unset-mutual-field-exclusion
- shared-cli-flags#req:min-affected-applicability
EOF
)"
```

---

## Task 9 — Cobra registration helpers

**Context:** Each verb wires up its flag surface by calling these helpers. Helpers are intentionally pass-through (`cmd.Flags().StringP(...)`) so the verb retains control over `MarkFlagRequired` and short-flag aliases.

Requirements: covered by all shared-cli-flags REQs that name a flag.

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/register.go`
- Create: `cmd/ingitdb/commands/sqlflags/register_test.go`

- [ ] **Step 9.1 — Write the failing tests**

```go
package sqlflags

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterAllFlags_DefinesEveryFlag(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "test"}
	RegisterFromFlag(cmd)
	RegisterIntoFlag(cmd)
	RegisterIDFlag(cmd)
	RegisterWhereFlag(cmd)
	RegisterSetFlag(cmd)
	RegisterUnsetFlag(cmd)
	RegisterAllFlag(cmd)
	RegisterMinAffectedFlag(cmd)
	RegisterOrderByFlag(cmd)
	RegisterFieldsFlag(cmd)

	expected := []string{
		"from", "into", "id", "where", "set", "unset",
		"all", "min-affected", "order-by", "fields",
	}
	for _, name := range expected {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestRegisterWhereFlag_IsRepeatable(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "test"}
	RegisterWhereFlag(cmd)
	flag := cmd.Flags().Lookup("where")
	if flag == nil {
		t.Fatal("--where not registered")
	}
	// StringArray indicates repeatable; StringSlice would also work.
	// We assert the underlying type allows repetition.
	if flag.Value.Type() != "stringArray" {
		t.Errorf("--where should be stringArray (repeatable), got %q", flag.Value.Type())
	}
}

func TestRegisterSetUnsetFlags_AreRepeatable(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "test"}
	RegisterSetFlag(cmd)
	RegisterUnsetFlag(cmd)
	for _, name := range []string{"set", "unset"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("--%s not registered", name)
		}
		if f.Value.Type() != "stringArray" {
			t.Errorf("--%s should be stringArray, got %q", name, f.Value.Type())
		}
	}
}
```

- [ ] **Step 9.2 — Run to confirm failure**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
```

Expected: FAIL with `undefined: RegisterFromFlag` etc.

- [ ] **Step 9.3 — Write the registration helpers**

```go
package sqlflags

import "github.com/spf13/cobra"

// RegisterFromFlag adds --from. Used by select, update, delete.
func RegisterFromFlag(cmd *cobra.Command) {
	cmd.Flags().String("from", "", "collection to read or modify (set mode)")
}

// RegisterIntoFlag adds --into. Used by insert only.
func RegisterIntoFlag(cmd *cobra.Command) {
	cmd.Flags().String("into", "", "target collection for insert")
}

// RegisterIDFlag adds --id. Used by select, update, delete.
func RegisterIDFlag(cmd *cobra.Command) {
	cmd.Flags().String("id", "", "record ID in the form <collection>/<key> (single-record mode)")
}

// RegisterWhereFlag adds repeatable --where -w. Used by select,
// update, delete in set mode.
func RegisterWhereFlag(cmd *cobra.Command) {
	cmd.Flags().StringArrayP("where", "w", nil, "filter expression (repeatable): field<op>value, op is ==, ===, !=, !==, >=, <=, >, <")
}

// RegisterSetFlag adds repeatable --set. Used by update.
func RegisterSetFlag(cmd *cobra.Command) {
	cmd.Flags().StringArray("set", nil, "assignment (repeatable): field=value (YAML-inferred type)")
}

// RegisterUnsetFlag adds repeatable --unset. Used by update.
func RegisterUnsetFlag(cmd *cobra.Command) {
	cmd.Flags().StringArray("unset", nil, "fields to remove (repeatable, comma-separated within each occurrence): field1,field2")
}

// RegisterAllFlag adds --all. Used by update, delete.
func RegisterAllFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "operate on every record in the target collection (mutually exclusive with --where)")
}

// RegisterMinAffectedFlag adds --min-affected. Used by select,
// update, delete.
func RegisterMinAffectedFlag(cmd *cobra.Command) {
	cmd.Flags().Int("min-affected", 0, "exit non-zero when fewer than N records would be affected (set mode only)")
}

// RegisterOrderByFlag adds --order-by. Used by select.
func RegisterOrderByFlag(cmd *cobra.Command) {
	cmd.Flags().String("order-by", "", "comma-separated fields; prefix '-' for descending")
}

// RegisterFieldsFlag adds --fields -f. Used by select.
func RegisterFieldsFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("fields", "f", "*", "fields to select: * = all, $id = record key, field1,field2 = specific fields")
}
```

- [ ] **Step 9.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
golangci-lint run ./cmd/ingitdb/commands/sqlflags/...
git add cmd/ingitdb/commands/sqlflags/register.go cmd/ingitdb/commands/sqlflags/register_test.go
git commit -m "$(cat <<'EOF'
feat(sqlflags): add cobra flag-registration helpers

One helper per shared flag. Each verb plan calls only the helpers for
the flags it accepts. Repeatable flags (--where, --set, --unset) use
stringArray; --all is bool; --min-affected is int with default 0
(meaning unset). Verb plans are free to mark any flag required.

Spec: shared-cli-flags (full set of accepted flags)
EOF
)"
```

---

## Task 10 — Final integration test and CLAUDE.md note

**Context:** Prove the package compiles and the public API is usable end-to-end by writing a single integration-style test that exercises the realistic call sequence a verb would use. Also add a short note in `CLAUDE.md` so future agents know the package exists.

**Files:**
- Create: `cmd/ingitdb/commands/sqlflags/example_test.go`
- Modify: `CLAUDE.md` (add a paragraph under "Architecture")

- [ ] **Step 10.1 — Write the integration test**

Write `cmd/ingitdb/commands/sqlflags/example_test.go`:

```go
package sqlflags

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestExampleVerbPipeline simulates how a verb command would consume
// sqlflags end-to-end: register flags, resolve mode, parse user input,
// enforce applicability. This is the API the verb plans depend on.
func TestExampleVerbPipeline(t *testing.T) {
	t.Parallel()

	// 1. A verb sets up its cobra command.
	cmd := &cobra.Command{Use: "select"}
	RegisterIDFlag(cmd)
	RegisterFromFlag(cmd)
	RegisterWhereFlag(cmd)
	RegisterOrderByFlag(cmd)
	RegisterFieldsFlag(cmd)
	RegisterMinAffectedFlag(cmd)

	// 2. The user invokes: select --from=countries --where='population>1,000,000' --order-by='-population' --fields='$id,name'
	if err := cmd.ParseFlags([]string{
		"--from=countries",
		"--where=population>1,000,000",
		"--order-by=-population",
		"--fields=$id,name",
	}); err != nil {
		t.Fatalf("flag parse: %v", err)
	}

	// 3. Resolve the operating mode.
	id, _ := cmd.Flags().GetString("id")
	from, _ := cmd.Flags().GetString("from")
	mode, err := ResolveMode(id, from)
	if err != nil {
		t.Fatalf("resolve mode: %v", err)
	}
	if mode != ModeFrom {
		t.Fatalf("want ModeFrom, got %v", mode)
	}

	// 4. Parse the predicates.
	whereExprs, _ := cmd.Flags().GetStringArray("where")
	if len(whereExprs) != 1 {
		t.Fatalf("expected 1 --where, got %d", len(whereExprs))
	}
	cond, err := ParseWhere(whereExprs[0])
	if err != nil {
		t.Fatalf("parse where: %v", err)
	}
	if cond.Field != "population" || cond.Op != OpGt || cond.Value != float64(1000000) {
		t.Errorf("unexpected condition: %+v", cond)
	}

	orderRaw, _ := cmd.Flags().GetString("order-by")
	orders, err := ParseOrderBy(orderRaw)
	if err != nil {
		t.Fatalf("parse order-by: %v", err)
	}
	if len(orders) != 1 || orders[0].Field != "population" || !orders[0].Descending {
		t.Errorf("unexpected order: %+v", orders)
	}

	fieldsRaw, _ := cmd.Flags().GetString("fields")
	fields := ParseFields(fieldsRaw)
	if len(fields) != 2 || fields[0] != "$id" || fields[1] != "name" {
		t.Errorf("unexpected fields: %v", fields)
	}

	// 5. Enforce applicability.
	allSupplied, _ := cmd.Flags().GetBool("all")
	whereSupplied := len(whereExprs) > 0
	if err := RejectSetModeFlags(SetModeFlags{
		WhereSupplied: whereSupplied,
		AllSupplied:   allSupplied,
	}, mode); err != nil {
		t.Errorf("applicability check failed: %v", err)
	}
}
```

- [ ] **Step 10.2 — Run all tests and the linter**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/sqlflags/...
golangci-lint run ./cmd/ingitdb/commands/sqlflags/...
```

Expected: PASS, 0 lint issues.

- [ ] **Step 10.3 — Run the whole-repo test suite to catch surprise regressions**

```bash
go build ./...
go test -timeout=10s ./...
```

Expected: PASS. Old `query_parser_test.go` and existing command tests MUST continue to pass — they share the `commands` package but not the `sqlflags` sub-package, so there should be no name collisions.

- [ ] **Step 10.4 — Add the CLAUDE.md note**

Open `CLAUDE.md`. Find the `## Architecture` section. Just before the `## Code Conventions` heading, insert a new bullet at the end of the Architecture list:

```markdown
- **`cmd/ingitdb/commands/sqlflags/`** — Shared CLI flag grammar for the
  SQL-verb redesign (select, insert, update, delete, drop). Parsers,
  mode resolution, applicability checks, and cobra registration helpers.
  Each verb command imports from here. Old verbs (`read record`,
  `create record`, `query`, etc.) do not — they keep using
  `cmd/ingitdb/commands/query_parser.go` until the final cleanup plan.
```

Re-read the file to confirm the bullet landed in the Architecture list.

- [ ] **Step 10.5 — Final commit**

```bash
go test -timeout=10s ./...
golangci-lint run
git add cmd/ingitdb/commands/sqlflags/example_test.go CLAUDE.md
git commit -m "$(cat <<'EOF'
test(sqlflags): add end-to-end pipeline test; document package

The example test exercises a realistic verb-side call sequence:
register flags on a cobra.Command, parse user input, resolve mode,
run the predicate parsers, and run applicability checks. Future verb
plans (cli/select, cli/insert, cli/update, cli/delete, cli/drop) can
mirror this pipeline.

CLAUDE.md gains a bullet under Architecture noting the new package
and that old verbs continue to use the legacy query_parser.go path
until the final cleanup plan.
EOF
)"
```

---

## Self-Review

**Spec coverage check.** Walking each REQ in `spec/features/shared-cli-flags/README.md`:

| REQ | Implemented by |
|---|---|
| `comparison-operators` | Task 2 (ParseWhere) |
| `loose-equality` | Task 2 (parseWhereValue produces float64 for numeric strings) |
| `strict-equality` | Task 2 (quoted strings stay strings; numerics stay float64) |
| `strict-equality-yaml-types` | **Not in this plan** — verb-level (schema lookup); deferred to cli/select's plan with a note |
| `where-repeatable` | Task 9 (StringArrayP) + Task 10 integration test |
| `numeric-comma-stripping` | Task 2 |
| `pseudo-id-field` | Task 2 (the parser treats `$id` as a normal field name; verb execution layer interprets it) |
| `set-assignment` | Task 3 |
| `set-value-types` | Task 3 |
| `from-flag` / `into-flag` / `id-flag` | Task 9 registration only — applicability rejection is verb-level (Task 8 covers cross-mode, not which-verb) |
| `exactly-one-mode` | Task 7 |
| `where-requires-set-mode` | Task 8 |
| `set-flag-applies-to-both-modes` | Implicitly: Task 9 registers --set; verb plan decides who calls it |
| `all-flag` | Task 8 + Task 9 |
| `unset-syntax` | Task 4 |
| `unset-semantics` / `unset-applicability` | Verb-level — Task 9 registers, verbs guard |
| `set-unset-mutual-field-exclusion` | Task 8 |
| `min-affected-syntax` | Task 5 |
| `min-affected-applicability` | Task 8 (ModeID rejection) |
| `min-affected-semantics` | **Not in this plan** — the threshold check happens during query execution, owned by each verb's plan |
| `order-by-syntax` / `order-by-applicability` | Task 6 + Task 9 (applicability is verb-level) |
| `fields-syntax` / `fields-applicability` | Task 6 + Task 9 (applicability is verb-level) |

**Gap explanation.** Three REQs are deliberately *not* in this plan because their behavior happens at query-execution time, which is per-verb: `strict-equality-yaml-types` (needs schema), `min-affected-semantics` (needs actual record count), `pseudo-id-field` (needs DAL access to record keys). Each verb plan will pick those up and reference this plan's parsers.

**Placeholder scan.** No `TBD`, `TODO`, `implement later`, or "fill in details" — every code block is complete.

**Type consistency.** Cross-checked: `Operator` (Task 2), `Condition` (Task 2), `Assignment` (Task 3), `OrderTerm` (Task 6), `Mode` (Task 7), `SetModeFlags` (Task 8) — names used identically in `applicability_test.go` (Task 8) and `example_test.go` (Task 10).

---

## Execution Handoff

**Plan complete and saved to `spec/plans/2026-05-12-shared-cli-flags.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
