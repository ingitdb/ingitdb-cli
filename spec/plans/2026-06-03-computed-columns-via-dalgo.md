# Plan: Computed Columns via dalgo (lazy delegation)

**Status:** Approved
**Source Feature:** computed-columns-via-dalgo
**Date:** 2026-06-03
**Owner:** alex
**Supersedes:** —

## Summary

Decomposes the `computed-columns-via-dalgo` Feature into five linear tasks that
migrate computed-column computation from ingitdb's eager read pipeline onto
dalgo's `recordset.Evaluator` contract: the lazy core first (evaluator adapter +
recordset reader + coerce/fail-loud accessor), then the consumer migrations
(`select` output, `select` filter/sort, then `delete`/`update`/TUI), ending with
removal of the eager `ApplyFormulasToRead` stage. All 11 acceptance criteria are
covered; none are deferred.

## Approach

Tasks follow the dependency chain. Task 1 lands the lazy core — the
Starlark-backed `recordset.Evaluator` adapter and the now-implemented
`ExecuteQueryToRecordsetReader` (adapter and reader are merged because the adapter
has no consumer without the reader). Task 2 adds the shared coerce-on-access
accessor that every consumer reads through, including fail-loud error wrapping.
Tasks 3 and 4 migrate `select` — output projection first, then `--where`/`order_by`
onto the lazy accessor. Task 5 migrates the remaining consumers (`delete`/`update`
set-mode WHERE and the TUI collection screen) and, once no consumer uses the old
reader, removes the eager `ApplyFormulasToRead` read-time stage. The `select`
migration is split (output vs filter/sort) to keep each task focused; the TUI has
no dedicated AC and rides Task 5 under the all-consumers requirement.

## Tasks

### Task 1: Starlark Evaluator adapter and recordset query reader

**Verifies:** computed-columns-via-dalgo#ac:evaluator-invoked-once-per-row, computed-columns-via-dalgo#ac:stored-only-projection-evaluates-nothing

Add a `recordset.Evaluator` (`formulaEvaluator`) that delegates to
`ingitdb.EvaluateFormula`, and implement the stubbed `ExecuteQueryToRecordsetReader`
to materialize a `recordset.Recordset` whose stored columns carry locale-normalized
values and whose computed columns are registered via `recordset.NewComputedColumn`.
Lazy resolution and per-row single evaluation come from the dalgo contract; verify
the evaluator is invoked once per row and never for unreferenced stored-only reads.

### Task 2: Shared coerce-on-access accessor with fail-loud error wrapping

**Verifies:** computed-columns-via-dalgo#ac:type-coercion-preserved, computed-columns-via-dalgo#ac:referenced-erroring-column-fails-loud

Add one accessor that reads a value from a `recordset.Row` and coerces it to the
column's declared `ColumnType` via `coerceFormulaResult`, wrapping any evaluator or
coercion error with the collection, record key, and column. This is the single
read path all consumers will use, keeping typed results identical and errors
fail-loud at access time.

### Task 3: Migrate select output projection onto the recordset reader

**Verifies:** computed-columns-via-dalgo#ac:select-renders-computed, computed-columns-via-dalgo#ac:string-helper-preserved, computed-columns-via-dalgo#ac:math-helper-preserved

Switch `select`'s record consumption from `ExecuteQueryToRecordsReader` to the
recordset reader, reading each projected/output field through the shared accessor,
so computed columns render byte-identically (including Starlark string and numeric
helper results) but are computed lazily per projected column.

### Task 4: Migrate select --where and order_by to the lazy accessor

**Verifies:** computed-columns-via-dalgo#ac:where-on-computed-still-works, computed-columns-via-dalgo#ac:order-by-computed-still-works, computed-columns-via-dalgo#ac:unreferenced-erroring-column-not-evaluated

Move `select`'s `--where` predicate evaluation and `order_by` sorting onto values
obtained through the shared lazy accessor instead of a pre-baked map, so only the
computed columns named in predicates or sort terms are evaluated and an unreferenced
erroring computed column no longer aborts the read.

### Task 5: Migrate delete/update and TUI consumers; remove eager formula bake

**Verifies:** computed-columns-via-dalgo#ac:delete-where-on-computed

Migrate `delete`/`update` set-mode WHERE and the TUI collection screen (visible-cell
reads only) onto the recordset reader via the shared accessor, then remove the
eager `ApplyFormulasToRead` read-time stage now that no consumer uses the old
records reader for computed values.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/plan-specification*
