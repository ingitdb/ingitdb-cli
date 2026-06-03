---
type: sidekick-seed
slug: implement-incremental-validation-commit-range
captured_at: 2026-06-03T14:25:20Z
captured_by: claude
captured_during: null
trigger: explicit
status: queued
synchestra_task: null
---
# Implement IncrementalValidator so `validate --from-commit/--to-commit` scopes validation to changed records instead of returning not-implemented

Gap found while reconciling cli/validate (2/3 ACs). `AC:scoped-validation` (commit-range, `--only=records --from-commit=A --to-commit=B`) is not functionally implemented: the `IncrementalValidator` interface (`pkg/ingitdb/datavalidator/interfaces.go`) has no production implementation, and `cmd/ingitdb/main.go` wires it as `nil`, so the real CLI returns "incremental validation (--from-commit/--to-commit) is not yet implemented". The "records outside the commit range are not opened" guarantee is unsatisfiable today. Tests only exercise a `mockIncrementalValidator`, so command plumbing is covered but the actual git-diff-driven behavior is not. Implement a real IncrementalValidator (diff the commit range, validate only changed record files) and test against real git/file logic. Note: the commit-range branch currently runs before `--only` scoping and ignores record-vs-definition selection.
