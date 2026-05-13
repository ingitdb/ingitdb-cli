# Cobra Migration Plan

Migrate `cmd/ingitdb` from `github.com/urfave/cli/v3` to `github.com/spf13/cobra`.

**Technical reference for agents**: `spec/plans/cobra-migration-guide.md`
**Lessons log (update as you go)**: `spec/plans/cobra-migration-lessons.md`

---

## Phase 0 — Foundation (go-engineer, main branch)

### New files to create

#### `cmd/ingitdb/commands/flags.go`

Create shared flag-registration helpers to eliminate the ~22-file repetition of `--path`,
~6-file repetition of `--remote`/`--token`, etc.

```go
package commands

import "github.com/spf13/cobra"

// addPathFlag adds --path flag (DB directory). Used by almost every command.
func addPathFlag(cmd *cobra.Command) {
    cmd.Flags().String("path", "", "path to the database directory (default: current directory)")
}

// addGitHubFlags adds --remote and --token flags. Used by record CRUD + list collections.
func addGitHubFlags(cmd *cobra.Command) {
    cmd.Flags().String("github", "", "GitHub source as owner/repo[@branch|tag|commit]")
    cmd.Flags().String("token", "", "GitHub personal access token (or set GITHUB_TOKEN env var)")
}

// addFormatFlag adds --format flag with a caller-specified default.
// Used by: query (default "csv"), read record (default "yaml"), watch, migrate.
func addFormatFlag(cmd *cobra.Command, defaultValue string) {
    cmd.Flags().String("format", defaultValue, "output format")
}

// addCollectionFlag adds --collection flag. Pass required=true to mark it required.
// Used by: query, truncate, read collection, delete collection, delete records, docs update.
func addCollectionFlag(cmd *cobra.Command, required bool) {
    cmd.Flags().StringP("collection", "c", "", "collection ID (e.g. todo.tags)")
    if required {
        _ = cmd.MarkFlagRequired("collection")
    }
}

// addMaterializeFlags adds the flags shared by materialize and ci commands.
func addMaterializeFlags(cmd *cobra.Command) {
    addPathFlag(cmd)
    cmd.Flags().String("views", "", "comma-separated list of views to materialize")
    cmd.Flags().Int("records-delimiter", 0,
        "write a '#-' delimiter after each record in INGR output; 0=default (enabled), 1=enabled, -1=disabled")
}
```

#### `cmd/ingitdb/commands/cobra_helpers.go`

Cobra-typed versions of shared helpers. Names include "Cobra" suffix to coexist with old
urfave/cli versions during Phase 1. Phase 2 renames them and deletes the old versions.

```go
package commands

import (
    "context"
    "fmt"

    "github.com/dal-go/dalgo/dal"
    "github.com/spf13/cobra"

    "github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// resolveDBPathCobra returns the database directory from --path or the working directory.
// Identical logic to the existing resolveDBPath but accepts *cobra.Command.
// Phase 2 renames this to resolveDBPath after the old urfave/cli version is deleted.
func resolveDBPathCobra(
    cmd *cobra.Command,
    homeDir func() (string, error),
    getWd func() (string, error),
) (string, error) {
    dirPath, _ := cmd.Flags().GetString("path")
    if dirPath == "" {
        wd, err := getWd()
        if err != nil {
            return "", fmt.Errorf("failed to get working directory: %w", err)
        }
        dirPath = wd
    }
    return expandHome(dirPath, homeDir) // expandHome stays in validate.go (no *cli.Command dep)
}

// resolveRecordContextCobra resolves DB + collection + record key for CRUD operations.
// Identical logic to the existing resolveRecordContext but accepts *cobra.Command.
// Phase 2 renames this to resolveRecordContext after the old urfave/cli version is deleted.
func resolveRecordContextCobra(
    ctx context.Context,
    cmd *cobra.Command,
    id string,
    homeDir func() (string, error),
    getWd func() (string, error),
    readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
    newDB func(string, *ingitdb.Definition) (dal.DB, error),
) (recordContext, error) {
    githubValue, _ := cmd.Flags().GetString("github")
    if githubValue != "" {
        return resolveGitHubRecordContext(ctx, cmd, id, githubValue) // needs cmd update — see below
    }
    return resolveLocalRecordContextCobra(cmd, id, homeDir, getWd, readDefinition, newDB)
}

func resolveLocalRecordContextCobra(
    cmd *cobra.Command,
    id string,
    homeDir func() (string, error),
    getWd func() (string, error),
    readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
    newDB func(string, *ingitdb.Definition) (dal.DB, error),
) (recordContext, error) {
    dirPath, resolveErr := resolveDBPathCobra(cmd, homeDir, getWd)
    if resolveErr != nil {
        return recordContext{}, resolveErr
    }
    def, readErr := readDefinition(dirPath)
    if readErr != nil {
        return recordContext{}, fmt.Errorf("failed to read database definition: %w", readErr)
    }
    colDef, recordKey, parseErr := dalgo2ingitdb.CollectionForKey(def, id)
    if parseErr != nil {
        return recordContext{}, fmt.Errorf("invalid --id: %w", parseErr)
    }
    db, err := newDB(dirPath, def)
    if err != nil {
        return recordContext{}, fmt.Errorf("failed to open database: %w", err)
    }
    return recordContext{db: db, colDef: colDef, recordKey: recordKey, dirPath: dirPath, def: def}, nil
}

// githubTokenCobra returns the GitHub token from --token flag or GITHUB_TOKEN env var.
// Identical logic to the existing githubToken but accepts *cobra.Command.
// Phase 2 renames this to githubToken after the old urfave/cli version is deleted.
func githubTokenCobra(cmd *cobra.Command) string {
    token, _ := cmd.Flags().GetString("token")
    if token == "" {
        token = os.Getenv("GITHUB_TOKEN")
    }
    return token
}
```

**Note on `resolveGitHubRecordContext`**: it currently takes `*cli.Command`. Phase 0 must also
update this private function to accept `*cobra.Command` (since it's called from
`resolveRecordContextCobra`). Extract the token using `githubTokenCobra(cmd)` internally.

#### `cmd/ingitdb/commands/helpers_test.go` — updated

Keep existing `runCLICommand` (still needed by unconverted tests during Phase 1), add:

```go
// runCobraCommand wraps a cobra command in a root and executes it with the given args.
// This is the cobra equivalent of runCLICommand.
func runCobraCommand(cmd *cobra.Command, args ...string) error {
    root := &cobra.Command{Use: "app", SilenceErrors: true, SilenceUsage: true}
    root.AddCommand(cmd)
    root.SetArgs(append([]string{cmd.Use}, args...))
    return root.ExecuteContext(context.Background())
}
```

#### `cmd/ingitdb/main.go` — stubbed

Keep the `run()` signature identical. Replace body with cobra stub that registers no commands yet:

```go
func run(
    args []string,
    homeDir func() (string, error),
    getWd func() (string, error),
    readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
    fatal func(error),
    logf func(...any),
) {
    rootCmd := &cobra.Command{
        Use:           "ingitdb",
        Short:         "Git-backed database CLI",
        SilenceUsage:  true,
        SilenceErrors: true,
    }
    rootCmd.SetErr(os.Stderr)
    // Commands wired in Phase 2
    rootCmd.SetArgs(args[1:])
    if err := rootCmd.ExecuteContext(context.Background()); err != nil {
        fatal(err)
    }
}
```

#### `cmd/ingitdb/main_test.go` — updated for stub

- `TestRun_NoSubcommand` — still passes (empty cobra root returns no error)
- `TestRun_Version`, `TestRun_ValidateSuccess`, `TestRun_AllCommands`, etc. — skip with
  `t.Skip("command routing tested in Phase 2 after full wiring")` until Phase 2
- `TestRun_ExitCoderWithNonZeroCode`, `TestRun_ExitCoderWithZeroCode` — remove entirely
  (urfave/cli-specific ExitCoder concept has no cobra equivalent)
- `TestRun_NonExitCoderError` — update to use a cobra-compatible error flow
- Build tag tests that call `run(...)` with specific subcommands: skip until Phase 2

**Phase 0 quality gate** (in order before committing):
1. `go build ./...`
2. `golangci-lint run` — fix **all** reported issues
3. `go test ./...`

---

## Phase 1 — Parallel Batch Migration

All 8 agents work simultaneously in separate worktrees created by the orchestrator.
Each agent reads `spec/plans/cobra-migration-guide.md` before starting.

### Worktree setup (orchestrator runs these, not agents)

```bash
# Run from repo root after Phase 0 is committed and pushed
git worktree add ../ingitdb-cli-b1 feat/cobra-batch-1
git worktree add ../ingitdb-cli-b2 feat/cobra-batch-2
git worktree add ../ingitdb-cli-b3 feat/cobra-batch-3
git worktree add ../ingitdb-cli-b4 feat/cobra-batch-4
git worktree add ../ingitdb-cli-b5 feat/cobra-batch-5
git worktree add ../ingitdb-cli-b6 feat/cobra-batch-6
git worktree add ../ingitdb-cli-b7 feat/cobra-batch-7
git worktree add ../ingitdb-cli-b8 feat/cobra-batch-8
```

Each agent receives its worktree path as its working directory.

**Every agent must pass this quality gate (in order) before committing:**
```bash
go build ./...
golangci-lint run   # fix ALL reported issues — no suppressions
go test ./cmd/ingitdb/...
```

### Worktree teardown (orchestrator runs after all merges complete)

```bash
git worktree remove ../ingitdb-cli-b1
# ... through b8
```

### Merge order (orchestrator, after all 8 agents report done)

```bash
# All merges are fast-forward or conflict-free (disjoint file sets)
git merge --no-ff feat/cobra-batch-1 -m "feat(cobra): merge batch-1"
git merge --no-ff feat/cobra-batch-2 -m "feat(cobra): merge batch-2"
# ... through batch-8
```

---

### Batch 1 — Simple stubs
**Worktree**: `../ingitdb-cli-b1`  
**Agent**: go-coder (haiku)  
**Files**:
- `cmd/ingitdb/commands/version.go`
- `cmd/ingitdb/commands/pull.go`
- `cmd/ingitdb/commands/setup.go`
- `cmd/ingitdb/commands/resolve.go`
- `cmd/ingitdb/commands/watch.go`
- `cmd/ingitdb/commands/migrate.go`
- `cmd/ingitdb/commands/find.go`

**Notes**:
- All commands currently return `cli.Exit("not yet implemented", 1)` in Action
- Replace with `return fmt.Errorf("not yet implemented")`
- Use `addPathFlag(cmd)` for any command that has `--path`
- `find.go` has `--limit` (IntFlag) and several string flags — use `cmd.Flags().Int("limit", 0, "...")`
- `migrate.go` has `--from`, `--to`, `--target` (all Required) — use `cmd.MarkFlagRequired("...")`
- No shared helper calls in these commands

---

### Batch 2 — Materialize + CI (with decoupling)
**Worktree**: `../ingitdb-cli-b2`  
**Agent**: go-coder (haiku)  
**Files**:
- `cmd/ingitdb/commands/materialize.go`
- `cmd/ingitdb/commands/ci.go`

**Notes**:
- `ci.go` currently borrows `mat.Flags` and `mat.Action` directly from `Materialize()` — this
  pattern does not exist in Cobra
- **Refactor**: extract `materializeRunE(...)` as a private function returning
  `func(*cobra.Command, []string) error`, then both `Materialize()` and `CI()` assign it to `RunE`
- Use `addMaterializeFlags(cmd)` (defined in Phase 0's `flags.go`) for both
- `cmd.Flags().Changed("records-delimiter")` replaces `cmd.IsSet("records-delimiter")`
- `expandHome` stays in `validate.go`; call it directly from materialize action
- Uses `resolveDBPathCobra`

---

### Batch 3 — Query
**Worktree**: `../ingitdb-cli-b3`  
**Agent**: go-coder (sonnet)  
**Files**:
- `cmd/ingitdb/commands/query.go`

**Notes**:
- Most complex flag set: `--collection` (required), `--fields`, `--where` (repeatable), `--order-by`,
  `--format`, `--path`
- `--where` was a `StringSliceFlag` with `DisableSliceFlagSeparator: true` → use
  `cmd.Flags().StringArray("where", nil, "...")` (StringArray never splits on comma; StringSlice does)
- Use `addPathFlag(cmd)`, `addCollectionFlag(cmd, true)`, `addFormatFlag(cmd, "csv")`
- `cmd.Flags().StringArray("where", nil, "filter expression (repeatable): field>value, field==value")` +
  `vals, _ := cmd.Flags().GetStringArray("where")`
- Uses `resolveDBPathCobra`

---

### Batch 4 — Delete group
**Worktree**: `../ingitdb-cli-b4`  
**Agent**: go-coder (haiku)  
**Files**:
- `cmd/ingitdb/commands/delete.go`
- `cmd/ingitdb/commands/delete_record.go`
- `cmd/ingitdb/commands/delete_records.go`
- `cmd/ingitdb/commands/delete_collection.go`
- `cmd/ingitdb/commands/delete_view.go`

**Notes**:
- `delete.go` is a parent command group: `cmd.AddCommand(deleteCollection(), deleteView(), deleteRecords(), deleteRecord(...))`
- `delete_record.go` uses `resolveRecordContextCobra`; needs `--path`, `--remote`, `--token`, `--id` (required)
- `delete_records.go`, `delete_collection.go`, `delete_view.go` are stubs — replace cli.Exit with fmt.Errorf
- Use `addPathFlag(cmd)`, `addGitHubFlags(cmd)` where applicable
- `delete.go` has alias `"d"` — `Aliases: []string{"d"}` works identically in Cobra

---

### Batch 5 — Read group
**Worktree**: `../ingitdb-cli-b5`  
**Agent**: go-coder (haiku)  
**Files**:
- `cmd/ingitdb/commands/read.go`
- `cmd/ingitdb/commands/read_record.go`
- `cmd/ingitdb/commands/read_collection.go`
- `cmd/ingitdb/commands/read_record_github.go`

**Notes**:
- `read.go` is parent group: `cmd.AddCommand(readRecord(...), readCollection(...))`; alias `"r"`
- `read_record.go` uses `resolveRecordContextCobra`; has `--format` (default "yaml")
  → use `addFormatFlag(cmd, "yaml")`
- `read_collection.go` uses `resolveDBPathCobra`; has `--collection` (required)
  → use `addCollectionFlag(cmd, true)`
- `read_record_github.go` contains `githubToken(*cli.Command)` — **do not convert this function
  here**; `githubTokenCobra` already exists in `cobra_helpers.go` from Phase 0.
  Just delete `githubToken` from this file (Phase 2 renames `githubTokenCobra`→`githubToken`)
  The other functions in read_record_github.go (`parseGitHubRepoSpec`, `newGitHubConfig`,
  `readRemoteDefinitionForID`, `resolveRemoteCollectionPath`, `listCollectionsFromFileReader`)
  do **not** take `*cli.Command` — they need no changes

---

### Batch 6 — Create + Update group
**Worktree**: `../ingitdb-cli-b6`  
**Agent**: go-coder (haiku)  
**Files**:
- `cmd/ingitdb/commands/create.go`
- `cmd/ingitdb/commands/create_record.go`
- `cmd/ingitdb/commands/update.go`
- `cmd/ingitdb/commands/update_record.go`

**Notes**:
- `create.go` and `update.go` are parent groups
- `create_record.go` and `update_record.go` both use `resolveRecordContextCobra`
- Both have `--path`, `--remote`, `--token`, `--id` (required), plus command-specific data flag
  → use `addPathFlag(cmd)`, `addGitHubFlags(cmd)`, `cmd.MarkFlagRequired("id")`
- `update_record.go` has `--set` (required): `cmd.Flags().String("set", "", "..."); _ = cmd.MarkFlagRequired("set")`

---

### Batch 7 — List + Truncate + Validate
**Worktree**: `../ingitdb-cli-b7`  
**Agent**: go-coder (haiku)  
**Files**:
- `cmd/ingitdb/commands/list.go`
- `cmd/ingitdb/commands/truncate.go`
- `cmd/ingitdb/commands/validate.go`

**Notes**:
- `validate.go` defines `resolveDBPath(*cli.Command, ...)` and `expandHome()`:
  - Keep `expandHome()` as-is (no `*cli.Command` dependency — pure path function)
  - Delete `resolveDBPath(*cli.Command, ...)` entirely (cobra version is in `cobra_helpers.go`)
- `validate.go` has `--from-commit` and `--to-commit` string flags (not required)
- `list.go` has nested subcommands: `collections()`, `listView()`, `subscribers()`
  - `collections()` uses `resolveDBPathCobra` + `githubTokenCobra` for GitHub branch
  - `listView()` and `subscribers()` are stubs
- `truncate.go` uses `resolveDBPathCobra` + `addCollectionFlag(cmd, true)`
- All three use `addPathFlag(cmd)`

---

### Batch 8 — Docs + Serve + Rebase
**Worktree**: `../ingitdb-cli-b8`  
**Agent**: go-coder (sonnet)  
**Files**:
- `cmd/ingitdb/commands/docs.go`
- `cmd/ingitdb/commands/docs_update.go`
- `cmd/ingitdb/commands/rebase.go`
- `cmd/ingitdb/commands/serve.go`
- `cmd/ingitdb/commands/serve_http.go` ← no `*cli.Command` usage, verify no changes needed
- `cmd/ingitdb/commands/serve_mcp.go` ← no `*cli.Command` usage, verify no changes needed

**Notes**:
- `docs.go` is a parent group → `cmd.AddCommand(docsUpdate(...))`
- `docs_update.go`:
  - Has inline `resolveDBPath`-equivalent logic; replace with `resolveDBPathCobra`
  - Has inline `expandHome`; keep as-is (it's a pure function still in validate.go)
  - Has `cli.Exit(...)` in two places → replace with `fmt.Errorf(...)`
  - `runDocsUpdate()` does **not** take `*cli.Command` — no changes needed to it
- `rebase.go` uses `cli.Exit(...)` in multiple places → replace with `fmt.Errorf(...)`
  - Does NOT use `resolveDBPath`; gets working dir via `getWd()` directly
- `serve.go` uses `resolveDBPathCobra`; has `StringSliceFlag` for `--api-domains`, `--mcp-domains`
  → `cmd.Flags().StringArray("api-domains", nil, "...")` (repeatable, no comma split)
  → `cmd.Flags().StringArray("mcp-domains", nil, "...")`
- `serve_http.go` and `serve_mcp.go` likely have no urfave/cli imports — verify, then leave unchanged

---

## Phase 2 — Integration (go-engineer, main branch)

After all 8 batch branches are merged:

1. **Rewrite `cmd/ingitdb/main.go`** fully with all commands wired:
   ```go
   rootCmd.AddCommand(
       commands.Version(version, commit, date),
       commands.Validate(homeDir, getWd, readDefinition, datavalidator.NewValidator(), nil, logf),
       commands.Query(homeDir, getWd, readDefinition, newDB, logf),
       // ... all others
   )
   ```
2. **Rename in `cobra_helpers.go`**: `resolveDBPathCobra` → `resolveDBPath`, etc.;
   then delete `cobra_helpers.go` (move functions to `validate.go` and `record_context.go`)
3. **Delete from `validate.go`**: old `resolveDBPath(*cli.Command, ...)` definition
4. **Delete from `record_context.go`**: old `resolveRecordContext(*cli.Command, ...)` and sub-functions
5. **Delete from `read_record_github.go`**: old `githubToken(*cli.Command)` (already removed in batch 5)
6. **Remove `runCLICommand` from `helpers_test.go`**
7. **Restore/rewrite `main_test.go` routing tests** for cobra (all the skipped tests)
8. **Add shell completion command** (Cobra built-in): `rootCmd.AddCommand(rootCmd.GenBashCompletionCmd())` or use `cobra.GenCompletion`
9. `go mod tidy` — urfave/cli drops out automatically
10. Quality gate (in order): `go build ./...` → `golangci-lint run` (fix all issues) → `go test ./...`

---

## Decoupling summary

| Repeated pattern | Files affected | Solution |
|-----------------|---------------|---------|
| `--path` flag definition | 22 files | `addPathFlag(cmd)` in flags.go |
| `--remote` + `--token` flags | 6 files | `addGitHubFlags(cmd)` in flags.go |
| `--format` flag | 4 files | `addFormatFlag(cmd, default)` in flags.go |
| `--collection` flag | 6 files | `addCollectionFlag(cmd, required)` in flags.go |
| `--path`+`--views`+`--records-delimiter` | 2 files | `addMaterializeFlags(cmd)` in flags.go |
| `ci.go` copies materialize flags+action | 1 file | Extract `materializeRunE()` (Batch 2) |
| `resolveDBPath` sig → cobra | 7 callers | `resolveDBPathCobra` in cobra_helpers.go |
| `resolveRecordContext` sig → cobra | 4 callers | `resolveRecordContextCobra` in cobra_helpers.go |
| `githubToken` sig → cobra | 3 callers | `githubTokenCobra` in cobra_helpers.go |
