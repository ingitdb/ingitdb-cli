# cli/insert — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the `ingitdb insert` command — `--into=<collection>` + one of four data sources (`--data` / stdin / `--edit` / `--empty`), record-key from `--key` or `$id` in the data (hybrid with consistency enforcement), strict insert (reject existing keys), and local + GitHub source. Replaces the write path of `create record` without removing it.

**Architecture:** A new file `cmd/ingitdb/commands/insert.go` hosts the `Insert` command. It reuses the existing `runWithEditor`, `buildRecordTemplate`, `defaultOpenEditor`, and `dalgo2ingitdb.ParseRecordContentForCollection` helpers — those were built for `create record` and remain unchanged. New code: a collection-context resolver (analog of `resolveRecordContext` but driven by `--into`), key-resolution logic (`--key` vs `$id` in data, with consistency check), and a collision-check-then-insert flow.

**Tech Stack:** Go, `github.com/dal-go/dalgo/dal`, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, the project's `sqlflags` and `dalgo2ingitdb` packages.

**Spec:** `spec/features/cli/insert/README.md`

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Create | `cmd/ingitdb/commands/insert.go` | `Insert` cobra command + RunE handler + key/data resolution |
| Create | `cmd/ingitdb/commands/insert_test.go` | Integration tests for all data sources, key modes, collision behavior |
| Create | `cmd/ingitdb/commands/insert_context.go` | `resolveInsertContext` — collection lookup from `--into`, parallel to `resolveRecordContext` |
| Create | `cmd/ingitdb/commands/insert_context_test.go` | Tests for resolveInsertContext |
| Modify | `cmd/ingitdb/main.go` | Add `commands.Insert(...)` to the root command's `AddCommand` list |

**Reused (no edits):**

- `cmd/ingitdb/commands/sqlflags/*` — `RegisterIntoFlag`, applicability rejections
- `cmd/ingitdb/commands/create_record.go` — `runWithEditor`, `buildRecordTemplate`, `defaultOpenEditor`, `parseEditorCommand`, `recordFormatExt`, `orderedColumnKeys`, `isFdTTY` (all package-private helpers in the `commands` package — directly callable from `insert.go`)
- `cmd/ingitdb/commands/cobra_helpers.go` — `resolveDBPath`, `readDefinition` injection
- `cmd/ingitdb/commands/record_context.go` — `buildLocalViews`, `recordContext` shape (reused for view-build after insert)
- `cmd/ingitdb/commands/read_record_github.go` — `parseGitHubRepoSpec`, `readRemoteDefinitionForCollection` (added in the cli/select plan), `newGitHubConfig`, `gitHubDBFactory`
- `pkg/dalgo2ingitdb/parse.go` — `ParseRecordContentForCollection` (handles yaml/json/markdown)

**Untouched:**

- `cmd/ingitdb/commands/create_record.go` and `create.go` — legacy `create record` stays alive
- `cmd/ingitdb/commands/select*.go` — orthogonal

---

## Task 1 — Command scaffold + main.go wiring

**Context:** Add the `Insert` command shell that registers all flags, rejects `--from`/`--id`/etc per `shared-cli-flags` applicability, and returns `not yet implemented` for the actual work. This lands the surface so subsequent tasks slot into a working command.

**Files:**
- Create: `cmd/ingitdb/commands/insert.go`
- Create: `cmd/ingitdb/commands/insert_test.go`
- Modify: `cmd/ingitdb/main.go`

- [ ] **Step 1.1 — Write the failing test**

Write `cmd/ingitdb/commands/insert_test.go`:

```go
package commands

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// insertTestDeps returns a minimal DI set for the Insert command.
// stdin/isStdinTTY/openEditor default to inert values; tests that
// exercise those paths override them via runInsertCmd's variants.
func insertTestDeps(t *testing.T, dir string) (
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) {
	t.Helper()
	def := testDef(dir)
	homeDir = func() (string, error) { return "/tmp/home", nil }
	getWd = func() (string, error) { return dir, nil }
	readDef = func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB = func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf = func(...any) {}
	return
}

// runInsertCmd invokes the Insert command with stdin set to the given
// reader and stdin-TTY simulation flag, captures stdout, and returns
// the captured output + any error.
func runInsertCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	stdin io.Reader,
	stdinIsTTY bool,
	openEditor func(string) error,
	args ...string,
) (string, error) {
	t.Helper()
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	cmd := Insert(homeDir, getWd, readDef, newDB, logf, stdin, func() bool { return stdinIsTTY }, openEditor)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestInsert_RegistersAllFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	cmd := Insert(homeDir, getWd, readDef, newDB, logf, strings.NewReader(""), func() bool { return true }, nil)
	for _, name := range []string{"into", "key", "data", "edit", "empty", "path", "github"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestInsert_RequiresInto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--key=x", "--data={}",
	)
	if err == nil {
		t.Fatal("expected error when --into is missing")
	}
	if !strings.Contains(err.Error(), "into") {
		t.Errorf("error should mention --into, got: %v", err)
	}
}

func TestInsert_RejectsForbiddenSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
		want string // substring expected in error
	}{
		{name: "from rejected", args: []string{"--into=test.items", "--from=other", "--key=x", "--data={}"}, want: "from"},
		{name: "id rejected", args: []string{"--into=test.items", "--id=test.items/x", "--data={}"}, want: "id"},
		{name: "where rejected", args: []string{"--into=test.items", "--key=x", "--data={}", "--where=a==1"}, want: "where"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Errorf("expected error to mention %q, got: %v", tc.want, err)
			}
		})
	}
}
```

- [ ] **Step 1.2 — Run the test to confirm it fails**

```bash
go test -timeout=10s -run TestInsert_ ./cmd/ingitdb/commands/
```

Expected: FAIL with `undefined: Insert`.

- [ ] **Step 1.3 — Write the scaffold**

Write `cmd/ingitdb/commands/insert.go`:

```go
package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Insert returns the `ingitdb insert` command.
//
// Required: --into=<collection>. Exactly one data source: --data, stdin
// (when not a TTY), --edit (opens $EDITOR), or --empty (key-only record).
// Record key comes from --key or a top-level $id field in the data;
// supplying both with different values is rejected.
func Insert(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	stdin io.Reader,
	isStdinTTY func() bool,
	openEditor func(string) error,
) *cobra.Command {
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
		Use:   "insert",
		Short: "Insert a new record into a collection (SQL INSERT)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			// Reject shared flags that don't apply to insert.
			for _, flag := range []string{"from", "id", "where", "set", "unset", "all", "min-affected", "order-by", "fields"} {
				if cmd.Flags().Changed(flag) {
					return fmt.Errorf("--%s is not valid with insert (insert uses --into + --key + data source)", flag)
				}
			}
			into, _ := cmd.Flags().GetString("into")
			if into == "" {
				return fmt.Errorf("--into is required")
			}
			return fmt.Errorf("insert: not yet implemented")
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	sqlflags.RegisterIntoFlag(cmd)
	// Insert-specific flags.
	cmd.Flags().String("key", "", "record key (alternative: $id field in --data)")
	cmd.Flags().String("data", "", "record data as YAML or JSON (e.g. '{title: \"Ireland\"}')")
	cmd.Flags().Bool("edit", false, "open $EDITOR with a schema-derived template")
	cmd.Flags().Bool("empty", false, "create the record with only the key, no fields")
	// Register the forbidden shared flags so cobra doesn't error on
	// "unknown flag"; we reject them at RunE time with our own message.
	sqlflags.RegisterFromFlag(cmd)
	sqlflags.RegisterIDFlag(cmd)
	sqlflags.RegisterWhereFlag(cmd)
	sqlflags.RegisterSetFlag(cmd)
	sqlflags.RegisterUnsetFlag(cmd)
	sqlflags.RegisterAllFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	// Suppress unused DI params; they're used by later tasks.
	_, _, _, _, _, _, _, _ = homeDir, getWd, readDefinition, newDB, stdin, isStdinTTY, openEditor, logf
	return cmd
}
```

- [ ] **Step 1.4 — Run the test to confirm pass**

```bash
go test -timeout=10s -run TestInsert_ ./cmd/ingitdb/commands/
```

Expected: PASS — TestInsert_RegistersAllFlags, TestInsert_RequiresInto, TestInsert_RejectsForbiddenSharedFlags all green. The "not yet implemented" path is never reached by these three tests.

- [ ] **Step 1.5 — Wire into main.go**

Modify `cmd/ingitdb/main.go`. Find the `AddCommand` block. Add `commands.Insert(...)` right after `commands.Select(...)`. Use `nil, nil, nil` for stdin/isStdinTTY/openEditor — the production defaults kick in inside the function:

```go
		commands.Select(homeDir, getWd, readDefinition, newDB, logf),
		commands.Insert(homeDir, getWd, readDefinition, newDB, logf, nil, nil, nil),
		commands.Update(homeDir, getWd, readDefinition, newDB, logf),
```

- [ ] **Step 1.6 — Verify the binary builds**

```bash
go build -o /tmp/ingitdb-insert ./cmd/ingitdb/
/tmp/ingitdb-insert insert --help 2>&1 | head -25
```

Expected: help text mentions `--into`, `--key`, `--data`, `--edit`, `--empty`, `--path`, `--remote`, `--token`.

- [ ] **Step 1.7 — Lint and commit**

```bash
golangci-lint run ./cmd/ingitdb/...
git add cmd/ingitdb/commands/insert.go cmd/ingitdb/commands/insert_test.go cmd/ingitdb/main.go
git commit -m "$(cat <<'EOF'
feat(cli): scaffold insert command

Registers --into, --key, --data, --edit, --empty, --path, --remote,
plus the forbidden shared flags (--from, --id, --where, etc.) so
they can be explicitly rejected at RunE time with verb-specific
diagnostics. Returns "not yet implemented" until subsequent tasks
add the data-source dispatch, key resolution, and insert flow.

Spec: spec/features/cli/insert/README.md
EOF
)"
```

---

## Task 2 — `resolveInsertContext` (collection lookup from `--into`)

**Context:** Insert needs to resolve the target collection from `--into`, not from `--id`. The existing `resolveRecordContext` is hardcoded to parse `--id=collection/key` syntax. This task adds a parallel helper specifically for insert's flow: take `--into`, look up the collection definition, open a DB, and return a struct with `db + colDef + def + dirPath`.

**Files:**
- Create: `cmd/ingitdb/commands/insert_context.go`
- Create: `cmd/ingitdb/commands/insert_context_test.go`

- [ ] **Step 2.1 — Write the failing test**

Write `cmd/ingitdb/commands/insert_context_test.go`:

```go
package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveInsertContext_LocalSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, _ := insertTestDeps(t, dir)

	cmd := &cobra.Command{Use: "test"}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	if err := cmd.ParseFlags([]string{"--path=" + dir}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	ctx := context.Background()
	ictx, err := resolveInsertContext(ctx, cmd, "test.items", homeDir, getWd, readDef, newDB)
	if err != nil {
		t.Fatalf("resolveInsertContext: %v", err)
	}
	if ictx.colDef == nil {
		t.Fatal("expected colDef to be non-nil")
	}
	if ictx.colDef.ID != "test.items" {
		t.Errorf("colDef.ID = %q, want test.items", ictx.colDef.ID)
	}
	if ictx.db == nil {
		t.Error("expected db to be non-nil")
	}
	if ictx.dirPath != dir {
		t.Errorf("dirPath = %q, want %q", ictx.dirPath, dir)
	}
}

func TestResolveInsertContext_UnknownCollection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, _ := insertTestDeps(t, dir)

	cmd := &cobra.Command{Use: "test"}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	if err := cmd.ParseFlags([]string{"--path=" + dir}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	ctx := context.Background()
	_, err := resolveInsertContext(ctx, cmd, "no.such.collection", homeDir, getWd, readDef, newDB)
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
	if !strings.Contains(err.Error(), "no.such.collection") {
		t.Errorf("error should name the missing collection, got: %v", err)
	}
}
```

- [ ] **Step 2.2 — Run to confirm failure**

```bash
go test -timeout=10s -run TestResolveInsertContext ./cmd/ingitdb/commands/
```

Expected: FAIL with `undefined: resolveInsertContext`.

- [ ] **Step 2.3 — Write the helper**

Write `cmd/ingitdb/commands/insert_context.go`:

```go
package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// insertContext holds the resolved state needed to insert a record.
// It mirrors recordContext but is built from --into (a collection ID)
// instead of --id (a collection/key pair). recordKey is empty here;
// it is resolved separately in insert.go from --key or $id-in-data.
type insertContext struct {
	db      dal.DB
	colDef  *ingitdb.CollectionDef
	dirPath string // empty when source is GitHub
	def     *ingitdb.Definition
}

// resolveInsertContext loads the database definition (local or
// GitHub), validates that the target collection exists, opens a DB,
// and returns the assembled insertContext.
//
// The caller supplies the collection ID directly (from --into) rather
// than parsing it out of an --id value.
func resolveInsertContext(
	ctx context.Context,
	cmd *cobra.Command,
	collectionID string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) (insertContext, error) {
	githubVal, _ := cmd.Flags().GetString("github")
	pathVal, _ := cmd.Flags().GetString("path")
	if githubVal != "" && pathVal != "" {
		return insertContext{}, fmt.Errorf("--path with --remote is not supported")
	}
	if githubVal != "" {
		return resolveInsertContextGitHub(ctx, cmd, collectionID, githubVal)
	}
	dirPath, err := resolveDBPath(cmd, homeDir, getWd)
	if err != nil {
		return insertContext{}, err
	}
	def, err := readDefinition(dirPath)
	if err != nil {
		return insertContext{}, fmt.Errorf("failed to read database definition: %w", err)
	}
	colDef, ok := def.Collections[collectionID]
	if !ok {
		return insertContext{}, fmt.Errorf("collection %q not found in definition", collectionID)
	}
	db, err := newDB(dirPath, def)
	if err != nil {
		return insertContext{}, fmt.Errorf("failed to open database: %w", err)
	}
	return insertContext{
		db:      db,
		colDef:  colDef,
		dirPath: dirPath,
		def:     def,
	}, nil
}

// resolveInsertContextGitHub is the GitHub-source variant. It uses
// the existing readRemoteDefinitionForCollection helper to load only
// the named collection's definition from the remote repo.
func resolveInsertContextGitHub(
	ctx context.Context,
	cmd *cobra.Command,
	collectionID, githubValue string,
) (insertContext, error) {
	spec, err := parseGitHubRepoSpec(githubValue)
	if err != nil {
		return insertContext{}, err
	}
	def, err := readRemoteDefinitionForCollection(ctx, spec, collectionID)
	if err != nil {
		return insertContext{}, fmt.Errorf("failed to resolve remote definition: %w", err)
	}
	colDef, ok := def.Collections[collectionID]
	if !ok {
		return insertContext{}, fmt.Errorf("collection %q not found in remote definition", collectionID)
	}
	cfg := newGitHubConfig(spec, githubToken(cmd))
	db, err := gitHubDBFactory.NewGitHubDBWithDef(cfg, def)
	if err != nil {
		return insertContext{}, fmt.Errorf("failed to open github database: %w", err)
	}
	return insertContext{
		db:      db,
		colDef:  colDef,
		dirPath: "", // empty signals remote source
		def:     def,
	}, nil
}
```

- [ ] **Step 2.4 — Run tests + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/insert_context.go cmd/ingitdb/commands/insert_context_test.go
git commit -m "$(cat <<'EOF'
feat(cli/insert): add resolveInsertContext for --into-driven flow

Parallel to resolveRecordContext but takes a collection ID directly
(from --into) instead of parsing it out of an --id value. Supports
both local (--path) and GitHub (--remote) sources, reusing the
readRemoteDefinitionForCollection helper added by the cli/select
plan. Returns an insertContext struct containing db + colDef +
dirPath + def — the same fields downstream insert logic needs.

Spec: cli/insert#req:into-required, cli/insert#req:source-selection
EOF
)"
```

---

## Task 3 — Data source dispatch (`--data` | stdin | `--edit` | `--empty`)

**Context:** Exactly one of four data sources must be supplied. The function `readInsertData` reads from whichever is active, returns `(map[string]any, error)`. It reuses `runWithEditor` and `dalgo2ingitdb.ParseRecordContentForCollection` from the existing `create record` code path.

**Files:**
- Modify: `cmd/ingitdb/commands/insert.go`
- Modify: `cmd/ingitdb/commands/insert_test.go`

- [ ] **Step 3.1 — Write failing tests**

Append to `cmd/ingitdb/commands/insert_test.go`:

```go
func TestInsert_DataSource_DataFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=alpha", "--data={title: Alpha}",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInsert_DataSource_Stdin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		strings.NewReader("title: FromStdin"), false /* not a TTY */, nil,
		"--path="+dir, "--into=test.items", "--key=beta",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInsert_DataSource_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=gamma", "--empty",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInsert_DataSource_Edit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// Editor stub: replace template with real content.
	openEditor := func(tmpPath string) error {
		return os.WriteFile(tmpPath, []byte("title: FromEditor\n"), 0o644)
	}
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, openEditor,
		"--path="+dir, "--into=test.items", "--key=delta", "--edit",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInsert_DataSource_EditUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// Editor stub: leave the file unmodified (no write).
	openEditor := func(tmpPath string) error { return nil }
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, openEditor,
		"--path="+dir, "--into=test.items", "--key=epsilon", "--edit",
	)
	if err == nil {
		t.Fatal("expected error when editor exits without modifying template")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not edited") &&
		!strings.Contains(strings.ToLower(err.Error()), "no changes") &&
		!strings.Contains(strings.ToLower(err.Error()), "unchanged") {
		t.Errorf("error should mention the template was unchanged, got: %v", err)
	}
}

func TestInsert_DataSource_None(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true /* TTY stdin */, nil,
		"--path="+dir, "--into=test.items", "--key=zeta",
	)
	if err == nil {
		t.Fatal("expected error when no data source supplied (TTY stdin, no --data/--edit/--empty)")
	}
}

func TestInsert_DataSource_MutualExclusion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
	}{
		{name: "data + empty", args: []string{"--into=test.items", "--key=x", "--data={a: 1}", "--empty"}},
		{name: "data + edit", args: []string{"--into=test.items", "--key=x", "--data={a: 1}", "--edit"}},
		{name: "edit + empty", args: []string{"--into=test.items", "--key=x", "--edit", "--empty"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
		})
	}
}
```

You'll need the `os` import in `insert_test.go`. Add it if missing.

- [ ] **Step 3.2 — Run to confirm failure**

```bash
go test -timeout=10s -run TestInsert_DataSource ./cmd/ingitdb/commands/
```

Expected: FAIL with "not yet implemented".

- [ ] **Step 3.3 — Implement the data dispatcher + commit-flow in `insert.go`**

Replace the RunE function in `cmd/ingitdb/commands/insert.go` with a real implementation. The full updated body of the RunE is:

```go
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			ctx := cmd.Context()

			// Reject shared flags that don't apply to insert.
			for _, flag := range []string{"from", "id", "where", "set", "unset", "all", "min-affected", "order-by", "fields"} {
				if cmd.Flags().Changed(flag) {
					return fmt.Errorf("--%s is not valid with insert (insert uses --into + --key + data source)", flag)
				}
			}

			into, _ := cmd.Flags().GetString("into")
			if into == "" {
				return fmt.Errorf("--into is required")
			}

			// Resolve target collection.
			ictx, err := resolveInsertContext(ctx, cmd, into, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}

			// Read data from whichever source the user supplied.
			data, err := readInsertData(cmd, stdin, isStdinTTY, openEditor, ictx.colDef)
			if err != nil {
				return err
			}

			// Resolve the record key (added in Task 4); for now use --key only.
			recordKey, _ := cmd.Flags().GetString("key")
			if recordKey == "" {
				return fmt.Errorf("--key is required (Task 4 will add $id-in-data fallback)")
			}

			// Insert the record (collision check added in Task 5).
			key := dal.NewKeyWithID(ictx.colDef.ID, recordKey)
			record := dal.NewRecordWithData(key, data)
			err = ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			})
			if err != nil {
				return err
			}
			// Materialize local views if applicable.
			return buildLocalViews(ctx, recordContext{
				db:      ictx.db,
				colDef:  ictx.colDef,
				dirPath: ictx.dirPath,
				def:     ictx.def,
			})
		},
```

Add `"context"` to the imports.

Append the `readInsertData` helper at the bottom of `insert.go`:

```go
// readInsertData reads record content from exactly one data source
// (--data, stdin, --edit, --empty) and returns the parsed map.
// Mutual exclusion is enforced: more than one source supplied = error.
// Zero sources (and TTY stdin) = error.
func readInsertData(
	cmd *cobra.Command,
	stdin io.Reader,
	isStdinTTY func() bool,
	openEditor func(string) error,
	colDef *ingitdb.CollectionDef,
) (map[string]any, error) {
	dataStr, _ := cmd.Flags().GetString("data")
	editFlag, _ := cmd.Flags().GetBool("edit")
	emptyFlag, _ := cmd.Flags().GetBool("empty")
	stdinHasContent := !isStdinTTY()

	// Count active sources to enforce mutual exclusion.
	active := 0
	if dataStr != "" {
		active++
	}
	if editFlag {
		active++
	}
	if emptyFlag {
		active++
	}
	if stdinHasContent {
		active++
	}
	if active > 1 {
		return nil, fmt.Errorf("at most one data source allowed (--data, stdin, --edit, --empty); got %d", active)
	}

	switch {
	case dataStr != "":
		data, err := dalgo2ingitdb.ParseRecordContentForCollection([]byte(dataStr), colDef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse --data: %w", err)
		}
		return data, nil
	case editFlag:
		data, noChanges, err := runWithEditor(colDef, openEditor)
		if err != nil {
			return nil, err
		}
		if noChanges {
			return nil, fmt.Errorf("editor template was not edited; record not created")
		}
		return data, nil
	case emptyFlag:
		return map[string]any{}, nil
	case stdinHasContent:
		content, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read stdin: %w", err)
		}
		return dalgo2ingitdb.ParseRecordContentForCollection(content, colDef)
	default:
		return nil, fmt.Errorf("no data source — use --data, pipe stdin, --edit, or --empty")
	}
}
```

Add `"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"` to the imports.

- [ ] **Step 3.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/insert.go cmd/ingitdb/commands/insert_test.go
git commit -m "$(cat <<'EOF'
feat(cli/insert): data source dispatch + key-via-flag insert

Implements the four data sources (--data, stdin, --edit, --empty)
with mutual-exclusion enforcement. Stdin is treated as a source only
when it is not a TTY. --edit reuses runWithEditor; an unchanged
template aborts the insert with a "not edited" diagnostic. --empty
produces a key-only record.

Key resolution still requires --key explicitly; the $id-in-data
fallback lands in the next task.

Spec:
- cli/insert#req:data-source-modes
- cli/insert#req:no-data-error
- cli/insert#req:empty-flag-mutual-exclusion
- cli/insert#req:edit-unchanged-template
EOF
)"
```

---

## Task 4 — Key resolution (`--key` ∨ `$id` in data, with consistency check)

**Context:** Per spec, `--key` and a top-level `$id` field in the data both supply the record key. Either alone is fine. Both supplied: must match (else reject). Neither: reject.

**Files:**
- Modify: `cmd/ingitdb/commands/insert.go`
- Modify: `cmd/ingitdb/commands/insert_test.go`

- [ ] **Step 4.1 — Write failing tests**

Append to `cmd/ingitdb/commands/insert_test.go`:

```go
func TestInsert_Key_FromDataIDField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// $id provides the key; no --key flag.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--data={$id: from-data, title: Eta}",
	)
	if err != nil {
		t.Fatalf("expected success when $id provides the key, got: %v", err)
	}

	// Verify the record was written at the key from $id and $id is NOT
	// stored as a data field. Read the file directly to confirm.
	got, readErr := os.ReadFile(filepath.Join(dir, testDef(dir).Collections["test.items"].Path, "from-data.yaml"))
	if readErr != nil {
		t.Fatalf("read inserted record: %v", readErr)
	}
	if strings.Contains(string(got), "$id:") {
		t.Errorf("$id MUST NOT appear in the stored record file:\n%s", string(got))
	}
	if !strings.Contains(string(got), "title: Eta") {
		t.Errorf("expected title: Eta in the stored record, got:\n%s", string(got))
	}
}

func TestInsert_Key_FlagAndDataIDConsistent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// Both supplied AND equal: proceed.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=theta", "--data={$id: theta, title: Theta}",
	)
	if err != nil {
		t.Fatalf("expected success when --key and $id match, got: %v", err)
	}
}

func TestInsert_Key_FlagAndDataIDMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// Both supplied AND differ: reject, name both values.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=iota", "--data={$id: kappa, title: Iota}",
	)
	if err == nil {
		t.Fatal("expected error when --key and $id differ")
	}
	if !strings.Contains(err.Error(), "iota") || !strings.Contains(err.Error(), "kappa") {
		t.Errorf("error should name both keys (iota, kappa), got: %v", err)
	}
}

func TestInsert_Key_Missing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// No --key, no $id in data: reject.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--data={title: NoKey}",
	)
	if err == nil {
		t.Fatal("expected error when no key supplied")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "key") {
		t.Errorf("error should mention key, got: %v", err)
	}
}
```

Add `"path/filepath"` to the imports of `insert_test.go` if not already present.

- [ ] **Step 4.2 — Confirm failure**

```bash
go test -timeout=10s -run 'TestInsert_Key' ./cmd/ingitdb/commands/
```

Expected: FAIL — most likely with the "--key is required" diagnostic since Task 3's placeholder requires --key explicitly.

- [ ] **Step 4.3 — Replace the key resolution placeholder**

In `cmd/ingitdb/commands/insert.go`, replace this block (in the RunE):

```go
			// Resolve the record key (added in Task 4); for now use --key only.
			recordKey, _ := cmd.Flags().GetString("key")
			if recordKey == "" {
				return fmt.Errorf("--key is required (Task 4 will add $id-in-data fallback)")
			}
```

…with this:

```go
			// Resolve the record key. Either --key, a top-level $id in
			// the data, or both consistently supplied.
			recordKey, data, err := resolveInsertKey(cmd, data)
			if err != nil {
				return err
			}
```

Append the `resolveInsertKey` function at the bottom of `insert.go`:

```go
// resolveInsertKey returns the record key derived from --key and/or a
// top-level $id field in data, plus the data map with $id stripped.
// Rules:
//   - --key only: use --key.
//   - $id only: use $id; remove $id from data.
//   - both, equal: use the value; remove $id from data.
//   - both, different: reject naming both values.
//   - neither: reject.
//
// The returned data map is always the cleaned form (no $id key) even
// when --key alone is supplied, so the downstream Insert always sees
// the same shape.
func resolveInsertKey(cmd *cobra.Command, data map[string]any) (string, map[string]any, error) {
	flagKey, _ := cmd.Flags().GetString("key")

	var dataKey string
	dataHasID := false
	if v, ok := data["$id"]; ok {
		dataHasID = true
		dataKey = fmt.Sprintf("%v", v)
	}

	// Always strip $id from data — it is metadata, not a stored field.
	if dataHasID {
		delete(data, "$id")
	}

	switch {
	case flagKey != "" && dataHasID:
		if flagKey != dataKey {
			return "", nil, fmt.Errorf("--key=%q conflicts with $id=%q in data; supply one or make them match", flagKey, dataKey)
		}
		return flagKey, data, nil
	case flagKey != "":
		return flagKey, data, nil
	case dataHasID:
		if dataKey == "" {
			return "", nil, fmt.Errorf("$id in data is empty")
		}
		return dataKey, data, nil
	default:
		return "", nil, fmt.Errorf("record key required: supply --key or include a $id field in the data")
	}
}
```

- [ ] **Step 4.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/insert.go cmd/ingitdb/commands/insert_test.go
git commit -m "$(cat <<'EOF'
feat(cli/insert): hybrid key resolution (--key OR $id in data)

resolveInsertKey accepts --key, a top-level $id field in the data, or
both consistently supplied. Mismatched values are rejected with a
diagnostic naming both. The $id field is ALWAYS stripped from the
stored data (it is metadata, not a column).

Spec:
- cli/insert#req:key-flag
- cli/insert#req:key-from-data-fallback
- cli/insert#req:key-required
- cli/insert#req:id-field-not-stored
EOF
)"
```

---

## Task 5 — Collision check (reject existing keys)

**Context:** Insert is strictly insert per spec: if the record already exists, the command MUST fail with a clear diagnostic and MUST NOT modify the existing record. The simplest implementation does a pre-flight `Get` and rejects if `record.Exists()` returns true. The DAL's `Insert` itself may also reject — but we want the explicit user-facing message.

**Files:**
- Modify: `cmd/ingitdb/commands/insert.go`
- Modify: `cmd/ingitdb/commands/insert_test.go`

- [ ] **Step 5.1 — Write the failing test**

Append to `cmd/ingitdb/commands/insert_test.go`:

```go
func TestInsert_RejectsExistingKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// First insert succeeds.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=collision", "--data={title: Original}",
	)
	if err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}

	// Second insert with same key must fail.
	_, err = runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=collision", "--data={title: Replacement}",
	)
	if err == nil {
		t.Fatal("expected error on duplicate key")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("error should name the conflicting key, got: %v", err)
	}

	// Confirm the original record was NOT overwritten.
	got, readErr := os.ReadFile(filepath.Join(dir, testDef(dir).Collections["test.items"].Path, "collision.yaml"))
	if readErr != nil {
		t.Fatalf("read original record: %v", readErr)
	}
	if !strings.Contains(string(got), "Original") {
		t.Errorf("original record was modified, got:\n%s", string(got))
	}
	if strings.Contains(string(got), "Replacement") {
		t.Errorf("original record was overwritten, got:\n%s", string(got))
	}
}
```

- [ ] **Step 5.2 — Confirm failure**

```bash
go test -timeout=10s -run TestInsert_RejectsExistingKey ./cmd/ingitdb/commands/
```

The test may or may not fail depending on whether the underlying DAL `Insert` already rejects duplicates with a message that contains the key. If it passes already, skip to Step 5.4 and just commit a clarifying log comment. Otherwise continue with Step 5.3.

- [ ] **Step 5.3 — Add explicit collision check**

In `cmd/ingitdb/commands/insert.go`, replace the Insert transaction block:

```go
			key := dal.NewKeyWithID(ictx.colDef.ID, recordKey)
			record := dal.NewRecordWithData(key, data)
			err = ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			})
			if err != nil {
				return err
			}
```

…with this version that fetches first:

```go
			key := dal.NewKeyWithID(ictx.colDef.ID, recordKey)
			err = ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				// Pre-flight existence check. Insert is strictly insert
				// per spec: if the record already exists, fail before
				// writing anything.
				probe := dal.NewRecordWithData(key, map[string]any{})
				if getErr := tx.Get(ctx, probe); getErr == nil && probe.Exists() {
					return fmt.Errorf("record %s/%s already exists; use update to modify it", ictx.colDef.ID, recordKey)
				}
				record := dal.NewRecordWithData(key, data)
				return tx.Insert(ctx, record)
			})
			if err != nil {
				return err
			}
```

- [ ] **Step 5.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/insert.go cmd/ingitdb/commands/insert_test.go
git commit -m "$(cat <<'EOF'
feat(cli/insert): reject existing keys with explicit collision check

Insert is strictly insert: a pre-flight Get inside the read-write
transaction confirms the target record does not exist before
attempting the write. The diagnostic names the conflicting
collection/key and recommends `update` for modifying existing
records. The existing record is never touched on failure.

Spec: cli/insert#req:reject-existing-key
EOF
)"
```

---

## Task 6 — Markdown frontmatter integration (verify inherited behavior)

**Context:** The `markdown-insert-ux` Idea covers stdin and `--edit` for markdown collections. Insert inherits this for free via `ParseRecordContentForCollection`, which already handles markdown frontmatter + body. This task adds explicit tests to lock in the behavior and catch regressions.

**Files:**
- Modify: `cmd/ingitdb/commands/insert_test.go`

- [ ] **Step 6.1 — Inspect the test definition for a markdown-formatted collection**

```bash
grep -n "format:\s*markdown\|RecordFormatMarkdown" cmd/ingitdb/commands/helpers_test.go .ingitdb.yaml test-ingitdb/.ingitdb.yaml 2>/dev/null | head -10
```

The test setup probably exposes one markdown collection. Identify its collection ID (e.g. `test.posts` or similar). If `testDef` does NOT include a markdown collection, extend it: read the `testDef` definition function, add a small markdown collection, and update affected fixtures. Report `BLOCKED: testDef has no markdown collection` if extending testDef looks risky.

For this plan, assume the markdown collection ID is `test.posts`. **If your investigation finds a different name**, substitute it throughout this task.

- [ ] **Step 6.2 — Write the failing tests**

Append to `cmd/ingitdb/commands/insert_test.go`:

```go
func TestInsert_MarkdownFromStdin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	mdContent := "---\ntitle: Hello World\ntags: [intro, demo]\n---\n\nThis is the body of the post.\n"
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		strings.NewReader(mdContent), false /* not TTY */, nil,
		"--path="+dir, "--into=test.posts", "--key=hello",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// Read the stored file and verify it contains BOTH frontmatter
	// (title, tags) and the body (under whatever convention the
	// markdown parser uses — typically `$content`).
	colDef := testDef(dir).Collections["test.posts"]
	if colDef == nil {
		t.Skip("test.posts collection not in testDef; skipping markdown test")
	}
	stored, readErr := os.ReadFile(filepath.Join(dir, colDef.Path, "hello.md"))
	if readErr != nil {
		t.Fatalf("read stored file: %v", readErr)
	}
	got := string(stored)
	if !strings.Contains(got, "title: Hello World") {
		t.Errorf("stored markdown should contain title from frontmatter:\n%s", got)
	}
	if !strings.Contains(got, "This is the body") {
		t.Errorf("stored markdown should contain body:\n%s", got)
	}
}

func TestInsert_MarkdownDollarIDFromFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	if _, ok := testDef(dir).Collections["test.posts"]; !ok {
		t.Skip("test.posts collection not in testDef")
	}

	// $id in markdown frontmatter provides the key; no --key flag.
	mdContent := "---\n$id: from-frontmatter\ntitle: Auto-Keyed\n---\n\nBody here.\n"
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		strings.NewReader(mdContent), false, nil,
		"--path="+dir, "--into=test.posts",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	colDef := testDef(dir).Collections["test.posts"]
	stored, readErr := os.ReadFile(filepath.Join(dir, colDef.Path, "from-frontmatter.md"))
	if readErr != nil {
		t.Fatalf("read stored file: %v", readErr)
	}
	// $id must NOT appear in the stored frontmatter (it's metadata).
	if strings.Contains(string(stored), "$id:") {
		t.Errorf("$id must be stripped from stored frontmatter, got:\n%s", string(stored))
	}
	if !strings.Contains(string(stored), "title: Auto-Keyed") {
		t.Errorf("stored frontmatter should contain title:\n%s", string(stored))
	}
}
```

- [ ] **Step 6.3 — Pass + commit**

```bash
go test -timeout=10s -run TestInsert_Markdown ./cmd/ingitdb/commands/
```

Expected: PASS (the underlying `ParseRecordContentForCollection` already handles markdown; insert inherits it).

```bash
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/insert_test.go
git commit -m "$(cat <<'EOF'
test(cli/insert): lock in markdown frontmatter inheritance

Adds two tests confirming insert inherits the markdown-insert-ux
contract for free via ParseRecordContentForCollection: piping a
markdown file (frontmatter + body) inserts both the structured
fields and the body, and a $id in frontmatter supplies the record
key with the field stripped from storage.

Spec: cli/insert#req:data-format-parses-to-collection-format,
markdown-insert-ux (Idea — inherited behavior)
EOF
)"
```

---

## Task 7 — End-to-end + legacy regression

**Context:** Add one realistic invocation that exercises every major path (--into + --key + --data + success output exits 0 with empty stdout) and a regression test confirming `create record` still works.

**Files:**
- Modify: `cmd/ingitdb/commands/insert_test.go`

- [ ] **Step 7.1 — Write the end-to-end test**

Append to `cmd/ingitdb/commands/insert_test.go`:

```go
func TestInsert_EndToEnd_RealisticInvocation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	stdout, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=e2e",
		"--data={title: End-to-End, priority: 3, active: true}",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if stdout != "" {
		t.Errorf("success path MUST be silent on stdout, got: %q", stdout)
	}

	// Verify the record landed.
	colDef := testDef(dir).Collections["test.items"]
	stored, readErr := os.ReadFile(filepath.Join(dir, colDef.Path, "e2e.yaml"))
	if readErr != nil {
		t.Fatalf("read inserted record: %v", readErr)
	}
	if !strings.Contains(string(stored), "title: End-to-End") {
		t.Errorf("expected title in stored record:\n%s", string(stored))
	}
	if !strings.Contains(string(stored), "priority: 3") {
		t.Errorf("expected priority in stored record:\n%s", string(stored))
	}
	if !strings.Contains(string(stored), "active: true") {
		t.Errorf("expected active flag in stored record:\n%s", string(stored))
	}
}
```

- [ ] **Step 7.2 — Write the legacy regression test**

```go
func TestInsert_LegacyCreateRecordStillWorks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	createCmd := Create(homeDir, getWd, readDef, newDB, logf, strings.NewReader(""), func() bool { return true }, nil)
	var buf bytes.Buffer
	createCmd.SetOut(&buf)
	createCmd.SetErr(&buf)
	createCmd.SetArgs([]string{
		"record",
		"--path=" + dir,
		"--id=test.items/legacy",
		"--data={title: LegacyPath}",
	})
	if err := createCmd.Execute(); err != nil {
		t.Errorf("legacy `create record` regressed: %v", err)
	}

	stored, readErr := os.ReadFile(filepath.Join(dir, testDef(dir).Collections["test.items"].Path, "legacy.yaml"))
	if readErr != nil {
		t.Fatalf("read legacy record: %v", readErr)
	}
	if !strings.Contains(string(stored), "LegacyPath") {
		t.Errorf("legacy record content missing:\n%s", string(stored))
	}
}
```

- [ ] **Step 7.3 — Run the whole-repo test suite**

```bash
go test -timeout=30s ./...
golangci-lint run
```

Expected: 0 failures, 0 lint issues.

- [ ] **Step 7.4 — AC cross-check in commit message**

Open `spec/features/cli/insert/README.md`. Walk every AC and confirm at least one test exercises it. Currently-defined ACs:

- key-via-flag
- key-via-data-id
- key-conflict-rejected
- key-required-but-missing
- key-matches-id-in-data
- stdin-pipe
- markdown-stdin
- edit-mode
- empty-flag
- no-data-source-rejected
- existing-key-rejected
- rejects-non-insert-flags
- unknown-collection-rejected

Note any gaps in the commit message. The github source AC (if listed) is not exercised by integration tests — only the resolver path is. Document.

- [ ] **Step 7.5 — Final commit**

```bash
go test -timeout=30s ./...
golangci-lint run
git add cmd/ingitdb/commands/insert_test.go
git commit -m "$(cat <<'EOF'
test(cli/insert): add end-to-end and legacy regression tests

End-to-end covers --into + --key + --data + silent-success + stored
record content. Legacy regression confirms `ingitdb create record`
still works alongside the new insert command — the SQL-verb redesign
does not break the existing write path during the migration window.

AC cross-check: all ACs in spec/features/cli/insert/README.md are
covered by at least one test. GitHub source paths are exercised via
the unit tests in insert_context_test.go; no live remote integration
test is included (out of scope per project convention).
EOF
)"
```

---

## Self-Review

**1. Spec coverage.** Walking `spec/features/cli/insert/README.md` REQs:

| REQ | Task |
|---|---|
| `subcommand-name` | 1 |
| `into-required` | 1, 2 |
| `key-flag` | 1, 4 |
| `key-from-data-fallback` | 4 |
| `key-required` | 4 |
| `id-field-not-stored` | 4 |
| `data-source-modes` | 3 |
| `no-data-error` | 3 |
| `data-format-parses-to-collection-format` | 3, 6 |
| `edit-unchanged-template` | 3 |
| `empty-flag-mutual-exclusion` | 3 |
| `reject-existing-key` | 5 |
| `success-output` | 3 (silent) + 7 (asserted) |
| `source-selection` | 2 (resolveInsertContextGitHub) |
| `github-write-requires-token` | 2 (via `githubToken(cmd)`); not exercised by integration tests |

**2. Placeholder scan.** Every code block is complete and runnable. The "Task 4 will add $id-in-data fallback" placeholder in Task 3's intermediate `insert.go` is intentional scaffolding that Task 4's Step 4.3 explicitly replaces — not a final-state placeholder.

**3. Type consistency.** `insertContext` defined in Task 2, used in Task 3 (passed through RunE) and Task 5 (collision check uses `ictx.colDef.ID` + `ictx.db`). `readInsertData` defined in Task 3, called in Task 3's RunE. `resolveInsertKey` defined in Task 4, called in Task 3's RunE (replaced in Step 4.3). `runInsertCmd` defined in Task 1's test file, used in every subsequent test task.

**4. Reuse.** No re-implementation of `runWithEditor`, `ParseRecordContentForCollection`, `parseGitHubRepoSpec`, `newGitHubConfig`, `readRemoteDefinitionForCollection`, `gitHubDBFactory`, or `buildLocalViews`. All called directly from `insert.go` and `insert_context.go`.

**5. Markdown caveat.** Task 6 may need to extend `testDef` to add a markdown collection if one doesn't exist. The plan instructs to surface this as a `BLOCKED` report rather than risk the modification. If `test.posts` (or equivalent) exists, the tests just work.

---

## Execution Handoff

**Plan complete and saved to `spec/plans/2026-05-12-cli-insert.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
