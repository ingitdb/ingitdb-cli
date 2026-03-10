# Cobra Migration — Lessons Learned

This file is a living log. **Every agent must append their section before reporting done.**

Use this template:
```
## [Phase X / Batch N] — <agent type> — <brief description>
- Unexpected: <anything that differed from the guide>
- API gaps: <cobra API not covered in the guide>
- Deviations: <any deviation from the plan and why>
- Guide updates made: <what was added/fixed in cobra-migration-guide.md>
- Gotchas for future agents: <anything the next agent should know>
```

If the guide needed updating, also edit `spec/plans/cobra-migration-guide.md` directly.

---

<!-- Agents append below this line -->

## [Phases 0-2 / Full Migration] — go-engineer — Complete urfave/cli → cobra migration

- **Unexpected**: The spec called for `cobra_helpers.go` functions named with `Cobra` suffix
  (`resolveDBPathCobra`, `resolveRecordContextCobra`, etc.) to be renamed in Phase 2, but
  since we did the full migration in one pass, we wrote them with the final names
  (`resolveDBPath`, `resolveRecordContext`, etc.) directly in `cobra_helpers.go`, avoiding
  the rename step entirely.

- **API gaps**:
  - cobra's `cmd.Name` is a **method** (`cmd.Name()`) not a field — all test files using it as
    a field needed updating to `cmd.Name()` in format strings, or `cmd.Use` for comparisons.
  - cobra's `cmd.Commands()` is a **method** not a field — test files accessing `.Commands`
    needed `cmd.Commands()`.
  - `cobra.Command.RunE` is the analog for `cli.Command.Action` — tests checking `cmd.Action == nil`
    became `cmd.RunE == nil`.
  - cobra's `--help` returns nil (no error, no exit) — `TestRun_ExitCoderWithZeroCode` was updated
    to reflect this (no fatal/exit called for --help).
  - `StringSliceFlag` must become `cmd.Flags().StringArrayP(...)` — use `StringArray`, never
    `StringSlice` (which splits on commas). The query command's `-w`/`--where` flag is the main example.
  - `cmd.IsSet("x")` → `cmd.Flags().Changed("x")` for checking if a flag was explicitly set.

- **Deviations**:
  - Skipped the phased approach (Phase 0 stub → Phase 1 convert → Phase 2 wire). Instead did a
    direct full migration in one pass since the test suite could be updated atomically.
  - The `cobra_helpers.go` file retains the helper functions (resolveDBPath, resolveRecordContext,
    resolveLocalRecordContext, githubToken) rather than moving them back into their original files
    (validate.go and record_context.go) as the spec suggested for Phase 2 cleanup. This is a
    valid final state since the file is small and well-named.
  - `TestRun_ExitCoderWithNonZeroCode` was updated to check `fatalCalled || exitCalled` since cobra
    silences errors itself (SilenceErrors=true) and calls fatal via ExecuteContext error return.

- **Guide updates made**: None — the guide was accurate for the migration patterns.

- **Gotchas for future agents**:
  1. `TestServe_HTTPBranch` and `TestServe_MCPBranch` need a **cancelled context** to prevent the
     serve loop from blocking indefinitely. Use `root.ExecuteContext(cancelledCtx)` instead of
     `runCobraCommand(cmd, ...)` (which uses `context.Background()`).
  2. The `runCobraCommand` test helper uses `context.Background()`. For commands that block on
     context cancellation (serve, watch), create a pre-cancelled context and call
     `root.ExecuteContext(cancelledCtx)` directly.
  3. `SilenceErrors: true` on the root command means cobra does not print errors to stderr —
     the `fatal` callback is the only place errors surface to the test.
  4. cobra's `MarkFlagRequired` returns an error that must be handled — use `_ = cmd.MarkFlagRequired(...)`.
  5. `cmd.Flags().GetStringArray("x")` returns `([]string, error)` — always assign and ignore the
     error with `vals, _ := cmd.Flags().GetStringArray("x")`.
  6. `go mod tidy` automatically drops urfave/cli after all source references are removed. Check
     that test files don't sneak in a stale `"github.com/urfave/cli/v3"` import.

