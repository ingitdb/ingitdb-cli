---
type: sidekick-seed
captured_by: claude
status: done
---
# Make `ingitdb rebase` run `git rebase --abort` and report unresolved paths on conflicts outside the --resolve scope, with a test

Gap found while reconciling cli/rebase (1/2 ACs). `AC:aborts-on-source-conflict` (REQ:aborts-on-unresolvable) says the command MUST abort the rebase when a conflict falls outside `--resolve`, but `Rebase()` in `cmd/ingitdb/commands/rebase.go` never calls `git rebase --abort` — on an unresolved/source conflict it returns an error and leaves the rebase halted in progress. No test feeds a non-README (source) conflict to verify the abort + unresolved-paths report. The underlying detection (`FindCollectionsForConflictingFiles`, `resolveGeneratedConflicts` returning `unresolved`) is unit-tested, but the command's abort/report path is not. Also the error text says "files other than README.md" even though `--resolve` is generically categorized — reword.
