# Feature: Computed Columns (Inline Starlark Formulas)

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/computed-columns?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/computed-columns?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/computed-columns?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/computed-columns?op=request-change) |
**Status:** Approved
**Date:** 2026-06-02
**Owner:** alexander.trakhimenok@gmail.com
**Source Ideas:** scripted-formulas-and-views
**Supersedes:** —

## Summary

Lets a schema author add an inline Starlark formula to a collection column so its
value is computed from the record's other fields at read time, rather than stored.
It serves schema authors who want derived values (full names, labels, simple
arithmetic) that stay in sync with their source fields and never drift in git.

## Problem

inGitDB columns today are purely stored: every value lives in the record file and
must be kept in sync by hand. There is no way to say "this column is `first_name`
plus `last_name`" or "this label is derived from two other fields." Authors either
duplicate data across fields (drift risk) or compute it in external tooling
(inconsistent across consumers). The schema needs a first-class, sandboxed,
deterministic way to derive a column's value from sibling fields, reusing inGitDB's
existing read-time transformation pipeline (`ApplyLocaleToRead`).

## Behavior

A computed column is an ordinary `ColumnDef` that carries a non-empty `formula`
attribute holding one inline Starlark expression. The value is derived, never stored:
it is produced by the embedded Starlark interpreter from the record's stored fields
and coerced to the column's declared type. It is evaluated on read for output,
filtering, and sorting; and evaluated at write time from the `insert`/`update`
payload for foreign-key checks. It is never written to record files.

### Declaring a computed column

#### REQ: declare-formula

A `ColumnDef` MAY declare an optional `formula` string. A column whose `formula`
is non-empty is a *computed column*. The `formula` MUST be a single valid Starlark
expression; a parse error, or a body containing statements rather than one
expression, MUST be reported as a schema validation error naming the collection and
column.

#### REQ: reject-stored-value

A computed column's value MUST NOT be sourced from storage. If a record file, or an
`insert`/`update` payload, supplies a value for a computed column, the operation
MUST fail with a validation/write error identifying the collection, record key, and
column. The computed value is the only source of truth for that column.

#### REQ: stored-fields-only

For this Feature, a formula MAY reference only stored (non-computed) sibling fields
of the same record. A formula that references another computed column MUST be
rejected as a schema validation error. (Chained computed columns — a formula
referencing another computed column — are deferred to a future Feature.)

### Evaluating formulas

#### REQ: evaluate-on-read

For every record read from a collection, each computed column's value MUST be
produced by evaluating its `formula` before the record is returned to the caller,
in the same read-time transformation stage as `ApplyLocaleToRead`. (Computation
timing is superseded by `computed-columns-via-dalgo`: values are produced lazily
on access via dalgo's `recordset.Row` rather than eagerly, so a computed column is
evaluated only when a consumer references it.)

#### REQ: sandboxed-deterministic

The formula MUST be evaluated by the embedded Starlark interpreter with the record's
stored fields bound as variables, in a sandbox that exposes no network, filesystem,
clock, or randomness. Given identical record input and identical schema, evaluation
MUST always yield identical output.

#### REQ: builtin-helpers

Formulas MUST have access to a curated set of deterministic, side-effect-free helper
functions: Starlark's native string methods (including `.strip()`, `.lower()`,
`.upper()`, `.replace()`, `.split()`, `.startswith()`, `.endswith()`), the universe
functions `len`, `min`, and `max`, and a numeric helper set providing at least `abs`,
`round`, `floor`, and `ceil`. Every exposed helper MUST be pure and deterministic; the
interpreter MUST NOT expose any helper that accesses the network, filesystem, clock,
or randomness, and MUST NOT load any module providing such capabilities. Helper
coverage MAY be extended later, but only with functions meeting this same
determinism-and-no-IO bar.

#### REQ: coerce-to-type

The evaluated result MUST be coerced to the column's declared `ColumnType`. The MVP
supports coercion to `string`, `int`, `float`, `bool`, and `any`. A declared type
outside that set on a computed column MUST be rejected as a schema validation error.

### Error handling

#### REQ: fail-loud

If a formula raises a runtime error, or its result cannot be coerced to the declared
type, the read/materialization MUST abort with an error identifying the collection,
record key, column, and underlying cause. The system MUST NOT emit a partial row or
silently null the offending cell. Because computation is lazy (see
`computed-columns-via-dalgo`), this abort fires when the computed column is
*referenced* — projected, filtered, or sorted; a computed column that no consumer
references in an operation is not evaluated and does not abort that operation.

### Query and reference integration

#### REQ: usable-in-filter-and-sort

Because computed columns are evaluated before query operations, a computed column
MUST be usable in `--where` predicates and `order_by` exactly as a stored column is.

#### REQ: reference-error-shape

Every referential-integrity error this Feature raises for a computed foreign key MUST
report a consistent set of fields — the *reference-error shape*: the referencing
collection, the referencing record key, the computed foreign-key column, and the
referenced collection. All foreign-key acceptance criteria assert this shape.

#### REQ: foreign-key-support

A computed column MAY declare `foreign_key`. Enforcement rides the existing
write-time referential-integrity path: on `insert`/`update`, the column's formula is
evaluated from the payload's stored fields and the derived value is validated against
the referenced collection, exactly as a stored foreign key is. A derived value that
does not resolve to an existing record in the target collection MUST be rejected with
a referential-integrity error in the reference-error shape (REQ:reference-error-shape).

#### REQ: foreign-key-revalidate-on-input-change

A computed foreign-key column is never written directly, so an `update` MUST NOT
skip its foreign-key check based on which columns were supplied: the check MUST be
re-run for every computed foreign key on the record being updated. (Revalidating all
computed foreign keys on every update is a conforming implementation; no formula
dependency analysis is required.)

#### REQ: foreign-key-parent-side

Deleting or renaming a record that a computed foreign key references MUST be detected.
Because the referencing value is derived and not stored, the system MUST scan the
referencing collection and recompute each computed foreign key to find records that
resolve to the affected key, then reject the operation with a referential-integrity
error. (A derived-value index to avoid the full scan is a future performance
optimization, out of scope for this Feature.)

## Acceptance Criteria

### AC: formula-declared-and-computed (verifies REQ:declare-formula, REQ:evaluate-on-read)

**Given** a collection `people` whose `definition.yaml` declares a `string` column `full_name` with `formula: 'first_name + " " + last_name'`
**When** a record with `first_name: "Ada"` and `last_name: "Lovelace"` is read via `select`
**Then** the returned record's `full_name` equals `"Ada Lovelace"`

### AC: formula-syntax-error (verifies REQ:declare-formula)

**Given** a column declaring `formula: 'first_name +'` (not a complete Starlark expression)
**When** the schema is loaded or validated
**Then** validation fails with an error naming the collection and the `full_name` column

### AC: reject-stored-computed-value (verifies REQ:reject-stored-value)

**Given** a computed column `full_name`
**When** an `insert` (or a record file under validation) supplies a `full_name` value
**Then** the operation fails with an error naming the collection, record key, and `full_name` column

### AC: reject-chained-computed-reference (verifies REQ:stored-fields-only)

**Given** a computed column `greeting` whose formula references another computed column `full_name`
**When** the schema is loaded or validated
**Then** validation fails with an error stating a formula may reference only stored fields

### AC: deterministic-evaluation (verifies REQ:sandboxed-deterministic)

**Given** any record and a computed column
**When** the column's formula is evaluated twice for the same input
**Then** both evaluations return byte-identical output and no network, filesystem, clock, or randomness is accessible to the formula

### AC: type-coercion-success (verifies REQ:coerce-to-type)

**Given** an `int` column `total` with `formula: 'qty * price'` and a record with `qty: 3`, `price: 4`
**When** the record is read
**Then** `total` equals integer `12`

### AC: unsupported-type-rejected (verifies REQ:coerce-to-type)

**Given** a computed column declared with type `datetime`
**When** the schema is loaded or validated
**Then** validation fails because computed columns support only `string`, `int`, `float`, `bool`, and `any` in this Feature

### AC: runtime-error-fails-read (verifies REQ:fail-loud)

**Given** a computed `int` column whose formula divides by a field that is zero for some record
**When** that record is read **and the computed column is referenced** (projected, filtered, or sorted)
**Then** the read aborts with an error naming the collection, record key, and column, and no partial row is emitted; **a computed column that no consumer references is not evaluated and does not abort the read**

### AC: filter-on-computed-column (verifies REQ:usable-in-filter-and-sort)

**Given** the `people` collection with computed `full_name`
**When** `select` is run with `--where 'full_name == "Ada Lovelace"'`
**Then** only records whose computed `full_name` equals `"Ada Lovelace"` are returned

### AC: order-by-computed-column (verifies REQ:usable-in-filter-and-sort)

**Given** the `people` collection with computed `full_name`
**When** `select` is run with `order_by full_name asc`
**Then** the returned records are ordered by their computed `full_name`

### AC: foreign-key-on-insert-violation (verifies REQ:foreign-key-support, REQ:reference-error-shape)

**Given** a computed column `owner_key` declaring `foreign_key` to collection `users`, whose formula yields a key absent from `users`
**When** a record is inserted into `things` whose input fields make `owner_key` resolve to that absent key
**Then** the `insert` fails with a referential-integrity error in the reference-error shape — naming referencing collection `things`, the referencing record key, the `owner_key` column, and referenced collection `users`

### AC: foreign-key-revalidates-on-input-change (verifies REQ:foreign-key-revalidate-on-input-change, REQ:reference-error-shape)

**Given** a record whose computed `owner_key` currently resolves to an existing `users` record
**When** an `update` changes an input field of the formula so `owner_key` would resolve to a non-existent `users` record
**Then** the `update` fails with a referential-integrity error in the reference-error shape, even though the `owner_key` column was not directly written

### AC: foreign-key-parent-delete-detected (verifies REQ:foreign-key-parent-side, REQ:reference-error-shape)

**Given** a `users` record referenced by some `things` record's computed `owner_key`
**When** that `users` record is deleted
**Then** the delete fails with a referential-integrity error in the reference-error shape — naming referencing collection `things`, the referencing record key, the `owner_key` column, and referenced collection `users`

### AC: foreign-key-parent-rename-detected (verifies REQ:foreign-key-parent-side, REQ:reference-error-shape)

**Given** a `users` record referenced by some `things` record's computed `owner_key`
**When** that `users` record's key is renamed, so the old key no longer exists
**Then** the rename fails with a referential-integrity error in the reference-error shape — naming referencing collection `things`, the referencing record key, the `owner_key` column, and referenced collection `users`

### AC: builtin-string-helper-available (verifies REQ:builtin-helpers)

**Given** a `string` column `display` with `formula: 'first_name.strip().upper()'` and a record with `first_name: " ada "`
**When** the record is read
**Then** the returned `display` equals `"ADA"`

### AC: builtin-math-helper-available (verifies REQ:builtin-helpers)

**Given** an `int` column `rounded` with `formula: 'round(score)'` and a record with `score: 4.6`
**When** the record is read
**Then** the returned `rounded` equals integer `5`

## Rehearse Integration

Every AC is testable through a concrete surface — CLI `select`/`insert` output,
`validate` errors, or pure-function formula evaluation — so one Rehearse Scenario
stub per AC is scaffolded under `_tests/`, each carrying a `## TODO` checklist for its
pending state. The foreign-key ACs depend on the referential-integrity engine.

## Open Questions

- The initial helper set (REQ:builtin-helpers) covers common string and numeric
  needs; which further helpers (e.g. date parsing, formatting) graduate from
  "nice-to-have" to required will be driven by real formulas in use.
- Temporal (`date`/`time`/`datetime`) and `map[locale]string` types are intentionally
  rejected for computed columns in this Feature (see REQ:coerce-to-type). A future
  Feature adding them must decide which representation a formula returns.

---
*This document follows the https://specscore.md/feature-specification*
