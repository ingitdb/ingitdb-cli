# Plan: TUI lazy computed-cell evaluation

**Status:** Completed
**Source Feature:** tui-lazy-computed-cells
**Date:** 2026-06-03
**Owner:** alex
**Supersedes:** —
**Mode:** full

## Summary

Decomposes the `tui-lazy-computed-cells` Feature into three linear tasks that make
the TUI collection screen evaluate computed columns only for painted cells: first
the lazy model/load/render core, then schema-based sizing/alignment so layout
never evaluates, then non-fatal rendering of an erroring visible cell. All seven
acceptance criteria are covered; none are deferred.

## Approach

Tasks follow the dependency chain. Task 1 lands the lazy core — `loadRecordsCmd`
hands the model the recordset + retained row handles (no eager evaluation) and
`cellValue` resolves each painted cell through `AccessValue`, so off-viewport rows
are never evaluated and per-row memoization survives re-renders. Task 2 removes the
remaining forced-evaluation sites by sizing/aligning computed columns from the
schema (header + declared `ColumnType`) while leaving stored-column scans — widths,
locale discovery, numeric detection — unchanged. Task 3 makes a painted computed
cell's evaluation error render a bounded indicator instead of aborting the screen
(off-viewport erroring columns are never evaluated, so they cannot affect it). Task
2 and Task 3 both build on Task 1's lazy `cellValue`/model.

## Tasks

### Task 1: Lazy model, load, and render-time computed evaluation

**Verifies:** tui-lazy-computed-cells#ac:off-viewport-rows-not-evaluated, tui-lazy-computed-cells#ac:scroll-evaluates-only-newly-visible
**Status:** done

Refactor `loadRecordsCmd` to return the query `recordset.Recordset` plus retained
per-record row handles and keys (no eager computed evaluation), have the collection
model hold them instead of pre-built `map[string]any` records, and resolve each
painted cell through `dalgo2ingitdb.AccessValue` so only visible rows' computed
columns evaluate and dalgo's per-row memoization survives re-renders and scroll-back.

### Task 2: Schema-based sizing and alignment for computed columns

**Verifies:** tui-lazy-computed-cells#ac:width-sizing-does-not-evaluate, tui-lazy-computed-cells#ac:numeric-alignment-does-not-evaluate, tui-lazy-computed-cells#ac:stored-locale-discovery-unchanged
**Status:** done

Derive a computed column's width from its header label and declared
`ColumnType`/length and its numeric alignment from the declared type, so neither
width computation nor alignment detection evaluates a computed column; leave
stored-column width sizing, locale discovery, and numeric detection scanning stored
values exactly as today.

### Task 3: Non-fatal rendering of erroring visible computed cells

**Verifies:** tui-lazy-computed-cells#ac:visible-computed-error-non-fatal, tui-lazy-computed-cells#ac:off-viewport-error-never-evaluated
**Status:** done

Render a bounded error indicator in a painted computed cell whose evaluation or
coercion fails, keeping the rest of the screen rendering without aborting the load
or crashing; because evaluation is lazy (Task 1), an off-viewport erroring computed
column is never evaluated and never affects the screen.

## Open Questions

- Task 1 should pin a concrete visible-window height in the test harness so the
  "zero off-viewport invocations" assertions are deterministic (AI-reviewer advisory
  from the Feature gate).

---
*This document follows the https://specscore.md/plan-specification*
