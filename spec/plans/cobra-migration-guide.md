# Cobra Migration — Technical Reference for Agents

This document is the **single source of truth** for agents performing the urfave/cli → Cobra migration.
Read it fully before writing any code.

**Update this document** if you discover API differences, edge cases, or patterns not covered here.
**Append lessons** to `spec/plans/cobra-migration-lessons.md` when you finish your batch.

---

## 1. Imports

```go
// Remove:
import "github.com/urfave/cli/v3"

// Add:
import "github.com/spf13/cobra"
```

Both packages coexist in go.mod during Phase 1 — do not run `go mod tidy` in your batch.

---

## 2. Command structure mapping

### Leaf command (no subcommands)

**Before (urfave/cli):**
```go
func MyCmd(dep string) *cli.Command {
    return &cli.Command{
        Name:    "mycmd",
        Aliases: []string{"m"},
        Usage:   "does something useful",
        Flags: []cli.Flag{
            &cli.StringFlag{Name: "path", Usage: "..."},
            &cli.StringFlag{Name: "id", Required: true, Usage: "..."},
        },
        Action: func(ctx context.Context, cmd *cli.Command) error {
            id := cmd.String("id")
            _ = id
            return nil
        },
    }
}
```

**After (Cobra):**
```go
func MyCmd(dep string) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "mycmd",
        Aliases: []string{"m"},
        Short: "does something useful",
        RunE:  func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            id, _ := cmd.Flags().GetString("id")
            _ = ctx
            _ = id
            return nil
        },
    }
    addPathFlag(cmd)
    cmd.Flags().String("id", "", "...")
    _ = cmd.MarkFlagRequired("id")
    return cmd
}
```

### Parent command (has subcommands)

**Before:**
```go
func Parent(...) *cli.Command {
    return &cli.Command{
        Name:     "parent",
        Aliases:  []string{"p"},
        Usage:    "parent group",
        Commands: []*cli.Command{sub1(...), sub2(...)},
    }
}
```

**After:**
```go
func Parent(...) *cobra.Command {
    cmd := &cobra.Command{
        Use:     "parent",
        Aliases: []string{"p"},
        Short:   "parent group",
    }
    cmd.AddCommand(sub1(...), sub2(...))
    return cmd
}
```

---

## 3. Flag type mapping

| urfave/cli type | Cobra equivalent | Access method |
|----------------|-----------------|---------------|
| `StringFlag` | `cmd.Flags().String("n","default","usage")` | `cmd.Flags().GetString("name")` |
| `StringFlag` + `Aliases` | `cmd.Flags().StringP("n","s","default","usage")` | same |
| `StringFlag` + `Required:true` | `cmd.Flags().String(...)` + `cmd.MarkFlagRequired("n")` | same |
| `StringSliceFlag` (any) | `cmd.Flags().StringArray("n",nil,"usage")` | `cmd.Flags().GetStringArray("name")` |
| `BoolFlag` | `cmd.Flags().Bool("n",false,"usage")` | `cmd.Flags().GetBool("name")` |
| `IntFlag` | `cmd.Flags().Int("n",0,"usage")` | `cmd.Flags().GetInt("name")` |

**Critical**: Always use `StringArray` (not `StringSlice`) for repeatable flags. `StringSlice`
splits on commas, which breaks filters like `--where "field>a,b"`. The codebase used
`DisableSliceFlagSeparator: true` specifically to avoid this — `StringArray` is the Cobra equivalent.

**Always use `_` for the error from `GetString`/`GetBool`/etc** — the error only occurs if the flag
doesn't exist (a programming error caught at startup). Checking the error in every call is noise:
```go
id, _ := cmd.Flags().GetString("id")  // correct
```

**Checking if a flag was explicitly set** (replaces `cmd.IsSet("name")`):
```go
if cmd.Flags().Changed("records-delimiter") {
    v, _ := cmd.Flags().GetInt("records-delimiter")
    // ...
}
```

---

## 4. Context handling

**Before:**
```go
Action: func(ctx context.Context, cmd *cli.Command) error {
    // ctx is passed in
}
```

**After:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context() // extract from cmd
    // use ctx normally
}
```

The root command's context is set via `rootCmd.ExecuteContext(ctx)` in main.go.

---

## 5. Error handling — replacing `cli.Exit`

**Before:**
```go
return cli.Exit("not yet implemented", 1)
return cli.Exit("base ref not provided", 1)
```

**After:**
```go
return fmt.Errorf("not yet implemented")
return fmt.Errorf("base ref not provided. Use --base_ref or set BASE_REF / GITHUB_BASE_REF environment variables")
```

Cobra propagates errors to the root's `Execute()` call. The `fatal()` function in `main.go` calls
`os.Exit(1)` when any error is returned. There is no multi-code exit in the new design.

**Do not** import `github.com/spf13/cobra` for error helpers — just use `fmt.Errorf`.

---

## 6. Shared helpers available after Phase 0

These functions are created in Phase 0. Import path: same package (`commands`).

### `flags.go` — flag registration helpers

```go
addPathFlag(cmd *cobra.Command)
addGitHubFlags(cmd *cobra.Command)               // adds --remote and --token
addFormatFlag(cmd *cobra.Command, defaultValue string)
addCollectionFlag(cmd *cobra.Command, required bool)
addMaterializeFlags(cmd *cobra.Command)          // --path, --views, --records-delimiter
```

### `cobra_helpers.go` — shared resolution helpers

```go
resolveDBPathCobra(cmd *cobra.Command, homeDir, getWd) (string, error)
resolveRecordContextCobra(ctx, cmd *cobra.Command, id, homeDir, getWd, readDefinition, newDB) (recordContext, error)
githubTokenCobra(cmd *cobra.Command) string
```

### `validate.go` — pure path helper (unchanged, no cobra dep)

```go
expandHome(path string, homeDir func() (string, error)) (string, error)
```

---

## 7. Test helper

```go
// Available in helpers_test.go after Phase 0:
func runCobraCommand(cmd *cobra.Command, args ...string) error

// Keep using runCLICommand if your test file still has unconverted test cases (Phase 1 only)
```

Usage:
```go
cmd := MyCmd("arg")
err := runCobraCommand(cmd, "--id=test.items/hello", "--path="+dir)
```

---

## 8. Batch 2 special case — Materialize + CI decoupling

Currently `ci.go` does:
```go
mat := Materialize(...)
return &cli.Command{Flags: mat.Flags, Action: mat.Action}  // borrows directly
```

This pattern does not exist in Cobra (no `.Flags` field to copy, `RunE` is a function value but
copying it would still require duplicating flag registration).

**Solution**: extract a `materializeRunE` function:

```go
// In materialize.go:
func materializeRunE(
    homeDir func() (string, error),
    getWd func() (string, error),
    readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
    viewBuilder materializer.ViewBuilder,
    logf func(...any),
) func(*cobra.Command, []string) error {
    return func(cmd *cobra.Command, _ []string) error {
        // all current materialize Action logic here
        // use resolveDBPathCobra, cmd.Flags().Changed("records-delimiter"), etc.
    }
}

func Materialize(...) *cobra.Command {
    cmd := &cobra.Command{Use: "materialize", Short: "...", RunE: materializeRunE(...)}
    addMaterializeFlags(cmd)
    return cmd
}

// In ci.go:
func CI(...) *cobra.Command {
    cmd := &cobra.Command{Use: "ci", Short: "...", RunE: materializeRunE(...)}
    addMaterializeFlags(cmd)
    return cmd
}
```

---

## 9. Worktree instructions for agents

Your orchestrator has created your worktree. Work **only in your assigned directory**.

```bash
# Verify you are in the right worktree
git branch   # should show feat/cobra-batch-N

# After converting your files — quality gate in order:
go build ./...             # must succeed
golangci-lint run          # fix ALL issues before continuing
go test ./cmd/ingitdb/...  # must pass

# Commit
git add cmd/ingitdb/commands/<your-files>.go
git commit -m "feat(cobra): migrate batch-N — version, pull, setup, ..."
```

Do **not** commit changes to `main.go` — that is Phase 2 work.

---

## 10. `Use` field conventions

Cobra's `Use` field is the command name (like `Name` in urfave/cli). For simple commands:
```go
Use: "version"
```

For commands that take positional args (none in this codebase currently):
```go
Use: "mycmd <arg>"
```

---

## 11. `Short` vs `Long` fields

| urfave/cli | Cobra |
|-----------|-------|
| `Usage` | `Short` (one line, shown in parent help) |
| — | `Long` (multi-line, shown in command's own help) |

All current commands only have `Usage` → map to `Short` only.

---

## 12. Complete worked example — `version.go`

**Before:**
```go
func Version(ver, commit, date string) *cli.Command {
    return &cli.Command{
        Name:  "version",
        Usage: "Print build version, commit hash, and build date",
        Action: func(_ context.Context, _ *cli.Command) error {
            fmt.Printf("ingitdb %s (%s) @ %s\n", ver, commit, date)
            return nil
        },
    }
}
```

**After:**
```go
func Version(ver, commit, date string) *cobra.Command {
    return &cobra.Command{
        Use:   "version",
        Short: "Print build version, commit hash, and build date",
        RunE: func(_ *cobra.Command, _ []string) error {
            fmt.Printf("ingitdb %s (%s) @ %s\n", ver, commit, date)
            return nil
        },
    }
}
```

---

## 13. Complete worked example — `delete_record.go`

**Before:**
```go
func deleteRecord(homeDir, getWd, readDefinition, newDB, logf) *cli.Command {
    return &cli.Command{
        Name:  "record",
        Usage: "Delete a single record by its ID",
        Flags: []cli.Flag{
            &cli.StringFlag{Name: "path", Usage: "..."},
            &cli.StringFlag{Name: "github", Usage: "..."},
            &cli.StringFlag{Name: "token", Usage: "..."},
            &cli.StringFlag{Name: "id", Required: true, Usage: "..."},
        },
        Action: func(ctx context.Context, cmd *cli.Command) error {
            _ = logf
            id := cmd.String("id")
            rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
            if err != nil { return err }
            key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
            err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
                return tx.Delete(ctx, key)
            })
            if err != nil { return err }
            return buildLocalViews(ctx, rctx)
        },
    }
}
```

**After:**
```go
func deleteRecord(homeDir, getWd, readDefinition, newDB, logf) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "record",
        Short: "Delete a single record by its ID",
        RunE: func(cmd *cobra.Command, _ []string) error {
            _ = logf
            ctx := cmd.Context()
            id, _ := cmd.Flags().GetString("id")
            rctx, err := resolveRecordContextCobra(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
            if err != nil { return err }
            key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
            err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
                return tx.Delete(ctx, key)
            })
            if err != nil { return err }
            return buildLocalViews(ctx, rctx)
        },
    }
    addPathFlag(cmd)
    addGitHubFlags(cmd)
    cmd.Flags().String("id", "", "record ID in the format collection/path/key (e.g. todo.tags/ie)")
    _ = cmd.MarkFlagRequired("id")
    return cmd
}
```

---

## 14. Files that need NO changes

These files have zero `*cli.Command` usage:
- `cmd/ingitdb/commands/query_output.go`
- `cmd/ingitdb/commands/query_parser.go`
- `cmd/ingitdb/commands/seams.go`
- `cmd/ingitdb/commands/serve_http.go` ← verify before assuming
- `cmd/ingitdb/commands/serve_mcp.go` ← verify before assuming
- `cmd/ingitdb/commands/view_builder_helper.go`

Verify by running: `grep -l 'urfave/cli' <file>` before wasting time on them.
