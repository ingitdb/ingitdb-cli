---
type: sidekick-seed
slug: tui-collection-screen-should-evaluate-only-painted-visible
captured_at: 2026-06-03T12:03:29Z
captured_by: specstudio:implement
captured_during: spec/features/computed-columns-via-dalgo
trigger: explicit
status: queued
synchestra_task: null
---
# TUI collection screen should evaluate only painted visible computed-column cells, not all cells at load

During `computed-columns-via-dalgo` Task 5 the TUI collection screen was migrated
onto `ExecuteQueryToRecordsetReader` + the shared `AccessValue` accessor, but it
still builds a full per-record map at load time (reading every column), so every
computed column is evaluated up front — byte-identical to the prior eager
behavior, but not the per-cell laziness `REQ:all-consumers-via-recordset`
envisions ("The TUI MUST read only the cells it renders, so hidden or
off-viewport computed columns are not evaluated").

Root cause: the TUI model holds `records []map[string]any` and scans **all
records × all columns** for column-width sizing, locale discovery
(`discoverLocales`), and numeric detection (`isNumeric(records[0][c])`). True
per-painted-cell laziness needs a model refactor to hold recordset rows + the
recordset and defer `AccessValue` to render time (and to bound column sizing to
the visible viewport).

Out of scope for the lazy migration Feature (no dedicated TUI AC; data-layer
`compute-only-on-reference` is satisfied and unit-verified via the recordset).
Worth a follow-up Feature if computed columns become expensive enough that
off-viewport evaluation hurts TUI responsiveness.
