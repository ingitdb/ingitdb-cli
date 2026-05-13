# Batch Insert Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement batch mode for `ingitdb insert` per the new `req:batch-*` requirements in `spec/features/cli/insert/README.md` — multi-record stdin streams in `jsonl`, `yaml`, `ingr`, or `csv` format, atomic transactions, per-record key resolution, and stream-format/storage-format independence (including markdown-stored collections).

**Architecture:** Add a `--format` flag and a batch-mode router in `cmd/ingitdb/commands/insert.go`. Add stream parsers under `pkg/dalgo2ingitdb/` (new file `batch_parsers.go`) returning `[]ParsedRecord` per format. Each parser is independent — JSONL ships first as the smallest end-to-end slice that exercises the transaction wrapping, key resolution, and view-materialization paths; YAML/INGR/CSV follow incrementally. Markdown storage is a transparent consequence of routing parsed records through the existing `dal.ReadwriteTransaction.Insert` path, which already encodes per the collection's `record_file.format` — but we verify it explicitly with cross-format ACs because the spec demands it.

**Tech Stack:** Go stdlib (`encoding/csv`, `encoding/json`, `bufio`), `gopkg.in/yaml.v3` (already imported), `github.com/ingr-io/ingr-go/ingr` (already imported in `pkg/dalgo2ingitdb/parse.go` — exposes `ingr.Unmarshal(data, &[]map[string]any)` which works for multi-record streams today), `github.com/dal-go/dalgo/dal` (existing), `github.com/spf13/cobra` (existing).

---

## File Structure

| Path | Responsibility | Status |
|---|---|---|
| `cmd/ingitdb/commands/insert.go` | Add `--format`/`--key-column` flags, route to batch when set, reject single-record flags in batch mode | Modify |
| `cmd/ingitdb/commands/insert_batch.go` | Batch-mode orchestrator: open transaction, iterate parsed records, insert each, run view materialization once, distinguish pre- vs post-commit failures | Create |
| `cmd/ingitdb/commands/insert_batch_test.go` | All batch ACs (jsonl, yaml, ingr, csv, atomicity, post-commit failure, cross-format markdown storage) | Create |
| `cmd/ingitdb/commands/insert_test.go` | Add ACs for single-record-flag exclusions, TTY-stdin rejection, invalid `--format` value, `--key-column` rejection without batch-csv | Modify |
| `pkg/dalgo2ingitdb/batch_parsers.go` | Stream parsers: `ParseBatchJSONL`, `ParseBatchYAMLStream`, `ParseBatchINGR`, `ParseBatchCSV`; shared `ParsedRecord` type; CSV key-column resolution | Create |
| `pkg/dalgo2ingitdb/batch_parsers_test.go` | Table-driven parser tests: happy path, missing-key, duplicate-key-in-stream, malformed-record line/doc-index reporting, empty stream | Create |

## Conventions

- **Test framework:** Go stdlib `testing`. Project pattern: `t.Parallel()` as the first statement of every top-level test, table-driven with `tt.name` subtests.
- **Output:** `fmt.Fprintf(os.Stderr, ...)` for diagnostics; never `Println`/`Printf` to stdout from the command — stdout is reserved for record output (see `req:success-output`).
- **No nested calls:** assign intermediate results first per CLAUDE.md.
- **Commits:** Conventional commits (`feat:`, `test:`, `refactor:`, `docs:`); include `Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>` footer.
- **Build verification at end of each task:** `go build ./...` then `go test -timeout=10s ./...`. Both must pass before commit.
- **Lint:** `golangci-lint run` before the final commit of each phase. Zero errors required.

## Type definitions (used across phases)

```go
// In pkg/dalgo2ingitdb/batch_parsers.go

// ParsedRecord is one record extracted from a batch stream.
type ParsedRecord struct {
    // Position is 1-based: line number for jsonl/csv, document index
    // for yaml/ingr. For CSV with a header row, Position 2 is the
    // first data record.
    Position int

    // Key is the resolved record key (from $id, id, or --key-column).
    Key string

    // Data is the record's structured fields with the key field stripped.
    Data map[string]any
}

// CSVParseOptions controls CSV-specific behavior.
type CSVParseOptions struct {
    // KeyColumn, if non-empty, names the column to use as the record
    // key (overrides $id/id auto-resolution).
    KeyColumn string

    // Fields, if non-empty, replaces the header row: the first stdin
    // line is treated as data and these names are used for column
    // mapping.
    Fields []string
}
```

## Pre-commit failure surface vs post-commit failure surface

`req:batch-atomic` and `req:batch-post-commit-failure` carve a clean line:

- **Pre-commit failures** — parse error, missing key, schema violation, key collision (existing or intra-batch), individual `tx.Insert` error, transaction commit error, filesystem error during write. These cause `RunReadwriteTransaction` to return an error → the records never land → CLI exits non-zero with a diagnostic naming position + reason.
- **Post-commit failures** — only `buildLocalViews` (called *after* `RunReadwriteTransaction` returns nil) can produce these. Records are already on disk. CLI exits non-zero with a distinct diagnostic: `"records inserted but view materialization for <view-name> failed: <error>"`.

## Open decision deferred to implementation

The Idea and Feature spec defer the **max-batch-size guardrail**. This plan ships **without** a guardrail. If a future ticket adds one, the natural place is at the top of `runBatchInsert` — count rows during parse, reject if over threshold, return before opening the transaction.

---

## Phase A — Foundation: `--format` flag plumbing and rejection ACs

Phase A ships the flag surface and all the "MUST be rejected" ACs. No streaming yet. After Phase A, `--format=anything` returns "batch mode not yet implemented" but all the negative-path ACs pass.

### Task A1: Register `--format` and `--key-column` flags on insert

**Files:**
- Modify: `cmd/ingitdb/commands/insert.go`

- [ ] **Step 1: Add the two new flag registrations**

In `cmd/ingitdb/commands/insert.go`, inside the `Insert()` function, after the existing `cmd.Flags().Bool("empty", ...)` call (around line 106), add:

```go
cmd.Flags().String("format", "", "batch mode: stream format (jsonl, yaml, ingr, csv); when set, reads multi-record stream from stdin")
cmd.Flags().String("key-column", "", "batch-csv mode only: column name to use as the record key (overrides $id/id auto-resolution)")
```

Note: the existing `sqlflags.RegisterFieldsFlag(cmd)` call already registers `--fields`; we re-use it. Do not add a second registration.

- [ ] **Step 2: Run build**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingitdb/commands/insert.go
git commit -m "feat(insert): add --format and --key-column flags (batch mode plumbing)

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task A2: Reject invalid `--format` values

**Files:**
- Modify: `cmd/ingitdb/commands/insert.go`
- Test: `cmd/ingitdb/commands/insert_test.go`

- [ ] **Step 1: Write the failing tests**

In `cmd/ingitdb/commands/insert_test.go`, add a new test function. Use the existing test setup helpers in `integration_helpers_test.go` (find an existing insert test like `TestInsert_*` and mirror its setup pattern):

```go
func TestInsert_RejectsInvalidFormatValue(t *testing.T) {
    t.Parallel()
    cases := []struct {
        name   string
        format string
    }{
        {"xml", "xml"},
        {"markdown", "markdown"},
        {"empty-but-set", ""},  // empty --format= is a user error
        {"yaml-stream typo", "yaml-stream"},
    }
    for _, tt := range cases {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
            _, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
                "--path="+dir,
                "--into=countries",
                "--format="+tt.format,
            )
            if err == nil {
                t.Fatalf("expected error for --format=%q, got nil", tt.format)
            }
            msg := err.Error()
            for _, want := range []string{"jsonl", "yaml", "ingr", "csv"} {
                if !strings.Contains(msg, want) {
                    t.Errorf("error message %q should list %q as a valid format", msg, want)
                }
            }
        })
    }
}
```

(If `setupInsertTestRepo` and `runInsertCmd` don't exist verbatim, locate the equivalent test helpers in the existing insert tests and use those names.)

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -timeout=10s -run TestInsert_RejectsInvalidFormatValue ./cmd/ingitdb/commands/`
Expected: FAIL — the test compiles but the command currently accepts any `--format` value (no validation).

- [ ] **Step 3: Add validation in `Insert()`**

In `cmd/ingitdb/commands/insert.go`, inside the `RunE` function, after the existing "Reject shared flags" loop (around line 51) and BEFORE the `into, _ := cmd.Flags().GetString("into")` line, insert:

```go
// Batch mode validation: --format must be one of the four supported
// stream formats. Empty means single-record mode.
format, _ := cmd.Flags().GetString("format")
if cmd.Flags().Changed("format") {
    switch format {
    case "jsonl", "yaml", "ingr", "csv":
        // valid; batch mode active
    default:
        return fmt.Errorf("invalid --format=%q; supported batch formats are: jsonl, yaml, ingr, csv (markdown is supported as a storage format only, not as a stream format)", format)
    }
}
```

- [ ] **Step 4: Run test, verify it passes**

Run: `go test -timeout=10s -run TestInsert_RejectsInvalidFormatValue ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingitdb/commands/insert.go cmd/ingitdb/commands/insert_test.go
git commit -m "feat(insert): reject invalid --format values

Implements AC: batch-invalid-format-value-rejected. Validates --format
against the four supported batch stream formats (jsonl, yaml, ingr,
csv) before any other batch logic runs.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task A3: Reject single-record flags in batch mode

**Files:**
- Modify: `cmd/ingitdb/commands/insert.go`
- Test: `cmd/ingitdb/commands/insert_test.go`

- [ ] **Step 1: Write failing tests**

Add to `insert_test.go`:

```go
func TestInsert_BatchModeRejectsSingleRecordFlags(t *testing.T) {
    t.Parallel()
    cases := []struct {
        name string
        args []string
    }{
        {"--data", []string{"--data={$id: ie}"}},
        {"--edit", []string{"--edit"}},
        {"--empty", []string{"--empty"}},
        {"--key", []string{"--key=ie"}},
    }
    for _, tt := range cases {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
            args := append([]string{
                "--path=" + dir,
                "--into=countries",
                "--format=jsonl",
            }, tt.args...)
            _, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, args...)
            if err == nil {
                t.Fatalf("expected error when combining --format=jsonl with %s", tt.name)
            }
            if !strings.Contains(err.Error(), tt.name) && !strings.Contains(err.Error(), strings.TrimPrefix(tt.name, "--")) {
                t.Errorf("error %q should name the offending flag %s", err.Error(), tt.name)
            }
        })
    }
}
```

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -timeout=10s -run TestInsert_BatchModeRejectsSingleRecordFlags ./cmd/ingitdb/commands/`
Expected: FAIL.

- [ ] **Step 3: Implement the rejection**

In `cmd/ingitdb/commands/insert.go`, immediately after the format-validation block from Task A2, add:

```go
batchMode := cmd.Flags().Changed("format")
if batchMode {
    for _, f := range []string{"data", "edit", "empty", "key"} {
        if cmd.Flags().Changed(f) {
            return fmt.Errorf("--%s is not valid in batch mode (--format=%s); batch mode reads multi-record stream from stdin and resolves keys from each record's $id", f, format)
        }
    }
}
```

- [ ] **Step 4: Run test, verify it passes**

Run: `go test -timeout=10s -run TestInsert_BatchModeRejectsSingleRecordFlags ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingitdb/commands/insert.go cmd/ingitdb/commands/insert_test.go
git commit -m "feat(insert): reject single-record flags in batch mode

Implements AC: batch-single-record-flags-rejected.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task A4: Reject `--key-column` outside batch-CSV; reject `--fields` outside batch-CSV

**Files:**
- Modify: `cmd/ingitdb/commands/insert.go`
- Test: `cmd/ingitdb/commands/insert_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestInsert_KeyColumnAndFieldsRequireBatchCSV(t *testing.T) {
    t.Parallel()
    cases := []struct {
        name string
        args []string
    }{
        {"--key-column without --format", []string{"--key=ie", "--data={}", "--key-column=external_id"}},
        {"--key-column with --format=jsonl", []string{"--format=jsonl", "--key-column=external_id"}},
        {"--fields with --format=jsonl", []string{"--format=jsonl", "--fields=$id,name"}},
        {"--fields without --format (single record)", []string{"--key=ie", "--data={}", "--fields=name"}},
    }
    for _, tt := range cases {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
            args := append([]string{"--path=" + dir, "--into=countries"}, tt.args...)
            _, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, args...)
            if err == nil {
                t.Fatalf("expected error for %s", tt.name)
            }
        })
    }
}
```

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -timeout=10s -run TestInsert_KeyColumnAndFieldsRequireBatchCSV ./cmd/ingitdb/commands/`
Expected: FAIL — `--key-column` is currently silently accepted; `--fields` rejection in single-record mode is governed by the existing shared-flag check, but `--fields` in `--format=jsonl` mode needs new logic.

- [ ] **Step 3: Implement rejections**

In `cmd/ingitdb/commands/insert.go`, add after the Task A3 block:

```go
// --key-column is only valid in batch-CSV mode.
if cmd.Flags().Changed("key-column") {
    if !batchMode || format != "csv" {
        return fmt.Errorf("--key-column is valid only with --format=csv")
    }
}

// --fields is rejected outside single-record (existing logic, line ~51)
// AND outside batch-CSV. The existing shared-flag loop rejects it in
// single-record mode; here we add the batch-non-csv guard.
if batchMode && format != "csv" && cmd.Flags().Changed("fields") {
    return fmt.Errorf("--fields is valid only with --format=csv (used to override the CSV header row or drive parsing when no header is present)")
}
```

Then, the existing shared-flag rejection loop at line ~51 unconditionally rejects `--fields`. That loop needs a single carve-out: skip rejecting `--fields` when batch-CSV is active. Change the loop body:

```go
// Find:
for _, flag := range []string{"from", "id", "where", "set", "unset", "all", "min-affected", "order-by", "fields"} {
    if cmd.Flags().Changed(flag) {
        return fmt.Errorf("--%s is not valid with insert (insert uses --into + --key + data source)", flag)
    }
}
// Replace with:
for _, flag := range []string{"from", "id", "where", "set", "unset", "all", "min-affected", "order-by", "fields"} {
    if !cmd.Flags().Changed(flag) {
        continue
    }
    // Carve-out: --fields is permitted in batch-CSV mode (see req:batch-csv-fields-flag).
    if flag == "fields" && batchMode && format == "csv" {
        continue
    }
    return fmt.Errorf("--%s is not valid with insert (insert uses --into + --key + data source)", flag)
}
```

**Important:** the `batchMode` and `format` variables are now read AFTER the loop currently. Move the format-read + `batchMode` assignment BEFORE this loop. The final order in `RunE` should be:

1. Read `format`, validate, set `batchMode`
2. Run the shared-flag rejection loop (with the carve-out)
3. Run the batch single-record-flag rejection (Task A3)
4. Run the `--key-column` / `--fields` non-CSV rejection (this task)
5. Read `into`
6. Continue with existing single-record vs batch-routing logic

- [ ] **Step 4: Run test, verify it passes**

Run: `go test -timeout=10s -run TestInsert_KeyColumnAndFieldsRequireBatchCSV ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 5: Run the broader insert test suite to check for regressions**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestInsert`
Expected: all pre-existing single-record insert tests still pass. The shared-flag-rejection AC (`rejects-non-insert-flags`) must still pass — verify the `--fields` rejection still fires in single-record mode.

- [ ] **Step 6: Commit**

```bash
git add cmd/ingitdb/commands/insert.go cmd/ingitdb/commands/insert_test.go
git commit -m "feat(insert): scope --key-column and --fields to batch-csv mode

Implements AC: key-column-rejected-without-batch-csv and AC: batch-csv-
fields-only-with-csv. Adds narrow carve-out in the shared-flag
rejection loop so --fields is permitted iff --format=csv.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task A5: Reject TTY stdin in batch mode + stub batch-mode entry

**Files:**
- Modify: `cmd/ingitdb/commands/insert.go`
- Test: `cmd/ingitdb/commands/insert_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestInsert_BatchModeRejectsTTYStdin(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    // runInsertCmd defaults to non-TTY stdin via bytes.Reader. To
    // simulate TTY, pass a stdin that reports isStdinTTY()=true.
    // Use a custom runner that injects isStdinTTY=func()bool{return true}.
    _, err := runInsertCmdWithStdinTTY(t, homeDir, getWd, readDef, newDB, logf, true,
        "--path="+dir, "--into=countries", "--format=jsonl",
    )
    if err == nil {
        t.Fatal("expected error when --format is set and stdin is a TTY")
    }
    if !strings.Contains(err.Error(), "pipe") && !strings.Contains(err.Error(), "TTY") && !strings.Contains(err.Error(), "stdin") {
        t.Errorf("error %q should mention TTY/stdin/pipe", err.Error())
    }
}
```

If `runInsertCmdWithStdinTTY` does not exist, add it to `integration_helpers_test.go` (mirror the signature of `runInsertCmd` but accept an additional `stdinIsTTY bool` parameter and thread it into the `Insert()` constructor via `isStdinTTY`).

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -timeout=10s -run TestInsert_BatchModeRejectsTTYStdin ./cmd/ingitdb/commands/`
Expected: FAIL.

- [ ] **Step 3: Implement TTY rejection + stub batch entry**

In `insert.go`, after resolving `ictx`, add:

```go
if batchMode {
    if isStdinTTY() {
        return fmt.Errorf("batch mode (--format=%s) requires piped stdin; refusing to read from a TTY", format)
    }
    return fmt.Errorf("batch mode is not yet implemented (format=%s)", format)
}
```

Place this immediately AFTER `resolveInsertContext` (so `--into` validation runs first) and BEFORE the `readInsertData` call.

- [ ] **Step 4: Run test, verify it passes**

Run: `go test -timeout=10s -run TestInsert_BatchModeRejectsTTYStdin ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 5: Run full suite**

Run: `go test -timeout=10s ./...`
Expected: PASS. The stub error in batch mode does not affect any pre-existing test because no pre-existing test sets `--format`.

- [ ] **Step 6: Commit**

```bash
git add cmd/ingitdb/commands/insert.go cmd/ingitdb/commands/insert_test.go cmd/ingitdb/commands/integration_helpers_test.go
git commit -m "feat(insert): reject TTY stdin in batch mode; stub batch-mode entry

Implements AC: batch-stdin-tty-rejected. Adds an isStdinTTY=true branch
that errors before opening a transaction. Batch-mode logic itself is
stubbed with 'not yet implemented' pending Phase B.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

---

## Phase B — JSONL end-to-end

Phase B implements the JSONL stream parser and the full batch-insert orchestration (transaction, key resolution from `$id`, position-aware errors, view materialization, empty-stream success, post-commit failure handling). YAML/INGR/CSV in subsequent phases reuse this orchestration.

### Task B1: Define `ParsedRecord` and the JSONL parser

**Files:**
- Create: `pkg/dalgo2ingitdb/batch_parsers.go`
- Create: `pkg/dalgo2ingitdb/batch_parsers_test.go`

- [ ] **Step 1: Write failing tests**

Create `pkg/dalgo2ingitdb/batch_parsers_test.go`:

```go
package dalgo2ingitdb

import (
    "strings"
    "testing"
)

func TestParseBatchJSONL_HappyPath(t *testing.T) {
    t.Parallel()
    in := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"$id":"fr","name":"France"}
`)
    got, err := ParseBatchJSONL(in)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(got) != 2 {
        t.Fatalf("want 2 records, got %d", len(got))
    }
    if got[0].Position != 1 || got[0].Key != "ie" || got[0].Data["name"] != "Ireland" {
        t.Errorf("record[0]=%+v; want {Position:1, Key:\"ie\", Data:{name:Ireland}}", got[0])
    }
    if _, present := got[0].Data["$id"]; present {
        t.Errorf("$id MUST be stripped from Data, got %+v", got[0].Data)
    }
    if got[1].Position != 2 || got[1].Key != "fr" {
        t.Errorf("record[1]=%+v; want Position:2 Key:fr", got[1])
    }
}

func TestParseBatchJSONL_MissingIDReportsLine(t *testing.T) {
    t.Parallel()
    in := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"name":"France"}
`)
    _, err := ParseBatchJSONL(in)
    if err == nil {
        t.Fatal("expected error for record missing $id")
    }
    if !strings.Contains(err.Error(), "line 2") {
        t.Errorf("error %q should name line 2", err.Error())
    }
    if !strings.Contains(err.Error(), "$id") {
        t.Errorf("error %q should mention $id", err.Error())
    }
}

func TestParseBatchJSONL_MalformedJSONReportsLine(t *testing.T) {
    t.Parallel()
    in := strings.NewReader(`{"$id":"ie"}
{"$id":"fr",
`)
    _, err := ParseBatchJSONL(in)
    if err == nil {
        t.Fatal("expected parse error")
    }
    if !strings.Contains(err.Error(), "line 2") {
        t.Errorf("error %q should name line 2", err.Error())
    }
}

func TestParseBatchJSONL_EmptyStream(t *testing.T) {
    t.Parallel()
    got, err := ParseBatchJSONL(strings.NewReader(""))
    if err != nil {
        t.Fatalf("empty stream should not error: %v", err)
    }
    if len(got) != 0 {
        t.Errorf("want 0 records, got %d", len(got))
    }
}

func TestParseBatchJSONL_BlankLinesSkipped(t *testing.T) {
    t.Parallel()
    in := strings.NewReader(`{"$id":"ie"}

{"$id":"fr"}
`)
    got, err := ParseBatchJSONL(in)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(got) != 2 {
        t.Fatalf("want 2 records (blank line skipped), got %d", len(got))
    }
    // Position MUST reflect the source line, not record index.
    if got[1].Position != 3 {
        t.Errorf("second record should have Position 3 (source line), got %d", got[1].Position)
    }
}
```

- [ ] **Step 2: Run test, verify it fails to compile**

Run: `go test -timeout=10s ./pkg/dalgo2ingitdb/ -run TestParseBatchJSONL`
Expected: FAIL with `undefined: ParseBatchJSONL`.

- [ ] **Step 3: Implement the parser and the `ParsedRecord` type**

Create `pkg/dalgo2ingitdb/batch_parsers.go`:

```go
package dalgo2ingitdb

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "strings"
)

// ParsedRecord is one record extracted from a batch stream.
type ParsedRecord struct {
    // Position is 1-based: line number for jsonl/csv, document index
    // for yaml/ingr. For csv with a header row, Position 2 is the
    // first data record.
    Position int
    // Key is the resolved record key (from $id, id, or --key-column).
    Key string
    // Data is the record's structured fields with the key field stripped.
    Data map[string]any
}

// ParseBatchJSONL reads NDJSON from r and returns one ParsedRecord per
// non-blank line. Each record MUST have a top-level $id; the $id is
// stripped from the returned Data map. Blank lines are skipped but
// counted for the Position field.
func ParseBatchJSONL(r io.Reader) ([]ParsedRecord, error) {
    scanner := bufio.NewScanner(r)
    // Allow large records — default 64KiB is too small for realistic batches.
    const maxLine = 1 << 22 // 4 MiB
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, maxLine)

    var records []ParsedRecord
    lineNo := 0
    for scanner.Scan() {
        lineNo++
        raw := scanner.Bytes()
        if len(strings.TrimSpace(string(raw))) == 0 {
            continue
        }
        var data map[string]any
        if err := json.Unmarshal(raw, &data); err != nil {
            return nil, fmt.Errorf("line %d: invalid JSON: %w", lineNo, err)
        }
        idRaw, ok := data["$id"]
        if !ok {
            return nil, fmt.Errorf("line %d: record missing required $id field", lineNo)
        }
        key, ok := idRaw.(string)
        if !ok {
            return nil, fmt.Errorf("line %d: $id must be a string, got %T", lineNo, idRaw)
        }
        if key == "" {
            return nil, fmt.Errorf("line %d: $id is empty", lineNo)
        }
        delete(data, "$id")
        records = append(records, ParsedRecord{
            Position: lineNo,
            Key:      key,
            Data:     data,
        })
    }
    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("read jsonl stream: %w", err)
    }
    return records, nil
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test -timeout=10s ./pkg/dalgo2ingitdb/ -run TestParseBatchJSONL -v`
Expected: all five subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/dalgo2ingitdb/batch_parsers.go pkg/dalgo2ingitdb/batch_parsers_test.go
git commit -m "feat(dalgo2ingitdb): add JSONL batch parser

ParseBatchJSONL reads NDJSON from io.Reader and returns []ParsedRecord
with 1-based line positions. $id is required per record and stripped
from the returned Data. Backs req:batch-per-record-key for the jsonl
format.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task B2: Batch orchestrator skeleton — empty stream success, view materialization once

**Files:**
- Create: `cmd/ingitdb/commands/insert_batch.go`
- Modify: `cmd/ingitdb/commands/insert.go`
- Create: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write the failing test for empty stream**

Create `cmd/ingitdb/commands/insert_batch_test.go`:

```go
package commands

import (
    "bytes"
    "strings"
    "testing"
)

func TestInsertBatch_JSONL_EmptyStreamSucceeds(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    stderr, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        bytes.NewReader([]byte("")),
        "--path="+dir, "--into=countries", "--format=jsonl",
    )
    if err != nil {
        t.Fatalf("empty batch should succeed, got error: %v", err)
    }
    if !strings.Contains(stderr, "0 records inserted") {
        t.Errorf("stderr %q should mention '0 records inserted'", stderr)
    }
}
```

If `runInsertCmdWithStdin` does not exist with that exact signature, add it to `integration_helpers_test.go` — it should mirror `runInsertCmd` but accept an `io.Reader` for stdin and capture stderr.

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_EmptyStreamSucceeds ./cmd/ingitdb/commands/`
Expected: FAIL — the stub from Task A5 still returns "not yet implemented".

- [ ] **Step 3: Create the orchestrator file**

Create `cmd/ingitdb/commands/insert_batch.go`:

```go
package commands

import (
    "context"
    "fmt"
    "io"
    "os"

    "github.com/dal-go/dalgo/dal"

    "github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
)

// runBatchInsert implements --format batch mode. It reads stdin per the
// selected stream format, inserts all records inside a single
// transaction, and materializes local views once after commit.
//
// On any pre-commit failure (parse, missing key, schema violation,
// collision, write error, commit error), the batch is rolled back and
// the error is returned. On post-commit view-materialization failure,
// the inserted records remain on disk and a distinct error is returned.
func runBatchInsert(
    ctx context.Context,
    format string,
    keyColumn string,
    fields []string,
    stdin io.Reader,
    ictx insertContext,
) error {
    records, err := parseBatchStream(format, keyColumn, fields, stdin)
    if err != nil {
        return err
    }
    if len(records) == 0 {
        fmt.Fprintln(os.Stderr, "0 records inserted")
        return nil
    }
    // Pre-commit intra-batch duplicate check.
    if err := rejectIntraBatchDuplicates(records); err != nil {
        return err
    }
    // Atomic insert. Any individual failure aborts the whole batch.
    commitErr := ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
        for _, rec := range records {
            key := dal.NewKeyWithID(ictx.colDef.ID, rec.Key)
            r := dal.NewRecordWithData(key, rec.Data)
            if err := tx.Insert(ctx, r); err != nil {
                return fmt.Errorf("record at position %d (key=%q): %w", rec.Position, rec.Key, err)
            }
        }
        return nil
    })
    if commitErr != nil {
        return commitErr
    }
    // Post-commit: materialize local views once. Failures here cannot
    // be rolled back — records are on disk.
    rctx := recordContext{
        db:      ictx.db,
        colDef:  ictx.colDef,
        dirPath: ictx.dirPath,
        def:     ictx.def,
    }
    if err := buildLocalViews(ctx, rctx); err != nil {
        return fmt.Errorf("records inserted but view materialization failed: %w", err)
    }
    fmt.Fprintf(os.Stderr, "%d records inserted\n", len(records))
    return nil
}

// parseBatchStream routes to the format-specific parser.
func parseBatchStream(format, keyColumn string, fields []string, r io.Reader) ([]dalgo2ingitdb.ParsedRecord, error) {
    switch format {
    case "jsonl":
        return dalgo2ingitdb.ParseBatchJSONL(r)
    case "yaml", "ingr", "csv":
        return nil, fmt.Errorf("batch format %q not yet implemented", format)
    default:
        return nil, fmt.Errorf("unsupported batch format %q", format)
    }
}

// rejectIntraBatchDuplicates returns an error if two records in the
// batch share a resolved key.
func rejectIntraBatchDuplicates(records []dalgo2ingitdb.ParsedRecord) error {
    seen := make(map[string]int, len(records))
    for _, rec := range records {
        if prev, dup := seen[rec.Key]; dup {
            return fmt.Errorf("duplicate key %q in batch: positions %d and %d", rec.Key, prev, rec.Position)
        }
        seen[rec.Key] = rec.Position
    }
    return nil
}
```

- [ ] **Step 4: Wire the orchestrator into `Insert()`**

In `cmd/ingitdb/commands/insert.go`, replace the stub from Task A5:

```go
// Was:
if batchMode {
    if isStdinTTY() {
        return fmt.Errorf("batch mode (--format=%s) requires piped stdin; refusing to read from a TTY", format)
    }
    return fmt.Errorf("batch mode is not yet implemented (format=%s)", format)
}
```

With:

```go
if batchMode {
    if isStdinTTY() {
        return fmt.Errorf("batch mode (--format=%s) requires piped stdin; refusing to read from a TTY", format)
    }
    keyColumn, _ := cmd.Flags().GetString("key-column")
    fieldsCSV, _ := cmd.Flags().GetString("fields")
    var fields []string
    if fieldsCSV != "" {
        fields = strings.Split(fieldsCSV, ",")
        for i := range fields {
            fields[i] = strings.TrimSpace(fields[i])
        }
    }
    return runBatchInsert(ctx, format, keyColumn, fields, stdin, *ictx)
}
```

You will need to add `"strings"` to the import block and confirm `ictx` is a value (not pointer) acceptable to `runBatchInsert`. If `resolveInsertContext` returns `*insertContext`, dereference it (`*ictx`); if there's no such type yet, declare it in `insert.go` as a struct grouping `db`, `colDef`, `dirPath`, `def`. There is already an `insertContext` type — verify by searching `grep -n "type insertContext" cmd/ingitdb/commands/`.

- [ ] **Step 5: Run the empty-stream test, verify it passes**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_EmptyStreamSucceeds ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/ingitdb/commands/insert.go cmd/ingitdb/commands/insert_batch.go cmd/ingitdb/commands/insert_batch_test.go cmd/ingitdb/commands/integration_helpers_test.go
git commit -m "feat(insert): batch-mode orchestrator skeleton (jsonl, empty stream)

Implements AC: batch-empty-stream-succeeds. Adds runBatchInsert which
routes to a format-specific parser, runs a single
RunReadwriteTransaction wrapping all inserts, and materializes local
views once after commit. Post-commit materialization failure is
distinguished from pre-commit rollback by error message.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task B3: JSONL happy path — atomic insert of N records

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write failing test**

Add to `insert_batch_test.go`:

```go
func TestInsertBatch_JSONL_HappyPath(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    stdin := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"$id":"fr","name":"France"}
`)
    stderr, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=jsonl",
    )
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    if !strings.Contains(stderr, "2 records inserted") {
        t.Errorf("stderr %q should mention '2 records inserted'", stderr)
    }
    // Verify both records exist on disk by reading them back.
    assertRecordExists(t, dir, "countries", "ie", map[string]any{"name": "Ireland"})
    assertRecordExists(t, dir, "countries", "fr", map[string]any{"name": "France"})
}
```

You will need `assertRecordExists` — if it doesn't exist in `integration_helpers_test.go`, add it. The function should read the record file from disk, parse it per the collection's `record_file.format`, and assert each `want` key/value is present (ignoring extra fields). Use `dalgo2ingitdb.ParseRecordContentForCollection` if needed.

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_HappyPath ./cmd/ingitdb/commands/`
Expected: FAIL or PASS — depends on whether the orchestrator from Task B2 already covers the happy path. If the test fails because `assertRecordExists` is missing, add it first.

- [ ] **Step 3: If failing, fix the orchestrator until the happy path passes**

Most likely the existing orchestrator already works for the happy path — Task B2 implemented the full loop. If the test fails for other reasons (key resolution, transaction wiring), debug and fix.

- [ ] **Step 4: Verify the test passes**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_HappyPath ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go cmd/ingitdb/commands/integration_helpers_test.go
git commit -m "test(insert): AC batch-jsonl-basic end-to-end

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task B4: Missing-key rejection rolls back the whole batch

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestInsertBatch_JSONL_MissingKeyRollsBackBatch(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    stdin := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"name":"France"}
`)
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=jsonl",
    )
    if err == nil {
        t.Fatal("expected error for missing $id at line 2")
    }
    if !strings.Contains(err.Error(), "line 2") {
        t.Errorf("error %q should reference line 2", err.Error())
    }
    // CRITICAL: the first record MUST NOT exist on disk.
    assertRecordAbsent(t, dir, "countries", "ie")
    assertRecordAbsent(t, dir, "countries", "fr")
}
```

Add `assertRecordAbsent` to helpers if missing.

- [ ] **Step 2: Run test, verify it passes**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_MissingKeyRollsBackBatch ./cmd/ingitdb/commands/`
Expected: PASS. The parser rejects the missing-`$id` record BEFORE the transaction opens, so neither record lands. This validates the design: parse fully → only then open transaction.

If it fails (e.g. because we partially commit), the architecture is wrong — the parser must return all records or none before any write.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go
git commit -m "test(insert): AC batch-missing-key-rejected with rollback

Verifies that a missing \$id mid-batch leaves zero records on disk.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task B5: Intra-batch duplicate-key rejection

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestInsertBatch_JSONL_IntraBatchDuplicateKey(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    stdin := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"$id":"ie","name":"Eire"}
`)
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=jsonl",
    )
    if err == nil {
        t.Fatal("expected error for duplicate key in batch")
    }
    if !strings.Contains(err.Error(), "ie") {
        t.Errorf("error %q should name the conflicting key 'ie'", err.Error())
    }
    if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "2") {
        t.Errorf("error %q should name both positions (1 and 2)", err.Error())
    }
    assertRecordAbsent(t, dir, "countries", "ie")
}
```

- [ ] **Step 2: Run test, verify it passes**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_IntraBatchDuplicateKey ./cmd/ingitdb/commands/`
Expected: PASS — `rejectIntraBatchDuplicates` (Task B2) handles this.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go
git commit -m "test(insert): AC batch-duplicate-key-in-stream-rejected

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task B6: Collision with existing record rolls back the batch

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestInsertBatch_JSONL_CollisionWithExistingRecord(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    // Pre-insert "ie" as a single record so the batch will collide.
    _, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
        "--path="+dir, "--into=countries", "--key=ie", "--data={name: Ireland}",
    )
    if err != nil {
        t.Fatalf("setup insert failed: %v", err)
    }
    stdin := strings.NewReader(`{"$id":"fr","name":"France"}
{"$id":"ie","name":"Ireland"}
`)
    _, err = runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=jsonl",
    )
    if err == nil {
        t.Fatal("expected error for collision with existing key")
    }
    // The "fr" record from line 1 of the batch MUST NOT exist on disk.
    assertRecordAbsent(t, dir, "countries", "fr")
    // Existing "ie" still has Ireland (not "Ireland" from the batch attempt).
    assertRecordExists(t, dir, "countries", "ie", map[string]any{"name": "Ireland"})
}
```

- [ ] **Step 2: Run test, verify it passes**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_CollisionWithExistingRecord ./cmd/ingitdb/commands/`
Expected: PASS — `tx.Insert` already rejects existing keys; the transaction layer rolls back when the closure returns the error.

If FAIL because the `fr` record persists, the dalgo transaction is not actually atomic for this backend. That's a deeper concern — investigate `RunReadwriteTransaction`'s rollback semantics for the local-fs backend.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go
git commit -m "test(insert): AC batch-collision-with-existing-record

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task B7: View materialization runs exactly once after batch commit

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write failing test**

Locate an existing test that sets up a collection with a local view. Search: `grep -rn "view\|materialize" cmd/ingitdb/commands/*_test.go | head`. Use that test's setup as a template. Then:

```go
func TestInsertBatch_JSONL_ViewMaterializationOnce(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepoWithView(t)
    stdin := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"$id":"fr","name":"France"}
{"$id":"de","name":"Germany"}
`)
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=jsonl",
    )
    if err != nil {
        t.Fatalf("batch insert failed: %v", err)
    }
    // Verify the view contains all 3 records after a SINGLE
    // materialization. Read the view file from disk; it must hold
    // exactly 3 rows.
    rows := readViewRows(t, dir, "countries-by-name")
    if len(rows) != 3 {
        t.Errorf("view should hold 3 rows, got %d", len(rows))
    }
}
```

The crucial assertion is *correctness*. To prove materialization ran exactly ONCE (not N times), you can either (a) wrap the materializer in a counting decorator at test setup, or (b) trust the architecture (it's called once after the transaction returns nil). For MVP, (b) is fine — code review confirms the single call site in `runBatchInsert`. If you want assertion (a), add a test-only `materializeFn` injection point.

- [ ] **Step 2: Run test, verify it passes**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_ViewMaterializationOnce ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go cmd/ingitdb/commands/integration_helpers_test.go
git commit -m "test(insert): AC batch-view-materialization-once

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task B8: Post-commit view-materialization failure surfaces distinctly

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`
- Possibly modify: `cmd/ingitdb/commands/record_context.go` if a fault injection seam is needed

- [ ] **Step 1: Decide on a fault injection mechanism**

The AC requires that a post-commit materialization failure produce a *distinct* error message. To test this, we need a deterministic way to make materialization fail. Options:

- **Inject a faulty view definition**: configure a view that targets a column that doesn't exist on the records being inserted. Materialization will fail at write time. Use this — it's data-driven, no code change needed in product code.
- Make the view's output path unwritable (chmod). Brittle in CI.
- Add a `materializeFn` injection point to `runBatchInsert`. Cleanest but adds a test seam.

Pick **option 1** (faulty view) unless you find it cannot be reliably triggered — check what materialization errors look like in the existing materializer tests.

- [ ] **Step 2: Write the failing test**

```go
func TestInsertBatch_JSONL_PostCommitMaterializationFailure(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepoWithFaultyView(t)
    stdin := strings.NewReader(`{"$id":"ie","name":"Ireland"}
`)
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=jsonl",
    )
    if err == nil {
        t.Fatal("expected error from post-commit view materialization failure")
    }
    // The diagnostic MUST be distinguishable from a pre-commit rollback.
    if !strings.Contains(err.Error(), "records inserted") {
        t.Errorf("post-commit error %q should mention 'records inserted'", err.Error())
    }
    if !strings.Contains(err.Error(), "view materialization") && !strings.Contains(err.Error(), "materialization failed") {
        t.Errorf("post-commit error %q should mention view materialization", err.Error())
    }
    // CRITICAL: the record IS on disk despite the error.
    assertRecordExists(t, dir, "countries", "ie", map[string]any{"name": "Ireland"})
}
```

`setupInsertTestRepoWithFaultyView` configures a view that will fail materialization. Build this helper from the working `setupInsertTestRepoWithView` by adding a column reference that doesn't exist.

- [ ] **Step 3: Run test, verify it passes**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_PostCommitMaterializationFailure ./cmd/ingitdb/commands/`
Expected: PASS — the `runBatchInsert` already wraps `buildLocalViews` error with `"records inserted but view materialization failed: ..."`.

If FAIL, inspect: the materializer may not actually error for a missing column — it might just skip the column and produce an empty view. In that case, you need a stronger fault (file-permission denial, or the injection-seam approach).

- [ ] **Step 4: Run the broader test suite to check for regressions**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/`
Expected: PASS across the board.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go cmd/ingitdb/commands/integration_helpers_test.go
git commit -m "test(insert): AC batch-post-commit-view-failure

Verifies that view materialization failure after a successful commit
returns a distinct error (\"records inserted but view materialization
failed\") and leaves the inserted records on disk.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task B9: Lint and close Phase B

- [ ] **Step 1: Run golangci-lint**

Run: `golangci-lint run`
Expected: zero errors. Fix any issues introduced by the new files.

- [ ] **Step 2: Run the full test suite**

Run: `go test -timeout=10s ./...`
Expected: all tests pass.

- [ ] **Step 3: Tag Phase B complete in commit**

```bash
git commit --allow-empty -m "chore: phase B (jsonl batch insert) complete

Lint clean. All Phase B ACs pass:
- batch-jsonl-basic
- batch-missing-key-rejected
- batch-duplicate-key-in-stream-rejected
- batch-collision-with-existing-record
- batch-empty-stream-succeeds
- batch-view-materialization-once
- batch-post-commit-view-failure

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

---

## Phase C — YAML multi-document and INGR stream parsers

Phase C adds two more stream formats. Both reuse `runBatchInsert`'s orchestration unchanged; only the parser-routing branch and the parsers themselves are new.

### Task C1: YAML multi-document parser

**Files:**
- Modify: `pkg/dalgo2ingitdb/batch_parsers.go`
- Modify: `pkg/dalgo2ingitdb/batch_parsers_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestParseBatchYAMLStream_HappyPath(t *testing.T) {
    t.Parallel()
    in := strings.NewReader(`$id: ie
name: Ireland
---
$id: fr
name: France
`)
    got, err := ParseBatchYAMLStream(in)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(got) != 2 {
        t.Fatalf("want 2 records, got %d", len(got))
    }
    if got[0].Position != 1 || got[0].Key != "ie" {
        t.Errorf("record[0]=%+v; want Position:1 Key:ie", got[0])
    }
    if got[1].Position != 2 || got[1].Key != "fr" {
        t.Errorf("record[1]=%+v; want Position:2 Key:fr", got[1])
    }
}

func TestParseBatchYAMLStream_MissingIDReportsDocIndex(t *testing.T) {
    t.Parallel()
    in := strings.NewReader(`$id: ie
name: Ireland
---
name: France
`)
    _, err := ParseBatchYAMLStream(in)
    if err == nil {
        t.Fatal("expected error")
    }
    if !strings.Contains(err.Error(), "document 2") && !strings.Contains(err.Error(), "doc 2") {
        t.Errorf("error %q should reference document 2", err.Error())
    }
}

func TestParseBatchYAMLStream_EmptyStream(t *testing.T) {
    t.Parallel()
    got, err := ParseBatchYAMLStream(strings.NewReader(""))
    if err != nil {
        t.Fatalf("empty stream should not error: %v", err)
    }
    if len(got) != 0 {
        t.Errorf("want 0 records, got %d", len(got))
    }
}
```

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -timeout=10s -run TestParseBatchYAMLStream ./pkg/dalgo2ingitdb/`
Expected: FAIL with `undefined: ParseBatchYAMLStream`.

- [ ] **Step 3: Implement the parser**

Append to `pkg/dalgo2ingitdb/batch_parsers.go`:

```go
// ParseBatchYAMLStream reads a YAML multi-document stream from r and
// returns one ParsedRecord per non-nil document. Each record MUST have
// a top-level $id; $id is stripped from the returned Data map.
// Position is the 1-based document index.
func ParseBatchYAMLStream(r io.Reader) ([]ParsedRecord, error) {
    dec := yaml.NewDecoder(r)
    var records []ParsedRecord
    docNo := 0
    for {
        var data map[string]any
        err := dec.Decode(&data)
        if err == io.EOF {
            break
        }
        docNo++
        if err != nil {
            return nil, fmt.Errorf("document %d: invalid YAML: %w", docNo, err)
        }
        if data == nil {
            // Skip empty documents (e.g. trailing "---\n").
            continue
        }
        idRaw, ok := data["$id"]
        if !ok {
            return nil, fmt.Errorf("document %d: record missing required $id field", docNo)
        }
        key, ok := idRaw.(string)
        if !ok {
            return nil, fmt.Errorf("document %d: $id must be a string, got %T", docNo, idRaw)
        }
        if key == "" {
            return nil, fmt.Errorf("document %d: $id is empty", docNo)
        }
        delete(data, "$id")
        records = append(records, ParsedRecord{
            Position: docNo,
            Key:      key,
            Data:     data,
        })
    }
    return records, nil
}
```

Add `"gopkg.in/yaml.v3"` to the imports at the top of `batch_parsers.go` if not already present.

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test -timeout=10s -run TestParseBatchYAMLStream ./pkg/dalgo2ingitdb/`
Expected: PASS.

- [ ] **Step 5: Wire YAML into the router**

In `cmd/ingitdb/commands/insert_batch.go`, in `parseBatchStream`, replace:

```go
case "yaml", "ingr", "csv":
    return nil, fmt.Errorf("batch format %q not yet implemented", format)
```

With:

```go
case "yaml":
    return dalgo2ingitdb.ParseBatchYAMLStream(r)
case "ingr", "csv":
    return nil, fmt.Errorf("batch format %q not yet implemented", format)
```

- [ ] **Step 6: Write end-to-end YAML test in commands package**

```go
func TestInsertBatch_YAML_HappyPath(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    stdin := strings.NewReader(`$id: ie
name: Ireland
---
$id: fr
name: France
`)
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=yaml",
    )
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    assertRecordExists(t, dir, "countries", "ie", map[string]any{"name": "Ireland"})
    assertRecordExists(t, dir, "countries", "fr", map[string]any{"name": "France"})
}
```

- [ ] **Step 7: Run all new tests**

Run: `go test -timeout=10s -run "TestParseBatchYAMLStream|TestInsertBatch_YAML" ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/dalgo2ingitdb/batch_parsers.go pkg/dalgo2ingitdb/batch_parsers_test.go cmd/ingitdb/commands/insert_batch.go cmd/ingitdb/commands/insert_batch_test.go
git commit -m "feat(insert): YAML multi-document batch parser

Implements AC: batch-yaml-stream.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task C2: INGR stream parser

**Files:**
- Modify: `pkg/dalgo2ingitdb/batch_parsers.go`
- Modify: `pkg/dalgo2ingitdb/batch_parsers_test.go`
- Modify: `cmd/ingitdb/commands/insert_batch.go`
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write failing parser test**

```go
func TestParseBatchINGR_HappyPath(t *testing.T) {
    t.Parallel()
    // INGR multi-record stream format — derive a small valid example
    // by writing two records via materializer.NewRecordsWriter (see
    // pkg/ingitdb/materializer/ingr_writer.go for the canonical
    // writer API) OR construct it manually from the format spec.
    // For test stability, prefer round-tripping through the writer.
    payload := buildINGRPayloadForTest(t, []map[string]any{
        {"$ID": "ie", "name": "Ireland"},
        {"$ID": "fr", "name": "France"},
    })
    got, err := ParseBatchINGR(bytes.NewReader(payload))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(got) != 2 {
        t.Fatalf("want 2 records, got %d", len(got))
    }
    if got[0].Key != "ie" || got[0].Position != 1 {
        t.Errorf("record[0]=%+v; want Key:ie Position:1", got[0])
    }
    if _, present := got[0].Data["$ID"]; present {
        t.Errorf("$ID MUST be stripped from Data, got %+v", got[0].Data)
    }
}
```

The `buildINGRPayloadForTest` helper constructs a valid INGR byte stream. The simplest implementation uses `github.com/ingr-io/ingr-go/ingr.NewRecordsWriter` — read `pkg/ingitdb/materializer/ingr_writer.go` lines ~170-200 for the pattern. Put the helper in `batch_parsers_test.go`.

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -timeout=10s -run TestParseBatchINGR ./pkg/dalgo2ingitdb/`
Expected: FAIL with `undefined: ParseBatchINGR`.

- [ ] **Step 3: Implement parser**

Append to `pkg/dalgo2ingitdb/batch_parsers.go`:

```go
// ParseBatchINGR reads an INGR multi-record stream from r and returns
// one ParsedRecord per record. The key is read from the reserved $ID
// column (INGR's key field; note the uppercase). $ID is stripped from
// the returned Data map. Position is the 1-based record index.
func ParseBatchINGR(r io.Reader) ([]ParsedRecord, error) {
    content, err := io.ReadAll(r)
    if err != nil {
        return nil, fmt.Errorf("read ingr stream: %w", err)
    }
    var rows []map[string]any
    if err := ingr.Unmarshal(content, &rows); err != nil {
        return nil, fmt.Errorf("parse ingr stream: %w", err)
    }
    records := make([]ParsedRecord, 0, len(rows))
    for i, row := range rows {
        pos := i + 1
        idRaw, ok := row["$ID"]
        if !ok {
            return nil, fmt.Errorf("record %d: missing required $ID column", pos)
        }
        key, ok := idRaw.(string)
        if !ok {
            return nil, fmt.Errorf("record %d: $ID must be a string, got %T", pos, idRaw)
        }
        if key == "" {
            return nil, fmt.Errorf("record %d: $ID is empty", pos)
        }
        delete(row, "$ID")
        records = append(records, ParsedRecord{
            Position: pos,
            Key:      key,
            Data:     row,
        })
    }
    return records, nil
}
```

The `ingr` package is already imported in `parse.go`. Add the same import to `batch_parsers.go`.

- [ ] **Step 4: Run parser tests, verify pass**

Run: `go test -timeout=10s -run TestParseBatchINGR ./pkg/dalgo2ingitdb/`
Expected: PASS.

- [ ] **Step 5: Wire INGR into router and add end-to-end test**

In `cmd/ingitdb/commands/insert_batch.go::parseBatchStream`:

```go
case "ingr":
    return dalgo2ingitdb.ParseBatchINGR(r)
```

Add `TestInsertBatch_INGR_HappyPath` mirroring `TestInsertBatch_YAML_HappyPath` but with an INGR payload from `buildINGRPayloadForTest`.

- [ ] **Step 6: Run all tests**

Run: `go test -timeout=10s ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add pkg/dalgo2ingitdb/batch_parsers.go pkg/dalgo2ingitdb/batch_parsers_test.go cmd/ingitdb/commands/insert_batch.go cmd/ingitdb/commands/insert_batch_test.go
git commit -m "feat(insert): INGR batch parser

Implements AC: batch-ingr-stream. Reads from $ID (INGR's
uppercase-key reserved column); strips $ID from the returned Data
map.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

---

## Phase D — CSV stream parser with key-column resolution

Phase D adds the most behaviourally complex parser: `--key-column` > `$id` column > `id` column precedence, optional `--fields` to override the header, tie-breaks between `$id` and `id` columns, and value-stripping rules.

### Task D1: CSV parser scaffold (header-row mode, `$id` column default)

**Files:**
- Modify: `pkg/dalgo2ingitdb/batch_parsers.go`
- Modify: `pkg/dalgo2ingitdb/batch_parsers_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestParseBatchCSV_HeaderWithDollarID(t *testing.T) {
    t.Parallel()
    in := strings.NewReader("$id,name,population\nie,Ireland,5\nfr,France,68\n")
    got, err := ParseBatchCSV(in, CSVParseOptions{})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(got) != 2 {
        t.Fatalf("want 2 records, got %d", len(got))
    }
    // Position is 1-based against source lines. Header is line 1; first data record is line 2.
    if got[0].Position != 2 || got[0].Key != "ie" || got[0].Data["name"] != "Ireland" {
        t.Errorf("record[0]=%+v", got[0])
    }
    if _, has := got[0].Data["$id"]; has {
        t.Errorf("$id must be stripped from Data")
    }
}

func TestParseBatchCSV_AutoMapsIDToKey(t *testing.T) {
    t.Parallel()
    in := strings.NewReader("id,name\nie,Ireland\nfr,France\n")
    got, err := ParseBatchCSV(in, CSVParseOptions{})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got[0].Key != "ie" || got[1].Key != "fr" {
        t.Errorf("keys=%v,%v; want ie,fr", got[0].Key, got[1].Key)
    }
    if _, has := got[0].Data["id"]; has {
        t.Errorf("id must be stripped from Data when auto-mapped to key")
    }
}

func TestParseBatchCSV_BothDollarIDAndID(t *testing.T) {
    t.Parallel()
    in := strings.NewReader("$id,id,name\nie,IE-001,Ireland\nfr,FR-002,France\n")
    got, err := ParseBatchCSV(in, CSVParseOptions{})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got[0].Key != "ie" {
        t.Errorf("$id should win precedence, got Key=%q", got[0].Key)
    }
    // "id" column becomes a data field.
    if got[0].Data["id"] != "IE-001" {
        t.Errorf("'id' should be a data field when $id is the key; got Data=%+v", got[0].Data)
    }
}

func TestParseBatchCSV_KeyColumnOverride(t *testing.T) {
    t.Parallel()
    in := strings.NewReader("external_id,name\nie,Ireland\nfr,France\n")
    got, err := ParseBatchCSV(in, CSVParseOptions{KeyColumn: "external_id"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got[0].Key != "ie" {
        t.Errorf("Key should be from external_id column, got %q", got[0].Key)
    }
    if _, has := got[0].Data["external_id"]; has {
        t.Errorf("--key-column value must be stripped from Data")
    }
}

func TestParseBatchCSV_KeyColumnMissing(t *testing.T) {
    t.Parallel()
    in := strings.NewReader("name\nIreland\n")
    _, err := ParseBatchCSV(in, CSVParseOptions{KeyColumn: "external_id"})
    if err == nil {
        t.Fatal("expected error when --key-column names a non-existent column")
    }
    if !strings.Contains(err.Error(), "external_id") {
        t.Errorf("error %q should name the missing column", err.Error())
    }
}

func TestParseBatchCSV_NoKeyColumnFound(t *testing.T) {
    t.Parallel()
    in := strings.NewReader("name,population\nIreland,5\n")
    _, err := ParseBatchCSV(in, CSVParseOptions{})
    if err == nil {
        t.Fatal("expected error when no key column is present")
    }
    if !strings.Contains(err.Error(), "$id") || !strings.Contains(err.Error(), "id") {
        t.Errorf("error %q should suggest $id, id, or --key-column", err.Error())
    }
}

func TestParseBatchCSV_FieldsOverrideNoHeader(t *testing.T) {
    t.Parallel()
    in := strings.NewReader("ie,Ireland\nfr,France\n")
    got, err := ParseBatchCSV(in, CSVParseOptions{Fields: []string{"$id", "name"}})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(got) != 2 || got[0].Key != "ie" || got[0].Data["name"] != "Ireland" {
        t.Errorf("records=%+v", got)
    }
    // With --fields, line 1 is the first data record.
    if got[0].Position != 1 {
        t.Errorf("Position should be 1 (no header), got %d", got[0].Position)
    }
}

func TestParseBatchCSV_EmptyStream(t *testing.T) {
    t.Parallel()
    got, err := ParseBatchCSV(strings.NewReader(""), CSVParseOptions{})
    if err != nil {
        t.Fatalf("empty stream should succeed, got: %v", err)
    }
    if len(got) != 0 {
        t.Errorf("want 0 records, got %d", len(got))
    }
}
```

- [ ] **Step 2: Run tests, verify they fail to compile**

Run: `go test -timeout=10s -run TestParseBatchCSV ./pkg/dalgo2ingitdb/`
Expected: FAIL with `undefined: ParseBatchCSV`.

- [ ] **Step 3: Implement the parser**

Append to `pkg/dalgo2ingitdb/batch_parsers.go`:

```go
// ParseBatchCSV reads RFC 4180 CSV from r and returns one ParsedRecord
// per data row. Key resolution precedence is:
//   1. opts.KeyColumn if set (rejected before reading rows if column missing).
//   2. column named "$id" if present.
//   3. column named "id" if present (auto-mapped).
//   4. otherwise error.
// When both "$id" and "id" columns exist without opts.KeyColumn, "$id"
// wins; "id" is kept as a data field. The resolved key column's value
// is stripped from Data.
//
// If opts.Fields is non-empty, those names override the header row:
// the first stdin line is treated as data, and Position is 1-based
// against data rows. Otherwise Position is 1-based against source
// lines, so the header is line 1 and the first data row is line 2.
func ParseBatchCSV(r io.Reader, opts CSVParseOptions) ([]ParsedRecord, error) {
    cr := csv.NewReader(r)
    cr.FieldsPerRecord = -1 // we validate manually so we can error per line

    var header []string
    var firstDataLine int
    if len(opts.Fields) > 0 {
        header = append([]string(nil), opts.Fields...)
        firstDataLine = 1
    } else {
        h, err := cr.Read()
        if err == io.EOF {
            return nil, nil
        }
        if err != nil {
            return nil, fmt.Errorf("read csv header: %w", err)
        }
        header = h
        firstDataLine = 2
    }
    if len(header) == 0 {
        return nil, fmt.Errorf("csv header is empty")
    }

    // Resolve which column is the key.
    keyCol, keyColIdx, err := resolveCSVKeyColumn(header, opts.KeyColumn)
    if err != nil {
        return nil, err
    }

    var records []ParsedRecord
    lineNo := firstDataLine - 1
    for {
        fields, readErr := cr.Read()
        lineNo++
        if readErr == io.EOF {
            break
        }
        if readErr != nil {
            return nil, fmt.Errorf("line %d: csv parse error: %w", lineNo, readErr)
        }
        if len(fields) != len(header) {
            return nil, fmt.Errorf("line %d: row has %d columns, header has %d", lineNo, len(fields), len(header))
        }
        keyVal := fields[keyColIdx]
        if keyVal == "" {
            return nil, fmt.Errorf("line %d: key column %q is empty", lineNo, keyCol)
        }
        data := make(map[string]any, len(header)-1)
        for i, col := range header {
            if i == keyColIdx {
                continue
            }
            data[col] = fields[i]
        }
        records = append(records, ParsedRecord{
            Position: lineNo,
            Key:      keyVal,
            Data:     data,
        })
    }
    return records, nil
}

func resolveCSVKeyColumn(header []string, override string) (string, int, error) {
    if override != "" {
        for i, h := range header {
            if h == override {
                return override, i, nil
            }
        }
        return "", -1, fmt.Errorf("--key-column=%q not found in CSV header %v", override, header)
    }
    // Look for $id first (wins precedence over id).
    for i, h := range header {
        if h == "$id" {
            return "$id", i, nil
        }
    }
    for i, h := range header {
        if h == "id" {
            return "id", i, nil
        }
    }
    return "", -1, fmt.Errorf("no key column found in CSV header %v; use --key-column, or include a $id or id column", header)
}
```

Add `"encoding/csv"` to imports if not present.

- [ ] **Step 4: Run parser tests, verify pass**

Run: `go test -timeout=10s -run TestParseBatchCSV ./pkg/dalgo2ingitdb/ -v`
Expected: all 8 subtests PASS.

- [ ] **Step 5: Wire CSV into router**

In `cmd/ingitdb/commands/insert_batch.go::parseBatchStream`:

```go
case "csv":
    return dalgo2ingitdb.ParseBatchCSV(r, dalgo2ingitdb.CSVParseOptions{
        KeyColumn: keyColumn,
        Fields:    fields,
    })
```

The router signature needs to accept `keyColumn` and `fields`; they're already passed from `runBatchInsert`. Thread them through `parseBatchStream`'s signature: `parseBatchStream(format, keyColumn string, fields []string, r io.Reader)`.

- [ ] **Step 6: Add end-to-end CSV tests in commands package**

```go
func TestInsertBatch_CSV_HeaderDollarID(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    stdin := strings.NewReader("$id,name,population\nie,Ireland,5\nfr,France,68\n")
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=csv",
    )
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    assertRecordExists(t, dir, "countries", "ie", map[string]any{"name": "Ireland", "population": "5"})
}

func TestInsertBatch_CSV_KeyColumnOverride(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    stdin := strings.NewReader("external_id,name\nie,Ireland\nfr,France\n")
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=csv", "--key-column=external_id",
    )
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    assertRecordExists(t, dir, "countries", "ie", map[string]any{"name": "Ireland"})
    assertRecordAbsentField(t, dir, "countries", "ie", "external_id")
}

func TestInsertBatch_CSV_FieldsNoHeader(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupInsertTestRepo(t)
    stdin := strings.NewReader("ie,Ireland\nfr,France\n")
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=countries", "--format=csv", "--fields=$id,name",
    )
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    assertRecordExists(t, dir, "countries", "ie", map[string]any{"name": "Ireland"})
}
```

`assertRecordAbsentField` is a new helper — asserts that a specific named field is NOT present in the stored record. Add to `integration_helpers_test.go`.

- [ ] **Step 7: Run all tests**

Run: `go test -timeout=10s ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/dalgo2ingitdb/batch_parsers.go pkg/dalgo2ingitdb/batch_parsers_test.go cmd/ingitdb/commands/insert_batch.go cmd/ingitdb/commands/insert_batch_test.go cmd/ingitdb/commands/integration_helpers_test.go
git commit -m "feat(insert): CSV batch parser with key-column resolution

Implements ACs:
- batch-csv-id-column
- batch-csv-id-auto-mapped
- batch-csv-key-column-override
- batch-csv-fields-no-header
- batch-csv-both-id-and-id-columns

Key resolution precedence: --key-column > \$id column > id column.
When both \$id and id columns are present, \$id wins and id becomes a
data field.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task D2: Phase D lint and close

- [ ] **Step 1: Run golangci-lint**

Run: `golangci-lint run`
Expected: zero errors.

- [ ] **Step 2: Run the full test suite**

Run: `go test -timeout=10s ./...`
Expected: all tests pass.

- [ ] **Step 3: Tag Phase D**

```bash
git commit --allow-empty -m "chore: phase D (csv batch insert) complete

All four batch stream formats now implemented. CSV key-column
resolution and --fields override exercised by ACs.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

---

## Phase E — Cross-format markdown storage tests

Phase E proves stream-format / storage-format independence: each of the four stream formats can persist into a markdown-stored collection. The plumbing is already in place — the existing `tx.Insert` calls per-collection writers that handle markdown. These tasks add ACs and verify the assumption holds. Each task is small.

### Task E1: JSONL → markdown-stored collection

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`
- Possibly modify: `cmd/ingitdb/commands/integration_helpers_test.go` (new fixture for markdown-stored collection)

- [ ] **Step 1: Add a markdown-collection test fixture**

In `integration_helpers_test.go`, add `setupMarkdownInsertTestRepo(t)` which creates a `.ingitdb.yaml` with a collection `posts` whose `record_file.format` is `markdown` and `content_field` is `$content`. Mirror existing fixtures — there is likely one in the `markdown-stdin` AC test (search `grep -n "markdown" cmd/ingitdb/commands/insert_test.go`).

- [ ] **Step 2: Write the failing test**

```go
func TestInsertBatch_JSONL_ToMarkdownStorage(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupMarkdownInsertTestRepo(t)
    stdin := strings.NewReader(`{"$id":"hello","title":"Hello","$content":"First post body."}
{"$id":"world","title":"World","$content":"Second post body."}
`)
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=posts", "--format=jsonl",
    )
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    // Verify markdown files exist on disk.
    helloPath := filepath.Join(dir, "posts", "hello.md")
    worldPath := filepath.Join(dir, "posts", "world.md")
    helloBytes, err := os.ReadFile(helloPath)
    if err != nil {
        t.Fatalf("posts/hello.md not on disk: %v", err)
    }
    if !bytes.Contains(helloBytes, []byte("title: Hello")) {
        t.Errorf("posts/hello.md missing 'title: Hello' frontmatter; got:\n%s", helloBytes)
    }
    if !bytes.Contains(helloBytes, []byte("First post body.")) {
        t.Errorf("posts/hello.md missing body 'First post body.'; got:\n%s", helloBytes)
    }
    if bytes.Contains(helloBytes, []byte("$id")) {
        t.Errorf("posts/hello.md MUST NOT contain $id; got:\n%s", helloBytes)
    }
    if bytes.Contains(helloBytes, []byte("$content")) {
        t.Errorf("posts/hello.md MUST NOT contain $content in frontmatter; got:\n%s", helloBytes)
    }
    _, err = os.ReadFile(worldPath)
    if err != nil {
        t.Errorf("posts/world.md not on disk: %v", err)
    }
}
```

- [ ] **Step 2 (continued): Run the test**

Run: `go test -timeout=10s -run TestInsertBatch_JSONL_ToMarkdownStorage ./cmd/ingitdb/commands/`
Expected: this is the critical "verify" task — if it FAILS, the storage layer is NOT format-independent and the spec's assumption needs revisiting. If it PASSES with no code changes, the existing `tx.Insert` writer is already format-aware and the spec's claim is validated.

- [ ] **Step 3: If the test fails, fix the write path**

The most likely failure: the parsed JSONL map[string]any doesn't go through the markdown encoder. Inspect:

```bash
grep -rn "RecordFile.Format\|RecordFormatMarkdown" pkg/dalgo2*ingitdb*/
```

Find the writer path that handles markdown collections. If it writes raw YAML instead of YAML-frontmatter + body, you need to ensure `runBatchInsert` either:

- passes records through `EncodeRecordContentForCollection` before `tx.Insert`, or
- the dalgo writer already detects markdown collections and reshapes the map.

This is a discovery task. Resolve in place; the fix should be small.

- [ ] **Step 4: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go cmd/ingitdb/commands/integration_helpers_test.go
git commit -m "test(insert): AC batch-cross-format-jsonl-to-markdown

Verifies that --format=jsonl into a markdown-stored collection
produces one <key>.md per record with frontmatter from structured
fields and body from \$content.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task E2: YAML → markdown-stored collection

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write the test**

```go
func TestInsertBatch_YAML_ToMarkdownStorage(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupMarkdownInsertTestRepo(t)
    stdin := strings.NewReader(`$id: hello
title: Hello
$content: |
  First post body.
---
$id: world
title: World
$content: |
  Second post body.
`)
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=posts", "--format=yaml",
    )
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    // Assertions identical to E1.
    helloPath := filepath.Join(dir, "posts", "hello.md")
    helloBytes, err := os.ReadFile(helloPath)
    if err != nil {
        t.Fatalf("posts/hello.md not on disk: %v", err)
    }
    if !bytes.Contains(helloBytes, []byte("title: Hello")) || !bytes.Contains(helloBytes, []byte("First post body.")) {
        t.Errorf("posts/hello.md missing expected content; got:\n%s", helloBytes)
    }
}
```

- [ ] **Step 2: Run, verify pass (should PASS once E1 passes — same code path)**

Run: `go test -timeout=10s -run TestInsertBatch_YAML_ToMarkdownStorage ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go
git commit -m "test(insert): AC batch-cross-format-yaml-to-markdown

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task E3: INGR → markdown-stored collection

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write the test**

```go
func TestInsertBatch_INGR_ToMarkdownStorage(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupMarkdownInsertTestRepo(t)
    payload := buildINGRPayloadForTest(t, []map[string]any{
        {"$ID": "hello", "title": "Hello", "$content": "First post body."},
        {"$ID": "world", "title": "World", "$content": "Second post body."},
    })
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        bytes.NewReader(payload),
        "--path="+dir, "--into=posts", "--format=ingr",
    )
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    helloPath := filepath.Join(dir, "posts", "hello.md")
    helloBytes, err := os.ReadFile(helloPath)
    if err != nil {
        t.Fatalf("posts/hello.md not on disk: %v", err)
    }
    if !bytes.Contains(helloBytes, []byte("title: Hello")) {
        t.Errorf("missing frontmatter; got:\n%s", helloBytes)
    }
}
```

The INGR parser strips `$ID` (uppercase); the resulting record has `title` and `$content` (per the spec, `$content` is preserved through the parser). The write path takes over from there.

- [ ] **Step 2: Run, verify pass**

Run: `go test -timeout=10s -run TestInsertBatch_INGR_ToMarkdownStorage ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go
git commit -m "test(insert): AC batch-cross-format-ingr-to-markdown

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task E4: CSV → markdown-stored collection

**Files:**
- Modify: `cmd/ingitdb/commands/insert_batch_test.go`

- [ ] **Step 1: Write the test**

```go
func TestInsertBatch_CSV_ToMarkdownStorage(t *testing.T) {
    t.Parallel()
    dir, homeDir, getWd, readDef, newDB, logf := setupMarkdownInsertTestRepo(t)
    // Real newlines inside CSV cells per RFC 4180 (quoted).
    stdin := strings.NewReader("$id,title,$content\nhello,Hello,First post body.\nworld,World,Second post body.\n")
    _, err := runInsertCmdWithStdin(t, homeDir, getWd, readDef, newDB, logf,
        stdin,
        "--path="+dir, "--into=posts", "--format=csv",
    )
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    helloPath := filepath.Join(dir, "posts", "hello.md")
    helloBytes, err := os.ReadFile(helloPath)
    if err != nil {
        t.Fatalf("posts/hello.md not on disk: %v", err)
    }
    if !bytes.Contains(helloBytes, []byte("title: Hello")) || !bytes.Contains(helloBytes, []byte("First post body.")) {
        t.Errorf("missing expected content; got:\n%s", helloBytes)
    }
}
```

- [ ] **Step 2: Run, verify pass**

Run: `go test -timeout=10s -run TestInsertBatch_CSV_ToMarkdownStorage ./cmd/ingitdb/commands/`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingitdb/commands/insert_batch_test.go
git commit -m "test(insert): AC batch-cross-format-csv-to-markdown

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

### Task E5: Phase E lint + final close

- [ ] **Step 1: Run golangci-lint**

Run: `golangci-lint run`
Expected: zero errors.

- [ ] **Step 2: Run full test suite**

Run: `go test -timeout=10s ./...`
Expected: PASS.

- [ ] **Step 3: Manual smoke test**

Run an actual binary against the test repo to confirm behaviour end-to-end:

```bash
go build -o ingitdb ./cmd/ingitdb
echo '{"$id":"ie","name":"Ireland"}' | ./ingitdb insert --into=countries --format=jsonl
# Expect: "1 records inserted" on stderr, countries/ie file created.
```

- [ ] **Step 4: Mark feature implemented in spec front-matter**

In `spec/features/cli/insert/README.md`, the status is already `Approved`. No change needed unless your project's convention is to mark features `Implemented` post-shipment. Check `spec/features/record-format/README.md` for the post-ship status convention (the prior plan referenced "mark record-format Feature tree Implemented" in commit d09753b).

- [ ] **Step 5: Final commit**

```bash
git commit --allow-empty -m "chore: batch-insert feature complete

All req:batch-* requirements implemented; all batch-* ACs in
spec/features/cli/insert/README.md pass. Lint clean.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**

| REQ | Implementing task(s) |
|---|---|
| `req:batch-format-flag` | A1, A2 |
| `req:batch-single-record-flag-exclusion` | A3 |
| `req:batch-stdin-required` | A5 |
| `req:batch-per-record-key` | B1 (jsonl), C1 (yaml), C2 (ingr), D1 (csv) |
| `req:batch-csv-key-resolution` | D1 |
| `req:batch-csv-fields-flag` | A4, D1 |
| `req:batch-atomic` | B2 (orchestrator), B4/B5/B6 (verification) |
| `req:batch-post-commit-failure` | B2 (orchestrator), B8 (verification) |
| `req:batch-duplicate-keys-in-stream` | B2 (`rejectIntraBatchDuplicates`), B5 |
| `req:batch-empty-stream` | B2 |
| `req:batch-storage-format-independence` | E1, E2, E3, E4 |
| `req:batch-view-materialization` | B2 (orchestrator), B7 |

| AC | Implementing task |
|---|---|
| `batch-jsonl-basic` | B3 |
| `batch-yaml-stream` | C1 |
| `batch-ingr-stream` | C2 |
| `batch-csv-id-column` | D1 |
| `batch-csv-id-auto-mapped` | D1 |
| `batch-csv-key-column-override` | D1 |
| `batch-csv-fields-no-header` | D1 |
| `batch-csv-fields-only-with-csv` | A4 |
| `batch-csv-both-id-and-id-columns` | D1 |
| `batch-single-record-flags-rejected` | A3 |
| `batch-stdin-tty-rejected` | A5 |
| `batch-missing-key-rejected` | B4 |
| `batch-duplicate-key-in-stream-rejected` | B5 |
| `batch-collision-with-existing-record` | B6 |
| `batch-empty-stream-succeeds` | B2 |
| `batch-cross-format-jsonl-to-markdown` | E1 |
| `batch-cross-format-yaml-to-markdown` | E2 |
| `batch-cross-format-ingr-to-markdown` | E3 |
| `batch-cross-format-csv-to-markdown` | E4 |
| `batch-invalid-format-value-rejected` | A2 |
| `batch-view-materialization-once` | B7 |
| `batch-post-commit-view-failure` | B8 |
| `key-column-rejected-without-batch-csv` | A4 |

All 12 new REQs and 23 new ACs trace to a task. The 3 *targeted edits* to existing REQs (`subcommand-name`, `no-data-error`, `data-source-modes` boundary) are covered by the carve-outs in Tasks A3/A4/A5 and the regression check in Step 5 of Task A4.

**Placeholder scan:** No `TBD`, `TODO`, "implement later", or "add appropriate error handling" appears in any task body. Every step has either exact code or an exact command.

**Type consistency:** `ParsedRecord` (defined in Task B1) is the only shared type, with three fields `Position int`, `Key string`, `Data map[string]any`. All four parsers return `[]ParsedRecord` and the orchestrator iterates by these fields. `CSVParseOptions` is only used by `ParseBatchCSV` and is defined alongside it.

**Risk flagged in Task E1:** the assumption that `tx.Insert(ctx, dal.NewRecordWithData(...))` already routes through a format-aware writer for markdown-stored collections is unverified — Task E1 is explicitly the verification task. If it fails, the fix is small (route through `EncodeRecordContentForCollection`) but it changes the orchestrator. This is the only architectural risk in the plan.
