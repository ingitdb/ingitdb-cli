# Plan: Computed Columns via dalgo (lazy delegation)

**Status:** Implemented
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

**Scope note (two packages).** The CLI consumers (`select`/`delete`/`update`/TUI)
run through `pkg/dalgo2fsingitdb` (the filesystem driver wired in `main.go` via
`NewLocalDBWithDef`), which delegates parsing/locale/formula/validation to
`pkg/dalgo2ingitdb`. Both packages carry their own readonly-tx with a
`ExecuteQueryToRecordsetReader` stub (`dalgo2ingitdb` returns `dal.ErrNotSupported`;
`dalgo2fsingitdb` panics) and their own eager `ApplyFormulasToRead` call sites.
To deliver the Feature's observable goal, the reusable pieces — the Starlark
`recordset.Evaluator` adapter, the recordset builder, the recordset reader, and the
shared coerce-on-access accessor — live in `pkg/dalgo2ingitdb`, and
`pkg/dalgo2fsingitdb`'s recordset method plus the CLI consumers are wired to them;
the eager bake is removed from both query paths. Task 1 implements both packages'
`ExecuteQueryToRecordsetReader`; Task 5 removes the eager bake from both.

**Implementation note (eager-bake retained for write-time FK).** During Task 5
the records-reader eager bake (`ApplyFormulasToRead` via `bakeStoredRecords`) was
found to also back **write-time computed-foreign-key validation** (`Set`/`Delete`
referential-integrity checks read child records and must see computed FK values).
Write-time FK evaluation is explicitly out of scope and unchanged (Feature → Not
Doing), so the bake is *not* deleted; instead all four read consumers were
migrated off the records reader onto the lazy recordset reader, so the **read
path** no longer eagerly evaluates formulas (`REQ:remove-eager-bake`). The
records-reader bake now serves only the out-of-scope write-time FK path.

**Implementation note (TUI laziness).** The TUI collection screen scans all
records × columns for column-width sizing, locale discovery, and numeric
detection, so strict per-visible-cell laziness conflicts with its current
architecture. The TUI was migrated onto the recordset reader + shared accessor
(byte-identical to prior behavior); true per-cell laziness is deferred as a
follow-up. The data-layer `REQ:compute-only-on-reference` is satisfied and
unit-verified via the recordset.

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
