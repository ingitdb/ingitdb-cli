# Feature: Computed Columns via dalgo (lazy delegation)

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/computed-columns-via-dalgo?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/computed-columns-via-dalgo?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/computed-columns-via-dalgo?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/computed-columns-via-dalgo?op=request-change) |
**Status:** Approved
**Date:** 2026-06-03
**Owner:** alex
**Source Ideas:** —
**Supersedes:** —

## Summary

Moves computed-column (FORMULA) value computation out of ingitdb's eager read
pipeline and onto dalgo's `recordset.Evaluator` contract (dalgo v0.46.0). ingitdb
keeps owning the Starlark language and schema validation, but delegates the
*computation wiring* to dalgo so computed values are resolved **lazily, per
accessed column**, through `recordset.Row`. It serves ingitdb users by avoiding
needless formula evaluation — a computed column is only computed when a consumer
actually reads it.

## Problem

Today every read goes through `executeQueryToRecordsReader`, which calls
`ApplyFormulasToRead` to evaluate **every** computed column for **every** record
and bake the results into a `map[string]any`. `select`, `delete`, `update`, and
the TUI then read that map. This is wasteful: `delete`/`update` only need a
computed column referenced by a `--where` predicate (and render none); the TUI
only needs cells it actually paints; `select` only needs projected, filtered, and
sorted columns. Heavy Starlark evaluation runs for columns no consumer reads.

dalgo v0.46.0 ships a neutral computed-column contract (`recordset.Evaluator`,
`NewComputedColumn`, lazy memoized resolution in `Row`). This Feature adopts it:
ingitdb supplies a Starlark-backed `Evaluator`, materializes query results as a
`recordset.Recordset`, and routes all read consumers through lazy `Row` access —
so a formula runs only when, and only as often as, its column is read.

## Behavior

### Delegating computation to dalgo

ingitdb stops evaluating formulas eagerly and instead hands dalgo an opaque
evaluator plus a recordset whose computed columns are marked as such.

#### REQ: starlark-evaluator-adapter

ingitdb MUST provide a `recordset.Evaluator` implementation that computes a
computed column's value by delegating to the existing
`ingitdb.EvaluateFormula(formula, stored)`. dalgo MUST NOT parse, compile, or
sandbox anything; the Starlark engine, its sandbox, and schema-load-time
validation remain entirely in ingitdb.

#### REQ: recordset-query-path

The previously stubbed `ExecuteQueryToRecordsetReader` MUST be implemented to
return a `dal.RecordsetReader` whose `recordset.Recordset` registers each stored
(non-computed) column as an ordinary column carrying the record's
locale-normalized stored value, and each computed column via
`recordset.NewComputedColumn` bound to the Starlark-backed `Evaluator`.

#### REQ: remove-eager-bake

The read path MUST NOT eagerly evaluate formulas into the record map: computed
values are produced only through `recordset.Row` access. `ApplyFormulasToRead`
MUST no longer run as a read-time stage. (Write-time foreign-key evaluation is
out of scope and unchanged — see Not Doing.)

### Lazy, per-reference computation

The point of the migration: never compute a computed column a consumer does not
read.

#### REQ: compute-only-on-reference

A computed column's formula MUST be evaluated only when a consumer reads that
column — via output projection, a `--where` predicate, an `order_by` term, or a
painted TUI cell. A computed column that no consumer references in a given
operation MUST NOT be evaluated.

#### REQ: compute-once-per-row

Within a single `recordset.Row` instance, a computed column MUST be evaluated at
most once regardless of how many times it is read (relying on dalgo's per-row
memoization); ingitdb's access path MUST NOT defeat that memoization by
reconstructing rows mid-operation.

### Preserving observable behavior

The migration is behavior-preserving for referenced columns; only the timing of
computation (and of error surfacing) changes.

#### REQ: coerce-on-access

A value read from a `recordset.Row` MUST be coerced to the column's declared
`ColumnType` (via the existing `coerceFormulaResult`) before it is used in a
comparison, a sort, or output. All read consumers MUST obtain values through one
shared accessor that performs this coercion, so typed results stay identical to
the eager pipeline.

#### REQ: fail-loud-on-access

If a formula raises a runtime error, or its result cannot be coerced, the error
MUST abort the operation when the column is accessed, with an error identifying
the collection, record key, and column (the shared accessor wraps dalgo's
evaluator error with that context). Because computation is lazy, this Feature
narrows `computed-columns#ac:runtime-error-fails-read`: an erroring computed
column that is never referenced in an operation MUST NOT abort that operation.

#### REQ: where-and-order-lazy

`--where` and `order_by` MUST continue to support computed columns, with their
values now obtained through the shared lazy accessor rather than a pre-baked map,
so only the computed columns named in the predicate or sort terms are evaluated.

#### REQ: all-consumers-via-recordset

`select`, `delete`, `update`, and the TUI collection screen MUST obtain query
results via `ExecuteQueryToRecordsetReader` and read every field value through the
shared coerce-on-access accessor. The TUI MUST read only the cells it renders, so
hidden or off-viewport computed columns are not evaluated.

## Acceptance Criteria

### AC: select-renders-computed (verifies REQ:starlark-evaluator-adapter, REQ:recordset-query-path)

**Given** a collection `people` with a `string` column `full_name` declaring `formula: 'first_name + " " + last_name'` and a record `first_name: "Ada"`, `last_name: "Lovelace"`
**When** the record is read via `select`
**Then** the returned `full_name` equals `"Ada Lovelace"`

### AC: type-coercion-preserved (verifies REQ:coerce-on-access)

**Given** an `int` column `total` with `formula: 'qty * price'` and a record with `qty: 3`, `price: 4`
**When** the record is read via `select`
**Then** `total` equals integer `12` (not a float or other type)

### AC: string-helper-preserved (verifies REQ:recordset-query-path, REQ:starlark-evaluator-adapter)

**Given** a `string` column `display` with `formula: 'first_name.strip().upper()'` and a record with `first_name: " ada "`
**When** the record is read via `select`
**Then** `display` equals `"ADA"`

### AC: math-helper-preserved (verifies REQ:coerce-on-access, REQ:starlark-evaluator-adapter)

**Given** an `int` column `rounded` with `formula: 'round(score)'` and a record with `score: 4.6`
**When** the record is read via `select`
**Then** `rounded` equals integer `5`

### AC: where-on-computed-still-works (verifies REQ:where-and-order-lazy, REQ:all-consumers-via-recordset)

**Given** the `people` collection with computed `full_name`
**When** `select` runs with `--where 'full_name == "Ada Lovelace"'`
**Then** only records whose computed `full_name` equals `"Ada Lovelace"` are returned

### AC: order-by-computed-still-works (verifies REQ:where-and-order-lazy)

**Given** the `people` collection with computed `full_name`
**When** `select` runs with `order_by full_name asc`
**Then** the returned records are ordered by their computed `full_name`

### AC: delete-where-on-computed (verifies REQ:all-consumers-via-recordset, REQ:where-and-order-lazy)

**Given** the `people` collection with computed `full_name` and a `delete` in set mode
**When** `delete` runs with `--where 'full_name == "Ada Lovelace"'`
**Then** exactly the records whose computed `full_name` equals `"Ada Lovelace"` are deleted

### AC: unreferenced-erroring-column-not-evaluated (verifies REQ:compute-only-on-reference, REQ:fail-loud-on-access)

**Given** a collection with a stored column `qty` and a computed `int` column `ratio` whose `formula: 'qty / 0'` raises at runtime
**When** `select` runs projecting only `qty` (with no `--where`/`order_by` referencing `ratio`)
**Then** the operation succeeds and returns `qty`, and the read does not abort with the `ratio` runtime error

### AC: referenced-erroring-column-fails-loud (verifies REQ:fail-loud-on-access)

**Given** the same collection with computed `int` column `ratio` whose `formula: 'qty / 0'`
**When** `select` runs projecting `ratio` (or with `--where` referencing it)
**Then** the operation aborts with an error naming the collection, record key, and the `ratio` column

### AC: stored-only-projection-evaluates-nothing (verifies REQ:remove-eager-bake, REQ:compute-only-on-reference)

**Given** a `recordset.Recordset` built for a record with a stored column `qty` and a computed column whose `Evaluator` increments a counter
**When** a consumer reads only `qty` (no projection, `--where`, or `order_by` references the computed column)
**Then** the counter equals `0` — the computed column's formula is never evaluated (no eager bake)

### AC: evaluator-invoked-once-per-row (verifies REQ:compute-once-per-row)

**Given** a `recordset.Recordset` built for a record with a computed column whose `Evaluator` increments a counter
**When** the same `Row` instance's computed value is read by both a `--where` predicate and output projection
**Then** the counter equals `1` for that row

## Architecture & Components

| Unit | What it does | Depends on |
|------|--------------|-----------|
| `formulaEvaluator` (new, `pkg/dalgo2ingitdb`) | Implements `recordset.Evaluator`; `Eval(stored)` → `ingitdb.EvaluateFormula(formula, stored)`. | `ingitdb.EvaluateFormula`, `recordset.Evaluator` |
| `ExecuteQueryToRecordsetReader` (implement stub, `tx_readonly.go`) | Materializes a `recordset.Recordset` per query: stored columns from locale-normalized fields, computed columns via `NewComputedColumn`; returns a `dal.RecordsetReader`. | `recordset`, `ColumnDef`, `ApplyLocaleToRead`, read-from-disk path |
| shared value accessor (new) | `value(row, rs, colDef)`: `row.GetValueByName` → `coerceFormulaResult(v, colDef.Type)`; wraps evaluator errors with collection/key/column. | `recordset.Row`, `coerceFormulaResult` |
| `select`/`delete`/`update`/TUI consumers | Switched from `ExecuteQueryToRecordsReader` to the recordset reader; read via the shared accessor; WHERE/ORDER BY evaluate through it. | recordset reader, shared accessor |

## Data Flow

1. Query resolves the collection; records are read from disk and locale-normalized
   (`ApplyLocaleToRead`) into stored fields — unchanged.
2. A `recordset.Recordset` is built: one column per stored field (value set), one
   `NewComputedColumn` per formula column bound to a `formulaEvaluator`.
3. Consumers pull rows from the `dal.RecordsetReader`. Any value read (WHERE,
   ORDER BY, output cell, TUI cell) goes through the shared accessor →
   `Row.GetValueByName` → (lazy) `Evaluator.Eval(stored siblings)` → coerce to
   declared type. dalgo memoizes per row; unreferenced computed columns are never
   evaluated.

## Error Handling & Failure Modes

- **Formula runtime error / coercion failure** → surfaced at access via the shared
  accessor, wrapped with collection + record key + column; aborts the operation
  (REQ:fail-loud-on-access). Never panics.
- **Unreferenced erroring computed column** → not evaluated, no error
  (REQ:compute-only-on-reference) — the deliberate narrowing of the eager
  fail-loud behavior.
- **Unknown column in `--where`/`order_by`/projection** → unchanged from today.

## Testing Strategy

Go tests across two surfaces: (1) `pkg/dalgo2ingitdb` unit tests for the evaluator
adapter, the recordset reader, the shared accessor (coercion + error wrapping +
invoked-once), using a counter-instrumented evaluator; (2) CLI-level tests
exercising `select`/`delete` against fixtures for the read, coercion, helper,
where/order, lazy-skip, and fail-loud ACs. Existing `computed-columns` ACs
(read-output, filter, sort) are re-run to confirm byte-identical results.

## Rehearse Integration

Every AC has a concrete surface (CLI `select`/`delete` output/errors, or
pure-function `recordset`-construction tests), so one Rehearse Scenario stub per
AC is scaffolded under `_tests/`. `evaluator-invoked-once-per-row` is a
pure-function/unit surface; the TUI viewport behavior is covered indirectly via
`compute-only-on-reference` (no dedicated TUI-render stub in this Feature).

## Not Doing / Out of Scope

- Changing the Starlark language, helper set, or schema-load-time validation —
  all remain in ingitdb unchanged.
- Write-time foreign-key evaluation and the computed-FK referential-integrity
  paths — they keep their current evaluation; only the read path migrates.
- Adding new computed-column capabilities (chained computed columns, new types) —
  behavior is preserved, not extended.
- Pushing coercion into dalgo — coercion stays on the ingitdb consumer side
  (dalgo returns raw values by contract).
- The `starlark4dalgo` shared-engine-library extraction — tracked separately.

## Assumption Carryover

From the dalgo-side Idea `recordset-computed-columns` (Phase A, shipped in
v0.46.0):

- **Validated / relied upon:** `recordset.Row` is the carrier; the evaluator
  receives the full stored-field map; lazy + per-row memoized; dalgo carries no
  scripting dependency; `EvaluateFormula(formula, fields)` already matches the
  `Eval(stored)` shape.
- **Now decided (was open):** the fail-loud timing shift is **accepted** — errors
  surface on access, and unreferenced erroring columns do not abort
  (REQ:fail-loud-on-access). This narrows `computed-columns#ac:runtime-error-fails-read`.
- **Coercion:** stays consumer-side (REQ:coerce-on-access), consistent with
  dalgo's no-coercion contract.

## Open Questions

- Resolved in this work: `computed-columns#ac:runtime-error-fails-read` and its
  `REQ:fail-loud` prose were amended to match REQ:fail-loud-on-access — the abort
  fires only when the computed column is referenced.

---
*This document follows the https://specscore.md/feature-specification*
