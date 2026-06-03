# Idea: Scripted Computed Fields & Materialized Views (Starlark)

**Status:** Implemented
**Date:** 2026-06-02
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** computed-columns
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we let schema authors compute field values (and later whole materialized views) from record data using one sandboxed, deterministic, Python-like scripting language, without breaking git reproducibility or inventing a bespoke expression grammar?

## Context

Triggered by a request to let users script materialized views and field formulas in an open-source, Go-embeddable, Python-like language. Today views are purely declarative (Columns, OrderBy, Top; Where is a stub) and ColumnDef has no computed-field concept; go.mod has no scripting engine. The Approved derived-record-keys Idea deliberately chose declarative transforms over arbitrary code, so introducing scripting is a real philosophical fork this Idea must justify: scripting is added only where declarative config runs out of room. Related: where-like-regex (the unimplemented Where predicate).

## Recommended Direction

Adopt Starlark (google/starlark-go) as the single embedded language, exposed in two modes, and ship formula mode first. Each mode lives in the file that already owns its concept: a formula is an optional attribute on a `ColumnDef` inside the collection's `.collection/definition.yaml`, holding one inline Starlark expression, evaluated per record with the record's sibling fields bound as variables, producing that column's value. Inline is the default and the only MVP shape; large or shared formulas getting their own reusable file is a deliberate future extension (see Not Doing). The sandbox is strict and deterministic by default: no network, filesystem, clock, or randomness, so the same record plus the same schema always yields the same output and git diffs stay truthful. Computed columns are derived output, not source-of-truth stored in record files. Phase 2 reuses the exact same language for view mode, honoring the existing one-file-per-view convention: a scripted view's definition file under the schema's `views/` directory (the view ID derives from its filename, as today) references a sibling `.star` Starlark script file that receives the collection's records and emits view rows — enabling grouping and aggregation the declarative view cannot express. Keeping the script in its own `.star` file gives proper syntax highlighting and clean, reviewable diffs rather than a YAML-embedded heredoc. One language, one sandbox, one security review, and a seamless formula-to-view upgrade path.

## Alternatives Considered

**Two engines — expr/CEL for formulas, Starlark for views.** Each tool is optimally shaped for its job (a pure terminating evaluator per record; an imperative language for view aggregation). Lost because it doubles the surface: two syntaxes across the schema (C-style ternary in a column's `definition.yaml` formula, Python-style in the per-view script file), two sandboxes to audit, two dependencies, two docs sets — and it breaks the original "one Python-like language" promise, since CEL and expr are C-flavored, not Python-like. A formula that outgrows expressions then faces a cross-language migration cliff.

**Expression-only (CEL or expr), never adopt full scripting.** Safest and fastest for per-record formulas. Lost because it cannot express the imperative grouping/aggregation that view scripting needs; the original request explicitly wants *scripts* for views, so this caps the vision short.

**Extend declarative transforms (the derived-record-keys DSL) to formulas.** Stays inside the project's existing declarative philosophy. Lost because formulas need conditionals, string ops, and arithmetic; a config-only DSL runs out of room immediately and ends up reinventing an expression language badly.

**Go plugins / hooks (arbitrary compiled Go).** Maximum power. Lost on every axis that matters here: not sandboxable, not deterministic, not portable across machines, and a security non-starter for a git-shared schema.

## MVP Scope

A two-to-three-week spike: a `formula` attribute on a `ColumnDef` (in the collection's `.collection/definition.yaml`) accepts a single inline Starlark expression, evaluated per record in a strict no-IO sandbox with sibling fields bound as variables, output coerced to the column's declared type (loud failure on mismatch), and the computed value surfaced in select/view output. No view scripting, no cross-collection reads, no aggregation. Verify determinism by evaluating the same input twice and asserting byte-identical output.

## Not Doing (and Why)

- View-scripting mode now — deferred to Phase 2; field formulas are the wedge that proves the engine and sandbox
- Cross-collection reads or joins in formulas — keeps the sandbox tight and per-record evaluation cheap; revisit for view mode
- Storing computed values as source-of-truth in record files — computed columns are derived output only, to avoid git drift
- Two-engine approach (expr/CEL for formulas + Starlark for views) — rejected for one Python-like syntax, one sandbox, and no migration cliff
- Non-deterministic builtins (clock, random, network, filesystem) — banned by the determinism guarantee that keeps materialization reproducible
- External/shared reusable formula files now — MVP formulas are inline. A future extension can let large or complex formulas live in a separate, reusable file shared across columns, via either a `script: <path>` reference or mutually exclusive `formula` | `script` fields on the `ColumnDef`

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | `google/starlark-go` can evaluate a single expression with the record's fields bound as globals, in a sandbox with no network/filesystem/clock/random, deterministically | Spike: evaluate a fixed expression over a fixed record twice; assert byte-identical output and confirm no I/O builtins are reachable |
| Must-be-true | Computed values can be coerced into inGitDB's declared column types reliably, failing loud on mismatch instead of silently corrupting output | Build a coercion test matrix (string/int/float/bool) including mismatch cases; assert clear errors |
| Should-be-true | Schema authors find a single Starlark expression natural enough to prefer over more declarative config | Show 3–5 real formulas (full_name, conditional price, formatted label) to target authors and observe friction |
| Should-be-true | Per-record evaluation is fast enough on realistic collection sizes that materialization stays acceptable | Benchmark formula evaluation across N records (e.g. 1k/10k/100k) and compare to current materialization cost |
| Might-be-true | The same engine and sandbox extend cleanly to Phase-2 view-scripting mode without a redesign | Prototype a minimal view script reusing the formula-mode runtime; confirm the API generalizes |


## SpecScore Integration

- **New Features this would create:** a computed-fields (formula mode) Feature for the MVP; a later scripted-views Feature for Phase 2
- **Existing Features affected:** record-format and output-formats (select must surface computed columns); shared-cli-flags (whether computed columns are usable in `--where`/`order_by`); view materialization (Phase 2)
- **Dependencies:** new module dependency `google/starlark-go`; relates to the where-like-regex Idea (the unimplemented `Where` predicate) and the Approved derived-record-keys Idea (the declarative-vs-code precedent this Idea consciously diverges from)

## Open Questions

- Compute-on-read vs materialize-and-store: are computed values produced fresh on every select/materialize (leaning yes, to avoid git drift), or cached anywhere?
- Are computed columns allowed to participate in `--where`, `order_by`, or as foreign keys — or are they output-only?
- How are evaluation errors surfaced — fail the whole read/materialization, or null the offending cell with a warning?
- Does formula mode bind only sibling fields, or also a small curated stdlib (string/number helpers) — and which builtins are explicitly disabled to preserve determinism?

---
*This document follows the https://specscore.md/idea-specification*
