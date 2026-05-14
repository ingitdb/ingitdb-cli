# Feature: Update

**Status:** Approved

## Summary

The `ingitdb update` command applies patch-style changes to records.
`--set field=value` adds or changes a field; `--unset field1,field2`
removes fields. Both flags are repeatable and may be combined. Two
modes inherited from
[shared-cli-flags](../../shared-cli-flags/README.md): single-record
(`--id=<collection>/<key>`) and set
(`--from=<collection>` with `--where` or `--all`). Patch is shallow at
the top level — fields not named in `--set`/`--unset` are preserved
unchanged. Success is silent on stdout; the exit code is the signal.
`--min-affected=N` opts a set-mode invocation into a non-zero exit
when fewer than N records are affected. `update` renames and supersedes
the prior `update record` command.

## Problem

Most edits to a record touch one or two fields. Reading the whole
record, mutating it, and writing it back is race-prone and forces
callers to know every existing field. Patch semantics make updates
surgical and idempotent under schema growth. The previous
`update record` only supported single-record updates; users querying
"set `active=false` on every record where `last_seen < 2020`" had to
script around the CLI. Adding set mode while keeping patch semantics
makes bulk maintenance routine.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb update`. It MUST accept the
shared flags from `shared-cli-flags` that apply: `--id`, `--from`,
`--where`, `--set`, `--unset`, `--all`. It MUST reject `--into`,
`--order-by`, `--fields` (per `shared-cli-flags` applicability rules).

#### REQ: mode-selection

Exactly one of `--id` or `--from` MUST be provided, per
`shared-cli-flags#req:exactly-one-mode`. Supplying both, or neither,
MUST be rejected.

#### REQ: patch-required

At least one of `--set` or `--unset` MUST be supplied. An invocation
with neither MUST be rejected with a diagnostic naming both flags
(there is nothing to do).

### Patch semantics

#### REQ: patch-shallow

`--set` and `--unset` MUST patch the record at the top level. A
`--set='metadata=…'` value MUST replace the entire `metadata` field;
`--set` MUST NOT descend into nested maps. Field names in `--set`
and `--unset` MUST be interpreted as literal top-level keys, not as
dot-paths. Fields not named in `--set` or `--unset` MUST be preserved
unchanged.

#### REQ: set-unset-field-exclusion-inherited

The same-field-in-both-flags rule from
`shared-cli-flags#req:set-unset-mutual-field-exclusion` MUST apply
here. This requirement exists to anchor the AC; the rule itself is
owned by `shared-cli-flags`.

### Single-record mode

#### REQ: single-record-not-found

When `--id` resolves to no existing record, `update` MUST exit
non-zero with a diagnostic that names the unresolved ID. No write
MUST occur.

#### REQ: single-record-rejected-flags

In single-record mode, `update` MUST reject `--where`, `--all`, and
`--min-affected`. Per `shared-cli-flags#req:where-requires-set-mode`,
`shared-cli-flags#req:all-flag`, and (for `--min-affected`) this
feature.

### Set mode

#### REQ: set-mode-shape

When `--from` is provided, `update` MUST apply the patch to every
record matching `--where` (AND-joined per
`shared-cli-flags#req:where-repeatable`) or, when `--all` is supplied,
to every record in the collection. Exactly one of `--where` or
`--all` MUST be present in set mode — supplying neither MUST be
rejected per `shared-cli-flags#req:all-flag`, and supplying both
MUST also be rejected by that same requirement.

#### REQ: set-mode-zero-matches-default

When `--from` set-mode affects zero records and `--min-affected` is
NOT supplied, `update` MUST exit `0`. No write occurs; no message is
emitted on stdout.

#### REQ: min-affected-flag

`update` MUST accept `--min-affected=N` per
`shared-cli-flags#req:min-affected-syntax`,
`shared-cli-flags#req:min-affected-applicability`, and
`shared-cli-flags#req:min-affected-semantics`. For `update`, the
"affected" count is the number of records that would be patched
(matched by `--where`, or the entire collection size when `--all` is
supplied). With `--all`, `--min-affected=N` becomes a guard against
operating on an unexpectedly small collection.

### Output and exit

#### REQ: success-output

On success — including the zero-matches case in set mode — `update`
MUST exit `0` and write nothing to stdout. Diagnostic and progress
messages MUST go to stderr.

### Source selection

#### REQ: source-selection

`update` MUST accept either `--path=PATH` (local) or
`--remote=HOST/OWNER/REPO[@REF]` (remote Git repository), but never
both, per [path-targeting](../../path-targeting/README.md) and
[remote-repo-access](../../remote-repo-access/README.md). When
neither is provided, the current working directory is used.

#### REQ: remote-write-requires-token

For `--remote` writes, a token MUST be supplied via `--token` or a
host-derived environment variable (e.g. `GITHUB_TOKEN` for
`github.com`). Each successful invocation MUST produce exactly one
commit in the remote repository, regardless of how many records the
patch touched (single-record mode commits the one record; set mode
commits the batch atomically).

## Dependencies

- [shared-cli-flags](../../shared-cli-flags/README.md) — `--id`,
  `--from`, `--where`, `--all`, `--set`, `--unset`, mode-selection,
  set/unset mutual field exclusion.
- [id-flag-format](../../id-flag-format/README.md) — `--id` syntax.
- [path-targeting](../../path-targeting/README.md) — `--path`.
- [remote-repo-access](../../remote-repo-access/README.md) —
  `--remote`.

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/update`):

- [`cmd/ingitdb/commands/update_new.go`](../../../cmd/ingitdb/commands/update_new.go)

## Acceptance Criteria

### AC: single-record-patch

**Requirements:** cli/update#req:subcommand-name, cli/update#req:mode-selection, cli/update#req:patch-required, cli/update#req:patch-shallow, cli/update#req:success-output, cli/update#req:remote-write-requires-token

Given a record `{name: Ireland, population: 5000000}`,
`ingitdb update --id=countries/ie --set='capital=Dublin'` MUST
produce `{name: Ireland, population: 5000000, capital: Dublin}`,
exit `0`, and write nothing to stdout. Repeating
`--set='currency=EUR'` in the same invocation MUST add a second
field in one commit.

### AC: single-record-unset

**Requirements:** cli/update#req:patch-shallow

Given a record `{name: Ireland, capital: Dublin, currency: EUR}`,
`ingitdb update --id=countries/ie --unset=currency` MUST produce
`{name: Ireland, capital: Dublin}` (the `currency` key removed
entirely, not set to null).

### AC: set-and-unset-combined

**Requirements:** cli/update#req:patch-required, cli/update#req:set-unset-field-exclusion-inherited

`ingitdb update --id=countries/ie --set='capital=Dublin' --unset=draft`
MUST apply both operations in one invocation.
`ingitdb update --id=countries/ie --set='capital=Dublin' --unset=capital`
MUST be rejected (per
`shared-cli-flags#req:set-unset-mutual-field-exclusion`).

### AC: no-patch-rejected

**Requirements:** cli/update#req:patch-required

`ingitdb update --id=countries/ie` (no `--set`, no `--unset`) MUST be
rejected with a diagnostic naming both flags.

### AC: single-record-not-found

**Requirements:** cli/update#req:single-record-not-found

`ingitdb update --id=countries/zz --set='active=true'` against a
collection where `zz` does not exist MUST exit non-zero with a
diagnostic that names `countries/zz`. No write MUST occur.

### AC: shallow-patch-replaces-nested-map

**Requirements:** cli/update#req:patch-shallow

Given a record `{metadata: {author: alice, draft: true}}`,
`ingitdb update --id=posts/hello --set='metadata={author: bob}'` MUST
produce `{metadata: {author: bob}}` — the entire `metadata` field is
replaced. `--set` MUST NOT merge into the existing nested map.
`ingitdb update --id=posts/hello --set='metadata.author=bob'` MUST
treat `metadata.author` as a literal top-level key (creating that
field if absent), NOT as a path into `metadata`.

### AC: set-mode-where-patch

**Requirements:** cli/update#req:set-mode-shape, cli/update#req:success-output

Given a collection `countries` with three records where one matches
`region==EU`,
`ingitdb update --from=countries --where='region==EU' --set='currency=EUR'`
MUST patch the matching record only, leave the other two unchanged,
exit `0`, and write nothing to stdout.

### AC: set-mode-all

**Requirements:** cli/update#req:set-mode-shape, cli/update#req:remote-write-requires-token

`ingitdb update --from=countries --all --set='last_audit=2026-05-12'`
MUST patch every record in `countries` with the new field, exit `0`,
and produce one commit (per `req:remote-write-requires-token` when
the target is a remote repository).
`ingitdb update --from=countries --all --where='region==EU' --set='…'`
MUST be rejected (per `shared-cli-flags#req:all-flag`).

### AC: set-mode-zero-matches-default-success

**Requirements:** cli/update#req:set-mode-zero-matches-default

`ingitdb update --from=countries --where='region==MARS' --set='active=false'`
where no record matches MUST exit `0`, perform no write, and write
nothing to stdout.

### AC: min-affected-fails-below-threshold

**Requirements:** cli/update#req:min-affected-flag

Given a collection `countries` where 2 records match `region==EU`,
`ingitdb update --from=countries --where='region==EU' --set='active=true' --min-affected=3`
MUST exit non-zero with a diagnostic stating "affected 2 records,
required at least 3", and MUST NOT patch any record (the two matching
records remain unchanged). The same invocation with `--min-affected=2`
MUST patch both matching records and exit `0` (count met). With
`--min-affected=1` against zero matches, the invocation MUST exit
non-zero and MUST NOT patch any record.

### AC: min-affected-with-all

**Requirements:** cli/update#req:min-affected-flag

Given a collection `countries` with 195 records,
`ingitdb update --from=countries --all --set='checked=true' --min-affected=100`
MUST exit `0` (195 ≥ 100). The same invocation against an empty
collection MUST exit non-zero (0 < 100).

### AC: min-affected-validation

**Requirements:** cli/update#req:min-affected-flag

`ingitdb update --from=countries --where='…' --set='…' --min-affected=0`
MUST be rejected. `--min-affected=-1` MUST be rejected.
`ingitdb update --id=countries/ie --set='…' --min-affected=1` MUST be
rejected (single-record mode).

### AC: rejects-non-update-flags

**Requirements:** cli/update#req:subcommand-name

`ingitdb update --id=countries/ie --set='…' --into=other` MUST be
rejected (per `shared-cli-flags#req:into-flag`).
`ingitdb update --id=countries/ie --set='…' --order-by=name` MUST be
rejected (per `shared-cli-flags#req:order-by-applicability`).
`ingitdb update --id=countries/ie --set='…' --fields=name` MUST be
rejected (per `shared-cli-flags#req:fields-applicability`).

### AC: remote-update-one-commit

**Requirements:** cli/update#req:source-selection, cli/update#req:remote-write-requires-token

With a valid token,
`ingitdb update --remote=github.com/owner/repo --id=countries/ie --set='capital=Dublin'`
MUST produce exactly one commit in `owner/repo` whose diff is limited
to the patched fields.
`ingitdb update --remote=github.com/owner/repo --from=countries --all --set='checked=true'`
MUST also produce exactly one commit, even when many records change.

## Outstanding Questions

- Dot-path traversal in `--set`/`--unset` (e.g.
  `--set='metadata.author=alice'`) as an opt-in mode? Defer to a
  future Idea; this feature pins literal top-level keys.
- Deep-merge mode for `--set` on nested map values? Defer; the prior
  `update-record` carried the same question and never escalated it.
- Should set-mode update report the count of affected records to
  stderr (e.g. `2 records updated`)? Cross-cutting decision: the
  parent Idea explicitly chose silent for set-scope DML; this feature
  honors that.

---
*This document follows the https://specscore.md/feature-specification*
