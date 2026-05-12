# Create Record: Stdin & Editor Input — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance `ingitdb create record` so that when `--data` is absent the command reads from piped stdin, or opens `$EDITOR` with a schema template when `--edit` is passed, and errors cleanly when neither source is available.

**Architecture:** All changes live in `cmd/ingitdb/commands/create_record.go` and `cmd/ingitdb/commands/create.go`. Three injectable dependencies (`stdin io.Reader`, `isStdinTTY func() bool`, `openEditor func(string) error`) are added for testability — all default to production implementations when `nil`. The command delegates stdin/file parsing to the existing `dalgo2ingitdb.ParseRecordContentForCollection`, which already handles all formats (yaml, yml, json, toml, markdown).

**Tech Stack:** Go standard library (`io`, `os`, `os/exec`, `sort`, `bytes`), `github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb`, `github.com/ingitdb/ingitdb-cli/pkg/ingitdb`.

**Spec:** `spec/features/cli/create-record/README.md`

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Modify | `cmd/ingitdb/commands/create_record.go` | All new logic + helpers |
| Modify | `cmd/ingitdb/commands/create.go` | Forward 3 new params |
| Modify | `cmd/ingitdb/main.go` | Pass `nil, nil, nil` for new params |
| Modify | `cmd/ingitdb/commands/create_record_test.go` | Add `nil, nil, nil` to existing calls; add new tests |
| Modify | `cmd/ingitdb/commands/create_record_github_test.go` | Add `nil, nil, nil` to existing `Create` calls |

---

## Task 1 — Make `--data` optional; add TTY error path

**Context:** `--data` is currently `MarkFlagRequired`. Removing that requirement is the prerequisite for all new input paths. When no data source is available (TTY stdin, no `--data`, no `--edit`) the command must exit 1 with a helpful message.

**Files:**
- Modify: `cmd/ingitdb/commands/create_record.go`
- Modify: `cmd/ingitdb/commands/create.go`
- Modify: `cmd/ingitdb/main.go`
- Modify: `cmd/ingitdb/commands/create_record_test.go`
- Modify: `cmd/ingitdb/commands/create_record_github_test.go`

- [ ] **Step 1.1 — Write the failing test**

Add to `cmd/ingitdb/commands/create_record_test.go`:

```go
func TestCreate_TTYError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	// Simulate a TTY: isStdinTTY returns true, no --data, no --edit.
	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		strings.NewReader(""),          // stdin (unused because TTY)
		func() bool { return true },    // isStdinTTY
		nil,                            // openEditor (unused)
	)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/x")
	if err == nil {
		t.Fatal("expected error when stdin is a TTY and no --data or --edit")
	}
	if !strings.Contains(err.Error(), "stdin") && !strings.Contains(err.Error(), "--edit") {
		t.Fatalf("error should mention stdin or --edit, got: %v", err)
	}
}
```

Add `"strings"` to the import block of `create_record_test.go`.

- [ ] **Step 1.2 — Run the test to confirm it fails**

```bash
go test -timeout=10s -run TestCreate_TTYError ./cmd/ingitdb/commands/
```

Expected: **compile error** — `Create` doesn't accept the new params yet.

- [ ] **Step 1.3 — Update `create.go` to accept new params**

Replace the entire `cmd/ingitdb/commands/create.go` with:

```go
package commands

import (
	"io"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Create returns the create command group.
func Create(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	stdin io.Reader,
	isStdinTTY func() bool,
	openEditor func(tmpPath string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"c"},
		Short:   "Create database objects",
	}
	cmd.AddCommand(createRecord(homeDir, getWd, readDefinition, newDB, logf, stdin, isStdinTTY, openEditor))
	return cmd
}
```

- [ ] **Step 1.4 — Rewrite `create_record.go` with the new signature and TTY error path**

Replace the entire `cmd/ingitdb/commands/create_record.go` with:

```go
package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func createRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	stdin io.Reader,
	isStdinTTY func() bool,
	openEditor func(tmpPath string) error,
) *cobra.Command {
	// Apply defaults for injectable deps.
	if stdin == nil {
		stdin = os.Stdin
	}
	if isStdinTTY == nil {
		isStdinTTY = func() bool { return isFdTTY(os.Stdin) }
	}
	if openEditor == nil {
		openEditor = defaultOpenEditor
	}

	cmd := &cobra.Command{
		Use:   "record",
		Short: "Create a new record in a collection",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			id, _ := cmd.Flags().GetString("id")
			dataStr, _ := cmd.Flags().GetString("data")
			editFlag, _ := cmd.Flags().GetBool("edit")

			rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}
			if rctx.dirPath != "" {
				logf("inGitDB db path: ", rctx.dirPath)
			}

			var data map[string]any
			switch {
			case dataStr != "":
				if unmarshalErr := yaml.Unmarshal([]byte(dataStr), &data); unmarshalErr != nil {
					return fmt.Errorf("failed to parse --data: %w", unmarshalErr)
				}
			case editFlag:
				var noChanges bool
				data, noChanges, err = runWithEditor(rctx.colDef, openEditor)
				if err != nil {
					return err
				}
				if noChanges {
					logf("no changes — record not created")
					return nil
				}
			case !isStdinTTY():
				content, readErr := io.ReadAll(stdin)
				if readErr != nil {
					return fmt.Errorf("failed to read stdin: %w", readErr)
				}
				data, err = dalgo2ingitdb.ParseRecordContentForCollection(content, rctx.colDef)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("no record content provided — use --data, --edit, or pipe content via stdin")
			}

			key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
			record := dal.NewRecordWithData(key, data)
			err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			})
			if err != nil {
				return err
			}
			return buildLocalViews(ctx, rctx)
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	cmd.Flags().String("id", "", "record ID in the format collection/path/key (e.g. todo.countries/ie)")
	_ = cmd.MarkFlagRequired("id")
	cmd.Flags().String("data", "", "record data as YAML or JSON (e.g. '{title: \"Ireland\"}')")
	cmd.Flags().Bool("edit", false, "open $EDITOR with a schema-derived template")
	return cmd
}

// isFdTTY reports whether f's file descriptor is a terminal.
func isFdTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// runWithEditor writes a schema-derived template to a temp file, opens it in
// the configured editor, and returns the parsed record data. Returns
// (nil, true, nil) when the file was not modified (no-op edit).
func runWithEditor(colDef *ingitdb.CollectionDef, openEditor func(string) error) (map[string]any, bool, error) {
	template := buildRecordTemplate(colDef)

	ext := recordFormatExt(colDef.RecordFile.Format)
	tmpFile, err := os.CreateTemp("", "ingitdb-*."+ext)
	if err != nil {
		return nil, false, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err = tmpFile.Write(template); err != nil {
		tmpFile.Close()
		return nil, false, fmt.Errorf("write template: %w", err)
	}
	tmpFile.Close()

	if err = openEditor(tmpPath); err != nil {
		return nil, false, fmt.Errorf("editor: %w", err)
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, false, fmt.Errorf("read edited file: %w", err)
	}

	if bytes.Equal(content, template) {
		return nil, true, nil
	}

	data, err := dalgo2ingitdb.ParseRecordContentForCollection(content, colDef)
	return data, false, err
}

// buildRecordTemplate returns a byte slice pre-filled with an empty record
// template for the given collection. For markdown the template has YAML
// frontmatter delimiters; for other formats it is a bare YAML skeleton.
func buildRecordTemplate(colDef *ingitdb.CollectionDef) []byte {
	keys := orderedColumnKeys(colDef)
	var buf bytes.Buffer
	if colDef.RecordFile != nil && colDef.RecordFile.Format == ingitdb.RecordFormatMarkdown {
		buf.WriteString("---\n")
		for _, k := range keys {
			buf.WriteString(k + ": \n")
		}
		buf.WriteString("---\n")
	} else {
		for _, k := range keys {
			buf.WriteString(k + ": \n")
		}
	}
	return buf.Bytes()
}

// orderedColumnKeys returns the column names in canonical order:
// ColumnsOrder entries first (skipping absent columns), then remaining
// columns alphabetically.
func orderedColumnKeys(colDef *ingitdb.CollectionDef) []string {
	seen := make(map[string]bool)
	var ordered []string
	for _, k := range colDef.ColumnsOrder {
		if _, ok := colDef.Columns[k]; !ok || seen[k] {
			continue
		}
		seen[k] = true
		ordered = append(ordered, k)
	}
	var rest []string
	for k := range colDef.Columns {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(ordered, rest...)
}

// recordFormatExt returns a file extension for the given record format,
// used when naming the editor temp file.
func recordFormatExt(format ingitdb.RecordFormat) string {
	switch format {
	case ingitdb.RecordFormatMarkdown:
		return "md"
	case ingitdb.RecordFormatJSON:
		return "json"
	case ingitdb.RecordFormatTOML:
		return "toml"
	default:
		return "yaml"
	}
}

// defaultOpenEditor opens tmpPath in $EDITOR (falling back to vi).
// Uses exec.Command to avoid shell injection.
func defaultOpenEditor(tmpPath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, tmpPath)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
```

- [ ] **Step 1.5 — Update `main.go` to pass `nil, nil, nil`**

In `cmd/ingitdb/main.go`, find the line:
```go
commands.Create(homeDir, getWd, readDefinition, newDB, logf),
```
Replace with:
```go
commands.Create(homeDir, getWd, readDefinition, newDB, logf, nil, nil, nil),
```

- [ ] **Step 1.6 — Update existing tests to pass new params**

In `cmd/ingitdb/commands/create_record_test.go`, every call to `Create(homeDir, getWd, readDef, newDB, logf)` must become `Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)`. There are 5 such calls (in `TestCreate_Success`, `TestCreate_MissingID`, `TestCreate_InvalidYAML`, `TestCreate_CollectionNotFound`, `TestCreate_ReadDefinitionError`).

In `cmd/ingitdb/commands/create_record_github_test.go`, do the same for each `Create(...)` call (there are 4).

- [ ] **Step 1.7 — Run all tests**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/ -run "TestCreate"
```

Expected: all existing `TestCreate_*` tests pass; `TestCreate_TTYError` passes.

- [ ] **Step 1.8 — Run the full build + test suite**

```bash
go build ./... && go test -timeout=10s ./...
```

Expected: no compile errors, no test failures.

- [ ] **Step 1.9 — Commit**

```bash
git add cmd/ingitdb/commands/create_record.go \
        cmd/ingitdb/commands/create.go \
        cmd/ingitdb/main.go \
        cmd/ingitdb/commands/create_record_test.go \
        cmd/ingitdb/commands/create_record_github_test.go
git commit -m "$(cat <<'EOF'
feat(cli): make create record --data optional; add TTY error

When neither --data nor --edit is supplied and stdin is a terminal,
the command now exits 1 with a message directing the user to use
--data, --edit, or pipe content. --data is no longer required by
cobra so stdin and editor paths can be reached.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2 — Stdin input path (yaml, markdown, toml)

**Context:** With `--data` optional and the TTY error in place, the stdin branch (`!isStdinTTY()`) now needs test coverage. `ParseRecordContentForCollection` already handles all formats — the tests just need the right collection definitions.

**Files:**
- Modify: `cmd/ingitdb/commands/create_record_test.go`
- Modify: `cmd/ingitdb/commands/helpers_test.go` (add markdown + toml test defs)

- [ ] **Step 2.1 — Add test collection helpers for markdown and TOML**

Add to `cmd/ingitdb/commands/helpers_test.go`:

```go
// testMarkdownDef returns a Definition with a single markdown SingleRecord
// collection at dirPath. The collection has title, category, and $content columns.
func testMarkdownDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.notes": {
				ID:      "test.notes",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.md",
					Format:     ingitdb.RecordFormatMarkdown,
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title":                              {Type: ingitdb.ColumnTypeString},
					"category":                           {Type: ingitdb.ColumnTypeString},
					ingitdb.DefaultMarkdownContentField:  {Type: ingitdb.ColumnTypeString},
				},
				ColumnsOrder: []string{"title", "category"},
			},
		},
	}
}

// testTOMLDef returns a Definition with a single TOML SingleRecord collection
// at dirPath. The collection has a single "name" column.
func testTOMLDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.things": {
				ID:      "test.things",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.toml",
					Format:     ingitdb.RecordFormatTOML,
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
}
```

- [ ] **Step 2.2 — Write the three failing stdin tests**

Add to `cmd/ingitdb/commands/create_record_test.go`:

```go
func TestCreate_StdinYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	stdinContent := strings.NewReader("name: Ireland\n")
	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		stdinContent,
		func() bool { return false }, // not a TTY
		nil,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/ie"); err != nil {
		t.Fatalf("Create via stdin YAML: %v", err)
	}

	path := filepath.Join(dir, "$records", "ie.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to be created: %v", path, err)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "Ireland") {
		t.Fatalf("expected record to contain Ireland, got: %s", content)
	}
}

func TestCreate_StdinMarkdown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testMarkdownDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	mdContent := "---\ntitle: Product 1\ncategory: software\n---\nBody here.\n"
	stdinContent := strings.NewReader(mdContent)
	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		stdinContent,
		func() bool { return false },
		nil,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.notes/p1"); err != nil {
		t.Fatalf("Create via stdin Markdown: %v", err)
	}

	path := filepath.Join(dir, "$records", "p1.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to be created: %v", path, err)
	}
	fileBytes, _ := os.ReadFile(path)
	fileStr := string(fileBytes)
	if !strings.Contains(fileStr, "title: Product 1") {
		t.Fatalf("expected frontmatter title, got: %s", fileStr)
	}
	if !strings.Contains(fileStr, "Body here.") {
		t.Fatalf("expected body in record, got: %s", fileStr)
	}
}

func TestCreate_StdinTOML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testTOMLDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	stdinContent := strings.NewReader("name = \"Ireland\"\n")
	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		stdinContent,
		func() bool { return false },
		nil,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.things/ie"); err != nil {
		t.Fatalf("Create via stdin TOML: %v", err)
	}

	path := filepath.Join(dir, "$records", "ie.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to be created: %v", path, err)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "Ireland") {
		t.Fatalf("expected record to contain Ireland, got: %s", content)
	}
}
```

Add `"path/filepath"` to the import block of `create_record_test.go` (it may already be there).

- [ ] **Step 2.3 — Run the new tests to confirm they fail**

```bash
go test -timeout=10s -run "TestCreate_Stdin" ./cmd/ingitdb/commands/
```

Expected: compile failure (test helpers not yet added) or test failures on record file assertions.

- [ ] **Step 2.4 — Run the tests after adding the helpers**

```bash
go test -timeout=10s -run "TestCreate_Stdin" ./cmd/ingitdb/commands/
```

Expected: all three pass.

- [ ] **Step 2.5 — Run full test suite**

```bash
go build ./... && go test -timeout=10s ./...
```

Expected: all pass.

- [ ] **Step 2.6 — Commit**

```bash
git add cmd/ingitdb/commands/create_record_test.go \
        cmd/ingitdb/commands/helpers_test.go
git commit -m "$(cat <<'EOF'
test(cli): add stdin input tests for create record (yaml, markdown, toml)

Covers AC: creates-yaml-record-via-stdin, creates-markdown-record-via-stdin,
and creates-toml-record-via-stdin from spec/features/cli/create-record/README.md.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3 — `--edit` flag: template generation and editor launch

**Context:** `--edit` already exists in the cobra flag definition and in the `switch` dispatch (from Task 1). `runWithEditor`, `buildRecordTemplate`, and `orderedColumnKeys` are already in `create_record.go` (written in Task 1). This task adds test coverage for the editor paths and tests the template helper directly.

**Files:**
- Modify: `cmd/ingitdb/commands/create_record_test.go`

- [ ] **Step 3.1 — Write failing tests for template generation**

Add to `cmd/ingitdb/commands/create_record_test.go`:

```go
func TestBuildRecordTemplate_Markdown(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatMarkdown,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"title":    {Type: ingitdb.ColumnTypeString},
			"category": {Type: ingitdb.ColumnTypeString},
		},
		ColumnsOrder: []string{"title", "category"},
	}

	got := string(buildRecordTemplate(colDef))
	want := "---\ntitle: \ncategory: \n---\n"
	if got != want {
		t.Fatalf("markdown template:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestBuildRecordTemplate_YAML(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
		ColumnsOrder: []string{"name"},
	}

	got := string(buildRecordTemplate(colDef))
	want := "name: \n"
	if got != want {
		t.Fatalf("yaml template:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestBuildRecordTemplate_AlphaFallback(t *testing.T) {
	t.Parallel()

	// No ColumnsOrder: columns appear alphabetically.
	colDef := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"zebra": {Type: ingitdb.ColumnTypeString},
			"alpha": {Type: ingitdb.ColumnTypeString},
		},
	}

	got := string(buildRecordTemplate(colDef))
	want := "alpha: \nzebra: \n"
	if got != want {
		t.Fatalf("alphabetical template:\ngot:  %q\nwant: %q", got, want)
	}
}
```

- [ ] **Step 3.2 — Run to confirm they pass (helpers are already written in Task 1)**

```bash
go test -timeout=10s -run "TestBuildRecordTemplate" ./cmd/ingitdb/commands/
```

Expected: all three pass.

- [ ] **Step 3.3 — Write failing tests for `--edit` no-changes path**

Add to `cmd/ingitdb/commands/create_record_test.go`:

```go
func TestCreate_EditNoChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	var logMessages []string
	logf := func(args ...any) {
		logMessages = append(logMessages, fmt.Sprint(args...))
	}

	// A no-op editor that does not modify the file.
	noOpEditor := func(tmpPath string) error { return nil }

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		nil,
		func() bool { return true }, // TTY — would error without --edit
		noOpEditor,
	)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/x", "--edit")
	if err != nil {
		t.Fatalf("expected no error for no-op edit, got: %v", err)
	}

	// Record file must NOT be written.
	if _, statErr := os.Stat(filepath.Join(dir, "$records", "x.yaml")); statErr == nil {
		t.Fatal("record file should not be created for a no-op edit")
	}

	// "no changes" must appear in a log message.
	found := false
	for _, msg := range logMessages {
		if strings.Contains(msg, "no changes") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'no changes' in log output, got: %v", logMessages)
	}
}
```

Add `"fmt"` to the import block of `create_record_test.go`.

- [ ] **Step 3.4 — Run to confirm the no-changes test passes**

```bash
go test -timeout=10s -run "TestCreate_EditNoChanges" ./cmd/ingitdb/commands/
```

Expected: passes.

- [ ] **Step 3.5 — Write failing test for `--edit` with content modification**

Add to `cmd/ingitdb/commands/create_record_test.go`:

```go
func TestCreate_EditInserts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir) // yaml collection: test.items, key field: name

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	// An editor that overwrites the temp file with a known YAML payload.
	writingEditor := func(tmpPath string) error {
		return os.WriteFile(tmpPath, []byte("name: Ireland\n"), 0o644)
	}

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		nil,
		func() bool { return true },
		writingEditor,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/ie", "--edit"); err != nil {
		t.Fatalf("Create via --edit: %v", err)
	}

	path := filepath.Join(dir, "$records", "ie.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to be created: %v", path, err)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "Ireland") {
		t.Fatalf("expected record to contain Ireland, got: %s", content)
	}
}
```

- [ ] **Step 3.6 — Run to confirm the insert test passes**

```bash
go test -timeout=10s -run "TestCreate_EditInserts" ./cmd/ingitdb/commands/
```

Expected: passes.

- [ ] **Step 3.7 — Run full test suite**

```bash
go build ./... && go test -timeout=10s ./...
```

Expected: all pass, no regressions.

- [ ] **Step 3.8 — Run linter**

```bash
golangci-lint run
```

Expected: no errors.

- [ ] **Step 3.9 — Commit**

```bash
git add cmd/ingitdb/commands/create_record_test.go
git commit -m "$(cat <<'EOF'
test(cli): add --edit and template tests for create record

Covers AC: edit-flag-no-changes, edit-flag-inserts-on-save,
and buildRecordTemplate unit tests from spec/features/cli/create-record/README.md.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

**Spec coverage check:**

| AC | Covered by |
|----|-----------|
| `creates-local-record-via-data-flag` | Pre-existing `TestCreate_Success` (unchanged) |
| `creates-markdown-record-via-stdin` | `TestCreate_StdinMarkdown` (Task 2) |
| `creates-yaml-record-via-stdin` | `TestCreate_StdinYAML` (Task 2) |
| `creates-toml-record-via-stdin` | `TestCreate_StdinTOML` (Task 2) |
| `tty-without-data-or-edit-errors` | `TestCreate_TTYError` (Task 1) |
| `edit-flag-no-changes` | `TestCreate_EditNoChanges` (Task 3) |
| `edit-flag-inserts-on-save` | `TestCreate_EditInserts` (Task 3) |
| `creates-github-record-with-token` | Pre-existing `create_record_github_test.go` (unchanged) |

**REQ coverage check:**

| REQ | Task |
|-----|------|
| `subcommand-name` | pre-existing |
| `id-required` | pre-existing |
| `data-flag` | Task 1 (removes `MarkFlagRequired`) |
| `stdin-input` | Task 1 (dispatch logic) + Task 2 (tests) |
| `edit-flag` | Task 1 (runWithEditor, buildRecordTemplate) + Task 3 (tests) |
| `tty-error` | Task 1 |
| `source-selection` | pre-existing |
| `fails-if-exists` | pre-existing |
| `github-write-requires-token` | pre-existing |

**Placeholder scan:** No TBD, TODO, or vague steps found.

**Type consistency:** `runWithEditor` returns `(map[string]any, bool, error)` — `noChanges` bool is the middle return. Used correctly in `createRecord`'s `switch` block. `buildRecordTemplate` returns `[]byte` — matches usage in `runWithEditor`.

**Potential issue:** `testDef` defines the `test.items` collection with `{key}.yaml` in a flat dir (no `$records/` subdirectory). But `RecordsBasePath()` returns `$records` when the name contains `{key}`. The existing `TestCreate_Success` checks `filepath.Join(dir, "$records", "hello.yaml")`, so this is already correct. ✓

---

## Execution Handoff

Plan saved to `spec/plans/2026-05-12-create-record-stdin-edit.md`. Two execution options:

**1. Subagent-Driven (recommended)** — Fresh subagent per task, review between tasks, fast iteration. Use `superpowers:subagent-driven-development`.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch with checkpoints.

Which approach?
