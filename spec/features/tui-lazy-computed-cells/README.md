# Feature: TUI lazy computed-cell evaluation

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/tui-lazy-computed-cells?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/tui-lazy-computed-cells?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/tui-lazy-computed-cells?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/tui-lazy-computed-cells?op=request-change) |
**Status:** Approved
**Date:** 2026-06-03
**Owner:** alex
**Source Ideas:** —
**Supersedes:** —
**Grade:** A

## Summary

Makes the TUI collection screen evaluate a computed (FORMULA) column only for
the cells it actually paints — the visible row × column window — so hidden and
off-viewport computed columns are never evaluated. It serves ingitdb users
browsing collections with expensive computed columns by keeping the screen
responsive regardless of row count.

## Problem

`computed-columns-via-dalgo` moved read consumers onto dalgo's lazy
`recordset.Evaluator`, but the TUI collection screen was migrated
byte-identically: `loadRecordsCmd` pre-builds a full `map[string]any` per record
(reading every column), and `computeColWidths`/`numericCol` scan all rows ×
columns. So every computed column is evaluated up front for every record, even
ones never scrolled into view — exactly what `computed-columns-via-dalgo`'s
`REQ:all-consumers-via-recordset` says the TUI must avoid ("The TUI MUST read
only the cells it renders, so hidden or off-viewport computed columns are not
evaluated"). Captured as seed
`spec/ideas/seeds/tui-collection-screen-should-evaluate-only-painted-visible.md`.

The screen already virtualizes row rendering to the visible window; the parts
that force full computed evaluation are (1) the pre-built per-record maps, (2)
column-width sizing across all rows, and (3) numeric-alignment sampling of the
first row. Locale discovery and stored-column sizing read only stored values, so
they never trigger formula evaluation and stay as they are.

## Behavior

### Lazy computed-cell evaluation

The collection screen holds the recordset and its row handles and resolves
computed values at paint time through the shared coerce-on-access accessor.

#### REQ: model-holds-recordset

The collection screen model MUST hold the query's `recordset.Recordset` and its
per-record row handles (plus record keys), rather than pre-evaluated
`map[string]any` records for computed columns. `loadRecordsCmd` MUST NOT eagerly
evaluate computed columns at load time.

#### REQ: render-time-computed-eval

A computed column's value MUST be produced only when its cell is painted — i.e.
the record is within the visible row window AND the column is within the visible
column window. A computed column for an off-viewport row or an off-viewport
column MUST NOT be evaluated.

#### REQ: per-row-memoization

The model MUST retain the recordset row handles across re-renders so that
dalgo's per-row memoization holds: a given record's computed cell is evaluated at
most once regardless of how many times it is repainted (re-render, scroll away
and back). The model MUST NOT reconstruct row handles per render frame.

### Sizing and layout without forcing evaluation

Computed columns must not be evaluated merely to lay out the table.

#### REQ: computed-width-from-schema

A computed (FORMULA) column's display width MUST be derived from its header label
and declared `ColumnType`/length, without sampling any row's value. Width sizing
MUST NOT evaluate a computed column. (Long computed values may therefore be
truncated to that width with the existing ellipsis behavior.)

#### REQ: numeric-alignment-from-type

A computed column's numeric (right) alignment MUST be determined from its
declared `ColumnType` (numeric types align right), not by sampling a record's
value. Determining alignment MUST NOT evaluate a computed column.

#### REQ: stored-columns-unchanged

Stored (non-computed) columns MUST retain today's behavior: column-width sizing,
locale discovery (the locale dropdown contents), and numeric-alignment detection
MAY scan all records' stored values. Reading stored values never triggers
formula evaluation, so these scans stay full and observably unchanged.

### Error surfacing

Because evaluation is lazy, a formula error can only occur for a painted cell.

#### REQ: visible-error-non-fatal

If a painted computed cell's evaluation (or coercion) errors, the cell MUST
render a bounded error indicator and the collection screen MUST continue to
render its other rows and columns — the error MUST NOT abort the screen or crash
the TUI. An off-viewport erroring computed column is never evaluated and
therefore MUST NOT affect the screen.

## Acceptance Criteria

### AC: off-viewport-rows-not-evaluated (verifies REQ:model-holds-recordset, REQ:render-time-computed-eval)

**Given** a collection with a computed column bound to a counting evaluator and more records than fit the visible row window of height `V`
**When** the collection screen renders without scrolling
**Then** the evaluator is invoked only for the computed cells of the `V` visible records, and zero times for the off-viewport records

### AC: scroll-evaluates-only-newly-visible (verifies REQ:render-time-computed-eval, REQ:per-row-memoization)

**Given** the same collection rendered once at the top
**When** the user scrolls so a previously off-viewport record becomes visible and a previously visible record is repainted
**Then** the newly-visible record's computed cell is evaluated, and the repainted record's computed cell is not evaluated a second time

### AC: width-sizing-does-not-evaluate (verifies REQ:computed-width-from-schema)

**Given** a collection with a computed column bound to a counting evaluator
**When** the screen computes column widths
**Then** the evaluator is not invoked during width computation, and the computed column's width equals the width derived from its header label and declared type

### AC: numeric-alignment-does-not-evaluate (verifies REQ:numeric-alignment-from-type)

**Given** an `int` computed column bound to a counting evaluator
**When** the screen determines column alignment
**Then** the column is right-aligned (numeric) and the evaluator is not invoked to decide alignment

### AC: stored-locale-discovery-unchanged (verifies REQ:stored-columns-unchanged)

**Given** a collection with an L10N stored column whose locale keys appear only on records outside the visible window
**When** the locale dropdown is built
**Then** every locale present across all records — including off-viewport ones — appears in the dropdown, exactly as before this Feature

### AC: visible-computed-error-non-fatal (verifies REQ:visible-error-non-fatal)

**Given** a collection with a computed column whose formula raises at runtime, positioned within the visible window
**When** the collection screen renders
**Then** the erroring cell shows a bounded error indicator and the screen still renders its other cells without crashing or aborting the load

### AC: off-viewport-error-never-evaluated (verifies REQ:render-time-computed-eval, REQ:visible-error-non-fatal)

**Given** a collection with a computed column whose formula raises at runtime, positioned only on off-viewport records
**When** the collection screen renders
**Then** the screen renders normally and the erroring computed column is never evaluated

## Architecture & Components

| Unit | What it does | Depends on |
|------|--------------|-----------|
| `collectionModel` (refactor, `cmd/ingitdb/tui`) | Holds `recordset.Recordset` + row handles + keys instead of `[]map[string]any` for computed values. | `dalgo2ingitdb` recordset reader, `AccessValue` |
| `loadRecordsCmd` (refactor) | Returns the recordset + row handles; stops pre-evaluating computed columns. | `ExecuteQueryToRecordsetReader` |
| `cellValue` (refactor) | Resolves a cell at paint time via `dalgo2ingitdb.AccessValue` (stored = pass-through, computed = lazy evaluate+coerce). | `AccessValue` |
| `computeColWidths` / numeric/locale helpers (refactor) | Computed columns sized/aligned from schema; stored columns keep scanning stored values. | `ColumnDef`, recordset stored columns |

## Data Flow

1. `loadRecordsCmd` opens `ExecuteQueryToRecordsetReader`, collects the row
   handles + keys + the recordset, and hands them to the model (no computed
   evaluation).
2. On render, the panel paints the visible row × column window; each painted
   cell calls `cellValue` → `AccessValue` (computed columns evaluate lazily and
   memoize per row; stored columns pass through).
3. Width/alignment for computed columns come from the schema; for stored columns
   from the existing stored-value scans. Locale discovery scans stored L10N
   columns across all records (no formula evaluation).

## Error Handling & Failure Modes

- **Painted computed cell errors** → bounded error indicator in that cell; screen
  keeps rendering (REQ:visible-error-non-fatal). No load abort.
- **Off-viewport computed column errors** → never evaluated, no effect.
- **Stored-value read** → unchanged; cannot trigger formula evaluation.

## Testing Strategy

Go tests in `cmd/ingitdb/tui` drive the model with a counting `recordset.Evaluator`
(as in `pkg/dalgo2ingitdb`'s recordset tests) and assert evaluation counts for
the visible vs off-viewport windows, width/alignment without evaluation, locale
discovery parity, and non-fatal visible-error rendering. Row virtualization is
already exercised by existing scroll tests.

## Rehearse Integration

Every AC has a concrete unit surface (the collection model's render/layout
functions with a counting evaluator and a bounded viewport), so one Rehearse
Scenario stub per AC is scaffolded under `_tests/`.

## Not Doing / Out of Scope

- Bounding stored-column scans (locale discovery, stored-column widths, numeric
  detection) to the viewport — stored reads never evaluate formulas, so they stay
  full and unchanged.
- Changing the data-layer lazy contract (`recordset.Evaluator`, `AccessValue`) —
  unchanged; this Feature only changes how the TUI consumes it.
- Caching computed values beyond dalgo's per-row memoization.
- Horizontal/vertical scrollbar or viewport UX changes beyond what evaluation
  laziness requires.

## Assumption Carryover

From `computed-columns-via-dalgo` (shipped): `AccessValue` coerces computed
columns and passes stored values through; dalgo memoizes computed values per row
instance. This Feature relies on both and additionally requires the model to
retain row instances so memoization survives re-renders.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
