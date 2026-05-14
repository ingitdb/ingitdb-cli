# Feature: Shared CLI Flags

**Status:** Approved

## Summary

Defines the CLI flag grammar shared by the `select`, `insert`, `update`,
`delete`, and `drop` verbs. A single specification for `--from`,
`--into`, `--where`, `--set`, `--unset`, `--id`, `--all`,
`--min-affected`, `--order-by`, and `--fields`, including operator
semantics, value parsing, type-strictness rules, and flag
mutual-exclusion. Every verb spec references this feature; nothing
here implements a verb itself.

## Problem

When several commands share a flag, each command's spec is free to drift on
parsing rules, operator support, and value semantics. Concentrating the
contract in one feature lets verb specs reduce to "this verb accepts the
shared flags X, Y, Z" and reserves verb-specific surface for verb-specific
behavior. It also gives a single place to evolve the grammar without
N-place edits.

## Behavior

### Operators in `--where`

#### REQ: comparison-operators

The `--where` flag MUST accept the following comparison operators:
`>=`, `<=`, `>`, `<`, `==`, `===`, `!=`, `!==`. The flag MUST reject any
expression that uses bare `=` for comparison.

#### REQ: loose-equality

`==` and `!=` MUST compare values with type coercion: a numeric column
value compared to a numeric-parsable string ("42") MUST match; a boolean
compared to "true"/"false" MUST match; null compares equal to null only.

#### REQ: strict-equality

`===` and `!==` MUST compare values WITHOUT type coercion. The operand
types MUST match on JSON scalar kind (number, string, boolean, null) for
the comparison to succeed.

#### REQ: strict-equality-yaml-types

For columns declared in the collection schema with a richer YAML type
(date, time, timestamp), `===` and `!==` MUST require the literal to
parse as the same kind to match. For columns not declared with such a
type, YAML date/time/timestamp values MUST be treated as strings under
the strict comparison rule.

#### REQ: where-repeatable

The `--where` flag MUST be repeatable. Multiple occurrences MUST be
combined with logical AND. No flag-level `OR` or parentheses are supported
in this feature.

### Value parsing in `--where`

#### REQ: numeric-comma-stripping

When the right-hand side of a comparison parses as a number after
stripping ASCII commas (e.g. `1,000,000` → `1000000`), the value MUST be
treated as numeric. This rule applies to both `==` and `===`. Commas
inside quoted strings MUST NOT be stripped.

#### REQ: pseudo-id-field

The pseudo-field `$id` MUST resolve to the record's key in `--where`
expressions. Example: `--where='$id===us'` matches the record whose key
is the literal string `us`.

### `--set` assignment syntax

#### REQ: set-assignment

The `--set` flag MUST use a single `=` for assignment: `field=value`.
The flag MUST be repeatable; each occurrence assigns one column. The
operator between the field name and the value MUST be a single `=`;
expressions whose between-field-and-value operator is `==`, `===`, `>=`,
`<=`, `>`, `<`, `!=`, or `!==` MUST be rejected. Characters inside the
value portion (e.g. `--set='note=x>=5'`) are part of the value and are
not subject to this rule.

#### REQ: set-value-types

`--set` values MUST follow YAML 1.2 scalar inference:
`active=true` is boolean, `count=42` is integer, `ratio=3.14` is float,
`name=Ireland` is a string, `name="Hello, world"` is a quoted string,
`tagline=` is the empty string, `parent=null` is null. Values containing
spaces, `=`, or other special characters MUST be quoted. Schema
validation of the assigned column happens at execution time and is out
of scope for this feature.

### Targeting flags

#### REQ: from-flag

The `--from` flag MUST take a single collection ID. It MUST be accepted
by `select`, `update`, and `delete`. It MUST be rejected by `insert`
(which uses `--into`) and by `drop` (which uses positional subcommands).

#### REQ: into-flag

The `--into` flag MUST take a single collection ID. It MUST be accepted
by `insert`. It MUST be rejected by every other verb.

#### REQ: id-flag

The `--id` flag MUST follow the syntax defined by
[id-flag-format](../id-flag-format/README.md): `<collection-id>/<record-key>`.
It MUST be accepted by `select`, `update`, and `delete`. It MUST be
rejected by `insert` (which derives the key from `--data` or from a
positional argument convention) and by `drop` (which uses positional
subcommands).

### Mode selection and mutual exclusion

#### REQ: exactly-one-mode

The verbs `select`, `update`, and `delete` MUST operate in exactly one
of two modes: single-record (driven by `--id`) or set (driven by
`--from`). For these three verbs, supplying both `--id` and `--from` in
the same invocation MUST be rejected with a clear diagnostic, and
supplying neither MUST also be rejected. This rule does NOT apply to
`insert` (which uses `--into` and has no mode concept) or to `drop`
(which uses positional subcommands).

#### REQ: where-requires-set-mode

The `--where` flag MUST be rejected when `--id` is supplied. It is
valid only in set mode (with `--from`).

#### REQ: set-flag-applies-to-both-modes

The `--set` flag is valid for `update` in both single-record mode
(`--id` + `--set`) and set mode (`--from` + `--where` + `--set`).

### `--unset` (field removal)

#### REQ: unset-syntax

The `--unset` flag MUST take a comma-separated list of field names:
`--unset=field1,field2`. The flag MUST be repeatable; multiple
occurrences are unioned. Empty field names (e.g. trailing commas) MUST
be rejected. Field names MUST NOT contain `=` or whitespace.

#### REQ: unset-semantics

`--unset=field` MUST remove the field from the record entirely. It is
NOT equivalent to `--set='field=null'`, which assigns the explicit
value `null`. Whether the field can be removed (or whether the schema
requires it) MUST be validated at execution time and is out of scope
for this feature.

#### REQ: unset-applicability

The `--unset` flag MUST be accepted by `update` only. It MUST be
rejected by every other verb. It is valid in both single-record mode
(`--id`) and set mode (`--from` + `--where` OR `--from` + `--all`).

#### REQ: set-unset-mutual-field-exclusion

A field name MUST NOT appear in both `--set` and `--unset` in the same
invocation. Doing so MUST be rejected with a diagnostic that names the
conflicting field.

### `--all` (full-collection scope)

#### REQ: all-flag

The `--all` flag MUST be required on `delete` and `update` when in set
mode (`--from` is supplied) and no `--where` clauses are supplied.
`--all` and `--where` MUST be mutually exclusive: supplying both in the
same invocation MUST be rejected. `--all` MUST be rejected in
single-record mode (`--id`-driven).

### `--min-affected` (count threshold)

#### REQ: min-affected-syntax

The `--min-affected=N` flag MUST take a positive integer (N ≥ 1).
`--min-affected=0` and negative values MUST be rejected with a clear
diagnostic.

#### REQ: min-affected-applicability

The `--min-affected` flag MUST be accepted by `select`, `update`, and
`delete`. It MUST be rejected by `insert` and `drop`. It MUST be
rejected in single-record mode (when `--id` is supplied): a
single-record operation affects at most one record, so the threshold
is more naturally expressed by the verb's existing not-found
behavior.

#### REQ: min-affected-semantics

When the number of records the operation would affect is less than N,
the verb MUST exit non-zero with a diagnostic stating the actual
count and the required minimum. The threshold check MUST happen
BEFORE any write or output, so the operation is all-or-nothing
relative to the threshold: no record is patched (`update`), deleted
(`delete`), or emitted (`select`). When the affected count is
greater than or equal to N, the operation MUST proceed normally and
exit `0`.

The interpretation of "affected" is verb-specific:

- `select`: records that match `--where` (or the entire collection
  when only `--from` is supplied) — counted BEFORE `--limit` is
  applied.
- `update`: records that would be patched.
- `delete`: records that would be deleted.

### `--order-by`

#### REQ: order-by-syntax

The `--order-by` flag MUST accept a comma-separated list of field names.
A leading `-` reverses sort order for that field (e.g. `-population` is
descending). Empty field names MUST be rejected.

#### REQ: order-by-applicability

`--order-by` MUST be accepted by `select`. It MUST be rejected by every
other verb.

### `--fields` (field projection)

#### REQ: fields-syntax

The `--fields`/`-f` flag MUST accept `*` (all fields, the default), `$id`
(the record key only), or a comma-separated list of field names. The
pseudo-field `$id` MUST be selectable alongside real fields.

#### REQ: fields-applicability

`--fields` MUST be accepted by `select`. It MUST be rejected by every
other verb.

## Dependencies

- [id-flag-format](../id-flag-format/README.md) — `--id` syntax is
  defined there and referenced by `req:id-flag`.

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/shared-cli-flags`):

- [`cmd/ingitdb/commands/cobra_helpers.go`](../../cmd/ingitdb/commands/cobra_helpers.go)
- [`cmd/ingitdb/commands/flags.go`](../../cmd/ingitdb/commands/flags.go)
- [`cmd/ingitdb/commands/sqlflags/applicability.go`](../../cmd/ingitdb/commands/sqlflags/applicability.go)
- [`cmd/ingitdb/commands/sqlflags/doc.go`](../../cmd/ingitdb/commands/sqlflags/doc.go)
- [`cmd/ingitdb/commands/sqlflags/fields.go`](../../cmd/ingitdb/commands/sqlflags/fields.go)
- [`cmd/ingitdb/commands/sqlflags/min_affected.go`](../../cmd/ingitdb/commands/sqlflags/min_affected.go)
- [`cmd/ingitdb/commands/sqlflags/mode.go`](../../cmd/ingitdb/commands/sqlflags/mode.go)
- [`cmd/ingitdb/commands/sqlflags/order_by.go`](../../cmd/ingitdb/commands/sqlflags/order_by.go)
- [`cmd/ingitdb/commands/sqlflags/register.go`](../../cmd/ingitdb/commands/sqlflags/register.go)
- [`cmd/ingitdb/commands/sqlflags/set.go`](../../cmd/ingitdb/commands/sqlflags/set.go)
- [`cmd/ingitdb/commands/sqlflags/unset.go`](../../cmd/ingitdb/commands/sqlflags/unset.go)
- [`cmd/ingitdb/commands/sqlflags/where.go`](../../cmd/ingitdb/commands/sqlflags/where.go)

## Acceptance Criteria

### AC: where-rejects-bare-equals

**Requirements:** shared-cli-flags#req:comparison-operators

`--where='active=true'` (bare `=`) MUST be rejected with a diagnostic
that points the user at `==` for loose equality or `===` for strict
equality.

### AC: loose-equality-coerces-numeric-string

**Requirements:** shared-cli-flags#req:loose-equality, shared-cli-flags#req:numeric-comma-stripping

Given a record `{"population": 1000000}`, `--where='population==1,000,000'`
MUST match, and `--where='population=="1000000"'` MUST also match
(string-to-number coercion).

### AC: strict-equality-rejects-coercion

**Requirements:** shared-cli-flags#req:strict-equality

Given a record `{"population": 1000000}`, `--where='population===1000000'`
MUST match, while `--where='population==="1000000"'` (quoted string)
MUST NOT match. `--where='active===true'` matches a boolean field with
value `true`; `--where='active==="true"'` MUST NOT match the same record.

### AC: strict-equality-yaml-date

**Requirements:** shared-cli-flags#req:strict-equality-yaml-types

Given a column `released` declared with YAML date type in the
collection schema, and a record where `released` is `2024-01-15`,
`--where='released===2024-01-15'` MUST match. Given a different
collection where no column is declared with date type but a string
column `note` contains the literal `2024-01-15`,
`--where='note==="2024-01-15"'` MUST match and
`--where='note===2024-01-15'` (unquoted) MUST also match — both are
strings to the parser under the schema-free rule.

### AC: not-equal-operators-supported

**Requirements:** shared-cli-flags#req:comparison-operators

`--where='status!=active'` and `--where='status!==active'` MUST both
parse and behave as the negations of `==` and `===` respectively.

### AC: pseudo-id-in-where

**Requirements:** shared-cli-flags#req:pseudo-id-field

Given a collection `countries` with records keyed `us`, `ie`, `de`,
`select --from=countries --where='$id===ie'` MUST return exactly the
record with key `ie`.

### AC: set-uses-single-equals

**Requirements:** shared-cli-flags#req:set-assignment, shared-cli-flags#req:set-value-types

`update --id=countries/ie --set='active=true' --set='population=5000000'`
MUST set `active` to boolean `true` and `population` to integer
`5000000`. `update --id=countries/ie --set='active==true'` (double
equals) MUST be rejected.

### AC: unset-removes-fields

**Requirements:** shared-cli-flags#req:unset-syntax, shared-cli-flags#req:unset-semantics, shared-cli-flags#req:unset-applicability

Given a record `{"name": "Ireland", "active": true, "note": "x"}`,
`update --id=countries/ie --unset=active,note` MUST produce
`{"name": "Ireland"}` (fields removed, not nulled).
`update --id=countries/ie --unset=active --unset=note` (repeated flag)
MUST produce the same result.
`update --id=countries/ie --unset=` (empty value) MUST be rejected.
`select --from=countries --unset=active` MUST be rejected.

### AC: set-unset-conflict-rejected

**Requirements:** shared-cli-flags#req:set-unset-mutual-field-exclusion

`update --id=countries/ie --set='active=true' --unset=active` MUST be
rejected with a diagnostic that names the conflicting field `active`.

### AC: set-value-yaml-inference

**Requirements:** shared-cli-flags#req:set-value-types

`--set='name=Ireland'` MUST assign the string `"Ireland"`.
`--set='tags=null'` MUST assign null.
`--set='greeting="Hello, world"'` MUST assign the string
`"Hello, world"` (including the embedded comma).

### AC: id-and-from-mutually-exclusive

**Requirements:** shared-cli-flags#req:exactly-one-mode

`select --id=countries/ie --from=countries` MUST be rejected with a
diagnostic naming both flags. `select` with neither `--id` nor `--from`
MUST also be rejected.

### AC: where-rejected-in-single-record-mode

**Requirements:** shared-cli-flags#req:where-requires-set-mode

`select --id=countries/ie --where='active===true'` MUST be rejected with
a diagnostic that names `--where` and `--id` as the conflicting flags.

### AC: into-and-from-verb-routing

**Requirements:** shared-cli-flags#req:from-flag, shared-cli-flags#req:into-flag

`insert --from=countries --data='...'` MUST be rejected (insert takes
`--into`, not `--from`). `select --into=countries` MUST be rejected
(select takes `--from`, not `--into`). `update --into=countries
--set='active=false'` MUST be rejected. `drop --from=countries` MUST
be rejected (drop uses positional `drop collection <name>` /
`drop view <name>`, not `--from`).

### AC: all-required-for-unfiltered-set-delete

**Requirements:** shared-cli-flags#req:all-flag

`delete --from=countries` (no `--where`, no `--all`) MUST be rejected
with a diagnostic recommending `--all` for the full-collection
intention. `delete --from=countries --all` MUST proceed.
`delete --from=countries --where='active===false' --all` MUST be
rejected (mutual exclusion). `delete --id=countries/ie --all` MUST be
rejected (`--all` invalid in single-record mode).

### AC: all-required-for-unfiltered-set-update

**Requirements:** shared-cli-flags#req:all-flag

`update --from=countries --set='active=false'` (no `--where`, no
`--all`) MUST be rejected with a diagnostic recommending `--all` for
the full-collection intention. `update --from=countries --all
--set='active=false'` MUST proceed. `update --from=countries
--where='active===true' --all --set='active=false'` MUST be rejected
(mutual exclusion). `update --id=countries/ie --all
--set='active=false'` MUST be rejected (`--all` invalid in
single-record mode).

### AC: order-by-descending-prefix

**Requirements:** shared-cli-flags#req:order-by-syntax

`select --from=countries --order-by='-population,name'` MUST order
results by `population` descending, then by `name` ascending.

### AC: fields-pseudo-id-selectable

**Requirements:** shared-cli-flags#req:fields-syntax

`select --from=countries --fields='$id,name'` MUST return rows with
only the `$id` and `name` keys.

### AC: min-affected-parse-validation

**Requirements:** shared-cli-flags#req:min-affected-syntax

`--min-affected=0` MUST be rejected with a diagnostic. `--min-affected=-1`
MUST be rejected. `--min-affected=foo` MUST be rejected.
`--min-affected=1` is the smallest valid value.

### AC: min-affected-applicability-matrix

**Requirements:** shared-cli-flags#req:min-affected-applicability

`select`, `update`, and `delete` MUST accept `--min-affected` in set
mode. `insert --into=col --key=k --data='…' --min-affected=1` MUST be
rejected. `drop collection col --min-affected=1` MUST be rejected.
`select --id=col/key --min-affected=1` MUST be rejected (single-record
mode); same for `update --id=col/key --set='…' --min-affected=1` and
`delete --id=col/key --min-affected=1`.

### AC: min-affected-atomicity

**Requirements:** shared-cli-flags#req:min-affected-semantics

Given a collection `countries` with 2 records matching `region==EU`,
`update --from=countries --where='region==EU' --set='active=true' --min-affected=3`
MUST exit non-zero with a diagnostic stating the actual count (2) and
the required minimum (3); the two matching records MUST remain
unchanged. The parallel `delete` and `select` invocations MUST
likewise leave records untouched and stdout empty.

### AC: flag-rejected-on-wrong-verb

**Requirements:** shared-cli-flags#req:order-by-applicability, shared-cli-flags#req:fields-applicability

`insert --into=countries --order-by=name` MUST be rejected.
`delete --from=countries --where='...' --fields=name` MUST be rejected.

## Outstanding Questions

- `LIKE`/regex predicates in `--where` are deferred to a separate Idea:
  [where-like-regex](../../ideas/where-like-regex.md). This feature
  remains comparison-only until that Idea is approved and specified.

---
*This document follows the https://specscore.md/feature-specification*
