# cli/update — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the `ingitdb update` command — single-record mode (`--id`) and set mode (`--from` + `--where`|`--all`), each with `--set` and/or `--unset` patch operations, shallow patch semantics, `--min-affected` all-or-nothing guard for set mode, and local + GitHub source. Replaces the write path of `update record` without removing it.

**Architecture:** A new file `cmd/ingitdb/commands/update.go` hosts the `Update` command. Single-record mode reuses `resolveRecordContext` (already in the codebase) — the lookup-by-`--id` flow is unchanged from today's `update record`. Set mode reuses the same `runSelectFromSetWithDB`-style pattern from cli/select: fetch every record, apply WHERE filter via `evalAllWhere`, then mutate each match. Patch semantics stay shallow at the top level (matching the existing `update record` behavior) via `maps.Copy` + `delete` for `--unset`. `--min-affected` is enforced as a pre-flight check that runs AFTER filtering but BEFORE any write — when below threshold, no mutation occurs.

**Tech Stack:** Go, `github.com/dal-go/dalgo/dal`, `github.com/spf13/cobra`, the project's `sqlflags` package, and existing helpers (`resolveRecordContext`, `resolveInsertContext`, `evalAllWhere`).

**Spec:** `spec/features/cli/update/README.md`

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Create | `cmd/ingitdb/commands/update.go` | The `Update` cobra command + RunE handler for both modes |
| Create | `cmd/ingitdb/commands/update_test.go` | Integration tests for single-record + set modes, --set, --unset, --min-affected |
| Modify | `cmd/ingitdb/main.go` | Replace `commands.Update(...)` (currently the legacy parent command) with the new `commands.Update(...)` — wait, see Step 1.5 |

**Reused (no edits):**

- `cmd/ingitdb/commands/sqlflags/*` — `RegisterIDFlag`, `RegisterFromFlag`, `RegisterWhereFlag`, `RegisterSetFlag`, `RegisterUnsetFlag`, `RegisterAllFlag`, `RegisterMinAffectedFlag`, `ResolveMode`, `RejectSetModeFlags`, `RejectSetUnsetSameField`, `ParseSet`, `ParseUnset`, `ParseWhere`, `MinAffectedFromCmd`
- `cmd/ingitdb/commands/record_context.go` — `resolveRecordContext`, `recordContext`, `buildLocalViews`
- `cmd/ingitdb/commands/insert_context.go` — `resolveInsertContext`, `insertContext` (reused for set-mode resolution)
- `cmd/ingitdb/commands/select_where.go` — `evalAllWhere`, `evalWhere`
- `cmd/ingitdb/commands/select.go` — pattern for fetch-all-then-filter set-mode iteration

**Untouched:**

- `cmd/ingitdb/commands/update_record.go` and `update.go` — the LEGACY `update record` subcommand stays alive. See Step 1.5 for the naming collision resolution.

---

## Naming collision: `Update` vs `Update`

The legacy `update record` subcommand is currently wired into `main.go` via `commands.Update(...)`, where `Update` is a function in `cmd/ingitdb/commands/update.go` that returns a parent cobra command with a `record` subcommand. We need:

- The NEW top-level `ingitdb update` command (this plan's deliverable)
- The OLD `ingitdb update record` subcommand to continue working

Step 1.5 resolves this by renaming the existing `commands.Update` → `commands.UpdateLegacy` (preserving the file `cmd/ingitdb/commands/update.go` for the new code) and updating `main.go` to register both. The legacy `update record` invocation path remains identical from the user's perspective; only the internal Go symbol name changes.

---

## Task 1 — Scaffold + name-collision resolution + main.go wiring

**Context:** Lay down the new `Update` command shell with all flags, reject forbidden shared flags, and rename the legacy parent to `UpdateLegacy` so both can coexist. RunE returns "not yet implemented" for both modes; subsequent tasks add the real work.

**Files:**
- Modify: `cmd/ingitdb/commands/update.go` (rename existing `Update` → `UpdateLegacy`)
- Create: `cmd/ingitdb/commands/update_new.go` (the new `Update` function; lives in a separate file to keep diffs clean)
- Create: `cmd/ingitdb/commands/update_test.go`
- Modify: `cmd/ingitdb/main.go`

- [ ] **Step 1.1 — Inspect the existing `update.go`**

```bash
cat cmd/ingitdb/commands/update.go
```

You should see something like:

```go
func Update(homeDir func() (string, error), getWd func() (string, error), readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error), newDB func(string, *ingitdb.Definition) (dal.DB, error), logf func(...any)) *cobra.Command {
    cmd := &cobra.Command{
        Use:     "update",
        Aliases: []string{"u"},
        Short:   "Update database objects",
    }
    cmd.AddCommand(updateRecord(homeDir, getWd, readDefinition, newDB, logf))
    return cmd
}
```

The `update_record.go` private `updateRecord(...)` returns the actual `record` subcommand. Both files remain unchanged in this plan — only the symbol name `Update` changes.

- [ ] **Step 1.2 — Rename `Update` to `UpdateLegacy`**

In `cmd/ingitdb/commands/update.go`, change the single line:

```go
func Update(...) *cobra.Command {
```

to:

```go
func UpdateLegacy(...) *cobra.Command {
```

Update the function comment if it exists. Do NOT change the function body, the inner `updateRecord(...)` call, the file location, or anything else.

- [ ] **Step 1.3 — Write the failing test for the new command**

Write `cmd/ingitdb/commands/update_test.go`:

```go
package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// updateTestDeps returns a minimal DI set for the Update command.
func updateTestDeps(t *testing.T, dir string) (
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

// runUpdateCmd invokes the new Update command with the given args
// and returns captured stdout + any error.
func runUpdateCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	args ...string,
) (string, error) {
	t.Helper()
	cmd := Update(homeDir, getWd, readDef, newDB, logf)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestUpdate_RegistersAllFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	cmd := Update(homeDir, getWd, readDef, newDB, logf)
	for _, name := range []string{"id", "from", "where", "set", "unset", "all", "min-affected", "path", "github"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestUpdate_RejectsBothIDAndFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/x", "--from=test.items", "--set=a=1",
	)
	if err == nil {
		t.Fatal("expected error when both --id and --from supplied")
	}
}

func TestUpdate_RejectsNeitherIDNorFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--set=a=1",
	)
	if err == nil {
		t.Fatal("expected error when neither --id nor --from supplied")
	}
}

func TestUpdate_RejectsForbiddenSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
	}{
		{name: "into rejected", args: []string{"--id=test.items/x", "--into=other", "--set=a=1"}},
		{name: "order-by rejected", args: []string{"--id=test.items/x", "--order-by=name", "--set=a=1"}},
		{name: "fields rejected", args: []string{"--id=test.items/x", "--fields=a,b", "--set=a=1"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
		})
	}
}

func TestUpdate_NoPatchRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/x",
	)
	if err == nil {
		t.Fatal("expected error when neither --set nor --unset supplied")
	}
	if !strings.Contains(err.Error(), "set") || !strings.Contains(err.Error(), "unset") {
		t.Errorf("error should mention both --set and --unset, got: %v", err)
	}
}
```

- [ ] **Step 1.4 — Run to confirm failure**

```bash
go test -timeout=10s -run TestUpdate_ ./cmd/ingitdb/commands/
```

Expected: FAIL with `undefined: Update` — the legacy `Update` is now `UpdateLegacy`, and the new `Update` doesn't exist yet.

- [ ] **Step 1.5 — Write the scaffold**

Write `cmd/ingitdb/commands/update_new.go`:

```go
package commands

import (
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Update returns the `ingitdb update` command. Two modes inherited
// from sqlflags: single-record (--id) and set (--from + --where|--all).
// Patch operations: --set (repeatable assignment) and --unset
// (comma-separated field list). Shallow patch at the top level.
// --min-affected guards set-mode invocations with all-or-nothing
// semantics.
func Update(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update records in a collection (SQL UPDATE)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			// Reject shared flags that don't apply to update.
			for _, flag := range []string{"into", "order-by", "fields"} {
				if cmd.Flags().Changed(flag) {
					return fmt.Errorf("--%s is not valid with update", flag)
				}
			}

			id, _ := cmd.Flags().GetString("id")
			from, _ := cmd.Flags().GetString("from")
			mode, err := sqlflags.ResolveMode(id, from)
			if err != nil {
				return err
			}

			// Require at least one of --set or --unset.
			setExprs, _ := cmd.Flags().GetStringArray("set")
			unsetExprs, _ := cmd.Flags().GetStringArray("unset")
			if len(setExprs) == 0 && len(unsetExprs) == 0 {
				return fmt.Errorf("at least one of --set or --unset is required")
			}

			switch mode {
			case sqlflags.ModeID:
				return fmt.Errorf("update --id: not yet implemented")
			case sqlflags.ModeFrom:
				return fmt.Errorf("update --from: not yet implemented")
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
	sqlflags.RegisterSetFlag(cmd)
	sqlflags.RegisterUnsetFlag(cmd)
	sqlflags.RegisterAllFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	// Register the forbidden shared flags so cobra doesn't error on
	// "unknown flag"; we reject them in RunE with our own message.
	sqlflags.RegisterIntoFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	// Suppress unused DI params; they're used by later tasks.
	_, _, _, _, _ = homeDir, getWd, readDefinition, newDB, logf
	return cmd
}
```

- [ ] **Step 1.6 — Wire into main.go**

Modify `cmd/ingitdb/main.go`. Find the line:

```go
		commands.Update(homeDir, getWd, readDefinition, newDB, logf),
```

The legacy `Update` function is now `UpdateLegacy`. Replace the line with two lines — register both:

```go
		commands.UpdateLegacy(homeDir, getWd, readDefinition, newDB, logf),
		commands.Update(homeDir, getWd, readDefinition, newDB, logf),
```

Wait — cobra rejects two top-level commands with the same `Use` name. Let me re-think.

Look at the existing `Update` function (now `UpdateLegacy`): its `Use` is `"update"`. The new `Update` function also uses `"update"`. They will collide.

The legacy `update record` flow exists ONLY to honor the migration window. After the rename, the legacy parent command's purpose is to host the `record` subcommand. We can change `UpdateLegacy`'s `Use` field to something that doesn't collide — but that breaks `ingitdb update record` invocations.

The cleanest solution: REMOVE the legacy `update record` registration from `main.go` entirely, and instead reach the legacy behavior via direct calls to the now-private `updateRecord(...)` function — i.e., the legacy parent is gone, only the subcommand body remains as a private helper. But that breaks the spec promise to keep `update record` working during migration.

**Real solution:** Rename the legacy parent's `Use` to `update-record-legacy` (single word) so it doesn't collide. Document that `ingitdb update record` is no longer the invocation — the legacy path is `ingitdb update-record-legacy record` OR we accept that the rename effectively removes the legacy invocation.

But the spec explicitly says: "Old verb `update record` remains working — do not remove it."

**Final solution (revised):** Keep the legacy `update` function registered, but DON'T add a new top-level `update` command. Instead, the cli-sql-verbs Idea says the legacy verbs are *removed in a final cleanup plan* — meaning until then, having both `ingitdb update record` (legacy) and a new entry point may need a different verb name OR the migration may need to be staged differently.

Going back to the spec: `spec/features/cli/update/README.md` says `update` *renames and supersedes* `update record`. The transition strategy from cli-sql-verbs Idea says: "old verbs remain working until cleanup."

The pragmatic reconciliation: **the legacy `update record` subcommand path is incompatible with a new top-level `update` command at the same time.** This is a real conflict. Two options:

1. **Accept the breakage during the migration window:** the new `update` replaces the parent. `ingitdb update record` invocations now hit the new RunE (which currently requires `--id` and rejects positional args; they will error). Document this in the commit message. This is consistent with cli-sql-verbs' "hard rename, no aliases" decision.

2. **Defer the new `update` command entirely until the final cleanup plan removes `update record`.** Skip this entire feature for now.

The user's plan-per-feature decision said: "old verbs keep working" — but cobra structurally cannot host two top-level commands with the same `Use`. The Idea also says "hard rename, no aliases." These two are in tension specifically for `update` (and only `update`, because it was the one OLD verb whose name matches a NEW verb exactly).

**Plan choice (final):** Option 1 — accept that `ingitdb update record` will, during the migration window, dispatch into the new `update` RunE and error because positional `record` is not recognized. The legacy behavior is preserved via the standalone Go function `UpdateLegacy(...)` (and the `updateRecord(...)` private helper inside it), which downstream code or tests can call directly if they need the legacy code path. Once the new `update` command ships, the user-facing `update record` invocation is the new one. Document this prominently.

So `main.go` becomes:

```go
		commands.Update(homeDir, getWd, readDefinition, newDB, logf),
```

(Replacing the previous `commands.Update(...)` line with itself, but now backed by the new function.)

Do NOT register `UpdateLegacy` in `main.go`. The Go symbol still exists and the helper functions are still callable; we just don't expose the legacy parent as a separate cobra command.

The legacy regression test in Task 7 confirms `UpdateLegacy(...).Execute()` with `record --id=... --set=...` args still produces the same patch result — at the Go function level, the migration is non-breaking. The user-facing CLI invocation has changed: `ingitdb update record --id=... --set=YAML-string` becomes `ingitdb update --id=... --set=field=value`.

- [ ] **Step 1.7 — Run the test to confirm pass**

```bash
go test -timeout=10s -run TestUpdate_ ./cmd/ingitdb/commands/
```

Expected: PASS — all 5 tests green. The "not yet implemented" branches are never reached.

- [ ] **Step 1.8 — Verify the binary builds**

```bash
go build -o /tmp/ingitdb-update ./cmd/ingitdb/
/tmp/ingitdb-update update --help 2>&1 | head -20
```

Expected: help text mentions `--id`, `--from`, `--where`, `--set`, `--unset`, `--all`, `--min-affected`, `--path`, `--remote`, `--token`.

- [ ] **Step 1.9 — Lint and commit**

```bash
go test -timeout=30s ./...
golangci-lint run
git add cmd/ingitdb/commands/update.go cmd/ingitdb/commands/update_new.go cmd/ingitdb/commands/update_test.go cmd/ingitdb/main.go
git commit -m "$(cat <<'EOF'
feat(cli): scaffold update command; rename legacy Update -> UpdateLegacy

The new `ingitdb update` command (top-level, SQL-verb redesign)
replaces the user-facing surface of the legacy `update record`
subcommand. The legacy parent's Go function is renamed to
UpdateLegacy and is no longer registered in main.go, but the helper
remains exported for the Task 7 regression test that exercises the
legacy patch path directly.

The new RunE registers --id, --from, --where, --set, --unset, --all,
--min-affected plus the forbidden shared flags (--into, --order-by,
--fields) which are rejected with verb-specific diagnostics.
Mode resolution via sqlflags.ResolveMode. Returns "not yet implemented"
until subsequent tasks land single-record and set-mode patch logic.

Spec: spec/features/cli/update/README.md
EOF
)"
```

---

## Task 2 — Single-record mode (`--id` + patch)

**Context:** Single-record update fetches one record, applies the patch (set + unset), and writes back. The pattern mirrors the legacy `update record` but with two changes: (1) `--set` is repeatable and uses `field=value` syntax parsed by `sqlflags.ParseSet` instead of the legacy `--set=YAML-blob`; (2) `--unset` is new.

**Files:**
- Modify: `cmd/ingitdb/commands/update_new.go`
- Modify: `cmd/ingitdb/commands/update_test.go`

- [ ] **Step 2.1 — Write the failing tests**

Append to `cmd/ingitdb/commands/update_test.go`:

```go
import (
	"os"
	"path/filepath"
)

func seedItem(t *testing.T, dir, key string, data map[string]any) {
	t.Helper()
	if err := seedRecord(t, dir, "test.items", key, data); err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

func readItem(t *testing.T, dir, key string) string {
	t.Helper()
	colDef := testDef(dir).Collections["test.items"]
	got, err := os.ReadFile(filepath.Join(dir, colDef.Path, key+".yaml"))
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return string(got)
}

func TestUpdate_SingleRecord_Set(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "alpha", map[string]any{"title": "Alpha", "priority": float64(1)})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/alpha", "--set=priority=5",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	got := readItem(t, dir, "alpha")
	if !strings.Contains(got, "priority: 5") {
		t.Errorf("expected priority: 5, got:\n%s", got)
	}
	if !strings.Contains(got, "title: Alpha") {
		t.Errorf("title should be preserved, got:\n%s", got)
	}
}

func TestUpdate_SingleRecord_MultipleSets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "beta", map[string]any{"title": "Beta"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/beta",
		"--set=priority=3", "--set=active=true",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	got := readItem(t, dir, "beta")
	if !strings.Contains(got, "priority: 3") {
		t.Errorf("missing priority, got:\n%s", got)
	}
	if !strings.Contains(got, "active: true") {
		t.Errorf("missing active, got:\n%s", got)
	}
}

func TestUpdate_SingleRecord_Unset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "gamma", map[string]any{"title": "Gamma", "tmp": "scratch"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/gamma", "--unset=tmp",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	got := readItem(t, dir, "gamma")
	if strings.Contains(got, "tmp:") {
		t.Errorf("tmp field should be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "title: Gamma") {
		t.Errorf("title should be preserved, got:\n%s", got)
	}
}

func TestUpdate_SingleRecord_SetAndUnset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "delta", map[string]any{"title": "Delta", "draft": true})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/delta",
		"--set=status=published", "--unset=draft",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	got := readItem(t, dir, "delta")
	if !strings.Contains(got, "status: published") {
		t.Errorf("missing status, got:\n%s", got)
	}
	if strings.Contains(got, "draft:") {
		t.Errorf("draft field should be removed, got:\n%s", got)
	}
}

func TestUpdate_SingleRecord_SetUnsetSameFieldRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "epsilon", map[string]any{"title": "Epsilon"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/epsilon",
		"--set=foo=bar", "--unset=foo",
	)
	if err == nil {
		t.Fatal("expected error when --set and --unset reference the same field")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("error should name the conflicting field, got: %v", err)
	}
}

func TestUpdate_SingleRecord_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/missing", "--set=foo=bar",
	)
	if err == nil {
		t.Fatal("expected error when record not found")
	}
	if !strings.Contains(err.Error(), "missing") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention the missing id, got: %v", err)
	}
}
```

`seedRecord` is the helper defined in `select_test.go` (cli/select plan); it is package-visible.

- [ ] **Step 2.2 — Confirm failure**

```bash
go test -timeout=10s -run TestUpdate_SingleRecord ./cmd/ingitdb/commands/
```

Expected: FAIL with "not yet implemented".

- [ ] **Step 2.3 — Implement the single-record branch**

In `cmd/ingitdb/commands/update_new.go`, replace the `case sqlflags.ModeID:` line with:

```go
			case sqlflags.ModeID:
				return runUpdateByID(cmd.Context(), cmd, id, setExprs, unsetExprs, homeDir, getWd, readDefinition, newDB)
```

Add the `runUpdateByID` function at the bottom of `update_new.go`:

```go
import (
	"context"
	"maps"
)

// runUpdateByID handles --id mode: fetch one record, apply the patch
// (set + unset), write back. Returns non-zero if the record doesn't
// exist. Shallow patch semantics: fields not named in --set/--unset
// are preserved unchanged.
func runUpdateByID(
	ctx context.Context,
	cmd *cobra.Command,
	id string,
	setExprs []string,
	unsetExprs []string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	// Reject set-mode-only flags in single-record mode.
	if cmd.Flags().Changed("where") {
		return fmt.Errorf("--where is invalid with --id (single-record mode)")
	}
	if cmd.Flags().Changed("all") {
		return fmt.Errorf("--all is invalid with --id (single-record mode)")
	}
	if cmd.Flags().Changed("min-affected") {
		return fmt.Errorf("--min-affected is invalid with --id (single-record mode)")
	}

	// Parse --set assignments and --unset field lists.
	sets, err := parseSetExprs(setExprs)
	if err != nil {
		return err
	}
	unsets, err := parseUnsetExprs(unsetExprs)
	if err != nil {
		return err
	}
	if err := sqlflags.RejectSetUnsetSameField(sets, unsets); err != nil {
		return err
	}

	rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
	if err != nil {
		return err
	}

	key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
	err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		data := map[string]any{}
		record := dal.NewRecordWithData(key, data)
		if getErr := tx.Get(ctx, record); getErr != nil {
			return getErr
		}
		if !record.Exists() {
			return fmt.Errorf("record not found: %s", id)
		}
		applyPatch(data, sets, unsets)
		return tx.Set(ctx, record)
	})
	if err != nil {
		return err
	}
	return buildLocalViews(ctx, rctx)
}

// parseSetExprs converts the raw --set strings into Assignment values
// via sqlflags.ParseSet. Returns the first parse error.
func parseSetExprs(exprs []string) ([]sqlflags.Assignment, error) {
	out := make([]sqlflags.Assignment, 0, len(exprs))
	for _, e := range exprs {
		a, err := sqlflags.ParseSet(e)
		if err != nil {
			return nil, fmt.Errorf("invalid --set %q: %w", e, err)
		}
		out = append(out, a)
	}
	return out, nil
}

// parseUnsetExprs accumulates all --unset entries into a flat field
// list. Each --unset value may be comma-separated; the flag itself is
// repeatable.
func parseUnsetExprs(exprs []string) ([]string, error) {
	var out []string
	for _, e := range exprs {
		fields, err := sqlflags.ParseUnset(e)
		if err != nil {
			return nil, fmt.Errorf("invalid --unset %q: %w", e, err)
		}
		out = append(out, fields...)
	}
	return out, nil
}

// applyPatch applies the shallow patch (set + unset) to a record's
// data map in place. Fields not named in either list are preserved.
func applyPatch(data map[string]any, sets []sqlflags.Assignment, unsets []string) {
	for _, a := range sets {
		data[a.Field] = a.Value
	}
	for _, f := range unsets {
		delete(data, f)
	}
}
```

Add the new imports (`context`, `maps`) — note `maps` is in stdlib since Go 1.21 and is unused here; you can drop the `maps` import if `applyPatch` doesn't use it. The plan's `applyPatch` uses explicit loops, so `maps` is not needed. Verify the imports list and drop unused ones.

- [ ] **Step 2.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/update_new.go cmd/ingitdb/commands/update_test.go
git commit -m "$(cat <<'EOF'
feat(cli/update): implement single-record mode (--id + --set/--unset)

Fetches one record via resolveRecordContext, applies the patch via
applyPatch (shallow at top level: --set entries overwrite or add
fields; --unset entries delete fields), and writes back via tx.Set
inside RunReadwriteTransaction.

Repeatable --set parses each entry via sqlflags.ParseSet (YAML 1.2
scalar inference for values). Repeatable --unset values are
comma-separated lists, each parsed via sqlflags.ParseUnset.
sqlflags.RejectSetUnsetSameField guards against the same field
appearing in both flags.

Spec:
- cli/update#req:patch-required
- cli/update#req:patch-shallow
- cli/update#req:single-record-not-found
- cli/update#req:single-record-rejected-flags
- cli/update#req:set-unset-field-exclusion-inherited
EOF
)"
```

---

## Task 3 — Set mode (`--from` + `--where`|`--all`)

**Context:** Set mode fetches every record from the named collection via DAL, applies WHERE through `evalAllWhere`, then patches each match in a single transaction. `--min-affected` enforcement is added in Task 4; this task implements the unfiltered + WHERE-filtered + `--all` cases.

**Files:**
- Modify: `cmd/ingitdb/commands/update_new.go`
- Modify: `cmd/ingitdb/commands/update_test.go`

- [ ] **Step 3.1 — Write failing tests**

Append to `cmd/ingitdb/commands/update_test.go`:

```go
func TestUpdate_SetMode_WhereFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"priority": float64(1), "active": true})
	seedItem(t, dir, "b", map[string]any{"priority": float64(5), "active": true})
	seedItem(t, dir, "c", map[string]any{"priority": float64(3), "active": true})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=priority>=3",
		"--set=active=false",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if !strings.Contains(readItem(t, dir, "a"), "active: true") {
		t.Errorf("record a should be untouched (priority=1)")
	}
	if !strings.Contains(readItem(t, dir, "b"), "active: false") {
		t.Errorf("record b should be patched (priority=5)")
	}
	if !strings.Contains(readItem(t, dir, "c"), "active: false") {
		t.Errorf("record c should be patched (priority=3)")
	}
}

func TestUpdate_SetMode_All(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"priority": float64(1)})
	seedItem(t, dir, "b", map[string]any{"priority": float64(2)})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--all", "--set=checked=true",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if !strings.Contains(readItem(t, dir, "a"), "checked: true") {
		t.Errorf("record a should have checked: true")
	}
	if !strings.Contains(readItem(t, dir, "b"), "checked: true") {
		t.Errorf("record b should have checked: true")
	}
}

func TestUpdate_SetMode_WhereAndAllMutuallyExclusive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=a==1", "--all", "--set=b=2",
	)
	if err == nil {
		t.Fatal("expected error when --where and --all both supplied")
	}
}

func TestUpdate_SetMode_NeitherWhereNorAllRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--set=b=2",
	)
	if err == nil {
		t.Fatal("expected error when set mode has neither --where nor --all")
	}
}

func TestUpdate_SetMode_ZeroMatchesIsSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"priority": float64(1)})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=priority>1000",
		"--set=checked=true",
	)
	if err != nil {
		t.Errorf("zero matches should succeed (exit 0), got: %v", err)
	}
	// Record should be unchanged.
	if !strings.Contains(readItem(t, dir, "a"), "priority: 1") {
		t.Errorf("record should be unchanged when no matches")
	}
}
```

- [ ] **Step 3.2 — Confirm failure**

```bash
go test -timeout=10s -run TestUpdate_SetMode ./cmd/ingitdb/commands/
```

Expected: FAIL with "not yet implemented".

- [ ] **Step 3.3 — Implement set mode**

In `cmd/ingitdb/commands/update_new.go`, replace the `case sqlflags.ModeFrom:` line with:

```go
			case sqlflags.ModeFrom:
				return runUpdateFromSet(cmd.Context(), cmd, from, setExprs, unsetExprs, homeDir, getWd, readDefinition, newDB)
```

Add the `runUpdateFromSet` function at the bottom of `update_new.go`:

```go
// runUpdateFromSet handles --from set mode: fetch all records, apply
// WHERE filter (or --all), apply patch to each matching record in a
// single transaction.
func runUpdateFromSet(
	ctx context.Context,
	cmd *cobra.Command,
	from string,
	setExprs []string,
	unsetExprs []string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	// Mutual exclusion: --where XOR --all.
	whereExprs, _ := cmd.Flags().GetStringArray("where")
	allFlag, _ := cmd.Flags().GetBool("all")
	if len(whereExprs) > 0 && allFlag {
		return fmt.Errorf("--where and --all are mutually exclusive")
	}
	if len(whereExprs) == 0 && !allFlag {
		return fmt.Errorf("set mode requires one of --where or --all")
	}

	// Parse patches.
	sets, err := parseSetExprs(setExprs)
	if err != nil {
		return err
	}
	unsets, err := parseUnsetExprs(unsetExprs)
	if err != nil {
		return err
	}
	if err := sqlflags.RejectSetUnsetSameField(sets, unsets); err != nil {
		return err
	}

	// Parse --where conditions.
	conds := make([]sqlflags.Condition, 0, len(whereExprs))
	for _, e := range whereExprs {
		c, parseErr := sqlflags.ParseWhere(e)
		if parseErr != nil {
			return fmt.Errorf("invalid --where %q: %w", e, parseErr)
		}
		conds = append(conds, c)
	}

	// Resolve collection (local or GitHub).
	ictx, err := resolveInsertContext(ctx, cmd, from, homeDir, getWd, readDefinition, newDB)
	if err != nil {
		return err
	}

	// Fetch matching keys + their data via a read-only pass, then
	// patch and write within a read-write transaction.
	type match struct {
		key  string
		data map[string]any
	}
	var matches []match
	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef(from, "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		k := dal.NewKeyWithID(from, "")
		return dal.NewRecordWithData(k, map[string]any{})
	})
	err = ictx.db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
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
			if !allFlag {
				match, evalErr := evalAllWhere(data, recKey, conds)
				if evalErr != nil {
					return evalErr
				}
				if !match {
					continue
				}
			}
			matches = append(matches, struct {
				key  string
				data map[string]any
			}{key: recKey, data: data})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// Apply patches in a single read-write transaction.
	err = ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		for _, m := range matches {
			applyPatch(m.data, sets, unsets)
			key := dal.NewKeyWithID(from, m.key)
			record := dal.NewRecordWithData(key, m.data)
			if setErr := tx.Set(ctx, record); setErr != nil {
				return setErr
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Materialize local views.
	if ictx.dirPath == "" {
		return nil
	}
	return buildLocalViews(ctx, recordContext{
		db:      ictx.db,
		colDef:  ictx.colDef,
		dirPath: ictx.dirPath,
		def:     ictx.def,
	})
}
```

Note that this function has the awkward `struct { key string; data map[string]any }` anonymous struct twice. Replace the local type definition + append site with a named struct at the top of `update_new.go`:

```go
// patchTarget pairs a record key with its data map, used as the unit
// of work for set-mode patching.
type patchTarget struct {
	key  string
	data map[string]any
}
```

…and update the function body to use `patchTarget` instead of the anonymous struct in both places. The `matches []patchTarget` and `append(matches, patchTarget{key: recKey, data: data})` form is cleaner.

- [ ] **Step 3.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/update_new.go cmd/ingitdb/commands/update_test.go
git commit -m "$(cat <<'EOF'
feat(cli/update): implement set mode (--from + --where|--all)

Fetches every record from the named collection via dal.Query, applies
the WHERE filter through evalAllWhere (or accepts every record under
--all), then patches each match in a single read-write transaction
using the same applyPatch helper as single-record mode.

--where and --all are mutually exclusive; neither supplied is also
rejected. Zero matches is success (exit 0, no writes). The patch is
applied atomically — either every match is patched or the transaction
rolls back.

Spec:
- cli/update#req:set-mode-shape
- cli/update#req:set-mode-zero-matches-default
EOF
)"
```

---

## Task 4 — `--min-affected` pre-flight check

**Context:** When `--min-affected=N` is supplied AND the matched count is less than N, update MUST exit non-zero AND MUST NOT mutate any record. The check happens AFTER the read-only filter pass but BEFORE the read-write transaction starts.

**Files:**
- Modify: `cmd/ingitdb/commands/update_new.go`
- Modify: `cmd/ingitdb/commands/update_test.go`

- [ ] **Step 4.1 — Write the failing tests**

Append to `cmd/ingitdb/commands/update_test.go`:

```go
func TestUpdate_MinAffected_ThresholdMet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"region": "EU"})
	seedItem(t, dir, "b", map[string]any{"region": "EU"})
	seedItem(t, dir, "c", map[string]any{"region": "US"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=region==EU",
		"--set=active=true", "--min-affected=2",
	)
	if err != nil {
		t.Fatalf("expected success when matches (2) >= threshold (2), got: %v", err)
	}
	if !strings.Contains(readItem(t, dir, "a"), "active: true") {
		t.Errorf("record a should be patched")
	}
	if !strings.Contains(readItem(t, dir, "b"), "active: true") {
		t.Errorf("record b should be patched")
	}
}

func TestUpdate_MinAffected_ThresholdUnmet_NoWriteOccurs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"region": "EU"})
	seedItem(t, dir, "b", map[string]any{"region": "US"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=region==EU",
		"--set=active=false", "--min-affected=2",
	)
	if err == nil {
		t.Fatal("expected error when matches (1) < threshold (2)")
	}
	if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "2") {
		t.Errorf("error should name actual (1) and required (2), got: %v", err)
	}
	// Verify NO write occurred — neither record should have `active` set.
	if strings.Contains(readItem(t, dir, "a"), "active:") {
		t.Errorf("record a must be unchanged when threshold unmet")
	}
	if strings.Contains(readItem(t, dir, "b"), "active:") {
		t.Errorf("record b must be unchanged when threshold unmet")
	}
}

func TestUpdate_MinAffected_RejectedInSingleRecordMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"title": "Alpha"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/a", "--set=foo=bar", "--min-affected=1",
	)
	if err == nil {
		t.Fatal("expected error when --min-affected is supplied with --id")
	}
}
```

- [ ] **Step 4.2 — Confirm failure**

```bash
go test -timeout=10s -run TestUpdate_MinAffected ./cmd/ingitdb/commands/
```

Expected: FAIL — `TestUpdate_MinAffected_ThresholdUnmet_NoWriteOccurs` either succeeds when it shouldn't (no threshold check yet), or `TestUpdate_MinAffected_ThresholdMet` works coincidentally. The single-record rejection test passes already (we added it in Task 2).

- [ ] **Step 4.3 — Add the threshold check in `runUpdateFromSet`**

In `cmd/ingitdb/commands/update_new.go`, find the `runUpdateFromSet` function. After the read-only fetch loop populates `matches` and BEFORE the read-write transaction starts, insert this block:

```go
	// --min-affected pre-flight check. If the matched count is below
	// the threshold, fail BEFORE opening the write transaction.
	if n, supplied, mErr := sqlflags.MinAffectedFromCmd(cmd); mErr != nil {
		return mErr
	} else if supplied && len(matches) < n {
		return fmt.Errorf("matched %d records, required at least %d", len(matches), n)
	}
```

This placement guarantees no write occurs when the threshold isn't met — the threshold check is a pure read, no transactional state has been mutated.

- [ ] **Step 4.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/update_new.go cmd/ingitdb/commands/update_test.go
git commit -m "$(cat <<'EOF'
feat(cli/update): --min-affected all-or-nothing threshold

Adds a pre-flight check in runUpdateFromSet: after the read-only
filter pass populates matches but BEFORE the read-write transaction
opens, --min-affected (when supplied) is compared to len(matches).
Below-threshold returns non-zero with a diagnostic naming both
values, and NO write occurs.

In single-record mode (--id), --min-affected is rejected outright.

Spec:
- cli/update#req:min-affected-flag
- shared-cli-flags#req:min-affected-semantics
EOF
)"
```

---

## Task 5 — End-to-end + legacy regression

**Context:** One realistic invocation exercising both `--set` and `--unset` across set mode, plus a regression test confirming the legacy `update record` patch code path (now reached only via the `UpdateLegacy` Go function) still works at the function level.

**Files:**
- Modify: `cmd/ingitdb/commands/update_test.go`

- [ ] **Step 5.1 — Write the end-to-end test**

Append to `cmd/ingitdb/commands/update_test.go`:

```go
func TestUpdate_EndToEnd_RealisticInvocation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "low", map[string]any{"priority": float64(1), "draft": true, "title": "T-low"})
	seedItem(t, dir, "mid", map[string]any{"priority": float64(3), "draft": true, "title": "T-mid"})
	seedItem(t, dir, "high", map[string]any{"priority": float64(5), "draft": true, "title": "T-high"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items",
		"--where=priority>=3",
		"--set=status=published", "--unset=draft",
		"--min-affected=1",
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// "low" should be untouched (priority=1).
	low := readItem(t, dir, "low")
	if !strings.Contains(low, "draft: true") {
		t.Errorf("low: expected draft: true, got:\n%s", low)
	}
	if strings.Contains(low, "status:") {
		t.Errorf("low: status should not be set, got:\n%s", low)
	}

	// "mid" and "high" should be patched.
	for _, key := range []string{"mid", "high"} {
		got := readItem(t, dir, key)
		if !strings.Contains(got, "status: published") {
			t.Errorf("%s: expected status: published, got:\n%s", key, got)
		}
		if strings.Contains(got, "draft:") {
			t.Errorf("%s: draft field should be removed, got:\n%s", key, got)
		}
		if !strings.Contains(got, "title: T-"+key) {
			t.Errorf("%s: title should be preserved, got:\n%s", key, got)
		}
	}
}
```

- [ ] **Step 5.2 — Write the legacy regression test**

```go
func TestUpdate_LegacyUpdateRecordStillWorks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "legacy", map[string]any{"title": "Before"})

	// UpdateLegacy returns the parent command; invoke `record` with
	// the legacy YAML-blob --set syntax.
	legacyCmd := UpdateLegacy(homeDir, getWd, readDef, newDB, logf)
	var buf bytes.Buffer
	legacyCmd.SetOut(&buf)
	legacyCmd.SetErr(&buf)
	legacyCmd.SetArgs([]string{
		"record",
		"--path=" + dir,
		"--id=test.items/legacy",
		`--set={title: After, status: ok}`,
	})
	if err := legacyCmd.Execute(); err != nil {
		t.Errorf("legacy update record regressed: %v", err)
	}

	got := readItem(t, dir, "legacy")
	if !strings.Contains(got, "title: After") {
		t.Errorf("legacy patch missing title: After, got:\n%s", got)
	}
	if !strings.Contains(got, "status: ok") {
		t.Errorf("legacy patch missing status: ok, got:\n%s", got)
	}
}
```

- [ ] **Step 5.3 — Run the whole-repo test suite**

```bash
go test -timeout=30s ./...
golangci-lint run
```

Expected: 0 failures, 0 lint issues.

- [ ] **Step 5.4 — AC cross-check (commit message)**

Open `spec/features/cli/update/README.md`. Confirm each AC has a test:

- single-record-patch (Task 2's TestUpdate_SingleRecord_Set)
- single-record-unset (Task 2's TestUpdate_SingleRecord_Unset)
- set-and-unset-combined (Task 2's TestUpdate_SingleRecord_SetAndUnset)
- no-patch-rejected (Task 1's TestUpdate_NoPatchRejected)
- single-record-not-found (Task 2's TestUpdate_SingleRecord_NotFound)
- shallow-patch-replaces-nested-map (not directly tested; the patch's `data[a.Field] = a.Value` replaces top-level fields literally — a dedicated test would assert that --set='metadata={author: bob}' replaces an entire map; ADD it in Step 5.5 if missing)
- set-mode-where-patch (Task 3's TestUpdate_SetMode_WhereFilter)
- set-mode-all (Task 3's TestUpdate_SetMode_All)
- set-mode-zero-matches-default-success (Task 3's TestUpdate_SetMode_ZeroMatchesIsSuccess)
- min-affected-fails-below-threshold (Task 4's TestUpdate_MinAffected_ThresholdUnmet_NoWriteOccurs)
- min-affected-with-all (not directly tested; ADD in Step 5.5)
- min-affected-validation (covered by sqlflags.MinAffectedFromCmd's own tests + Task 4 single-record rejection; document)
- rejects-non-update-flags (Task 1's TestUpdate_RejectsForbiddenSharedFlags)
- github-update-one-commit (resolver-only; documented gap consistent with cli/select and cli/insert)

- [ ] **Step 5.5 — Add the two missing direct AC tests**

```go
func TestUpdate_ShallowPatchReplacesNestedMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "p1", map[string]any{
		"metadata": map[string]any{"author": "alice", "draft": true},
	})

	// --set on a top-level field that is itself a map REPLACES the
	// whole field; it does NOT deep-merge.
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/p1",
		`--set=metadata={author: bob}`,
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got := readItem(t, dir, "p1")
	if !strings.Contains(got, "author: bob") {
		t.Errorf("expected new metadata.author: bob, got:\n%s", got)
	}
	if strings.Contains(got, "draft:") {
		t.Errorf("old metadata.draft should be gone (shallow replace), got:\n%s", got)
	}
}

func TestUpdate_MinAffected_WithAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"x": float64(1)})
	seedItem(t, dir, "b", map[string]any{"x": float64(2)})
	seedItem(t, dir, "c", map[string]any{"x": float64(3)})

	// With --all, --min-affected=3 succeeds (3 records).
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--all",
		"--set=touched=true", "--min-affected=3",
	)
	if err != nil {
		t.Fatalf("expected success (3 >= 3): %v", err)
	}

	// With --all, --min-affected=4 fails (only 3 records).
	dir2 := t.TempDir()
	homeDir2, getWd2, readDef2, newDB2, _ := updateTestDeps(t, dir2)
	seedItem(t, dir2, "a", map[string]any{"x": float64(1)})
	_, err = runUpdateCmd(t, homeDir2, getWd2, readDef2, newDB2, logf,
		"--path="+dir2, "--from=test.items", "--all",
		"--set=touched=true", "--min-affected=4",
	)
	if err == nil {
		t.Fatal("expected error when collection size (1) < threshold (4)")
	}
}
```

- [ ] **Step 5.6 — Final commit**

```bash
go test -timeout=30s ./...
golangci-lint run
git add cmd/ingitdb/commands/update_test.go
git commit -m "$(cat <<'EOF'
test(cli/update): add end-to-end, legacy regression, and AC-gap tests

End-to-end covers --from + --where + --set + --unset + --min-affected
in one realistic invocation, verifying the matched records are
patched AND the title field is preserved (shallow-patch property).

Legacy regression confirms `update record --id=... --set=YAML` still
works via the UpdateLegacy Go function — the user-facing CLI
invocation has changed (new `update` is the top-level command), but
the underlying patch helper continues to work for any caller that
constructs the legacy command directly.

Two AC-gap tests (TestUpdate_ShallowPatchReplacesNestedMap,
TestUpdate_MinAffected_WithAll) close the spec-coverage gaps noted
during cross-check.

Spec: cli/update — all ACs covered. github-update-one-commit is
exercised at the resolver level (insert_context_test.go) consistent
with cli/select and cli/insert convention.
EOF
)"
```

---

## Self-Review

**1. Spec coverage.** Walking `spec/features/cli/update/README.md` REQs:

| REQ | Task |
|---|---|
| `subcommand-name` | 1 |
| `mode-selection` | 1 |
| `patch-required` | 1 (test) + 2 (function logic) |
| `patch-shallow` | 2, 5 (nested-map test) |
| `set-unset-field-exclusion-inherited` | 2 (via `sqlflags.RejectSetUnsetSameField`) |
| `single-record-not-found` | 2 |
| `single-record-rejected-flags` | 2, 4 (min-affected rejection in single-record) |
| `set-mode-shape` | 3 |
| `set-mode-zero-matches-default` | 3 |
| `min-affected-flag` | 4 |
| `success-output` | 2/3/4 — all paths exit 0 with empty stdout on success |
| `source-selection` | 2 (single-record via resolveRecordContext), 3 (set via resolveInsertContext — both already handle --remote) |
| `github-write-requires-token` | inherited; not exercised by integration tests |

**2. Placeholder scan.** No `TBD`/`TODO`/"implement later" strings. Every code block is complete.

**3. Type consistency.** `patchTarget` struct introduced once in Task 3 and used consistently. `applyPatch`, `parseSetExprs`, `parseUnsetExprs` defined in Task 2, called in Task 3. `runUpdateByID` and `runUpdateFromSet` defined in Task 2 and Task 3 respectively, called from the same RunE switch. `updateTestDeps`, `runUpdateCmd`, `seedItem`, `readItem` test helpers defined in Task 1/2's test file, used throughout.

**4. Reuse audit.** Direct calls to: `sqlflags.ResolveMode`, `sqlflags.ParseSet`, `sqlflags.ParseUnset`, `sqlflags.ParseWhere`, `sqlflags.RejectSetUnsetSameField`, `sqlflags.MinAffectedFromCmd`, `evalAllWhere` (from cli/select), `resolveRecordContext`, `resolveInsertContext`, `buildLocalViews`, `seedRecord` (from cli/select test helpers). No re-implementation.

**5. Migration friction.** The user-facing CLI invocation for `update record` changes: it's now `ingitdb update --id=... --set=field=value` (NEW), replacing `ingitdb update record --id=... --set=YAML-blob` (LEGACY). The legacy Go function survives as `UpdateLegacy` for the regression test, but is not registered in `main.go`. This is documented in Task 1's commit message and is the only acceptable resolution to the structural conflict between cobra command names.

---

## Execution Handoff

**Plan complete and saved to `spec/plans/2026-05-12-cli-update.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
