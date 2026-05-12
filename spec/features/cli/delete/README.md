# Feature: Delete

**Status:** Approved

## Summary

The `ingitdb delete` command removes records from a collection. Two
modes inherited from
[shared-cli-flags](../../shared-cli-flags/README.md): single-record
(`--id=<collection>/<key>`) and set
(`--from=<collection>` with `--where` or `--all`). Success is silent
on stdout; the exit code is the signal. `--min-affected=N` opts a
set-mode invocation into a non-zero exit when fewer than N records
are deleted. `delete` replaces and consolidates the prior
`delete record` and `delete records` commands. It operates on records
only; schema-object deletion (collections, views) is owned by `drop`.

## Problem

Two predecessor commands today (`delete record` and `delete records`,
the latter never implemented) split single-record and bulk deletion
across separate verbs with separate flag surfaces. SQL has one
`DELETE` for both. Folding them into `delete` with mode selection
from `shared-cli-flags` removes the split and lets the same set-mode
flags (`--from` / `--where` / `--all` / `--min-affected`) compose the
same way they do for `update`.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb delete`. It MUST accept the
shared flags from `shared-cli-flags` that apply: `--id`, `--from`,
`--where`, `--all`. It MUST reject `--into`, `--set`, `--unset`,
`--order-by`, `--fields` (per `shared-cli-flags` applicability
rules).

#### REQ: mode-selection

Exactly one of `--id` or `--from` MUST be provided, per
`shared-cli-flags#req:exactly-one-mode`. Supplying both, or neither,
MUST be rejected.

### Single-record mode

#### REQ: single-record-delete

When `--id` is provided, `delete` MUST remove the record identified
by `--id`. On success it MUST exit `0`.

#### REQ: single-record-not-found

When `--id` resolves to no existing record, `delete` MUST exit
non-zero with a diagnostic that names the unresolved ID. No write
MUST occur.

#### REQ: single-record-rejected-flags

In single-record mode, `delete` MUST reject `--where`, `--all`, and
`--min-affected`. Per `shared-cli-flags#req:where-requires-set-mode`,
`shared-cli-flags#req:all-flag`, and (for `--min-affected`) this
feature.

### Set mode

#### REQ: set-mode-shape

When `--from` is provided, `delete` MUST remove every record matching
`--where` (AND-joined per `shared-cli-flags#req:where-repeatable`) or,
when `--all` is supplied, every record in the collection. Exactly one
of `--where` or `--all` MUST be present in set mode — supplying
neither MUST be rejected per `shared-cli-flags#req:all-flag`, and
supplying both MUST also be rejected by that same requirement.

#### REQ: set-mode-zero-matches-default

When `--from` set-mode matches zero records and `--min-affected` is
NOT supplied, `delete` MUST exit `0`. No write occurs; no message is
emitted on stdout.

#### REQ: min-affected-flag

`delete` MUST accept `--min-affected=N` per
`shared-cli-flags#req:min-affected-syntax`,
`shared-cli-flags#req:min-affected-applicability`, and
`shared-cli-flags#req:min-affected-semantics`. For `delete`, the
"affected" count is the number of records that would be deleted
(matched by `--where`, or the entire collection size when `--all` is
supplied).

### Output and exit

#### REQ: success-output

On success — including the zero-matches case in set mode without
`--min-affected` — `delete` MUST exit `0` and write nothing to
stdout. Diagnostic and progress messages MUST go to stderr.

### Source selection

#### REQ: source-selection

`delete` MUST accept either `--path=PATH` (local) or
`--github=OWNER/REPO[@REF]` (remote GitHub), but never both, per
[path-targeting](../../path-targeting/README.md) and
[github-direct-access](../../github-direct-access/README.md). When
neither is provided, the current working directory is used.

#### REQ: github-write-requires-token

For `--github` writes, a token MUST be supplied via `--token` or the
`GITHUB_TOKEN` environment variable. Each successful invocation MUST
produce exactly one commit in the remote repository, regardless of
how many records were deleted.

## Dependencies

- [shared-cli-flags](../../shared-cli-flags/README.md) — `--id`,
  `--from`, `--where`, `--all`, and applicability matrix.
- [id-flag-format](../../id-flag-format/README.md) — `--id` syntax.
- [path-targeting](../../path-targeting/README.md) — `--path`.
- [github-direct-access](../../github-direct-access/README.md) —
  `--github`.

## Acceptance Criteria

### AC: single-record-delete

**Requirements:** cli/delete#req:subcommand-name, cli/delete#req:mode-selection, cli/delete#req:single-record-delete, cli/delete#req:success-output, cli/delete#req:github-write-requires-token

Given a local database containing a record `countries/ie`,
`ingitdb delete --id=countries/ie` MUST remove the record, exit `0`,
and write nothing to stdout. Targeting the same record against a
GitHub repository MUST produce exactly one commit.

### AC: single-record-not-found

**Requirements:** cli/delete#req:single-record-not-found

`ingitdb delete --id=countries/zz` against a collection where `zz`
does not exist MUST exit non-zero with a diagnostic that names
`countries/zz`. No write MUST occur.

### AC: set-mode-where-delete

**Requirements:** cli/delete#req:set-mode-shape, cli/delete#req:success-output

Given a collection `countries` with three records where one matches
`region==MARS`,
`ingitdb delete --from=countries --where='region==MARS'` MUST remove
the matching record only, leave the other two unchanged, exit `0`,
and write nothing to stdout.

### AC: set-mode-all-delete

**Requirements:** cli/delete#req:set-mode-shape, cli/delete#req:github-write-requires-token

`ingitdb delete --from=countries --all` MUST remove every record in
`countries` and produce one commit when targeting GitHub.
`ingitdb delete --from=countries` (no `--where`, no `--all`) MUST be
rejected per `shared-cli-flags#req:all-flag`.
`ingitdb delete --from=countries --all --where='region==EU'` MUST be
rejected (mutual exclusion).

### AC: set-mode-zero-matches-default-success

**Requirements:** cli/delete#req:set-mode-zero-matches-default

`ingitdb delete --from=countries --where='region==NOWHERE'` where no
record matches MUST exit `0`, perform no write, and write nothing to
stdout.

### AC: min-affected-fails-below-threshold

**Requirements:** cli/delete#req:min-affected-flag

Given a collection `countries` where 2 records match `region==EU`,
`ingitdb delete --from=countries --where='region==EU' --min-affected=3`
MUST exit non-zero with a diagnostic stating "affected 2 records,
required at least 3", and MUST NOT delete any record. The same
invocation with `--min-affected=2` MUST delete both matching records
and exit `0`. With `--min-affected=1` against zero matches, the
invocation MUST exit non-zero.

### AC: min-affected-with-all

**Requirements:** cli/delete#req:min-affected-flag

Given a collection `countries` with 195 records,
`ingitdb delete --from=countries --all --min-affected=100` MUST
delete all 195 records and exit `0`. The same invocation against an
empty collection MUST exit non-zero and perform no write.

### AC: min-affected-validation

**Requirements:** cli/delete#req:min-affected-flag

`ingitdb delete --from=countries --where='…' --min-affected=0` MUST
be rejected. `--min-affected=-1` MUST be rejected.
`ingitdb delete --id=countries/ie --min-affected=1` MUST be rejected
(single-record mode).

### AC: rejects-non-delete-flags

**Requirements:** cli/delete#req:subcommand-name

`ingitdb delete --id=countries/ie --into=other` MUST be rejected
(per `shared-cli-flags#req:into-flag`).
`ingitdb delete --id=countries/ie --set='active=false'` MUST be
rejected (per `shared-cli-flags#req:set-flag-applies-to-both-modes`,
which scopes `--set` to `update`).
`ingitdb delete --id=countries/ie --unset=draft` MUST be rejected
(per `shared-cli-flags#req:unset-applicability`).
`ingitdb delete --from=countries --where='…' --order-by=name` MUST
be rejected (per `shared-cli-flags#req:order-by-applicability`).
`ingitdb delete --from=countries --where='…' --fields=name` MUST be
rejected (per `shared-cli-flags#req:fields-applicability`).

## Outstanding Questions

- Should single-record `delete --id=…` against a missing record
  support an `--ignore-missing` flag (idempotent delete, like
  `rm -f`)? Defer until a real use case appears; today's default of
  non-zero exit is preserved.
- Should `delete` support a soft-delete mode (move records into a
  trash directory rather than removing them)? Out of scope; git
  history serves as the recovery mechanism.

---
*This document follows the https://specscore.md/feature-specification*
