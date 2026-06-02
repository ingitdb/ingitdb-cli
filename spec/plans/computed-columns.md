# Plan: Computed Columns (Inline Starlark Formulas)

**Status:** Implementing
**Source Feature:** computed-columns
**Date:** 2026-06-02
**Owner:** alexander.trakhimenok@gmail.com
**Supersedes:** —

## Summary

Decomposes the `computed-columns` Feature into seven linear tasks that build the
inline-Starlark formula capability bottom-up: schema and validation first, then the
sandboxed evaluator, then read-time integration, and finally query and foreign-key
integration. All 16 acceptance criteria are covered; none are deferred.

## Approach

Tasks follow the natural dependency chain in the codebase. Task 1 introduces the
`formula` attribute and all static (load-time) validation. Task 2 builds the
evaluation engine and its deterministic helper set in isolation so it can be unit
tested without the read path. Task 3 wires evaluation into the existing
`ApplyLocaleToRead` read stage, adding coercion and fail-loud behavior. Task 4 guards
the write/validate path against stored values. Task 5 confirms computed columns flow
through `--where`/`order_by` (which work for free once evaluation precedes query ops,
but need their own tests). Tasks 6 and 7 extend the existing write-time
referential-integrity engine to computed foreign keys — child side first, then the
parent-side delete/rename scan. ACs are grouped by the unit of implementation work,
never split into AC-wrapper tasks.

**Testing standard (every task).** No task is complete until the code it introduces
has 100% test coverage, verified with `go test -coverprofile`. Tests must exercise the
task's acceptance criteria plus every error path and branch in the new code; any line
that is genuinely unreachable must be refactored away rather than left uncovered. This
bar applies to all seven tasks.

## Tasks

### Task 1: Add `formula` attribute and load-time validation

**Status:** done
**Verifies:** computed-columns#ac:formula-syntax-error, computed-columns#ac:unsupported-type-rejected, computed-columns#ac:reject-chained-computed-reference

Add a `formula` string field to `ColumnDef` and extend schema validation: the formula
must parse as a single Starlark expression (introduces the `google/starlark-go`
dependency), a statement body or parse error is a validation error naming collection
and column, a computed column declared with a type outside `{string, int, float,
bool, any}` is rejected, and a formula that references another computed column is
rejected as a stored-fields-only violation.

### Task 2: Sandboxed deterministic evaluator and builtin helpers

**Status:** done
**Verifies:** computed-columns#ac:deterministic-evaluation, computed-columns#ac:builtin-string-helper-available, computed-columns#ac:builtin-math-helper-available

Build the formula evaluation function: evaluate a parsed Starlark expression with the
record's stored fields bound as variables, in a sandbox exposing no network,
filesystem, clock, or randomness, guaranteeing identical output for identical input.
Expose the curated deterministic helper set — native Starlark string methods,
`len`/`min`/`max`, and numeric `abs`/`round`/`floor`/`ceil` — and confirm no
non-deterministic or I/O-capable builtin or module is reachable.

### Task 3: Compute-on-read integration, type coercion, and fail-loud errors

**Status:** done
**Verifies:** computed-columns#ac:formula-declared-and-computed, computed-columns#ac:type-coercion-success, computed-columns#ac:runtime-error-fails-read

Invoke the evaluator in the read path alongside `ApplyLocaleToRead` so every read
record gains its computed column values, coerced to the declared `ColumnType`. A
runtime error or a result that cannot coerce aborts the read with an error naming
collection, record key, column, and cause — no partial row, no silent null.

### Task 4: Reject stored values for computed columns

**Status:** done
**Verifies:** computed-columns#ac:reject-stored-computed-value

On `insert`/`update` and during file validation, reject any record that supplies a
value for a computed column, with an error naming the collection, record key, and
column. The computed value remains the sole source of truth.

### Task 5: Computed columns usable in `--where` and `order_by`

**Status:** done
**Verifies:** computed-columns#ac:filter-on-computed-column, computed-columns#ac:order-by-computed-column

Confirm and test that, because evaluation precedes query operations, computed columns
filter under `--where` predicates and sort under `order_by` exactly as stored columns
do; close any gap where filtering/sorting reads pre-evaluation record state.

### Task 6: Write-time foreign-key enforcement for computed columns (child side)

**Status:** done
**Verifies:** computed-columns#ac:foreign-key-on-insert-violation, computed-columns#ac:foreign-key-revalidates-on-input-change

Extend the existing write-time referential-integrity path: on `insert`/`update`,
evaluate a computed `foreign_key` column from the payload and validate the derived
value against the referenced collection, raising a referential-integrity error in the
reference-error shape (referencing collection, record key, computed column, referenced
collection). Re-run the check for every computed foreign key on every update, since a
computed foreign-key column is never written directly.

### Task 7: Parent-side foreign-key enforcement (delete and rename)

**Status:** done
**Verifies:** computed-columns#ac:foreign-key-parent-delete-detected, computed-columns#ac:foreign-key-parent-rename-detected

When a referenced record is deleted or its key renamed, scan the referencing
collection and recompute each computed foreign key (full scan, no index) to find
records that resolve to the affected key, blocking the operation with a
referential-integrity error in the reference-error shape.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/plan-specification*
