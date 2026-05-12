# cli/delete — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the `ingitdb delete` command — single-record mode (`--id`) and set mode (`--from` + `--where`|`--all`), `--min-affected` all-or-nothing destructive atomicity (no record is deleted when threshold unmet), local + GitHub source. Removes the legacy `Delete` parent command (`delete record`, `delete records`, `delete collection`, `delete view` subcommands) in the same release per the user's clarified migration policy.

**Architecture:** New `Delete` function lives in `cmd/ingitdb/commands/delete.go` (replacing the legacy parent body in the same file). The legacy single-record helper logic — `resolveRecordContext` + `tx.Delete` inside `RunReadwriteTransaction` — is preserved by being re-implemented inline in the new code. Set mode reuses `resolveInsertContext` + `evalAllWhere` (just like `cli/update`), collecting matching keys in a read-only pass, enforcing `--min-affected` BEFORE opening the write transaction, then deleting each key in one read-write transaction. Destructive atomicity: when the threshold isn't met, no `tx.Delete` is ever called.

**Tech Stack:** Go, `github.com/dal-go/dalgo/dal`, `github.com/spf13/cobra`, the project's `sqlflags` package and existing helpers (`resolveRecordContext`, `resolveInsertContext`, `evalAllWhere`, `buildLocalViews`).

**Spec:** `spec/features/cli/delete/README.md`

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Modify | `cmd/ingitdb/commands/delete.go` | Replace the legacy `Delete` parent with the new SQL-style `Delete` command. Same function signature, same name, completely new body. |
| Create | `cmd/ingitdb/commands/delete_test.go` | Integration tests for both modes |
| Delete | `cmd/ingitdb/commands/delete_record.go` | Legacy `deleteRecord` subcommand — gone |
| Delete | `cmd/ingitdb/commands/delete_record_test.go` | Legacy test — gone |
| Delete | `cmd/ingitdb/commands/delete_record_github_test.go` | Legacy GitHub test — gone |
| Delete | `cmd/ingitdb/commands/delete_record_integration_test.go` | Legacy integration test — gone |
| Delete | `cmd/ingitdb/commands/delete_records.go` | Stub — gone |
| Delete | `cmd/ingitdb/commands/delete_records_test.go` | Stub test — gone |
| Delete | `cmd/ingitdb/commands/delete_collection.go` | Stub — gone |
| Delete | `cmd/ingitdb/commands/delete_collection_test.go` | Stub test — gone |
| Delete | `cmd/ingitdb/commands/delete_view.go` | Stub — gone |
| Delete | `cmd/ingitdb/commands/delete_view_test.go` | Stub test — gone |
| Modify | `cmd/ingitdb/commands/crud_record_integration_test.go` | If it calls legacy `commands.Delete(...)` parent flow with `record` subcommand, update to call the new `Delete` directly; if migration is non-trivial, delete those test cases |
| Modify | `cmd/ingitdb/main.go` | No call-site change needed — `commands.Delete(homeDir, getWd, readDefinition, newDB, logf)` is the same signature; the body it dispatches to is replaced. |

**Reused (no edits):**

- `cmd/ingitdb/commands/sqlflags/*` — `RegisterIDFlag`, `RegisterFromFlag`, `RegisterWhereFlag`, `RegisterAllFlag`, `RegisterMinAffectedFlag`, `ResolveMode`, `MinAffectedFromCmd`, `ParseWhere`
- `cmd/ingitdb/commands/record_context.go` — `resolveRecordContext`, `recordContext`, `buildLocalViews`
- `cmd/ingitdb/commands/insert_context.go` — `resolveInsertContext`, `insertContext` (reused for set-mode collection resolution)
- `cmd/ingitdb/commands/select_where.go` — `evalAllWhere` (used for set-mode WHERE filtering)

---

## Task 1 — Remove legacy files + scaffold new `Delete`

**Context:** Per the user's clarified migration policy ("we can delete old commands"), this task removes the legacy `Delete` parent and all four of its subcommand files in the same release that ships the new `cli/delete`. Cobra cannot host two top-level commands with the same `Use` name, so the rename approach (`UpdateLegacy`) used by cli/update is unnecessary here — we just remove the old code outright.

**Files:**
- Modify: `cmd/ingitdb/commands/delete.go` (replace body)
- Create: `cmd/ingitdb/commands/delete_test.go`
- Delete: 9 legacy files (see File Map)
- Modify (if needed): `cmd/ingitdb/commands/crud_record_integration_test.go`

- [ ] **Step 1.1 — Inspect the legacy `crud_record_integration_test.go` for breakage**

```bash
grep -n "Delete\|delete record" cmd/ingitdb/commands/crud_record_integration_test.go 2>/dev/null
```

Report what you find. The test likely calls `commands.Delete(...)` then runs `Execute()` with `record --id=...` args. After this task, the new `Delete` doesn't have a `record` subcommand and won't recognize positional args — those test cases will break.

Two strategies:
- **A.** Migrate the legacy invocation to the new top-level form: `Delete(...).SetArgs([]string{"--id=test.items/x"}).Execute()`. Cleanest.
- **B.** Delete the affected test cases if migration is mechanical-but-tedious.

Use strategy A unless the test depends on the parent-subcommand routing in a way that doesn't translate. Report the chosen strategy in the commit message.

- [ ] **Step 1.2 — Write the failing test**

Write `cmd/ingitdb/commands/delete_test.go`:

```go
package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// deleteTestDeps returns a minimal DI set for the Delete command.
func deleteTestDeps(t *testing.T, dir string) (
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

// runDeleteCmd invokes the Delete command with the given args and
// returns captured stdout + any error.
func runDeleteCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	args ...string,
) (string, error) {
	t.Helper()
	cmd := Delete(homeDir, getWd, readDef, newDB, logf)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// deleteSeedItem seeds a record under test.items.
func deleteSeedItem(t *testing.T, dir, key string, data map[string]any) {
	t.Helper()
	if err := seedRecord(t, dir, "test.items", key, data); err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

// itemExists reports whether a record file exists on disk.
func itemExists(t *testing.T, dir, key string) bool {
	t.Helper()
	colDef := testDef(dir).Collections["test.items"]
	_, err := os.Stat(filepath.Join(dir, colDef.Path, key+".yaml"))
	return err == nil
}

func TestDelete_RegistersAllFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	cmd := Delete(homeDir, getWd, readDef, newDB, logf)
	for _, name := range []string{"id", "from", "where", "all", "min-affected", "path", "github"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestDelete_RejectsBothIDAndFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/x", "--from=test.items",
	)
	if err == nil {
		t.Fatal("expected error when both --id and --from supplied")
	}
}

func TestDelete_RejectsNeitherIDNorFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir,
	)
	if err == nil {
		t.Fatal("expected error when neither --id nor --from supplied")
	}
}

func TestDelete_RejectsForbiddenSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
	}{
		{name: "into rejected", args: []string{"--id=test.items/x", "--into=other"}},
		{name: "set rejected", args: []string{"--id=test.items/x", "--set=foo=bar"}},
		{name: "unset rejected", args: []string{"--id=test.items/x", "--unset=foo"}},
		{name: "order-by rejected", args: []string{"--id=test.items/x", "--order-by=name"}},
		{name: "fields rejected", args: []string{"--id=test.items/x", "--fields=a,b"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
		})
	}
}
```

- [ ] **Step 1.3 — Run to confirm failure**

```bash
go test -timeout=10s -run TestDelete_ ./cmd/ingitdb/commands/
```

Expected: COMPILE error — `Delete` now refers to the legacy parent which doesn't register `--from`/`--where`/`--all`/`--min-affected`. Tests that look for those flags will fail.

If the legacy `delete record` subcommand happens to register `--id` (it does), some tests may pass accidentally. The "rejects forbidden flags" tests will fail because the legacy command doesn't reject those.

- [ ] **Step 1.4 — Delete the 9 legacy files**

```bash
rm cmd/ingitdb/commands/delete_record.go
rm cmd/ingitdb/commands/delete_record_test.go
rm cmd/ingitdb/commands/delete_record_github_test.go
rm cmd/ingitdb/commands/delete_record_integration_test.go
rm cmd/ingitdb/commands/delete_records.go
rm cmd/ingitdb/commands/delete_records_test.go
rm cmd/ingitdb/commands/delete_collection.go
rm cmd/ingitdb/commands/delete_collection_test.go
rm cmd/ingitdb/commands/delete_view.go
rm cmd/ingitdb/commands/delete_view_test.go
```

- [ ] **Step 1.5 — Write the new `Delete` scaffold**

Replace the entire contents of `cmd/ingitdb/commands/delete.go`:

```go
package commands

import (
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Delete returns the `ingitdb delete` command. Two modes inherited
// from sqlflags: single-record (--id) and set (--from + --where|--all).
// --min-affected guards set-mode invocations with all-or-nothing
// destructive atomicity: when the matched count is below the
// threshold, NO record is deleted.
//
// This command replaces the legacy `delete record`, `delete records`,
// `delete collection`, and `delete view` subcommands. Per
// cli-sql-verbs Idea: when a new verb's name collides with an old
// top-level command, the legacy parent is removed in the same release.
func Delete(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete records from a collection (SQL DELETE)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			// Reject shared flags that don't apply to delete.
			for _, flag := range []string{"into", "set", "unset", "order-by", "fields"} {
				if cmd.Flags().Changed(flag) {
					return fmt.Errorf("--%s is not valid with delete", flag)
				}
			}

			id, _ := cmd.Flags().GetString("id")
			from, _ := cmd.Flags().GetString("from")
			mode, err := sqlflags.ResolveMode(id, from)
			if err != nil {
				return err
			}

			switch mode {
			case sqlflags.ModeID:
				return fmt.Errorf("delete --id: not yet implemented")
			case sqlflags.ModeFrom:
				return fmt.Errorf("delete --from: not yet implemented")
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
	sqlflags.RegisterAllFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	// Register the forbidden shared flags so cobra doesn't error on
	// "unknown flag"; we reject them in RunE with our own message.
	sqlflags.RegisterIntoFlag(cmd)
	sqlflags.RegisterSetFlag(cmd)
	sqlflags.RegisterUnsetFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	// Suppress unused DI params; they're used by later tasks.
	_, _, _, _, _ = homeDir, getWd, readDefinition, newDB, logf
	return cmd
}
```

- [ ] **Step 1.6 — Address `crud_record_integration_test.go`**

If Step 1.1 found legacy `Delete(...)` calls that invoke `record --id=...`, migrate them to the new shape:

```go
// Before
deleteCmd := Delete(homeDir, getWd, readDef, newDB, logf)
deleteCmd.SetArgs([]string{"record", "--path=" + dir, "--id=test.items/x"})

// After
deleteCmd := Delete(homeDir, getWd, readDef, newDB, logf)
deleteCmd.SetArgs([]string{"--path=" + dir, "--id=test.items/x"})
```

(Drop the `record` positional arg.)

If the test depends on a behavior that doesn't translate cleanly, delete the affected sub-test and note it in the commit message.

- [ ] **Step 1.7 — Verify the package builds**

```bash
go build ./cmd/ingitdb/...
```

Expected: clean build. If symbols from the deleted files are still referenced anywhere outside the deleted test files, `go build` will catch it. Fix the references.

- [ ] **Step 1.8 — Run tests**

```bash
go test -timeout=30s ./...
```

Expected: PASS. The 4 new `TestDelete_*` tests pass. All other tests in the package continue to pass (they exercise other commands).

- [ ] **Step 1.9 — Lint and commit**

```bash
golangci-lint run
git add -A cmd/ingitdb/commands/delete.go cmd/ingitdb/commands/delete_test.go cmd/ingitdb/commands/crud_record_integration_test.go
git add -u cmd/ingitdb/commands/  # picks up the deleted files
git commit -m "$(cat <<'EOF'
feat(cli): scaffold new delete command; remove legacy delete-* subcommands

Replaces the legacy `Delete` parent (which hosted `delete record`,
`delete records`, `delete collection`, `delete view`) with the new
SQL-style top-level `delete`. Per the cli-sql-verbs Idea: when a new
verb name collides with an old top-level command, the legacy parent
is removed in the same release.

Legacy files removed:
- delete_record.go (+ 3 test files)
- delete_records.go (+ test, was a stub)
- delete_collection.go (+ test, was a stub)
- delete_view.go (+ test, was a stub)

The new RunE registers --id, --from, --where, --all, --min-affected
plus the forbidden shared flags (--into, --set, --unset, --order-by,
--fields) which are rejected at RunE time with verb-specific
diagnostics. Returns "not yet implemented" until subsequent tasks
land single-record and set-mode delete logic.

crud_record_integration_test.go: migrated to the new top-level form
(dropped the `record` positional arg).

Spec: spec/features/cli/delete/README.md
EOF
)"
```

---

## Task 2 — Single-record mode (`--id`)

**Context:** Fetch the record (via `resolveRecordContext`), confirm it exists, then `tx.Delete`. Returns non-zero if the record doesn't exist.

**Files:**
- Modify: `cmd/ingitdb/commands/delete.go`
- Modify: `cmd/ingitdb/commands/delete_test.go`

- [ ] **Step 2.1 — Write the failing tests**

Append to `cmd/ingitdb/commands/delete_test.go`:

```go
func TestDelete_SingleRecord_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	deleteSeedItem(t, dir, "alpha", map[string]any{"title": "Alpha"})

	stdout, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/alpha",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if stdout != "" {
		t.Errorf("success path MUST be silent on stdout, got: %q", stdout)
	}
	if itemExists(t, dir, "alpha") {
		t.Errorf("record alpha should be gone after delete")
	}
}

func TestDelete_SingleRecord_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)

	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/missing",
	)
	if err == nil {
		t.Fatal("expected error when record not found")
	}
	if !strings.Contains(err.Error(), "missing") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should name the missing id, got: %v", err)
	}
}

func TestDelete_SingleRecord_RejectsSetModeFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	deleteSeedItem(t, dir, "beta", map[string]any{"title": "Beta"})

	cases := []struct {
		name string
		args []string
	}{
		{name: "where rejected", args: []string{"--id=test.items/beta", "--where=a==1"}},
		{name: "all rejected", args: []string{"--id=test.items/beta", "--all"}},
		{name: "min-affected rejected", args: []string{"--id=test.items/beta", "--min-affected=1"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
		})
	}
	// Verify the record still exists after the rejections.
	if !itemExists(t, dir, "beta") {
		t.Errorf("record beta should remain untouched after rejected invocations")
	}
}
```

- [ ] **Step 2.2 — Confirm failure**

```bash
go test -timeout=10s -run TestDelete_SingleRecord ./cmd/ingitdb/commands/
```

Expected: FAIL with "not yet implemented".

- [ ] **Step 2.3 — Implement the single-record branch**

In `cmd/ingitdb/commands/delete.go`, replace the `case sqlflags.ModeID:` line with:

```go
			case sqlflags.ModeID:
				return runDeleteByID(cmd.Context(), cmd, id, homeDir, getWd, readDefinition, newDB)
```

Add the new function at the bottom of the file:

```go
import (
	"context"
)

// runDeleteByID handles --id mode: fetch one record to confirm it
// exists, then delete it inside RunReadwriteTransaction. Returns
// non-zero if the record doesn't exist.
func runDeleteByID(
	ctx context.Context,
	cmd *cobra.Command,
	id string,
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

	rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
	if err != nil {
		return err
	}

	key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
	err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		// Pre-flight existence check. tx.Delete may or may not error
		// on missing keys depending on the backend; we want the
		// explicit user-facing diagnostic.
		probe := dal.NewRecordWithData(key, map[string]any{})
		if getErr := tx.Get(ctx, probe); getErr != nil {
			return getErr
		}
		if !probe.Exists() {
			return fmt.Errorf("record not found: %s", id)
		}
		return tx.Delete(ctx, key)
	})
	if err != nil {
		return err
	}
	return buildLocalViews(ctx, rctx)
}
```

Add `"context"` to the imports if not already present.

- [ ] **Step 2.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/delete.go cmd/ingitdb/commands/delete_test.go
git commit -m "$(cat <<'EOF'
feat(cli/delete): implement single-record mode (--id)

Fetches the record via resolveRecordContext, confirms existence inside
the read-write transaction (explicit "record not found" diagnostic),
then deletes via tx.Delete. Single-record mode rejects --where, --all,
--min-affected with verb-specific diagnostics. Success is silent on
stdout.

Spec:
- cli/delete#req:single-record-delete
- cli/delete#req:single-record-not-found
- cli/delete#req:single-record-rejected-flags
EOF
)"
```

---

## Task 3 — Set mode (`--from` + `--where`|`--all`)

**Context:** Fetch every record from the named collection, apply WHERE filter (or accept all under `--all`), collect matching keys in a read-only pass, then delete each key in a single read-write transaction. Mirrors the cli/update set-mode pattern.

**Files:**
- Modify: `cmd/ingitdb/commands/delete.go`
- Modify: `cmd/ingitdb/commands/delete_test.go`

- [ ] **Step 3.1 — Write failing tests**

Append to `cmd/ingitdb/commands/delete_test.go`:

```go
func TestDelete_SetMode_WhereFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	deleteSeedItem(t, dir, "a", map[string]any{"region": "EU"})
	deleteSeedItem(t, dir, "b", map[string]any{"region": "US"})
	deleteSeedItem(t, dir, "c", map[string]any{"region": "EU"})

	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=region==EU",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if itemExists(t, dir, "a") {
		t.Errorf("record a (EU) should be deleted")
	}
	if !itemExists(t, dir, "b") {
		t.Errorf("record b (US) should remain")
	}
	if itemExists(t, dir, "c") {
		t.Errorf("record c (EU) should be deleted")
	}
}

func TestDelete_SetMode_All(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	deleteSeedItem(t, dir, "a", map[string]any{"x": float64(1)})
	deleteSeedItem(t, dir, "b", map[string]any{"x": float64(2)})

	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--all",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if itemExists(t, dir, "a") {
		t.Errorf("record a should be deleted")
	}
	if itemExists(t, dir, "b") {
		t.Errorf("record b should be deleted")
	}
}

func TestDelete_SetMode_WhereAndAllMutuallyExclusive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=a==1", "--all",
	)
	if err == nil {
		t.Fatal("expected error when --where and --all both supplied")
	}
}

func TestDelete_SetMode_NeitherWhereNorAllRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items",
	)
	if err == nil {
		t.Fatal("expected error when set mode has neither --where nor --all")
	}
}

func TestDelete_SetMode_ZeroMatchesIsSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	deleteSeedItem(t, dir, "a", map[string]any{"x": float64(1)})

	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=x>1000",
	)
	if err != nil {
		t.Errorf("zero matches should succeed (exit 0), got: %v", err)
	}
	if !itemExists(t, dir, "a") {
		t.Errorf("record a should be unchanged when no matches")
	}
}
```

- [ ] **Step 3.2 — Confirm failure**

```bash
go test -timeout=10s -run TestDelete_SetMode ./cmd/ingitdb/commands/
```

Expected: FAIL with "not yet implemented".

- [ ] **Step 3.3 — Implement set mode**

In `cmd/ingitdb/commands/delete.go`, replace the `case sqlflags.ModeFrom:` line with:

```go
			case sqlflags.ModeFrom:
				return runDeleteFromSet(cmd.Context(), cmd, from, homeDir, getWd, readDefinition, newDB)
```

Add the new function at the bottom of the file:

```go
// runDeleteFromSet handles --from set mode: fetch all records, apply
// WHERE filter (or --all), then delete each matching record in a
// single transaction.
func runDeleteFromSet(
	ctx context.Context,
	cmd *cobra.Command,
	from string,
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

	// Read-only pass: collect matching keys.
	var matchedKeys []string
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
			recKey := fmt.Sprintf("%v", rec.Key().ID)
			if !allFlag {
				data, ok := rec.Data().(map[string]any)
				if !ok {
					continue
				}
				match, evalErr := evalAllWhere(data, recKey, conds)
				if evalErr != nil {
					return evalErr
				}
				if !match {
					continue
				}
			}
			matchedKeys = append(matchedKeys, recKey)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// Read-write pass: delete each matching key.
	err = ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		for _, k := range matchedKeys {
			key := dal.NewKeyWithID(from, k)
			if delErr := tx.Delete(ctx, key); delErr != nil {
				return delErr
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Materialize local views (no-op when source is GitHub).
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

- [ ] **Step 3.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/delete.go cmd/ingitdb/commands/delete_test.go
git commit -m "$(cat <<'EOF'
feat(cli/delete): implement set mode (--from + --where|--all)

Fetches every record from the named collection via dal.Query, applies
the WHERE filter through evalAllWhere (or accepts every record under
--all), collects matching keys in a read-only pass, then deletes each
key in a single read-write transaction. --where and --all are
mutually exclusive; neither supplied is also rejected.

Zero matches is success (exit 0, no writes). Each delete is atomic
within the transaction; if any tx.Delete errors, the whole batch
rolls back.

Spec:
- cli/delete#req:set-mode-shape
- cli/delete#req:set-mode-zero-matches-default
EOF
)"
```

---

## Task 4 — `--min-affected` with destructive atomicity

**Context:** For delete, `--min-affected` is more critical than for update — the operation is destructive. The spec explicitly says: "when the count falls short, no record is deleted." The check must happen AFTER the read-only key collection but BEFORE the read-write transaction starts.

**Files:**
- Modify: `cmd/ingitdb/commands/delete.go`
- Modify: `cmd/ingitdb/commands/delete_test.go`

- [ ] **Step 4.1 — Write the failing tests**

Append to `cmd/ingitdb/commands/delete_test.go`:

```go
func TestDelete_MinAffected_ThresholdMet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	deleteSeedItem(t, dir, "a", map[string]any{"region": "EU"})
	deleteSeedItem(t, dir, "b", map[string]any{"region": "EU"})

	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=region==EU",
		"--min-affected=2",
	)
	if err != nil {
		t.Fatalf("expected success when matches (2) >= threshold (2): %v", err)
	}
	if itemExists(t, dir, "a") || itemExists(t, dir, "b") {
		t.Errorf("both records should be deleted")
	}
}

func TestDelete_MinAffected_ThresholdUnmet_NoRecordDeleted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	deleteSeedItem(t, dir, "a", map[string]any{"region": "EU"})
	deleteSeedItem(t, dir, "b", map[string]any{"region": "US"})

	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=region==EU",
		"--min-affected=2",
	)
	if err == nil {
		t.Fatal("expected error when matches (1) < threshold (2)")
	}
	if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "2") {
		t.Errorf("error should name actual (1) and required (2), got: %v", err)
	}
	// Destructive atomicity: NEITHER record should be deleted.
	if !itemExists(t, dir, "a") {
		t.Errorf("record a (EU) MUST NOT be deleted when threshold unmet")
	}
	if !itemExists(t, dir, "b") {
		t.Errorf("record b (US) MUST NOT be deleted when threshold unmet")
	}
}
```

- [ ] **Step 4.2 — Confirm failure**

```bash
go test -timeout=10s -run TestDelete_MinAffected ./cmd/ingitdb/commands/
```

Expected: FAIL — `TestDelete_MinAffected_ThresholdUnmet_NoRecordDeleted` will likely fail at the post-condition (record a was deleted before the threshold check existed).

- [ ] **Step 4.3 — Add the threshold check in `runDeleteFromSet`**

In `cmd/ingitdb/commands/delete.go`, find `runDeleteFromSet`. After the read-only loop populates `matchedKeys` and BEFORE the read-write transaction starts, insert:

```go
	// --min-affected pre-flight check. If the matched count is below
	// the threshold, fail BEFORE opening the write transaction.
	// Destructive atomicity: no record is deleted when below
	// threshold.
	if n, supplied, mErr := sqlflags.MinAffectedFromCmd(cmd); mErr != nil {
		return mErr
	} else if supplied && len(matchedKeys) < n {
		return fmt.Errorf("matched %d records, required at least %d", len(matchedKeys), n)
	}
```

- [ ] **Step 4.4 — Pass + lint + commit**

```bash
go test -timeout=10s ./cmd/ingitdb/commands/
golangci-lint run ./cmd/ingitdb/commands/...
git add cmd/ingitdb/commands/delete.go cmd/ingitdb/commands/delete_test.go
git commit -m "$(cat <<'EOF'
feat(cli/delete): --min-affected all-or-nothing destructive atomicity

Adds a pre-flight check in runDeleteFromSet: after the read-only key-
collection pass populates matchedKeys but BEFORE the read-write
transaction opens, --min-affected (when supplied) is compared to
len(matchedKeys). Below-threshold returns non-zero with a diagnostic
naming both values, and NO record is deleted — destructive
atomicity is preserved.

Spec:
- cli/delete#req:min-affected-flag
- shared-cli-flags#req:min-affected-semantics
EOF
)"
```

---

## Task 5 — End-to-end + AC cross-check

**Context:** One realistic invocation exercising the full pipeline, plus AC verification.

**Files:**
- Modify: `cmd/ingitdb/commands/delete_test.go`

- [ ] **Step 5.1 — Write the end-to-end test**

Append to `cmd/ingitdb/commands/delete_test.go`:

```go
func TestDelete_EndToEnd_RealisticInvocation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	deleteSeedItem(t, dir, "low", map[string]any{"priority": float64(1), "title": "T-low"})
	deleteSeedItem(t, dir, "mid", map[string]any{"priority": float64(3), "title": "T-mid"})
	deleteSeedItem(t, dir, "high", map[string]any{"priority": float64(5), "title": "T-high"})

	stdout, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items",
		"--where=priority>=3",
		"--min-affected=2",
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout != "" {
		t.Errorf("success path MUST be silent on stdout, got: %q", stdout)
	}

	// "low" should remain (priority=1).
	if !itemExists(t, dir, "low") {
		t.Errorf("low should still exist (priority=1, didn't match filter)")
	}
	// "mid" and "high" should be deleted.
	if itemExists(t, dir, "mid") {
		t.Errorf("mid should be deleted")
	}
	if itemExists(t, dir, "high") {
		t.Errorf("high should be deleted")
	}
}

func TestDelete_MinAffected_WithAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	deleteSeedItem(t, dir, "a", map[string]any{"x": float64(1)})
	deleteSeedItem(t, dir, "b", map[string]any{"x": float64(2)})
	deleteSeedItem(t, dir, "c", map[string]any{"x": float64(3)})

	// With --all, --min-affected=3 succeeds (3 records).
	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--all", "--min-affected=3",
	)
	if err != nil {
		t.Fatalf("expected success (3 >= 3): %v", err)
	}
	if itemExists(t, dir, "a") || itemExists(t, dir, "b") || itemExists(t, dir, "c") {
		t.Errorf("all 3 records should be deleted")
	}

	// With --all, --min-affected=4 fails (only 0 records left).
	_, err = runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--all", "--min-affected=4",
	)
	if err == nil {
		t.Fatal("expected error when collection (0) < threshold (4)")
	}
}
```

- [ ] **Step 5.2 — Run the whole-repo test suite**

```bash
go test -timeout=30s ./...
golangci-lint run
```

Expected: 0 failures, 0 lint issues.

- [ ] **Step 5.3 — AC cross-check (commit message)**

Open `spec/features/cli/delete/README.md`. Walk every AC and confirm at least one test exercises it:

- single-record-delete (TestDelete_SingleRecord_Success)
- single-record-not-found (TestDelete_SingleRecord_NotFound)
- set-mode-where-delete (TestDelete_SetMode_WhereFilter)
- set-mode-all-delete (TestDelete_SetMode_All + TestDelete_SetMode_WhereAndAllMutuallyExclusive)
- set-mode-zero-matches-default-success (TestDelete_SetMode_ZeroMatchesIsSuccess)
- min-affected-fails-below-threshold (TestDelete_MinAffected_ThresholdUnmet_NoRecordDeleted)
- min-affected-with-all (TestDelete_MinAffected_WithAll)
- min-affected-validation (sqlflags.MinAffectedFromCmd's own tests + TestDelete_SingleRecord_RejectsSetModeFlags' min-affected case)
- rejects-non-delete-flags (TestDelete_RejectsForbiddenSharedFlags)

- [ ] **Step 5.4 — Final commit**

```bash
go test -timeout=30s ./...
golangci-lint run
git add cmd/ingitdb/commands/delete_test.go
git commit -m "$(cat <<'EOF'
test(cli/delete): add end-to-end and --min-affected-with-all tests

End-to-end covers --from + --where + --min-affected + silent-success
+ stored-state verification (low remains, mid/high deleted).

--min-affected with --all covers both the success path (collection
size >= threshold) and the failure path (collection size < threshold),
confirming destructive atomicity.

AC cross-check: all ACs in spec/features/cli/delete/README.md are
covered by at least one test. The github-write-requires-token path
is exercised via insert_context_test.go's resolver tests, consistent
with cli/select, cli/insert, and cli/update convention.
EOF
)"
```

---

## Self-Review

**1. Spec coverage.** Walking `spec/features/cli/delete/README.md` REQs:

| REQ | Task |
|---|---|
| `subcommand-name` | 1 |
| `mode-selection` | 1 |
| `single-record-delete` | 2 |
| `single-record-not-found` | 2 |
| `single-record-rejected-flags` | 2 |
| `set-mode-shape` | 3 |
| `set-mode-zero-matches-default` | 3 |
| `min-affected-flag` | 4 |
| `success-output` | 2, 3 (silent), 5 (asserted) |
| `source-selection` | 2 (single-record via resolveRecordContext), 3 (set via resolveInsertContext — both handle --github) |
| `github-write-requires-token` | inherited; not exercised by integration tests |

**2. Placeholder scan.** No `TBD`/`TODO`/"implement later". Every code block is complete.

**3. Type consistency.** `runDeleteByID` defined Task 2, called from RunE switch. `runDeleteFromSet` defined Task 3, called from RunE switch (replaces Task 1's placeholder). `matchedKeys []string` used in Tasks 3 and 4. `deleteSeedItem` and `itemExists` test helpers defined Task 1's test file, used throughout.

**4. Reuse audit.** Direct calls to: `sqlflags.ResolveMode`, `sqlflags.ParseWhere`, `sqlflags.MinAffectedFromCmd`, `evalAllWhere` (cli/select), `resolveRecordContext` (record_context.go), `resolveInsertContext` (insert_context.go), `buildLocalViews` (record_context.go), `seedRecord` (cli/select test helpers). No re-implementation.

**5. Legacy removal.** Nine files deleted, no `DeleteLegacy` Go-function survivor. The new `Delete` lives in the same file as the legacy parent did; main.go's `commands.Delete(...)` call site is unchanged. User-facing CLI changes: `ingitdb delete record/records/collection/view` invocations no longer work. `ingitdb delete --id=col/key` and `ingitdb delete --from=col --where=...` are the new entrypoints.

---

## Execution Handoff

**Plan complete and saved to `spec/plans/2026-05-12-cli-delete.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
