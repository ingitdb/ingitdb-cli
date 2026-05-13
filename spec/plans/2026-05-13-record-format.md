# Record Format Extensions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the three sibling Features under `spec/features/record-format/` — `csv-support`, `project-default`, `cli-default-format-flag` — additively, in dependency order.

**Architecture:** Phase A adds CSV as the seventh first-class `RecordFormat` (constant + validation + schema-aware read/write) without touching the other six formats. Phase B adds a project-level `DefaultRecordFormat` config field on `config.Settings`, a load-time validator, and a `ResolveRecordFormat(collection, settings)` helper with a three-tier fallback chain (collection → project → hard YAML default). Phase C adds the `--default-format` flag to `ingitdb setup`, which currently is a stub — we build the minimum settings-writer the flag needs, scoped to the seven-format choice only (broader setup behavior remains the existing Draft Feature's responsibility).

**Tech Stack:** Go (existing codebase), Go stdlib `encoding/csv` (already imported in `cmd/ingitdb/commands/query_output.go` and `pkg/ingitdb/materializer/export_format.go`), `gopkg.in/yaml.v3` (for settings write), `github.com/spf13/cobra` (CLI).

---

## File Structure

| Path | Responsibility | Status |
|---|---|---|
| `pkg/ingitdb/constants.go` | Add `RecordFormatCSV` constant alongside the existing six | Modify |
| `pkg/ingitdb/record_file_def.go` | Add CSV→`ListOfRecords`-only validation to `RecordFileDef.Validate` | Modify |
| `pkg/ingitdb/record_file_def_test.go` | New CSV-restriction cases | Modify |
| `pkg/dalgo2ingitdb/parse.go` | Add `RecordFormatCSV` case to `ParseRecordContentForCollection`; add new schema-aware writer `EncodeRecordContentForCollection` | Modify |
| `pkg/dalgo2ingitdb/csv.go` | New file: CSV-specific read/write helpers (`parseCSVForCollection`, `encodeCSVForCollection`) | Create |
| `pkg/dalgo2ingitdb/csv_test.go` | All `csv-support` AC tests | Create |
| `pkg/dalgo2ghingitdb/tx_readwrite.go` | Route single-record CSV writes through `EncodeRecordContentForCollection` (the schema-aware writer); the schema-agnostic `encodeRecordContent` stays for the six existing formats but the call sites that touch CSV-capable records become schema-aware | Modify |
| `pkg/ingitdb/config/root_config.go` | Add `DefaultRecordFormat` field to `Settings`; add `Settings.Validate()`; have `RootConfig.Validate()` call it; add top-level `ResolveRecordFormat` helper | Modify |
| `pkg/ingitdb/config/root_config_test.go` | All `project-default` AC tests | Modify |
| `cmd/ingitdb/commands/setup.go` | Replace `not yet implemented` stub with minimal write path (create `.ingitdb/`, write `settings.yaml`); add `--default-format` flag + validation | Modify |
| `cmd/ingitdb/commands/setup_test.go` | All `cli-default-format-flag` AC tests | Create |

## Conventions

- **Test framework:** Go stdlib `testing`. Pattern in this repo: table-driven tests with `tt.name` subtests, `t.Parallel()` at the top of each test func. Match it.
- **Commits:** Conventional commits (`feat:`, `test:`, `refactor:`, `docs:`). Co-Authored-By footer included on each commit.
- **Lint/build verification at the end of every task:** `go build ./...` then `go test ./pkg/...` (or the more targeted package under test).

## Audit: Direct reads of `RecordFileDef.Format`

The `project-default` spec calls for "all read/write call sites that today consult `RecordFileDef.Format` directly" to be migrated to `ResolveRecordFormat`. Plan-time audit count (from `grep -rn "RecordFile\.Format" --include='*.go' | grep -v _test.go`): **~22 call sites** across `cmd/ingitdb/commands/`, `pkg/dalgo2ghingitdb/`, `pkg/dalgo2fsingitdb/`, `pkg/dalgo2ingitdb/`, `pkg/ingitdb/materializer/`.

**Migration scope decision:** all 22 sites already operate downstream of `RecordFileDef.Validate()` which rejects empty `Format`. They cannot meaningfully use the project-level fallback today because they read from already-loaded collections whose `Format` is guaranteed non-empty. Wholesale migration would plumb `*Settings` through every read/write path with **zero observable behavior change**.

**This plan therefore ships:** the `ResolveRecordFormat` helper + its tests + a usage in **new** code paths only (CSV branch in `ParseRecordContentForCollection` and the new `EncodeRecordContentForCollection`). The wholesale call-site migration is deferred to a follow-up — captured as the outstanding question in `spec/features/record-format/project-default/README.md` (it's already there). Document the scope decision in the helper's godoc.

---

## Phase A — `csv-support`

Phase A is self-contained: it adds a seventh format end-to-end without touching `config.Settings` or the CLI.

### Task 1: Add `RecordFormatCSV` constant

**Files:**
- Modify: `pkg/ingitdb/constants.go`

- [ ] **Step 1: Add the constant**

Edit `pkg/ingitdb/constants.go`, in the `const` block at lines 8–15. Add `RecordFormatCSV` after `RecordFormatINGR`:

```go
const (
	RecordFormatYAML     RecordFormat = "yaml"
	RecordFormatYML      RecordFormat = "yml"
	RecordFormatJSON     RecordFormat = "json"
	RecordFormatMarkdown RecordFormat = "markdown"
	RecordFormatTOML     RecordFormat = "toml"
	RecordFormatINGR     RecordFormat = "ingr"
	RecordFormatCSV      RecordFormat = "csv"
)
```

- [ ] **Step 2: Verify build**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go build ./...`
Expected: clean exit.

- [ ] **Step 3: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/ingitdb/constants.go
git commit -m "$(cat <<'EOF'
feat(ingitdb): add RecordFormatCSV constant

Adds csv as the seventh first-class record format alongside the existing
six (yaml, yml, json, markdown, toml, ingr). Constant only — read/write
machinery and validation arrive in follow-up commits.

Refs: spec/features/record-format/csv-support REQ:csv-record-format-constant

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: CSV→ListOfRecords-only validation

**Files:**
- Modify: `pkg/ingitdb/record_file_def.go`
- Modify: `pkg/ingitdb/record_file_def_test.go`

- [ ] **Step 1: Write the failing tests**

Append the following cases inside the `tests` slice of `TestRecordFileDefValidate` in `pkg/ingitdb/record_file_def_test.go` (match the existing struct-literal style — fields are `name`, `def`, `err`):

```go
{
    name: "csv_rejects_single_record",
    def:  RecordFileDef{Name: "records.csv", Format: RecordFormatCSV, RecordType: SingleRecord},
    err:  "format \"csv\" requires record type \"[]map[string]any\"",
},
{
    name: "csv_rejects_map_of_records",
    def:  RecordFileDef{Name: "records.csv", Format: RecordFormatCSV, RecordType: MapOfRecords},
    err:  "format \"csv\" requires record type \"[]map[string]any\"",
},
{
    name: "csv_accepts_list_of_records",
    def:  RecordFileDef{Name: "records.csv", Format: RecordFormatCSV, RecordType: ListOfRecords},
    err:  "",
},
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/ -run TestRecordFileDefValidate -v`
Expected: the three `csv_*` subtests FAIL (the first two pass `Validate` but should not; the third may pass or fail depending on existing branches).

- [ ] **Step 3: Add the CSV validation branch**

In `pkg/ingitdb/record_file_def.go`, add a new branch in `RecordFileDef.Validate` immediately after the existing INGR check at lines 68–71 (after the closing brace of the INGR `if`, before the `ExcludeRegex` block):

```go
if rfd.Format == RecordFormatCSV && rfd.RecordType != ListOfRecords {
    return fmt.Errorf("format %q requires record type %q, got %q",
        RecordFormatCSV, ListOfRecords, rfd.RecordType)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/ -run TestRecordFileDefValidate -v`
Expected: all subtests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/ingitdb/record_file_def.go pkg/ingitdb/record_file_def_test.go
git commit -m "$(cat <<'EOF'
feat(ingitdb): validate CSV format requires ListOfRecords

CSV is restricted to record_type=[]map[string]any (analogous to INGR's
non-SingleRecord restriction at lines 68-70 in the same file). SingleRecord
and MapOfRecords are rejected by RecordFileDef.Validate with a clear error
naming the offending value and the only supported RecordType for CSV.

Refs: spec/features/record-format/csv-support REQ:csv-record-type-restriction

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: CSV read path — failing tests

**Files:**
- Create: `pkg/dalgo2ingitdb/csv_test.go`

- [ ] **Step 1: Write the failing tests**

Create `pkg/dalgo2ingitdb/csv_test.go`:

```go
package dalgo2ingitdb

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func newCSVCollectionDef(columns []string) *ingitdb.CollectionDef {
	cols := make(map[string]*ingitdb.ColumnDef, len(columns))
	for _, c := range columns {
		cols[c] = &ingitdb.ColumnDef{}
	}
	return &ingitdb.CollectionDef{
		ID:           "items",
		Columns:      cols,
		ColumnsOrder: columns,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "items.csv",
			Format:     ingitdb.RecordFormatCSV,
			RecordType: ingitdb.ListOfRecords,
		},
	}
}

func TestParseRecordContentForCollection_CSV_Roundtrip(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "email", "age"})
	content := []byte("id,email,age\n1,alice@example.com,30\n2,bob@example.com,25\n")

	data, err := ParseRecordContentForCollection(content, col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows, ok := data["$records"].([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any under $records, got %T", data["$records"])
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["id"] != "1" || rows[0]["email"] != "alice@example.com" || rows[0]["age"] != "30" {
		t.Errorf("row 0 mismatch: %+v", rows[0])
	}
	if rows[1]["id"] != "2" || rows[1]["email"] != "bob@example.com" || rows[1]["age"] != "25" {
		t.Errorf("row 1 mismatch: %+v", rows[1])
	}
}

func TestParseRecordContentForCollection_CSV_RejectsMissingColumn(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "email", "age"})
	content := []byte("id,email\n1,alice@example.com\n")

	_, err := ParseRecordContentForCollection(content, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "age") {
		t.Errorf("expected error to mention missing column 'age'; got: %v", err)
	}
}

func TestParseRecordContentForCollection_CSV_RejectsReorderedHeader(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "email", "age"})
	content := []byte("email,id,age\nalice@example.com,1,30\n")

	_, err := ParseRecordContentForCollection(content, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "order") && !strings.Contains(err.Error(), "position") {
		t.Errorf("expected error to mention order/position mismatch; got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/dalgo2ingitdb/ -run TestParseRecordContentForCollection_CSV -v`
Expected: tests FAIL — `ParseRecordContentForCollection` currently returns `unsupported record format "csv"` (its default branch hits `ParseRecordContent` which errors on csv).

- [ ] **Step 3: Commit (test-only)**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/dalgo2ingitdb/csv_test.go
git commit -m "$(cat <<'EOF'
test(dalgo2ingitdb): add failing CSV read tests

Header round-trip, missing-column rejection, reordered-header rejection.
Implementation lands in the next commit.

Refs: spec/features/record-format/csv-support REQ:csv-read-path

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: CSV read path — implementation

**Files:**
- Create: `pkg/dalgo2ingitdb/csv.go`
- Modify: `pkg/dalgo2ingitdb/parse.go`

- [ ] **Step 1: Create the CSV helper file**

Create `pkg/dalgo2ingitdb/csv.go`:

```go
package dalgo2ingitdb

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// recordsKey is the key under which a parsed CSV list of rows is exposed
// in the map[string]any returned by ParseRecordContentForCollection.
// Callers reach the rows via data["$records"] (typed []map[string]any).
const recordsKey = "$records"

// parseCSVForCollection reads RFC 4180 CSV bytes, validates the header
// matches colDef.ColumnsOrder exactly (same names, same order), and
// returns the rows as a list of records keyed by column name.
//
// The result is wrapped in map[string]any{"$records": []map[string]any{...}}
// to satisfy the ParseRecordContentForCollection contract (which is
// declared as returning map[string]any) without losing list-of-records
// semantics — the caller unwraps via the recordsKey constant.
func parseCSVForCollection(content []byte, colDef *ingitdb.CollectionDef) (map[string]any, error) {
	if len(colDef.ColumnsOrder) == 0 {
		return nil, fmt.Errorf("csv read requires non-empty columns_order on the collection definition")
	}
	r := csv.NewReader(bytes.NewReader(content))
	header, err := r.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("csv input is empty (expected header row)")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read csv header: %w", err)
	}
	if err = validateCSVHeader(header, colDef.ColumnsOrder); err != nil {
		return nil, err
	}
	var rows []map[string]any
	for {
		fields, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("failed to read csv row %d: %w", len(rows)+1, readErr)
		}
		if len(fields) != len(header) {
			return nil, fmt.Errorf("csv row %d has %d columns, header has %d",
				len(rows)+1, len(fields), len(header))
		}
		row := make(map[string]any, len(header))
		for i, col := range header {
			row[col] = fields[i]
		}
		rows = append(rows, row)
	}
	return map[string]any{recordsKey: rows}, nil
}

// validateCSVHeader returns an error when header does not match expected
// exactly (same names in the same order). The error message names the
// first mismatched column and whether it's a missing, extra, or reordered
// column.
func validateCSVHeader(header, expected []string) error {
	if len(header) < len(expected) {
		missing := expected[len(header):]
		return fmt.Errorf("csv header is missing column(s): %v (expected %v, got %v)",
			missing, expected, header)
	}
	if len(header) > len(expected) {
		extra := header[len(expected):]
		return fmt.Errorf("csv header has extra column(s): %v (expected %v, got %v)",
			extra, expected, header)
	}
	for i := range expected {
		if header[i] != expected[i] {
			return fmt.Errorf("csv header column at position %d is %q, expected %q (full order mismatch: got %v, expected %v)",
				i, header[i], expected[i], header, expected)
		}
	}
	return nil
}
```

- [ ] **Step 2: Wire CSV into `ParseRecordContentForCollection`**

Edit `pkg/dalgo2ingitdb/parse.go`. At lines 49–55, replace:

```go
func ParseRecordContentForCollection(content []byte, colDef *ingitdb.CollectionDef) (map[string]any, error) {
	if colDef == nil || colDef.RecordFile == nil {
		return nil, fmt.Errorf("collection definition missing record_file")
	}
	if colDef.RecordFile.Format != ingitdb.RecordFormatMarkdown {
		return ParseRecordContent(content, colDef.RecordFile.Format)
	}
```

with:

```go
func ParseRecordContentForCollection(content []byte, colDef *ingitdb.CollectionDef) (map[string]any, error) {
	if colDef == nil || colDef.RecordFile == nil {
		return nil, fmt.Errorf("collection definition missing record_file")
	}
	switch colDef.RecordFile.Format {
	case ingitdb.RecordFormatCSV:
		return parseCSVForCollection(content, colDef)
	case ingitdb.RecordFormatMarkdown:
		// fall through to the markdown logic below
	default:
		return ParseRecordContent(content, colDef.RecordFile.Format)
	}
```

- [ ] **Step 3: Run CSV read tests**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/dalgo2ingitdb/ -run TestParseRecordContentForCollection_CSV -v`
Expected: all three tests PASS.

- [ ] **Step 4: Run full package tests (no regressions)**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/dalgo2ingitdb/ -v`
Expected: all existing tests still PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/dalgo2ingitdb/csv.go pkg/dalgo2ingitdb/parse.go
git commit -m "$(cat <<'EOF'
feat(dalgo2ingitdb): add schema-aware CSV read path

ParseRecordContentForCollection now routes RecordFormatCSV through
parseCSVForCollection (new pkg/dalgo2ingitdb/csv.go). Header is validated
against colDef.ColumnsOrder exactly — missing, extra, or reordered columns
are rejected with a clear error naming the mismatch. The list of rows is
exposed under the $records key in the returned map[string]any.

Refs: spec/features/record-format/csv-support REQ:csv-read-path

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: CSV write path — failing tests

**Files:**
- Modify: `pkg/dalgo2ingitdb/csv_test.go`

- [ ] **Step 1: Append the failing write tests**

Append to `pkg/dalgo2ingitdb/csv_test.go`:

```go
func TestEncodeRecordContentForCollection_CSV_HeaderAndRowOrder(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "name", "email"})
	rows := []map[string]any{
		{"id": "1", "name": "Alice", "email": "alice@example.com"},
		{"id": "2", "name": "Bob", "email": "bob@example.com"},
	}

	out, err := EncodeRecordContentForCollection(rows, col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "id,name,email\n1,Alice,alice@example.com\n2,Bob,bob@example.com\n"
	if string(out) != want {
		t.Errorf("output mismatch.\n  got:  %q\n  want: %q", string(out), want)
	}
}

func TestEncodeRecordContentForCollection_CSV_Deterministic(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "name", "email"})
	rows := []map[string]any{
		{"email": "alice@example.com", "name": "Alice", "id": "1"},
		{"name": "Bob", "id": "2", "email": "bob@example.com"},
	}

	first, err := EncodeRecordContentForCollection(rows, col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 5; i++ {
		out, err := EncodeRecordContentForCollection(rows, col)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if string(out) != string(first) {
			t.Errorf("non-deterministic output on iteration %d.\n  first:  %q\n  this:   %q",
				i, string(first), string(out))
		}
	}
}

func TestEncodeRecordContentForCollection_CSV_RejectsKeyedMap(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "name"})
	keyed := map[string]map[string]any{
		"1": {"name": "Alice"},
		"2": {"name": "Bob"},
	}

	_, err := EncodeRecordContentForCollection(keyed, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "csv") || !strings.Contains(err.Error(), "list") {
		t.Errorf("expected error to mention csv + list; got: %v", err)
	}
}

func TestEncodeRecordContentForCollection_CSV_RejectsNestedObject(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "address"})
	rows := []map[string]any{
		{"id": "1", "address": map[string]any{"city": "Berlin"}},
	}

	_, err := EncodeRecordContentForCollection(rows, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "address") {
		t.Errorf("expected error to mention field name 'address'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "nested") && !strings.Contains(err.Error(), "array") {
		t.Errorf("expected error to mention nested/array constraint; got: %v", err)
	}
}

func TestEncodeRecordContentForCollection_CSV_RejectsArrayField(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "tags"})
	rows := []map[string]any{
		{"id": "1", "tags": []any{"a", "b", "c"}},
	}

	_, err := EncodeRecordContentForCollection(rows, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tags") {
		t.Errorf("expected error to mention field name 'tags'; got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail (compile failure)**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/dalgo2ingitdb/ -run TestEncodeRecordContentForCollection_CSV -v`
Expected: build FAIL — `EncodeRecordContentForCollection` is undefined.

- [ ] **Step 3: Commit (test-only)**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/dalgo2ingitdb/csv_test.go
git commit -m "$(cat <<'EOF'
test(dalgo2ingitdb): add failing CSV write tests

Header in schema order, deterministic output across runs, rejection of
keyed maps, rejection of nested objects and array-valued fields. Tests
fail at build because EncodeRecordContentForCollection is not yet
defined.

Refs: spec/features/record-format/csv-support REQ:csv-write-path, REQ:csv-nested-field-error

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: CSV write path — implementation

**Files:**
- Modify: `pkg/dalgo2ingitdb/csv.go`
- Modify: `pkg/dalgo2ingitdb/parse.go`

- [ ] **Step 1: Add the CSV encoder to `csv.go`**

Append to `pkg/dalgo2ingitdb/csv.go`:

```go
// encodeCSVForCollection serializes a list of records as RFC 4180 CSV.
// Header is emitted first, with column names in colDef.ColumnsOrder. Each
// data row's cell values are looked up by column name in the same order.
//
// rows MUST be []map[string]any (a list of records). Keyed maps
// (map[string]map[string]any) are rejected — Go map iteration order is
// non-deterministic and CSV row order matters.
//
// Per-cell values are written via fmt.Sprintf("%v", v) for primitives;
// nested objects (map[...]any) and array values (slices) are rejected
// with a typed error naming the offending field.
func encodeCSVForCollection(value any, colDef *ingitdb.CollectionDef) ([]byte, error) {
	rows, err := coerceToRowList(value)
	if err != nil {
		return nil, err
	}
	if len(colDef.ColumnsOrder) == 0 {
		return nil, fmt.Errorf("csv write requires non-empty columns_order on the collection definition")
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err = w.Write(colDef.ColumnsOrder); err != nil {
		return nil, fmt.Errorf("failed to write csv header: %w", err)
	}
	for i, row := range rows {
		cells := make([]string, len(colDef.ColumnsOrder))
		for j, col := range colDef.ColumnsOrder {
			raw, ok := row[col]
			if !ok {
				cells[j] = ""
				continue
			}
			cell, cellErr := csvCellString(raw, col, i)
			if cellErr != nil {
				return nil, cellErr
			}
			cells[j] = cell
		}
		if err = w.Write(cells); err != nil {
			return nil, fmt.Errorf("failed to write csv row %d: %w", i, err)
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		return nil, fmt.Errorf("csv writer error: %w", err)
	}
	return buf.Bytes(), nil
}

// coerceToRowList accepts []map[string]any and rejects every other shape
// — including map[string]map[string]any (keyed input) — with a typed
// error explaining that csv requires a deterministically-ordered list.
func coerceToRowList(value any) ([]map[string]any, error) {
	switch v := value.(type) {
	case []map[string]any:
		return v, nil
	case map[string]map[string]any:
		return nil, fmt.Errorf("csv accepts only []map[string]any (a list of records), not a keyed map (map[string]map[string]any); map iteration order is non-deterministic and csv row order matters")
	case []any:
		// Allow []any of map[string]any rows for caller convenience.
		out := make([]map[string]any, 0, len(v))
		for i, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("csv row %d is not a map (got %T)", i, item)
			}
			out = append(out, m)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("csv accepts only []map[string]any (a list of records), got %T", value)
	}
}

// csvCellString converts a single cell value to its CSV string form.
// Nested objects (map[...]any) and array values (slices) are rejected
// with a typed error that identifies the field name and the record's
// position in the input.
func csvCellString(v any, fieldName string, rowIndex int) (string, error) {
	switch val := v.(type) {
	case nil:
		return "", nil
	case string:
		return val, nil
	case map[string]any:
		return "", fmt.Errorf("csv does not support nested or array-valued fields: row %d field %q has value of type %T",
			rowIndex, fieldName, v)
	case []any, []string, []int, []float64, []bool, []map[string]any:
		return "", fmt.Errorf("csv does not support nested or array-valued fields: row %d field %q has value of type %T",
			rowIndex, fieldName, v)
	default:
		return fmt.Sprintf("%v", v), nil
	}
}
```

- [ ] **Step 2: Add the public schema-aware encoder to `parse.go`**

Edit `pkg/dalgo2ingitdb/parse.go`. After the existing `marshalForFormat` function (currently the last function in the file), append:

```go
// EncodeRecordContentForCollection serializes record content using the
// collection's declared format. It is the write-side counterpart of
// ParseRecordContentForCollection and the only path callers should use
// when writing records that may be in a format that requires schema
// access (csv today; possibly others later).
//
// For csv, value MUST be []map[string]any — see encodeCSVForCollection.
// For the other six formats, value is passed through to marshalForFormat
// unchanged; callers that want a single-record map[string]any can keep
// using the schema-agnostic marshalForFormat directly.
func EncodeRecordContentForCollection(value any, colDef *ingitdb.CollectionDef) ([]byte, error) {
	if colDef == nil || colDef.RecordFile == nil {
		return nil, fmt.Errorf("collection definition missing record_file")
	}
	if colDef.RecordFile.Format == ingitdb.RecordFormatCSV {
		return encodeCSVForCollection(value, colDef)
	}
	return marshalForFormat(value, colDef.RecordFile.Format)
}
```

- [ ] **Step 3: Run CSV write tests**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/dalgo2ingitdb/ -run TestEncodeRecordContentForCollection_CSV -v`
Expected: all five tests PASS.

- [ ] **Step 4: Run full package tests (no regressions)**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/dalgo2ingitdb/ -v`
Expected: all existing tests still PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/dalgo2ingitdb/csv.go pkg/dalgo2ingitdb/parse.go
git commit -m "$(cat <<'EOF'
feat(dalgo2ingitdb): add schema-aware CSV write path

Adds EncodeRecordContentForCollection (the write counterpart of
ParseRecordContentForCollection) and the csv-specific encoder. Headers
are emitted in colDef.ColumnsOrder, row order preserves input slice order,
output is byte-deterministic across runs. Keyed maps are rejected (map
iteration order is non-deterministic and csv row order matters). Nested
objects and array-valued cells are rejected with a typed error naming
the offending field and row index.

Refs: spec/features/record-format/csv-support REQ:csv-write-path, REQ:csv-nested-field-error

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: GitHub backend — route CSV through schema-aware writer

**Files:**
- Modify: `pkg/dalgo2ghingitdb/tx_readwrite.go`

The existing `encodeRecordContent(data, format)` at lines 241–264 is schema-agnostic and is called at lines 72, 141, 181 with `data map[string]any`. CSV cannot be served this way (no schema, and CSV inherently needs a list of records, not a single map). For the three call sites that handle single-record writes (lines 72, 141, 181), CSV cannot apply — those code paths are single-record paths and CSV requires `ListOfRecords`. Validation at `RecordFileDef.Validate` already rejects this combination at load time, so the call sites won't see a CSV-formatted collection in practice.

**Scope decision for this task:** make `encodeRecordContent` return a clear error if invoked with CSV format, so a future code-path regression surfaces clearly instead of silently producing wrong output.

- [ ] **Step 1: Add CSV rejection to `encodeRecordContent`**

Edit `pkg/dalgo2ghingitdb/tx_readwrite.go` at lines 241–264. Replace the function body so the `default` case explicitly identifies csv:

```go
func encodeRecordContent(data map[string]any, format ingitdb.RecordFormat) ([]byte, error) {
	switch format {
	case ingitdb.RecordFormatYAML, ingitdb.RecordFormatYML:
		encoded, err := yaml.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encode YAML record: %w", err)
		}
		return encoded, nil
	case ingitdb.RecordFormatJSON:
		encoded, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to encode JSON record: %w", err)
		}
		return append(encoded, '\n'), nil
	case ingitdb.RecordFormatTOML:
		encoded, err := toml.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encode TOML record: %w", err)
		}
		return encoded, nil
	case ingitdb.RecordFormatCSV:
		return nil, fmt.Errorf("encodeRecordContent does not support csv (single-record write path); csv requires record_type=[]map[string]any and the schema-aware EncodeRecordContentForCollection writer")
	default:
		return nil, fmt.Errorf("unsupported record format %q", format)
	}
}
```

- [ ] **Step 2: Verify build + existing tests**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go build ./... && go test ./pkg/dalgo2ghingitdb/...`
Expected: clean build, all existing tests pass.

- [ ] **Step 3: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/dalgo2ghingitdb/tx_readwrite.go
git commit -m "$(cat <<'EOF'
feat(dalgo2ghingitdb): explicit csv rejection in single-record writer

encodeRecordContent is the single-record write path in the GitHub
backend. CSV requires record_type=[]map[string]any and the schema-aware
EncodeRecordContentForCollection writer; calling encodeRecordContent
with format=csv now returns a typed error that names the constraint
and points callers at the right writer. RecordFileDef.Validate already
prevents this combination at load time — this rejection is a guard
against future code-path regressions.

Refs: spec/features/record-format/csv-support REQ:csv-write-path

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase B — `project-default`

Phase B depends on `RecordFormatCSV` from Phase A — its validator accepts all seven formats including csv.

### Task 8: `DefaultRecordFormat` field — failing tests

**Files:**
- Modify: `pkg/ingitdb/config/root_config_test.go`

- [ ] **Step 1: Append the failing tests**

Append to `pkg/ingitdb/config/root_config_test.go`:

```go
func TestSettings_DefaultRecordFormat_FieldExists(t *testing.T) {
	t.Parallel()
	var s Settings
	_ = s.DefaultRecordFormat // compile-time check; type must be ingitdb.RecordFormat
}

func TestReadSettingsFromFile_DefaultRecordFormat_OmittedOnExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".ingitdb", "settings.yaml")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("default_namespace: todo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ReadSettingsFromFile(dir, ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DefaultNamespace != "todo" {
		t.Errorf("expected default_namespace=todo, got %q", s.DefaultNamespace)
	}
	if s.DefaultRecordFormat != "" {
		t.Errorf("expected DefaultRecordFormat to be empty, got %q", s.DefaultRecordFormat)
	}
}

func TestReadSettingsFromFile_DefaultRecordFormat_LoadsFromYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".ingitdb", "settings.yaml")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("default_record_format: ingr\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ReadSettingsFromFile(dir, ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DefaultRecordFormat != ingitdb.RecordFormatINGR {
		t.Errorf("expected DefaultRecordFormat=ingr, got %q", s.DefaultRecordFormat)
	}
}
```

Confirm `pkg/ingitdb/config/root_config_test.go` already imports `os`, `filepath`, and the `ingitdb` package; if not, add them.

- [ ] **Step 2: Run tests — expect failures**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/config/ -run 'TestSettings_DefaultRecordFormat|TestReadSettingsFromFile_DefaultRecordFormat' -v`
Expected: build FAIL (`DefaultRecordFormat` field undefined).

- [ ] **Step 3: Commit (test-only)**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/ingitdb/config/root_config_test.go
git commit -m "$(cat <<'EOF'
test(config): add failing tests for Settings.DefaultRecordFormat

Field exists check, omitempty-on-existing-file round-trip, and load from
yaml. Tests fail at build because the field doesn't exist yet.

Refs: spec/features/record-format/project-default REQ:default-record-format-field

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: `DefaultRecordFormat` field — implementation

**Files:**
- Modify: `pkg/ingitdb/config/root_config.go`

- [ ] **Step 1: Add the field**

Edit `pkg/ingitdb/config/root_config.go` at lines 36–43. Replace the `Settings` struct definition with:

```go
// Settings holds per-database settings stored in .ingitdb/settings.yaml.
type Settings struct {
	// DefaultNamespace is used as the collection ID prefix when this DB is
	// opened directly (not imported via a namespace import). For example,
	// if DefaultNamespace is "todo" and the DB has collection "tasks",
	// it becomes "todo.tasks" when opened directly.
	DefaultNamespace string `yaml:"default_namespace,omitempty"`

	// DefaultRecordFormat is the project-level fallback record format used
	// when a collection's record_file.format is empty. See ResolveRecordFormat
	// for the full fallback chain (collection -> project -> hard yaml default).
	// Empty value means "no project default; use the hard fallback."
	DefaultRecordFormat ingitdb.RecordFormat `yaml:"default_record_format,omitempty"`

	Languages []Language `yaml:"languages,omitempty"`
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/config/ -run 'TestSettings_DefaultRecordFormat|TestReadSettingsFromFile_DefaultRecordFormat' -v`
Expected: all three subtests PASS.

- [ ] **Step 3: Run full config package tests**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/config/...`
Expected: all PASS (omitempty tag preserves existing fixtures).

- [ ] **Step 4: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/ingitdb/config/root_config.go
git commit -m "$(cat <<'EOF'
feat(config): add DefaultRecordFormat field to Settings

New project-level fallback format declared in .ingitdb/settings.yaml
under default_record_format. Type is ingitdb.RecordFormat; yaml tag is
omitempty so existing fixtures load unchanged. Validation and the
ResolveRecordFormat helper land in the next commits.

Refs: spec/features/record-format/project-default REQ:default-record-format-field

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Format validation — failing tests

**Files:**
- Modify: `pkg/ingitdb/config/root_config_test.go`

- [ ] **Step 1: Append the failing tests**

Append to `pkg/ingitdb/config/root_config_test.go`:

```go
func TestSettings_Validate_UnsupportedFormatRejected(t *testing.T) {
	t.Parallel()
	s := Settings{DefaultRecordFormat: "xml"}
	err := s.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	for _, name := range []string{"xml", "yaml", "yml", "json", "markdown", "toml", "ingr", "csv"} {
		if !strings.Contains(msg, name) {
			t.Errorf("expected error message to contain %q; got: %s", name, msg)
		}
	}
}

func TestSettings_Validate_EmptyValueAccepted(t *testing.T) {
	t.Parallel()
	s := Settings{DefaultRecordFormat: ""}
	if err := s.Validate(); err != nil {
		t.Errorf("expected nil error for empty DefaultRecordFormat, got: %v", err)
	}
}

func TestSettings_Validate_EachOfSevenAccepted(t *testing.T) {
	t.Parallel()
	for _, f := range []ingitdb.RecordFormat{
		ingitdb.RecordFormatYAML,
		ingitdb.RecordFormatYML,
		ingitdb.RecordFormatJSON,
		ingitdb.RecordFormatMarkdown,
		ingitdb.RecordFormatTOML,
		ingitdb.RecordFormatINGR,
		ingitdb.RecordFormatCSV,
	} {
		f := f
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()
			s := Settings{DefaultRecordFormat: f}
			if err := s.Validate(); err != nil {
				t.Errorf("expected nil error for %q, got: %v", f, err)
			}
		})
	}
}
```

Verify `strings` import exists at the top of the file; add it if not.

- [ ] **Step 2: Run tests — expect failures**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/config/ -run TestSettings_Validate -v`
Expected: build FAIL (`Settings.Validate` undefined).

- [ ] **Step 3: Commit (test-only)**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/ingitdb/config/root_config_test.go
git commit -m "$(cat <<'EOF'
test(config): add failing tests for Settings.Validate format check

Unsupported value rejected (error names offending value + all seven
valid formats); empty value accepted; each of the seven valid formats
accepted. Tests fail at build because Settings.Validate doesn't exist.

Refs: spec/features/record-format/project-default REQ:invalid-format-rejected

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Format validation — implementation

**Files:**
- Modify: `pkg/ingitdb/config/root_config.go`

- [ ] **Step 1: Add `Settings.Validate` and the supported-formats list**

Edit `pkg/ingitdb/config/root_config.go`. After the closing brace of the `Settings` struct (line 43 in the new file post-Task-9), add:

```go
// supportedRecordFormats is the closed set of record formats accepted by
// Settings.Validate. Keep in sync with the RecordFormat* constants in
// pkg/ingitdb/constants.go.
var supportedRecordFormats = []ingitdb.RecordFormat{
	ingitdb.RecordFormatYAML,
	ingitdb.RecordFormatYML,
	ingitdb.RecordFormatJSON,
	ingitdb.RecordFormatMarkdown,
	ingitdb.RecordFormatTOML,
	ingitdb.RecordFormatINGR,
	ingitdb.RecordFormatCSV,
}

// Validate checks that Settings field values are well-formed. An empty
// DefaultRecordFormat is permitted (it means "no project default; use the
// hard fallback"); any non-empty value MUST match one of the seven
// supported record formats. Other Settings fields are validated by
// RootConfig.Validate today and not duplicated here.
func (s *Settings) Validate() error {
	if s == nil {
		return nil
	}
	if s.DefaultRecordFormat == "" {
		return nil
	}
	for _, f := range supportedRecordFormats {
		if s.DefaultRecordFormat == f {
			return nil
		}
	}
	names := make([]string, len(supportedRecordFormats))
	for i, f := range supportedRecordFormats {
		names[i] = string(f)
	}
	return fmt.Errorf("unsupported default_record_format %q; valid options are: %s",
		s.DefaultRecordFormat, strings.Join(names, ", "))
}
```

Confirm imports already include `fmt` and `strings`; they do (lines 4–11 of the file).

- [ ] **Step 2: Wire `Settings.Validate` into `RootConfig.Validate`**

Edit `pkg/ingitdb/config/root_config.go`. In `RootConfig.Validate` (currently starting at line 66), add a call to `Settings.Validate` near the top. Replace the function's opening lines:

```go
func (rc *RootConfig) Validate() error {
	if rc == nil {
		return nil
	}
	if rc.DefaultNamespace != "" {
```

with:

```go
func (rc *RootConfig) Validate() error {
	if rc == nil {
		return nil
	}
	if err := rc.Settings.Validate(); err != nil {
		return err
	}
	if rc.DefaultNamespace != "" {
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/config/ -run TestSettings_Validate -v`
Expected: all subtests PASS.

- [ ] **Step 4: Run full config package tests (no regressions)**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/config/...`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/ingitdb/config/root_config.go
git commit -m "$(cat <<'EOF'
feat(config): validate Settings.DefaultRecordFormat at load time

New Settings.Validate accepts the empty string (means 'no project
default') or one of the seven supported record formats. An unsupported
value yields a typed error that names the offending value and lists all
seven valid options. RootConfig.Validate now invokes Settings.Validate
before its other checks, so the error surfaces at load time rather than
at first read.

Refs: spec/features/record-format/project-default REQ:invalid-format-rejected

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: `ResolveRecordFormat` helper — failing tests

**Files:**
- Modify: `pkg/ingitdb/config/root_config_test.go`

- [ ] **Step 1: Append the failing tests**

Append to `pkg/ingitdb/config/root_config_test.go`:

```go
func TestResolveRecordFormat_CollectionSettingWins(t *testing.T) {
	t.Parallel()
	col := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatJSON},
	}
	s := &Settings{DefaultRecordFormat: ingitdb.RecordFormatINGR}
	got := ResolveRecordFormat(col, s)
	if got != ingitdb.RecordFormatJSON {
		t.Errorf("expected json, got %q", got)
	}
}

func TestResolveRecordFormat_ProjectDefaultWhenCollectionUnset(t *testing.T) {
	t.Parallel()
	col := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{Format: ""},
	}
	s := &Settings{DefaultRecordFormat: ingitdb.RecordFormatINGR}
	got := ResolveRecordFormat(col, s)
	if got != ingitdb.RecordFormatINGR {
		t.Errorf("expected ingr, got %q", got)
	}
}

func TestResolveRecordFormat_HardFallbackWhenBothUnset(t *testing.T) {
	t.Parallel()
	col := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{Format: ""},
	}
	s := &Settings{DefaultRecordFormat: ""}
	got := ResolveRecordFormat(col, s)
	if got != ingitdb.RecordFormatYAML {
		t.Errorf("expected yaml, got %q", got)
	}
}

func TestResolveRecordFormat_NilCollectionUsesProjectDefault(t *testing.T) {
	t.Parallel()
	s := &Settings{DefaultRecordFormat: ingitdb.RecordFormatCSV}
	got := ResolveRecordFormat(nil, s)
	if got != ingitdb.RecordFormatCSV {
		t.Errorf("expected csv, got %q", got)
	}
}

func TestResolveRecordFormat_NilSettingsUsesHardFallback(t *testing.T) {
	t.Parallel()
	got := ResolveRecordFormat(nil, nil)
	if got != ingitdb.RecordFormatYAML {
		t.Errorf("expected yaml, got %q", got)
	}
}

func TestResolveRecordFormat_NilRecordFileFallsThrough(t *testing.T) {
	t.Parallel()
	col := &ingitdb.CollectionDef{RecordFile: nil}
	s := &Settings{DefaultRecordFormat: ingitdb.RecordFormatINGR}
	got := ResolveRecordFormat(col, s)
	if got != ingitdb.RecordFormatINGR {
		t.Errorf("expected ingr, got %q", got)
	}
}

func TestResolveRecordFormat_EmptyFormatStringFallsThrough(t *testing.T) {
	t.Parallel()
	col := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{Format: ""},
	}
	s := &Settings{DefaultRecordFormat: ingitdb.RecordFormatJSON}
	got := ResolveRecordFormat(col, s)
	if got != ingitdb.RecordFormatJSON {
		t.Errorf("expected json, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests — expect failures**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/config/ -run TestResolveRecordFormat -v`
Expected: build FAIL (`ResolveRecordFormat` undefined).

- [ ] **Step 3: Commit (test-only)**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/ingitdb/config/root_config_test.go
git commit -m "$(cat <<'EOF'
test(config): add failing tests for ResolveRecordFormat helper

Seven cases covering the full fallback chain: collection-wins,
project-default-when-collection-unset, hard-yaml-fallback-when-both-
unset, nil-collection, nil-settings, nil-recordfile, empty-format-string.
Tests fail at build because the helper is not yet defined.

Refs: spec/features/record-format/project-default REQ:fallback-resolution

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 13: `ResolveRecordFormat` helper — implementation

**Files:**
- Modify: `pkg/ingitdb/config/root_config.go`

- [ ] **Step 1: Add the helper**

Edit `pkg/ingitdb/config/root_config.go`. Append after `Settings.Validate` (added in Task 11):

```go
// ResolveRecordFormat returns the effective RecordFormat for a collection,
// applying the fallback chain:
//
//	collection.RecordFile.Format -> settings.DefaultRecordFormat -> ingitdb.RecordFormatYAML.
//
// The helper tolerates a nil collection, a collection with a nil RecordFile,
// an empty Format string, and a nil settings — each is treated as "no
// per-tier setting" and the helper falls through to the next tier.
//
// Note on call-site migration: existing call sites that read
// CollectionDef.RecordFile.Format directly operate downstream of
// RecordFileDef.Validate (which rejects empty Format), so they cannot
// observe a project-level fallback today. Wholesale migration is therefore
// deferred — see the outstanding question in
// spec/features/record-format/project-default/README.md. This helper is
// the canonical entry point for new code paths (csv read/write,
// programmatic content writers, future dalgo schema-modification consumers)
// and SHOULD be used wherever the caller cannot guarantee Format is set.
func ResolveRecordFormat(collection *ingitdb.CollectionDef, settings *Settings) ingitdb.RecordFormat {
	if collection != nil && collection.RecordFile != nil && collection.RecordFile.Format != "" {
		return collection.RecordFile.Format
	}
	if settings != nil && settings.DefaultRecordFormat != "" {
		return settings.DefaultRecordFormat
	}
	return ingitdb.RecordFormatYAML
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/config/ -run TestResolveRecordFormat -v`
Expected: all seven tests PASS.

- [ ] **Step 3: Run full config package tests**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./pkg/ingitdb/config/...`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add pkg/ingitdb/config/root_config.go
git commit -m "$(cat <<'EOF'
feat(config): add ResolveRecordFormat fallback helper

Implements the three-tier resolution chain (collection -> project ->
hard yaml default) declared by REQ:fallback-resolution. Nil-safe across
collection, RecordFile, and settings. Godoc explains the call-site
migration deferral: existing direct readers of RecordFile.Format operate
downstream of validation and cannot observe a project-level fallback;
the helper is the canonical entry point for new code paths.

Refs: spec/features/record-format/project-default REQ:fallback-resolution

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase C — `cli-default-format-flag`

Phase C depends on Phase B's `DefaultRecordFormat` field as its write target.

### Task 14: Minimal `ingitdb setup` write path — failing tests

The current `cmd/ingitdb/commands/setup.go` returns `not yet implemented`. We extract the smallest write path needed to satisfy this Feature's ACs (create `.ingitdb/` and write `settings.yaml`). The broader `setup` Feature spec (idempotency, default-namespace prompts, root-collections seed) is Draft and remains out of scope.

**Files:**
- Create: `cmd/ingitdb/commands/setup_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/ingitdb/commands/setup_test.go`:

```go
package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetup_WritesEmptySettingsYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := runSetup(dir, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".ingitdb", "settings.yaml"))
	if err != nil {
		t.Fatalf("expected settings.yaml to exist: %v", err)
	}
	// AC-2 allows either omission of the field or empty string. Our impl
	// emits an empty file when no flags are set.
	if strings.Contains(string(got), "default_record_format:") &&
		!strings.Contains(string(got), "default_record_format: \"\"") &&
		!strings.Contains(string(got), "default_record_format: ''") {
		// Field present and non-empty — only allowed if flag was passed.
		t.Errorf("expected default_record_format to be absent or empty; got: %q", string(got))
	}
}
```

- [ ] **Step 2: Run test — expect failure**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./cmd/ingitdb/commands/ -run TestSetup_WritesEmptySettingsYAML -v`
Expected: build FAIL (`runSetup` undefined).

- [ ] **Step 3: Commit (test-only)**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add cmd/ingitdb/commands/setup_test.go
git commit -m "$(cat <<'EOF'
test(setup): add failing test for minimal setup writer

Verifies setup creates .ingitdb/settings.yaml with no
default_record_format field when the flag is omitted. Test fails at
build because the runSetup function is not yet defined.

Refs: spec/features/record-format/cli-default-format-flag REQ:default-format-flag AC-2

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 15: Minimal `ingitdb setup` write path — implementation

**Files:**
- Modify: `cmd/ingitdb/commands/setup.go`

- [ ] **Step 1: Replace the setup stub**

Overwrite `cmd/ingitdb/commands/setup.go` with:

```go
package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

// Setup returns the setup command.
func Setup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up a new inGitDB database",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, _ := cmd.Flags().GetString("path")
			if path == "" {
				path = "."
			}
			defaultFormat, _ := cmd.Flags().GetString("default-format")
			return runSetup(path, defaultFormat)
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("default-format", "",
		"project-level default record format (one of: yaml, yml, json, markdown, toml, ingr, csv)")
	return cmd
}

// runSetup creates .ingitdb/ and writes settings.yaml at the given path.
// When defaultFormatFlag is non-empty, the value is validated against
// the seven supported record formats and written to
// settings.yaml#default_record_format. When empty, the file is created
// with no default_record_format key.
//
// This is the minimum write path the --default-format flag needs. The
// broader setup behavior (idempotency on already-initialised directory,
// default-namespace prompts, root-collections seed) is governed by the
// existing setup Feature spec and is intentionally out of scope here.
func runSetup(path, defaultFormatFlag string) error {
	settings := config.Settings{}
	if defaultFormatFlag != "" {
		f, err := parseDefaultFormat(defaultFormatFlag)
		if err != nil {
			return err
		}
		settings.DefaultRecordFormat = f
	}
	if validateErr := settings.Validate(); validateErr != nil {
		return validateErr
	}
	configDir := filepath.Join(path, config.IngitDBDirName)
	if mkErr := os.MkdirAll(configDir, 0o755); mkErr != nil {
		return fmt.Errorf("failed to create %s directory: %w", configDir, mkErr)
	}
	out, marshalErr := yaml.Marshal(settings)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal settings: %w", marshalErr)
	}
	settingsPath := filepath.Join(configDir, config.SettingsFileName)
	if writeErr := os.WriteFile(settingsPath, out, 0o644); writeErr != nil {
		return fmt.Errorf("failed to write %s: %w", settingsPath, writeErr)
	}
	return nil
}

// parseDefaultFormat validates the --default-format flag value against
// the seven supported record formats. Comparison is case-sensitive (the
// canonical forms are lowercase); the error message names the offending
// value and lists all seven valid options.
func parseDefaultFormat(raw string) (ingitdb.RecordFormat, error) {
	valid := []ingitdb.RecordFormat{
		ingitdb.RecordFormatYAML,
		ingitdb.RecordFormatYML,
		ingitdb.RecordFormatJSON,
		ingitdb.RecordFormatMarkdown,
		ingitdb.RecordFormatTOML,
		ingitdb.RecordFormatINGR,
		ingitdb.RecordFormatCSV,
	}
	for _, f := range valid {
		if string(f) == raw {
			return f, nil
		}
	}
	names := make([]string, len(valid))
	for i, f := range valid {
		names[i] = string(f)
	}
	return "", fmt.Errorf("unsupported --default-format=%q; valid options are: %s",
		raw, joinComma(names))
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
```

- [ ] **Step 2: Run test**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./cmd/ingitdb/commands/ -run TestSetup_WritesEmptySettingsYAML -v`
Expected: PASS.

- [ ] **Step 3: Verify build**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go build ./...`
Expected: clean exit.

- [ ] **Step 4: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add cmd/ingitdb/commands/setup.go
git commit -m "$(cat <<'EOF'
feat(setup): minimal write path + --default-format flag

Replaces the not-yet-implemented stub with a minimum setup writer that
creates .ingitdb/ and writes settings.yaml. New --default-format flag
accepts one of the seven supported record formats; the value is
validated against the closed set, written through to settings.Yaml
via config.Settings, and re-validated via Settings.Validate before
write so unsupported values surface a clear error before any file is
created.

Broader setup behavior (idempotency on already-initialised directory,
default-namespace prompts, root-collections seed) is governed by the
existing setup Feature spec and remains out of scope.

Refs: spec/features/record-format/cli-default-format-flag REQ:default-format-flag, REQ:flag-validation

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 16: `--default-format` flag — remaining ACs

**Files:**
- Modify: `cmd/ingitdb/commands/setup_test.go`

- [ ] **Step 1: Append the AC tests**

Append to `cmd/ingitdb/commands/setup_test.go`:

```go
func TestSetup_DefaultFormatFlag_WritesIngrToSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := runSetup(dir, "ingr"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".ingitdb", "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "default_record_format: ingr") {
		t.Errorf("expected settings.yaml to contain 'default_record_format: ingr'; got: %q", string(got))
	}
}

func TestSetup_DefaultFormatFlag_RejectsUnsupportedValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := runSetup(dir, "xml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	for _, name := range []string{"xml", "yaml", "yml", "json", "markdown", "toml", "ingr", "csv"} {
		if !strings.Contains(msg, name) {
			t.Errorf("expected error to mention %q; got: %s", name, msg)
		}
	}
	// AC: no .ingitdb/ directory should have been created on rejection.
	if _, statErr := os.Stat(filepath.Join(dir, ".ingitdb")); !os.IsNotExist(statErr) {
		t.Errorf("expected .ingitdb directory to NOT exist after rejection; stat err: %v", statErr)
	}
}

func TestSetup_DefaultFormatFlag_AcceptsAllSevenFormats(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"yaml", "yml", "json", "markdown", "toml", "ingr", "csv"} {
		f := f
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := runSetup(dir, f); err != nil {
				t.Errorf("expected nil error for %q, got: %v", f, err)
			}
		})
	}
}

func TestSetup_DefaultFormatFlag_LoadsBackCleanly(t *testing.T) {
	t.Parallel()
	// Round-trip: write via setup, then re-read via ReadSettingsFromFile.
	// This exercises Phase B's Settings field + load path end-to-end.
	dir := t.TempDir()
	if err := runSetup(dir, "csv"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Lazy local import to avoid widening the package's import list.
	// (config is already an import dependency of setup.go in this task.)
}
```

Note: the round-trip test is a placeholder that asserts setup succeeded with `csv`. A fuller round-trip lives in Task 17.

- [ ] **Step 2: Run tests**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./cmd/ingitdb/commands/ -run TestSetup_DefaultFormatFlag -v`
Expected: all four subtests PASS.

- [ ] **Step 3: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add cmd/ingitdb/commands/setup_test.go
git commit -m "$(cat <<'EOF'
test(setup): --default-format flag AC coverage

Adds AC-level tests: writes default_record_format: ingr to settings.yaml
on valid input; rejects 'xml' without creating any files and the error
names the offending value plus all seven valid options; accepts each of
the seven canonical format names; round-trip placeholder for the full
load-back test in the next task.

Refs: spec/features/record-format/cli-default-format-flag REQ:default-format-flag, REQ:flag-validation

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 17: End-to-end round-trip integration test

**Files:**
- Modify: `cmd/ingitdb/commands/setup_test.go`

- [ ] **Step 1: Replace the placeholder round-trip test**

Edit `cmd/ingitdb/commands/setup_test.go`. Add the `config` and `ingitdb` imports at the top if not already present:

```go
import (
	// existing imports...
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)
```

Then replace `TestSetup_DefaultFormatFlag_LoadsBackCleanly` (the placeholder from Task 16) with the real test:

```go
func TestSetup_DefaultFormatFlag_LoadsBackCleanly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := runSetup(dir, "csv"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	settings, err := config.ReadSettingsFromFile(dir, ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("failed to read back settings: %v", err)
	}
	if settings.DefaultRecordFormat != ingitdb.RecordFormatCSV {
		t.Errorf("expected DefaultRecordFormat=csv, got %q", settings.DefaultRecordFormat)
	}
	// Resolver picks the project default when the collection's format is empty.
	got := config.ResolveRecordFormat(&ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{Format: ""},
	}, &settings)
	if got != ingitdb.RecordFormatCSV {
		t.Errorf("ResolveRecordFormat returned %q, expected csv", got)
	}
}
```

- [ ] **Step 2: Run the round-trip test**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./cmd/ingitdb/commands/ -run TestSetup_DefaultFormatFlag_LoadsBackCleanly -v`
Expected: PASS.

- [ ] **Step 3: Run the full test suite**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go test ./...`
Expected: all PASS.

- [ ] **Step 4: Run lint**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && go vet ./...`
Expected: clean exit.

- [ ] **Step 5: Commit**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add cmd/ingitdb/commands/setup_test.go
git commit -m "$(cat <<'EOF'
test(setup): end-to-end round-trip through ResolveRecordFormat

Replaces the placeholder round-trip test with a real end-to-end check:
runSetup writes default_record_format: csv -> ReadSettingsFromFile
loads it back as RecordFormatCSV -> ResolveRecordFormat returns csv when
the collection's RecordFile.Format is empty. Exercises all three
Features end-to-end.

Refs: spec/features/record-format umbrella

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 18: Mark Features Implementing and update index

**Files:**
- Modify: `spec/features/record-format/README.md`
- Modify: `spec/features/record-format/csv-support/README.md`
- Modify: `spec/features/record-format/project-default/README.md`
- Modify: `spec/features/record-format/cli-default-format-flag/README.md`
- Modify: `spec/features/README.md`

- [ ] **Step 1: Flip Status: Approved → Implemented in each Feature**

In each of the four `record-format/**/README.md` files, replace the line `**Status:** Approved` with `**Status:** Implemented`.

- [ ] **Step 2: Update the top-level features index**

In `spec/features/README.md`, change the `record-format` row's Status cell from `Approved` to `Implemented`.

- [ ] **Step 3: Verify lint**

Run: `cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli && specscore spec lint --severity error 2>&1 | grep -v "ideas/cli-sql-verbs\|ideas/markdown-insert-ux\|ideas/where-like-regex" || echo "clean"`
Expected: only the pre-existing 3 unrelated Idea-file lint errors remain (cli-sql-verbs, markdown-insert-ux, where-like-regex); no errors in `features/record-format/`.

- [ ] **Step 4: Final commit + push**

```bash
cd /Users/alexandertrakhimenok/projects/ingitdb/ingitdb-cli
git add spec/features/README.md spec/features/record-format/
git commit -m "$(cat <<'EOF'
docs(spec): mark record-format Feature tree Implemented

All three child Features (csv-support, project-default,
cli-default-format-flag) plus the umbrella now reflect their shipped
status. The plan at spec/plans/2026-05-13-record-format.md is fully
executed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git push
```

---

## Verification Checklist

- [ ] `RecordFormatCSV` constant exists and equals `"csv"`
- [ ] `RecordFileDef.Validate` rejects `csv` + `SingleRecord` and `csv` + `MapOfRecords`; accepts `csv` + `ListOfRecords`
- [ ] `ParseRecordContentForCollection` parses CSV with header matching `ColumnsOrder`; rejects missing, extra, and reordered columns
- [ ] `EncodeRecordContentForCollection` emits CSV with header in `ColumnsOrder`; deterministic across runs; rejects keyed maps, nested objects, and array fields
- [ ] `encodeRecordContent` (GitHub backend single-record path) emits a clear error for `csv` format
- [ ] `Settings.DefaultRecordFormat` field exists with `yaml:"default_record_format,omitempty"`; round-trips through YAML; existing fixtures load unchanged
- [ ] `Settings.Validate` accepts the empty string and each of the seven valid formats; rejects others with an error naming the offending value and all seven options
- [ ] `RootConfig.Validate` invokes `Settings.Validate` before its other checks
- [ ] `ResolveRecordFormat` returns collection > project > hard-yaml in the right order; nil-safe across collection, RecordFile, and settings
- [ ] `ingitdb setup` creates `.ingitdb/settings.yaml`; with `--default-format=<f>` writes the value; without the flag omits the field; rejects unsupported values without creating any files
- [ ] End-to-end round-trip (setup → ReadSettingsFromFile → ResolveRecordFormat) returns the expected format
- [ ] All four `spec/features/record-format/**/README.md` files at Status: Implemented
- [ ] Top-level `spec/features/README.md` row at Status: Implemented
- [ ] `go test ./...` clean
- [ ] `go vet ./...` clean

---

## Out of Scope (Carried from Spec)

- Wholesale migration of the ~22 existing direct readers of `RecordFile.Format` to `ResolveRecordFormat`. Helper exists; new code paths use it; existing readers stay until a separate audit task lands (captured as the outstanding question in `spec/features/record-format/project-default/README.md`).
- Broader `ingitdb setup` behavior (idempotency on already-initialised directory, `default_namespace` prompts, root-collections seed). The existing `cli/setup` Feature spec is Draft and is the right home for that work.
- A collection-level `--format` flag. Reserved naming convention for the future `ingitdb create-collection` command — explicitly deferred from this Feature batch.
- Migration of existing collections from one format to another. Separate one-shot command if users want it.
- Empty/null cell semantics for CSV, boolean serialization for CSV, UTF-8 BOM emission, floating-point round-trip precision documentation. Note in CSV reader/writer godoc as time permits, but no behavior change beyond Go stdlib `encoding/csv` defaults.

---

*This document follows the plan structure recommended by `superpowers:writing-plans`.*
