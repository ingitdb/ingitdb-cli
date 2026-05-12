# cli/select — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the `ingitdb select` command — single-record mode (`--id`), set mode (`--from + --where + --order-by + --fields + --limit + --min-affected`), and five output formats (yaml/json/csv/md/ingr). Replaces the read path of `read record` and `query` without removing those commands.

**Architecture:** A new file `cmd/ingitdb/commands/select.go` hosts the `Select` cobra command. It uses the `sqlflags` package (already shipped) for flag registration, parsing, mode resolution, and applicability checks. WHERE evaluation, sorting, and limiting happen in Go *after* fetching every record from the DAL — this trades performance for correctness across operators DAL doesn't natively support (`!=`, `!==`, `===`, `$id`). Output reuses the existing helpers in `query_output.go`. `main.go` registers the new command alongside the legacy `query` / `read record` ones, which stay working.

**Tech Stack:** Go, `github.com/dal-go/dalgo/dal`, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, the project's `sqlflags` package.

**Spec:** `spec/features/cli/select/README.md`

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Create | `cmd/ingitdb/commands/select.go` | The `Select` cobra command + RunE handler |
| Create | `cmd/ingitdb/commands/select_test.go` | Integration tests for both modes + all formats |
| Create | `cmd/ingitdb/commands/select_where.go` | `evalWhere(record, conditions)` post-filter; condition evaluator for all 8 operators |
| Create | `cmd/ingitdb/commands/select_where_test.go` | Unit tests for evalWhere |
| Create | `cmd/ingitdb/commands/select_output.go` | Single-record yaml/json helpers (bare mapping vs list) + INGR adapter that wraps the materializer's exported `FormatINGR` for both modes |
| Create | `cmd/ingitdb/commands/select_output_test.go` | Tests for single-record output including INGR one-row table |
| Modify | `pkg/ingitdb/materializer/ingr_writer.go` | Expose a public `FormatINGR(viewName, headers, records, opts...)` wrapper around the private `formatINGR` so callers outside the materializer package can produce INGR output |
| Modify | `pkg/ingitdb/materializer/ingr_writer_test.go` (or create if absent) | Test the new public wrapper |
| Modify | `cmd/ingitdb/main.go` | Add `commands.Select(...)` to the root command's `AddCommand` list |

**Reused (no edits):**

- `cmd/ingitdb/commands/sqlflags/*` — parsers, mode resolver, applicability, registration helpers
- `cmd/ingitdb/commands/query_output.go` — `writeCSV`, `writeJSON`, `writeYAML`, `writeMarkdown` for set-mode output
- `pkg/ingitdb/materializer/ingr_writer.go` — `formatINGR(viewName, opts, headers, records)` for INGR output (signature requires `[]ingitdb.IRecordEntry`; the adapter in `select_output.go` converts `map[string]any` to that type)
- `cmd/ingitdb/commands/record_context.go` — `resolveRecordContext` for `--id` single-record fetch
- `cmd/ingitdb/commands/seams.go` and `cobra_helpers.go` — DI helpers, `resolveDBPath`, `readDefinition`

**Untouched:**

- `cmd/ingitdb/commands/query.go`, `query_parser.go`, `read_record.go`, `read_record_github.go` — legacy verbs stay alive

---

## Task 1 — Command scaffold + main.go wiring

**Context:** Add the `Select` command shell that registers all sqlflags, parses them, but returns `not yet implemented` for both modes. This lands the surface so subsequent tasks slot into a working command.

**Files:**
- Create: `cmd/ingitdb/commands/select.go`
- Create: `cmd/ingitdb/commands/select_test.go`
- Modify: `cmd/ingitdb/main.go`

- [ ] **Step 1.1 — Write the failing test**

Write `cmd/ingitdb/commands/select_test.go`:

```go
package commands

import (
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// selectTestDeps returns a minimal DI set for the Select command.
func selectTestDeps(t *testing.T, dir string) (
	func() (string, error),
	func() (string, error),
	func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	func(string, *ingitdb.Definition) (dal.DB, error),
	func(...any),
) {
	t.Helper()
	def := testDef(dir)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}
	return homeDir, getWd, readDef, newDB, logf
}

func TestSelect_RegistersAllSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	for _, name := range []string{"id", "from", "where", "order-by", "fields", "limit", "min-affected", "format", "path", "github"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestSelect_RejectsBothIDAndFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--path="+dir, "--id=todo.items/x", "--from=todo.items")
	if err == nil {
		t.Fatal("expected error when both --id and --from supplied")
	}
	if !strings.Contains(err.Error(), "--id") && !strings.Contains(err.Error(), "--from") {
		t.Errorf("error should name --id or --from, got: %v", err)
	}
}

func TestSelect_RejectsNeitherIDNorFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected error when neither --id nor --from supplied")
	}
}
```

The tests reference `testDef(dir)` and `runCobraCommand(...)` — these helpers already exist in the `commands` package test files (used by `create_record_test.go`, `query_test.go`). Confirm they are accessible:

```bash
grep -n "func testDef\|func runCobraCommand" cmd/ingitdb/commands/*_test.go
```

Expected: both helpers found. If `testDef` is package-private, the new test file is in the same package, so it works.

- [ ] **Step 1.2 — Run the test to confirm it fails**

```bash
go test -timeout=10s -run TestSelect_ ./cmd/ingitdb/commands/
```

Expected: FAIL with `undefined: Select`.

- [ ] **Step 1.3 — Write the scaffold**

Write `cmd/ingitdb/commands/select.go`:

```go
package commands

import (
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Select returns the `ingitdb select` command. It queries records from
// a single collection in either single-record mode (--id) or set mode
// (--from with optional --where/--order-by/--fields/--limit/--min-affected).
// Output format defaults to yaml in single-record mode and csv in set
// mode.
func Select(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "select",
		Short: "Query records from a collection (SQL SELECT)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			id, _ := cmd.Flags().GetString("id")
			from, _ := cmd.Flags().GetString("from")
			mode, err := sqlflags.ResolveMode(id, from)
			if err != nil {
				return err
			}
			switch mode {
			case sqlflags.ModeID:
				return fmt.Errorf("select --id: not yet implemented")
			case sqlflags.ModeFrom:
				return fmt.Errorf("select --from: not yet implemented")
			default:
				return fmt.Errorf("invalid mode")
			}
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	sqlflags.RegisterIDFlag(cmd)
	sqlflags.RegisterFromFlag(cmd)
	sqlflags.RegisterWhereFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	cmd.Flags().Int("limit", 0, "maximum number of records to return (0 = no limit; set mode only)")
	addFormatFlag(cmd, "")
	// Suppress dependency-injection params; they are used by later tasks.
	_, _, _, _, _ = homeDir, getWd, readDefinition, newDB, logf
	return cmd
}
```

- [ ] **Step 1.4 — Run the test to confirm pass**

```bash
go test -timeout=10s -run TestSelect_ ./cmd/ingitdb/commands/
```

Expected: PASS (the "not yet implemented" branches are never hit by these three tests).

- [ ] **Step 1.5 — Wire into main.go**

Modify `cmd/ingitdb/main.go`. Find the `AddCommand` block (the long list inside `rootCmd.AddCommand(...)`). Add `commands.Select(...)` right after the existing `commands.Read(...)` line:

```go
		commands.Read(homeDir, getWd, readDefinition, newDB, logf),
		commands.Select(homeDir, getWd, readDefinition, newDB, logf),
		commands.Update(homeDir, getWd, readDefinition, newDB, logf),
```

The full list now has `Select` between `Read` and `Update`.

- [ ] **Step 1.6 — Verify the binary builds and the new command appears in help**

```bash
go build -o /tmp/ingitdb-select ./cmd/ingitdb/
/tmp/ingitdb-select select --help 2>&1 | head -20
```

Expected: help text mentions all registered flags (`--id`, `--from`, `--where`, `--order-by`, `--fields`, `--limit`, `--min-affected`, `--format`, `--path`, `--github`, `--token`).

- [ ] **Step 1.7 — Lint and commit**

```bash
golangci-lint run ./cmd/ingitdb/...
git add cmd/ingitdb/commands/select.go cmd/ingitdb/commands/select_test.go cmd/ingitdb/main.go
git commit -m "$(cat <<'EOF'
feat(cli): scaffold select command with sqlflags wiring

Adds the `ingitdb select` command shell. Registers --id, --from,
--where, --order-by, --fields, --limit, --min-affected, --format,
--path, --github, and --token. Resolves mode via sqlflags.ResolveMode
and returns "not yet implemented" for both branches; subsequent
commits add single-record and set-mode execution paths.

Spec: spec/features/cli/select/README.md
EOF
)"
```

---

## Task 2 — `evalWhere` post-filter for all eight operators

**Context:** The DAL only supports `Equal`, `GreaterThen`, `GreaterOrEqual`, `LessThen`, `LessOrEqual` and stores comparison-style filters. It does NOT support `!=`, `!==`, strict equality, or filtering by the `$id` pseudo-field. To honor every operator from the spec we fetch every record and apply WHERE in Go. This task adds the evaluator function; later tasks call it from set-mode.

**Files:**
- Create: `cmd/ingitdb/commands/select_where.go`
- Create: `cmd/ingitdb/commands/select_where_test.go`

- [ ] **Step 2.1 — Write the failing tests**

Write `cmd/ingitdb/commands/select_where_test.go`:

```go
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
```

- [ ] **Step 2.2 — Run to confirm failure**

```bash
go test -timeout=10s -run 'TestEvalWhere' ./cmd/ingitdb/commands/
```

Expected: FAIL with `undefined: evalWhere` and `undefined: evalAllWhere`.

- [ ] **Step 2.3 — Write the evaluator**

Write `cmd/ingitdb/commands/select_where.go`:

```go
package commands

import (
	"fmt"
	"strconv"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
)

// evalAllWhere returns true when every condition matches the record
// (logical AND). The record's key is used when a condition's field is
// the "$id" pseudo-field.
func evalAllWhere(record map[string]any, key string, conds []sqlflags.Condition) (bool, error) {
	for _, c := range conds {
		ok, err := evalWhere(record, key, c)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// evalWhere returns true when the record matches a single condition.
func evalWhere(record map[string]any, key string, c sqlflags.Condition) (bool, error) {
	lhs, present := resolveField(record, key, c.Field)
	switch c.Op {
	case sqlflags.OpLooseEq:
		if !present {
			return false, nil
		}
		return looseEqual(lhs, c.Value), nil
	case sqlflags.OpStrictEq:
		if !present {
			return false, nil
		}
		return strictEqual(lhs, c.Value), nil
	case sqlflags.OpLooseNeq:
		if !present {
			return true, nil
		}
		return !looseEqual(lhs, c.Value), nil
	case sqlflags.OpStrictNeq:
		if !present {
			return true, nil
		}
		return !strictEqual(lhs, c.Value), nil
	case sqlflags.OpGt, sqlflags.OpLt, sqlflags.OpGte, sqlflags.OpLte:
		if !present {
			return false, nil
		}
		return compareOrdered(lhs, c.Value, c.Op)
	default:
		return false, fmt.Errorf("unsupported operator: %v", c.Op)
	}
}

// resolveField returns (value, present). The pseudo-field "$id"
// resolves to the record key.
func resolveField(record map[string]any, key, field string) (any, bool) {
	if field == "$id" {
		return key, true
	}
	v, ok := record[field]
	return v, ok
}

// strictEqual returns true only when the two operands have identical
// Go types AND identical values.
func strictEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	// fmt.Sprintf("%T") is the simplest type-identity check across
	// arbitrary interface values; for our scalar set (string, float64,
	// bool, nil) this is correct and cheap.
	if fmt.Sprintf("%T", a) != fmt.Sprintf("%T", b) {
		return false
	}
	return a == b
}

// looseEqual returns true when the operands compare equal under
// type coercion: numeric vs numeric-parsable-string, bool vs
// bool-parsable-string, etc.
func looseEqual(a, b any) bool {
	if a == b {
		return true
	}
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok && bok {
		return af == bf
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// asFloat tries to coerce v to a float64. Returns the value and a
// success flag. Booleans are NOT coerced (true != 1.0 for our spec).
func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// compareOrdered evaluates >, <, >=, <= using numeric coercion when
// possible, falling back to lexicographic string comparison.
func compareOrdered(lhs, rhs any, op sqlflags.Operator) (bool, error) {
	lf, lok := asFloat(lhs)
	rf, rok := asFloat(rhs)
	if lok && rok {
		switch op {
		case sqlflags.OpGt:
			return lf > rf, nil
		case sqlflags.OpLt:
			return lf < rf, nil
		case sqlflags.OpGte:
			return lf >= rf, nil
		case sqlflags.OpLte:
			return lf <= rf, nil
		}
	}
	ls := fmt.Sprintf("%v", lhs)
	rs := fmt.Sprintf("%v", rhs)
	switch op {
	case sqlflags.OpGt:
		return ls > rs, nil
	case sqlflags.OpLt:
		return ls < rs, nil
	case sqlflags.OpGte:
		return ls >= rs, nil
	case sqlflags.OpLte:
		return ls <= rs, nil
	}
	return false, fmt.Errorf("compareOrdered: unsupported op %v", op)
}
```

- [ ] **Step 2.4 — Pass + lint + commit**

```bash
go test -timeout=10s -run 'TestEvalWhere' ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/select_where.go cmd/ingitdb/commands/select_where_test.go
git commit -m "$(cat <<'EOF'
feat(cli/select): add evalWhere post-filter for eight operators

evalAllWhere AND-joins a slice of sqlflags.Condition against one
record. evalWhere implements all eight operators: ==, ===, !=, !==,
>=, <=, >, <. Strict equality compares Go types via fmt.Sprintf("%T");
loose equality coerces via float64 or string. The $id pseudo-field
resolves to the record key. Missing fields short-circuit as 'not
equal'.

Spec:
- cli/select#req:set-mode-shape (WHERE evaluation)
- shared-cli-flags#req:strict-equality
- shared-cli-flags#req:pseudo-id-field
EOF
)"
```

---

## Task 3 — Single-record mode (`--id`)

**Context:** The `--id` path is the simplest end-to-end path: fetch one record via the existing `resolveRecordContext`, project fields via `sqlflags.ParseFields`, and emit yaml or json (default yaml). This task makes `ingitdb select --id=todo.items/x` work end-to-end against a local database.

**Files:**
- Modify: `cmd/ingitdb/commands/select.go`
- Modify: `cmd/ingitdb/commands/select_test.go`
- Create: `cmd/ingitdb/commands/select_output.go`
- Create: `cmd/ingitdb/commands/select_output_test.go`

- [ ] **Step 3.1 — Write the failing test**

Append to `cmd/ingitdb/commands/select_test.go`:

```go
func TestSelect_SingleRecord_DefaultYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)

	// Seed a record in the test collection (testDef creates "todo.items"
	// or similar — verify by reading testDef or seedRecord helpers used
	// by other test files in this package).
	if err := seedRecord(t, dir, "todo.items", "alpha", map[string]any{"title": "Alpha", "done": false}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--id=todo.items/alpha")
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, "title: Alpha") {
		t.Errorf("expected YAML field title: Alpha, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "done: false") {
		t.Errorf("expected YAML field done: false, got:\n%s", stdout)
	}
}

func TestSelect_SingleRecord_FormatJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "todo.items", "beta", map[string]any{"title": "Beta"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--id=todo.items/beta", "--format=json")
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, `"title": "Beta"`) {
		t.Errorf("expected JSON title:Beta, got:\n%s", stdout)
	}
	// Single-record JSON must be a bare object, NOT an array.
	if strings.HasPrefix(strings.TrimSpace(stdout), "[") {
		t.Errorf("single-record JSON must be an object, got array:\n%s", stdout)
	}
}

func TestSelect_SingleRecord_FormatINGR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "todo.items", "gamma", map[string]any{"title": "Gamma", "done": true}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--id=todo.items/gamma", "--fields=$id,title,done", "--format=ingr")
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(stdout, "# INGR.io | select: ") {
		t.Errorf("missing INGR header:\n%s", stdout)
	}
	if !strings.Contains(stdout, "# 1 record") {
		t.Errorf("single-record INGR must have '# 1 record' footer:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"Gamma"`) {
		t.Errorf("missing title cell:\n%s", stdout)
	}
}

func TestSelect_SingleRecord_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--path="+dir, "--id=todo.items/missing")
	if err == nil {
		t.Fatal("expected error when record not found")
	}
	if !strings.Contains(err.Error(), "todo.items/missing") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should name the missing id, got: %v", err)
	}
}
```

`seedRecord` and `captureOutput` are helpers already used in other test files in this package. Confirm:

```bash
grep -n "func seedRecord\|func captureOutput" cmd/ingitdb/commands/*_test.go
```

If either helper is missing, define a minimal one at the top of `select_test.go`:

```go
// seedRecord writes a YAML file at <dir>/<collection-path>/<key>.yaml.
// Used by select_test.go and parallel tests in this package.
func seedRecord(t *testing.T, dir, collectionID, key string, data map[string]any) error {
	t.Helper()
	// Resolve the on-disk path for this collection from the test definition.
	def := testDef(dir)
	col, ok := def.Collections[collectionID]
	if !ok {
		return fmt.Errorf("collection %s not in test def", collectionID)
	}
	colDir := filepath.Join(dir, col.Path)
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		return err
	}
	out, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(colDir, key+".yaml"), out, 0o644)
}
```

(If `testDef` returns a Definition that already references an existing `test-ingitdb/` fixture at `<dir>`, prefer copying that fixture into the temp dir rather than re-implementing the writer. Inspect `testDef` to decide.)

- [ ] **Step 3.2 — Confirm failure**

```bash
go test -timeout=10s -run TestSelect_SingleRecord ./cmd/ingitdb/commands/
```

Expected: FAIL with "not yet implemented".

- [ ] **Step 3.3 — Expose `FormatINGR` from the materializer package**

The private `formatINGR` in `pkg/ingitdb/materializer/ingr_writer.go` does the actual byte serialization. Expose a public wrapper so callers outside the package can produce INGR output.

Append to `pkg/ingitdb/materializer/ingr_writer.go`:

```go
// FormatINGR is the public entry point for producing INGR bytes from
// a slice of IRecordEntry. It accepts the same viewName + headers +
// records arguments as the private formatINGR and exposes the same
// option list via ExportOption variadics. Used by the CLI's `select`
// command and by future callers that need INGR output without
// invoking the full materialize pipeline.
func FormatINGR(viewName string, headers []string, records []ingitdb.IRecordEntry, options ...ExportOption) ([]byte, error) {
	opts := ExportOptions{}
	for _, apply := range options {
		apply(&opts)
	}
	return formatINGR(viewName, opts, headers, records)
}
```

Add (or extend) `pkg/ingitdb/materializer/ingr_writer_test.go` with a smoke test:

```go
package materializer

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestFormatINGR_Public(t *testing.T) {
	t.Parallel()
	records := []ingitdb.IRecordEntry{
		ingitdb.RecordEntry{ID: "1", Data: map[string]any{"name": "Alice", "age": float64(30)}},
		ingitdb.RecordEntry{ID: "2", Data: map[string]any{"name": "Bob", "age": float64(25)}},
	}
	got, err := FormatINGR("test/view", []string{"$ID", "name", "age"}, records)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(got)
	if !strings.HasPrefix(out, "# INGR.io | test/view: ") {
		t.Errorf("missing INGR header in:\n%s", out)
	}
	if !strings.Contains(out, "# 2 records") {
		t.Errorf("missing record-count footer in:\n%s", out)
	}
}
```

Verify the existing private tests still pass and the new public test does too:

```bash
go test -timeout=10s ./pkg/ingitdb/materializer/...
```

Commit this small refactor separately so the materializer change has its own trail:

```bash
git add pkg/ingitdb/materializer/ingr_writer.go pkg/ingitdb/materializer/ingr_writer_test.go
git commit -m "$(cat <<'EOF'
feat(materializer): export FormatINGR for external callers

Adds a public wrapper around the private formatINGR. Same signature
shape (viewName, headers, records, opts) with ExportOption variadics
folded into ExportOptions. The CLI's new select command will call
this for --format=ingr output.

Spec: cli/select#req:format-flag
EOF
)"
```

- [ ] **Step 3.4 — Add single-record output helpers**

Write `cmd/ingitdb/commands/select_output.go`:

```go
package commands

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

// writeSingleRecord writes one record's projected fields as a bare
// mapping (yaml) or bare object (json). It does NOT wrap the record in
// a list. csv, md, and ingr formats fall back to a single-row table by
// invoking the tabular helpers with a one-element slice.
func writeSingleRecord(w io.Writer, record map[string]any, format string, columns []string) error {
	switch format {
	case "yaml", "yml", "":
		out, err := yaml.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		_, err = w.Write(out)
		return err
	case "json":
		out, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		_, err = fmt.Fprintf(w, "%s\n", out)
		return err
	case "csv":
		return writeCSV(w, []map[string]any{record}, columns)
	case "md", "markdown":
		return writeMarkdown(w, []map[string]any{record}, columns)
	case "ingr":
		return writeINGR(w, []map[string]any{record}, columns)
	default:
		return fmt.Errorf("unknown format %q, use yaml, json, csv, md, or ingr", format)
	}
}

// writeINGR emits records in the project's native INGR format
// (`# INGR.io | …` header, JSON-encoded cells one per line,
// `# N records` footer). Delegates to materializer.FormatINGR after
// adapting map[string]any rows into ingitdb.RecordEntry values.
//
// columns may be nil — in that case the union of keys across rows is
// used in deterministic order (matching writeCSV's collectColumns).
// The viewName is "select" — a synthetic identifier that flows into
// the INGR header line.
func writeINGR(w io.Writer, rows []map[string]any, columns []string) error {
	if columns == nil {
		columns = collectColumns(rows)
	}
	entries := make([]ingitdb.IRecordEntry, 0, len(rows))
	for _, row := range rows {
		id := ""
		if v, ok := row["$id"]; ok {
			id = fmt.Sprintf("%v", v)
		}
		// Strip $id from the data payload — the INGR header lists
		// columns explicitly, and $id is conveyed via GetID(). Cloning
		// avoids mutating the caller's map.
		data := make(map[string]any, len(row))
		for k, v := range row {
			if k == "$id" {
				continue
			}
			data[k] = v
		}
		entries = append(entries, ingitdb.RecordEntry{ID: id, Data: data})
	}
	// Filter $id out of the columns slice too — it would otherwise
	// produce a duplicate header column. If the caller asked for $id
	// explicitly via --fields, INGR's header already encodes the key
	// position implicitly.
	cleaned := make([]string, 0, len(columns))
	for _, c := range columns {
		if c == "$id" {
			continue
		}
		cleaned = append(cleaned, c)
	}
	out, err := materializer.FormatINGR("select", cleaned, entries)
	if err != nil {
		return fmt.Errorf("format ingr: %w", err)
	}
	_, err = w.Write(out)
	return err
}
```

- [ ] **Step 3.5 — Add unit tests for the output helpers**

Write `cmd/ingitdb/commands/select_output_test.go`:

```go
package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteSingleRecord(t *testing.T) {
	t.Parallel()

	record := map[string]any{"$id": "ie", "name": "Ireland", "population": float64(5000000)}

	tests := []struct {
		name    string
		format  string
		columns []string
		want    []string // substrings that must appear
		wantNot []string
	}{
		{name: "yaml default", format: "", want: []string{"name: Ireland", "population: 5"}, wantNot: []string{"["}},
		{name: "yaml explicit", format: "yaml", want: []string{"name: Ireland"}, wantNot: []string{"["}},
		{name: "json bare object", format: "json", want: []string{`"name": "Ireland"`}, wantNot: []string{"["}},
		{name: "ingr single row", format: "ingr", columns: []string{"$id", "name", "population"}, want: []string{"# INGR.io | select", "Ireland", "# 1 record"}, wantNot: []string{"["}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := writeSingleRecord(&buf, record, tt.format, tt.columns); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := buf.String()
			for _, s := range tt.want {
				if !strings.Contains(got, s) {
					t.Errorf("expected %q in output:\n%s", s, got)
				}
			}
			for _, s := range tt.wantNot {
				if strings.Contains(got, s) {
					t.Errorf("did not expect %q in output:\n%s", s, got)
				}
			}
		})
	}
}
```

- [ ] **Step 3.6 — Implement the single-record branch in `select.go`**

Modify `cmd/ingitdb/commands/select.go`. Replace the entire RunE function and the unused-params suppression. The full updated file body:

```go
package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Select returns the `ingitdb select` command. It queries records from
// a single collection in either single-record mode (--id) or set mode
// (--from with optional --where/--order-by/--fields/--limit/--min-affected).
// Output format defaults to yaml in single-record mode and csv in set
// mode.
func Select(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "select",
		Short: "Query records from a collection (SQL SELECT)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			ctx := cmd.Context()

			id, _ := cmd.Flags().GetString("id")
			from, _ := cmd.Flags().GetString("from")
			mode, err := sqlflags.ResolveMode(id, from)
			if err != nil {
				return err
			}

			fieldsRaw, _ := cmd.Flags().GetString("fields")
			fields, parseErr := sqlflags.ParseFields(fieldsRaw)
			if parseErr != nil {
				return parseErr
			}
			format, _ := cmd.Flags().GetString("format")
			format = strings.ToLower(format)

			switch mode {
			case sqlflags.ModeID:
				return runSelectByID(ctx, cmd, id, fields, format, homeDir, getWd, readDefinition, newDB)
			case sqlflags.ModeFrom:
				return fmt.Errorf("select --from: not yet implemented")
			default:
				return fmt.Errorf("invalid mode")
			}
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	sqlflags.RegisterIDFlag(cmd)
	sqlflags.RegisterFromFlag(cmd)
	sqlflags.RegisterWhereFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	cmd.Flags().Int("limit", 0, "maximum number of records to return (0 = no limit; set mode only)")
	addFormatFlag(cmd, "")
	return cmd
}

// runSelectByID handles --id mode: fetch one record, project fields,
// emit a bare mapping / object.
func runSelectByID(
	ctx context.Context,
	cmd *cobra.Command,
	id string,
	fields []string,
	format string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	// Reject set-mode flags per shared-cli-flags applicability rules.
	whereExprs, _ := cmd.Flags().GetStringArray("where")
	orderByVal, _ := cmd.Flags().GetString("order-by")
	limitVal, _ := cmd.Flags().GetInt("limit")
	if n, supplied, mErr := sqlflags.MinAffectedFromCmd(cmd); mErr != nil {
		return mErr
	} else if supplied {
		_ = n
		return fmt.Errorf("--min-affected is invalid with --id (single-record mode)")
	}
	if len(whereExprs) > 0 {
		return fmt.Errorf("--where is invalid with --id (single-record mode); use --from for set queries")
	}
	if orderByVal != "" {
		return fmt.Errorf("--order-by is invalid with --id (single-record mode)")
	}
	if limitVal != 0 {
		return fmt.Errorf("--limit is invalid with --id (single-record mode)")
	}

	rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
	if err != nil {
		return err
	}

	data := map[string]any{}
	key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
	record := dal.NewRecordWithData(key, data)
	err = rctx.db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		return err
	}
	if !record.Exists() {
		return fmt.Errorf("record not found: %s", id)
	}
	projected := projectRecord(data, rctx.recordKey, fields)
	if format == "" {
		format = "yaml"
	}
	return writeSingleRecord(os.Stdout, projected, format, fields)
}
```

Note the imports added at the top: `context`, `os`, `strings`.

- [ ] **Step 3.7 — Run tests, pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/select.go cmd/ingitdb/commands/select_test.go cmd/ingitdb/commands/select_output.go cmd/ingitdb/commands/select_output_test.go
git commit -m "$(cat <<'EOF'
feat(cli/select): implement single-record mode (--id)

Fetches one record via resolveRecordContext + dal.Get, projects
fields via sqlflags.ParseFields, and emits a bare mapping (yaml) or
object (json). Single-record JSON is NOT wrapped in an array. csv and
md fall back to a one-row table.

Rejects --where, --order-by, --limit, --min-affected in single-record
mode per shared-cli-flags applicability rules. Returns non-zero on
record-not-found.

Spec:
- cli/select#req:single-record-shape
- cli/select#req:single-record-not-found
- cli/select#req:single-record-rejected-flags
- cli/select#req:format-flag
EOF
)"
```

---

## Task 4 — Set-mode query execution (`--from + --where`)

**Context:** Set mode fetches every record from the named collection via the DAL, applies the parsed WHERE conditions through `evalAllWhere`, projects fields, and emits the result. ORDER BY, LIMIT, and MIN-AFFECTED are added in Task 5; this task delivers the unfiltered + WHERE-filtered cases.

**Files:**
- Modify: `cmd/ingitdb/commands/select.go`
- Modify: `cmd/ingitdb/commands/select_test.go`

- [ ] **Step 4.1 — Write failing tests**

Append to `cmd/ingitdb/commands/select_test.go`:

```go
func TestSelect_SetMode_NoFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	for _, key := range []string{"a", "b", "c"} {
		if err := seedRecord(t, dir, "todo.items", key, map[string]any{"title": "T-" + key}); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--from=todo.items", "--format=yaml")
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Set-mode YAML is a list of mappings — count occurrences of "title".
	if c := strings.Count(stdout, "title: T-"); c != 3 {
		t.Errorf("want 3 records, got %d:\n%s", c, stdout)
	}
}

func TestSelect_SetMode_WhereFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "todo.items", "a", map[string]any{"priority": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := seedRecord(t, dir, "todo.items", "b", map[string]any{"priority": float64(5)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--from=todo.items", "--where=priority>2", "--format=yaml")
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, "priority: 5") {
		t.Errorf("expected priority:5 in output:\n%s", stdout)
	}
	if strings.Contains(stdout, "priority: 1") {
		t.Errorf("did NOT expect priority:1 in output:\n%s", stdout)
	}
}

func TestSelect_SetMode_EmptyResult_CSV(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "todo.items", "a", map[string]any{"priority": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--from=todo.items", "--where=priority>1000", "--fields=$id,priority")
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// CSV is the default for set mode. Header-only output expected.
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Errorf("want header-only CSV (1 line), got %d lines:\n%s", len(lines), stdout)
	}
	if !strings.Contains(lines[0], "$id") || !strings.Contains(lines[0], "priority") {
		t.Errorf("header row missing expected columns:\n%s", stdout)
	}
}

func TestSelect_SetMode_EmptyResult_JSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--from=todo.items", "--format=json")
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got := strings.TrimSpace(stdout)
	if got != "[]" {
		t.Errorf("empty set JSON must be `[]`, got: %s", got)
	}
}

func TestSelect_SetMode_INGR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "todo.items", "a", map[string]any{"title": "Alpha", "priority": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := seedRecord(t, dir, "todo.items", "b", map[string]any{"title": "Beta", "priority": float64(2)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--from=todo.items", "--fields=$id,title,priority", "--format=ingr")
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(stdout, "# INGR.io | select: ") {
		t.Errorf("missing INGR header in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "# 2 records") {
		t.Errorf("missing record-count footer:\n%s", stdout)
	}
	// Each cell appears on its own line, JSON-encoded.
	if !strings.Contains(stdout, `"Alpha"`) || !strings.Contains(stdout, `"Beta"`) {
		t.Errorf("missing JSON-encoded titles:\n%s", stdout)
	}
}

func TestSelect_SetMode_INGR_EmptyResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--from=todo.items", "--fields=$id,title", "--format=ingr")
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(stdout, "# INGR.io | select: ") {
		t.Errorf("missing INGR header in empty-result output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "# 0 records") {
		t.Errorf("empty INGR must have '# 0 records' footer:\n%s", stdout)
	}
}
```

- [ ] **Step 4.2 — Confirm failure**

```bash
go test -timeout=10s -run TestSelect_SetMode ./cmd/ingitdb/commands/
```

Expected: FAIL with "not yet implemented".

- [ ] **Step 4.3 — Implement set-mode in `select.go`**

In `cmd/ingitdb/commands/select.go`, replace the `case sqlflags.ModeFrom:` branch with a real call:

```go
			case sqlflags.ModeFrom:
				return runSelectFromSet(ctx, cmd, from, fields, format, homeDir, getWd, readDefinition, newDB)
```

Add the new function at the end of the file:

```go
// runSelectFromSet handles --from set mode: fetch every record from
// the collection, apply WHERE conditions, project fields, and emit
// the result. Order-by, limit, and min-affected are layered on in
// later tasks.
func runSelectFromSet(
	ctx context.Context,
	cmd *cobra.Command,
	from string,
	fields []string,
	format string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	whereExprs, _ := cmd.Flags().GetStringArray("where")
	conds := make([]sqlflags.Condition, 0, len(whereExprs))
	for _, expr := range whereExprs {
		c, err := sqlflags.ParseWhere(expr)
		if err != nil {
			return fmt.Errorf("invalid --where %q: %w", expr, err)
		}
		conds = append(conds, c)
	}

	dirPath, err := resolveDBPath(cmd, homeDir, getWd)
	if err != nil {
		return err
	}
	def, err := readDefinition(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read database definition: %w", err)
	}
	colDef, ok := def.Collections[from]
	if !ok {
		return fmt.Errorf("collection %q not found in definition", from)
	}
	_ = colDef
	db, err := newDB(dirPath, def)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef(from, "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		key := dal.NewKeyWithID(from, "")
		return dal.NewRecordWithData(key, map[string]any{})
	})

	var rows []map[string]any
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, qerr := tx.ExecuteQueryToRecordsReader(ctx, q)
		if qerr != nil {
			return qerr
		}
		defer func() { _ = reader.Close() }()
		for {
			rec, nextErr := reader.Next()
			if nextErr != nil {
				break
			}
			data, ok := rec.Data().(map[string]any)
			if !ok {
				continue
			}
			recKey := fmt.Sprintf("%v", rec.Key().ID)
			match, evalErr := evalAllWhere(data, recKey, conds)
			if evalErr != nil {
				return evalErr
			}
			if !match {
				continue
			}
			rows = append(rows, projectRecord(data, recKey, fields))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if format == "" {
		format = "csv"
	}
	return writeSetMode(os.Stdout, rows, format, fields)
}

// writeSetMode is the set-mode output dispatcher. Empty rows still
// produce format-appropriate output (csv header / [] for json/yaml /
// md header / INGR header + "# 0 records" footer).
func writeSetMode(w io.Writer, rows []map[string]any, format string, columns []string) error {
	if rows == nil {
		rows = []map[string]any{}
	}
	switch format {
	case "csv":
		return writeCSV(w, rows, columns)
	case "json":
		return writeJSON(w, rows)
	case "yaml", "yml":
		return writeYAML(w, rows)
	case "md", "markdown":
		return writeMarkdown(w, rows, columns)
	case "ingr":
		return writeINGR(w, rows, columns)
	default:
		return fmt.Errorf("unknown format %q, use csv, json, yaml, md, or ingr", format)
	}
}
```

Add `"io"` to the import block of `select.go`.

- [ ] **Step 4.4 — Verify empty-set output for each format**

Check the existing `writeJSON`, `writeYAML`, `writeCSV`, `writeMarkdown` helpers' behavior when given an empty `rows` slice:

```bash
grep -A 20 "^func writeJSON\|^func writeYAML\|^func writeCSV\|^func writeMarkdown" cmd/ingitdb/commands/query_output.go
```

If any helper would emit `null` instead of `[]` (e.g. `json.Marshal(nil)` → `null`), patch this BEFORE running the tests by normalizing `rows` to `[]map[string]any{}` (the `if rows == nil` guard above does this). If `writeYAML` emits `null\n` for an empty slice, add a similar fix in `writeSetMode` for yaml:

```go
	case "yaml", "yml":
		if len(rows) == 0 {
			_, err := fmt.Fprintln(w, "[]")
			return err
		}
		return writeYAML(w, rows)
```

Apply the same guard for `json` if it emits `null`:

```go
	case "json":
		if len(rows) == 0 {
			_, err := fmt.Fprintln(w, "[]")
			return err
		}
		return writeJSON(w, rows)
```

- [ ] **Step 4.5 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/select.go cmd/ingitdb/commands/select_test.go
git commit -m "$(cat <<'EOF'
feat(cli/select): implement set mode (--from + --where)

Fetches every record from the named collection via dal.Query, applies
sqlflags-parsed WHERE conditions through evalAllWhere, projects via
projectRecord, and emits via writeSetMode. Empty-result outputs are
format-appropriate ([] for json/yaml; header-only for csv/md).

ORDER BY, LIMIT, and MIN-AFFECTED are added in subsequent commits.

Spec:
- cli/select#req:set-mode-shape
- cli/select#req:set-mode-empty-result
- cli/select#req:format-flag
- cli/select#req:format-tabular-columns
- cli/select#req:format-yaml-json-shape
EOF
)"
```

---

## Task 5 — `--order-by`, `--limit`, `--min-affected`

**Context:** Layer three more set-mode-only features on top of Task 4. Ordering happens after WHERE; LIMIT after ordering; MIN-AFFECTED check before output. Each is independent; tests cover combinations.

**Files:**
- Modify: `cmd/ingitdb/commands/select.go`
- Modify: `cmd/ingitdb/commands/select_test.go`

- [ ] **Step 5.1 — Write failing tests**

Append to `cmd/ingitdb/commands/select_test.go`:

```go
func TestSelect_SetMode_OrderByThenLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "todo.items", "a", map[string]any{"priority": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := seedRecord(t, dir, "todo.items", "b", map[string]any{"priority": float64(5)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := seedRecord(t, dir, "todo.items", "c", map[string]any{"priority": float64(3)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd,
			"--path="+dir, "--from=todo.items",
			"--order-by=-priority", "--limit=1",
			"--fields=$id,priority", "--format=csv",
		)
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("want header + 1 data row, got %d lines:\n%s", len(lines), stdout)
	}
	// Highest priority first; --limit=1 yields only "b".
	if !strings.Contains(lines[1], "b") {
		t.Errorf("expected 'b' (priority=5) as the single row, got: %s", lines[1])
	}
}

func TestSelect_SetMode_LimitValidation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--path="+dir, "--from=todo.items", "--limit=-1")
	if err == nil {
		t.Fatal("expected error for --limit=-1")
	}
}

func TestSelect_SetMode_MinAffected_Met(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	for _, k := range []string{"a", "b", "c"} {
		if err := seedRecord(t, dir, "todo.items", k, map[string]any{"x": float64(1)}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--from=todo.items", "--min-affected=2", "--format=yaml")
	})
	if err != nil {
		t.Fatalf("expected success (3 >= 2), got: %v", err)
	}
	if !strings.Contains(stdout, "x: 1") {
		t.Errorf("expected output, got:\n%s", stdout)
	}
}

func TestSelect_SetMode_MinAffected_Unmet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "todo.items", "a", map[string]any{"x": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd, "--path="+dir, "--from=todo.items", "--min-affected=5")
	})
	if err == nil {
		t.Fatalf("expected error (1 < 5), got nil. stdout: %s", stdout)
	}
	if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "5") {
		t.Errorf("error should name actual=1 and required=5, got: %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout MUST be empty when threshold unmet, got: %s", stdout)
	}
}
```

- [ ] **Step 5.2 — Confirm failure**

```bash
go test -timeout=10s -run TestSelect_SetMode_Order ./cmd/ingitdb/commands/
go test -timeout=10s -run TestSelect_SetMode_Limit ./cmd/ingitdb/commands/
go test -timeout=10s -run TestSelect_SetMode_MinAffected ./cmd/ingitdb/commands/
```

Expected: FAIL.

- [ ] **Step 5.3 — Implement order-by + limit + min-affected in `runSelectFromSet`**

In `cmd/ingitdb/commands/select.go`, modify `runSelectFromSet`. Find the point right after the loop that populates `rows`, and BEFORE `writeSetMode`. Insert:

```go
	// --order-by: sort the result slice after filtering.
	orderRaw, _ := cmd.Flags().GetString("order-by")
	orders, err := sqlflags.ParseOrderBy(orderRaw)
	if err != nil {
		return err
	}
	if len(orders) > 0 {
		sort.SliceStable(rows, func(i, j int) bool {
			for _, o := range orders {
				cmp := compareValues(rows[i][o.Field], rows[j][o.Field])
				if cmp == 0 {
					continue
				}
				if o.Descending {
					return cmp > 0
				}
				return cmp < 0
			}
			return false
		})
	}

	// --min-affected pre-flight check (after WHERE, before --limit).
	if n, supplied, mErr := sqlflags.MinAffectedFromCmd(cmd); mErr != nil {
		return mErr
	} else if supplied && len(rows) < n {
		return fmt.Errorf("matched %d records, required at least %d", len(rows), n)
	}

	// --limit: cap the result count after ordering.
	limitVal, _ := cmd.Flags().GetInt("limit")
	if limitVal < 0 {
		return fmt.Errorf("--limit must be >= 0, got %d", limitVal)
	}
	if limitVal > 0 && len(rows) > limitVal {
		rows = rows[:limitVal]
	}
```

Add `"sort"` to the imports.

Add a `compareValues` helper to `select_where.go`:

```go
// compareValues returns -1, 0, +1 comparing a and b. Numeric comparison
// is preferred when both can coerce; otherwise lexicographic on the
// fmt-formatted strings.
func compareValues(a, b any) int {
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok && bok {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	}
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	default:
		return 0
	}
}
```

- [ ] **Step 5.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/select.go cmd/ingitdb/commands/select_where.go cmd/ingitdb/commands/select_test.go
git commit -m "$(cat <<'EOF'
feat(cli/select): add --order-by, --limit, --min-affected

ORDER BY is applied after WHERE filtering via sort.SliceStable and a
compareValues helper that prefers numeric over lexicographic
comparison. LIMIT caps the result count after ORDER BY; --limit=0
means no cap; negatives rejected. --min-affected runs as a pre-flight
check before LIMIT and before any output is written; below-threshold
invocations return a diagnostic with the actual and required counts.

Spec:
- cli/select#req:limit-flag
- cli/select#req:order-then-limit
- cli/select#req:min-affected-flag
EOF
)"
```

---

## Task 6 — GitHub source support

**Context:** The spec requires `select --github=owner/repo[@REF]` to read from a remote repository without a local clone. The existing `read_record_github.go` shows the pattern (`resolveGitHubRecordContext` + `gitHubDBFactory.NewGitHubDBWithDef`). This task wires both modes to use the GitHub path when `--github` is supplied.

**Files:**
- Modify: `cmd/ingitdb/commands/select.go`
- Modify: `cmd/ingitdb/commands/select_test.go`

- [ ] **Step 6.1 — Inspect the GitHub seam**

```bash
grep -n "parseGitHubRepoSpec\|resolveGitHubRecordContext\|newGitHubConfig\|gitHubDBFactory" cmd/ingitdb/commands/read_record_github.go cmd/ingitdb/commands/record_context.go cmd/ingitdb/commands/seams.go
```

Verify the helpers exist. The single-record GitHub path is `resolveGitHubRecordContext(ctx, cmd, id, githubValue)`. The set-mode equivalent is `readRemoteDefinitionForCollection(ctx, spec, collectionID)` if it exists; if not, build it from the same primitives by calling `readRemoteDefinitionForID` with a synthetic ID like `<collectionID>/` (verify by reading the function).

If a clean set-mode GitHub helper does NOT exist, this task may need to introduce one — in which case STOP and report the gap rather than guess. Report `BLOCKED: need a set-mode GitHub-source reader; only single-record one exists.`

- [ ] **Step 6.2 — Write the failing test for --path/--github mutual exclusion**

Append to `cmd/ingitdb/commands/select_test.go`:

```go
func TestSelect_PathAndGitHubMutuallyExclusive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--path="+dir, "--github=foo/bar", "--id=todo.items/x")
	if err == nil {
		t.Fatal("expected error when both --path and --github supplied")
	}
}
```

- [ ] **Step 6.3 — Confirm failure (or pass if pre-existing helpers already reject)**

```bash
go test -timeout=10s -run TestSelect_PathAndGitHub ./cmd/ingitdb/commands/
```

If the test already passes because `resolveRecordContext` rejects the combo, that's fine — no implementation needed for single-record. Move to set-mode GitHub support.

- [ ] **Step 6.4 — Implement GitHub branch in `runSelectFromSet`**

In `runSelectFromSet`, before resolving `dirPath`, check for `--github`:

```go
	githubVal, _ := cmd.Flags().GetString("github")
	pathVal, _ := cmd.Flags().GetString("path")
	if githubVal != "" && pathVal != "" {
		return fmt.Errorf("--path with --github is not supported")
	}
	if githubVal != "" {
		spec, parseErr := parseGitHubRepoSpec(githubVal)
		if parseErr != nil {
			return parseErr
		}
		def, readErr := readRemoteDefinition(ctx, spec)
		if readErr != nil {
			return fmt.Errorf("failed to resolve remote definition: %w", readErr)
		}
		if _, ok := def.Collections[from]; !ok {
			return fmt.Errorf("collection %q not found in definition", from)
		}
		cfg := newGitHubConfig(spec, githubToken(cmd))
		db, dbErr := gitHubDBFactory.NewGitHubDBWithDef(cfg, def)
		if dbErr != nil {
			return fmt.Errorf("failed to open github database: %w", dbErr)
		}
		return runSelectFromSetWithDB(ctx, cmd, from, fields, format, db, cmd.Flags())
	}
```

`readRemoteDefinition` may not exist with that exact name; the existing `read_record_github.go` calls `readRemoteDefinitionForID` and parses an ID. For set mode, use whatever helper exposes "give me the Definition for this repo" — if only the `ForID` variant exists, document a follow-up plan to add a `ForCollection` variant and **for now** treat unknown helper names as BLOCKED.

Extract the post-resolution body of `runSelectFromSet` (everything from `qb := dal.NewQueryBuilder(...)` through the final `writeSetMode`) into a helper `runSelectFromSetWithDB(ctx, cmd, from, fields, format, db, flags)` so both the local and GitHub paths share the same execution code.

- [ ] **Step 6.5 — Run all tests; commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/select.go cmd/ingitdb/commands/select_test.go
git commit -m "$(cat <<'EOF'
feat(cli/select): add --github source support for both modes

Single-record mode inherits GitHub resolution via the existing
resolveRecordContext seam. Set mode now branches on --github: it
parses the repo spec, reads the remote definition, opens a GitHub-
backed DB, and reuses runSelectFromSetWithDB for the actual query.
--path and --github stay mutually exclusive.

Spec: cli/select#req:source-selection
EOF
)"
```

---

## Task 7 — Final integration test + spec cross-check

**Context:** Add one end-to-end test that exercises a realistic invocation and a regression test confirming the legacy `query` and `read record` commands still work. Verify every AC in `cli/select/README.md` has a corresponding passing test.

**Files:**
- Modify: `cmd/ingitdb/commands/select_test.go`

- [ ] **Step 7.1 — Write the end-to-end test**

Append to `cmd/ingitdb/commands/select_test.go`:

```go
func TestSelect_EndToEnd_RealisticInvocation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	for i, key := range []string{"low", "mid", "high"} {
		pri := float64((i + 1) * 10)
		if err := seedRecord(t, dir, "todo.items", key, map[string]any{
			"priority": pri,
			"title":    "T-" + key,
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	stdout, _, err := captureOutput(t, func() error {
		return runCobraCommand(cmd,
			"--path="+dir, "--from=todo.items",
			"--where=priority>=20",
			"--order-by=-priority",
			"--fields=$id,priority,title",
			"--format=json",
			"--min-affected=1",
		)
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Result should be JSON array with two rows ordered high → mid.
	if !strings.Contains(stdout, `"$id": "high"`) {
		t.Errorf("missing $id:high in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"$id": "mid"`) {
		t.Errorf("missing $id:mid in output:\n%s", stdout)
	}
	if strings.Contains(stdout, `"$id": "low"`) {
		t.Errorf("unexpected $id:low (priority=10 should be filtered out):\n%s", stdout)
	}
	idxHigh := strings.Index(stdout, `"$id": "high"`)
	idxMid := strings.Index(stdout, `"$id": "mid"`)
	if idxHigh > idxMid {
		t.Errorf("expected high before mid (descending priority), got high@%d, mid@%d", idxHigh, idxMid)
	}
}

func TestSelect_LegacyCommandsStillWork(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "todo.items", "a", map[string]any{"title": "Alpha"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// read record (legacy)
	readCmd := Read(homeDir, getWd, readDef, newDB, logf)
	_, _, err := captureOutput(t, func() error {
		return runCobraCommand(readCmd, "record", "--path="+dir, "--id=todo.items/a")
	})
	if err != nil {
		t.Errorf("legacy `read record` regressed: %v", err)
	}
	// query (legacy)
	queryCmd := Query(homeDir, getWd, readDef, newDB, logf)
	_, _, err = captureOutput(t, func() error {
		return runCobraCommand(queryCmd, "--path="+dir, "--collection=todo.items")
	})
	if err != nil {
		t.Errorf("legacy `query` regressed: %v", err)
	}
}
```

- [ ] **Step 7.2 — Run the whole-repo test suite**

```bash
go test -timeout=30s ./...
golangci-lint run
```

Expected: 0 failures, 0 lint issues. If `TestSelect_EndToEnd_RealisticInvocation` fails on the high-vs-mid ordering check, the JSON marshaller may not preserve insertion order — adapt the assertion to extract the ordered slice via `json.Unmarshal` into `[]map[string]any` and assert the slice order directly rather than substring positions.

- [ ] **Step 7.3 — Spec AC cross-check**

Open `spec/features/cli/select/README.md`. For each of the 9 ACs (single-record-yaml-default, single-record-not-found, single-record-rejects-set-flags, set-mode-csv-default, where-filter-and-order, set-mode-single-match-still-a-list, empty-set-result, limit-validation, reads-from-github, rejects-non-select-flags), find at least one test in `select_test.go` that exercises it. List any gaps in the commit message below.

- [ ] **Step 7.4 — Final commit**

```bash
go test -timeout=30s ./...
golangci-lint run
git add cmd/ingitdb/commands/select_test.go
git commit -m "$(cat <<'EOF'
test(cli/select): add end-to-end and legacy-regression tests

End-to-end covers --from + --where + --order-by + --fields + --format
+ --min-affected in one realistic invocation, asserting the JSON
result is the expected filtered, ordered, projected shape.

Legacy regression confirms `ingitdb read record --id` and
`ingitdb query --collection` still work alongside the new `select`
command — the SQL-verb redesign does not break the existing CLI
surface during the migration window.
EOF
)"
```

---

## Self-Review

**1. Spec coverage check.** Walking `cli/select/README.md` REQs:

| REQ | Task |
|---|---|
| `subcommand-name` | 1 (scaffold) |
| `mode-selection` | 1 (ResolveMode wired) |
| `single-record-shape` | 3 |
| `single-record-not-found` | 3 |
| `single-record-rejected-flags` | 3 (explicit rejects for --where/--order-by/--limit/--min-affected) |
| `set-mode-shape` | 4 |
| `set-mode-empty-result` | 4 (with format-appropriate empty body) |
| `limit-flag` | 5 |
| `order-then-limit` | 5 |
| `format-flag` | 3 + 4 (mode-dependent default) |
| `format-tabular-columns` | 4 (delegated to writeCSV/writeMarkdown) |
| `format-yaml-json-shape` | 3 + 4 (bare vs list shape) |
| `source-selection` | 6 |
| (verb-level) `min-affected-semantics` | 5 |
| (verb-level) `strict-equality-yaml-types` | 2 (evalWhere with type-strict path; YAML schema-aware date/time is deferred — note as follow-up) |

**Gap:** `strict-equality-yaml-types` for schema-declared date/time columns is not implemented; this requires schema lookup. The plan covers strict equality on JSON scalars (the bulk of `req:strict-equality`); YAML date/time strictness is deferred to a follow-up plan when a real use case appears. Document this in Task 7's commit message.

**2. Placeholder scan.** No `TBD`, `TODO`, or `implement later` in any task. Every code block is complete and runnable.

**3. Type consistency.** `sqlflags.Condition` used in Tasks 2, 4. `sqlflags.OrderTerm` used in Task 5. `evalAllWhere` defined in Task 2, called in Task 4. `compareValues` defined in Task 5, used in Task 5. `writeSingleRecord` defined in Task 3, called in Task 3. `writeSetMode` defined in Task 4, called in Tasks 4 and 5. `runSelectFromSetWithDB` defined in Task 6; Task 4's `runSelectFromSet` is refactored in Task 6 to call it — call sites match.

---

## Execution Handoff

**Plan complete and saved to `spec/plans/2026-05-12-cli-select.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
