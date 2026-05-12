# Feature: Select

**Status:** Approved

## Summary

The `ingitdb select` command queries records from a single collection.
It operates in two modes inherited from
[shared-cli-flags](../../shared-cli-flags/README.md): single-record
(`--id=<collection>/<key>`) and set (`--from=<collection>` with optional
`--where`, `--order-by`, `--fields`, `--limit`). Output format defaults
to YAML in single-record mode and CSV in set mode; `--format` overrides
in either direction. `--min-affected=N` opts a set-mode invocation
into a non-zero exit when fewer than N records match (a safety guard
parallel to `update` and `delete`). `select` replaces the prior
`read record` and `query` commands.

## Problem

Two commands today (`read record` and `query`) serve the same conceptual
operation — fetch records from a collection — with different defaults,
different flag surfaces, and different output expectations. Users
fluent in SQL expect a single `SELECT` verb. Collapsing both flows into
`select` removes a class of "wait, which command do I want?" friction
and lets the same flag grammar (`--from`, `--where`, `--id`,
`--order-by`, `--fields`) compose across single-record and set queries.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb select`. It MUST accept the
shared flags defined in `shared-cli-flags` that apply to a read verb:
`--id`, `--from`, `--where`, `--order-by`, `--fields`. It MUST reject
`--into`, `--set`, `--unset`, and `--all` (per the applicability rules
in `shared-cli-flags`).

#### REQ: mode-selection

Exactly one of `--id` or `--from` MUST be provided, per
`shared-cli-flags#req:exactly-one-mode`. Supplying both, or neither,
MUST be rejected.

### Single-record mode

#### REQ: single-record-shape

When `--id` is provided, `select` MUST resolve the record by its
collection-and-key and emit the record's fields. The output is a single
record value (not wrapped in a list or array) in YAML and JSON formats.

#### REQ: single-record-not-found

When `--id` resolves to no record, `select` MUST exit non-zero with a
diagnostic that names the unresolved ID. No output is written to
stdout.

#### REQ: single-record-rejected-flags

In single-record mode, `select` MUST reject `--where` (per
`shared-cli-flags#req:where-requires-set-mode`), `--order-by`,
`--limit`, and `--min-affected`. The mode-rejection of `--order-by`,
`--limit`, and `--min-affected` is owned by this requirement: those
flags have no meaning when the result is at most one record.
`--fields` is permitted and projects only the requested fields onto
the single output record.

### Set mode

#### REQ: set-mode-shape

When `--from` is provided, `select` MUST emit zero or more records
filtered by `--where` (AND-joined per
`shared-cli-flags#req:where-repeatable`), sorted by `--order-by`, and
projected by `--fields`. The output is a sequence of records — a CSV
table, a JSON array, a YAML list, or a Markdown table — even when the
result is empty or a single record.

#### REQ: set-mode-empty-result

When `--from` set-mode produces zero records, `select` MUST exit `0`.
The output MUST be format-appropriate:

- `csv`: the header row only (column names from `--fields` if supplied;
  otherwise from the collection's declared `columns_order` only — with
  no records to inspect, "additional fields appended alphabetically"
  from `req:format-tabular-columns` does not apply; the body is empty)
- `json`: `[]`
- `yaml`: `[]`
- `md`: the header row only (no data rows), with column derivation
  matching the `csv` rule above

#### REQ: limit-flag

The `--limit=N` flag MUST cap the number of records returned in set
mode. `N` MUST be a non-negative integer; `N=0` means "no limit" (the
default when the flag is omitted); negative values MUST be rejected.
`--limit` MUST be rejected in single-record mode.

#### REQ: min-affected-flag

`select` MUST accept `--min-affected=N` per
`shared-cli-flags#req:min-affected-syntax`,
`shared-cli-flags#req:min-affected-applicability`, and
`shared-cli-flags#req:min-affected-semantics`. For `select`, the
"affected" count is the number of records matching `--where` (or the
entire collection when only `--from` is supplied), measured BEFORE
`--limit` is applied. When the matched count is below N, `select`
MUST NOT write any record to stdout.

#### REQ: order-then-limit

When both `--order-by` and `--limit` are supplied, ordering MUST be
applied first and the limit MUST be applied to the sorted sequence.

### Output formats

#### REQ: format-flag

The `--format` flag MUST accept `yaml`, `json`, `csv`, and `md`
(Markdown table). The default MUST be `yaml` in single-record mode and
`csv` in set mode. The user MUST be able to override the default in
either direction.

#### REQ: format-tabular-columns

In CSV and Markdown table output, the column order MUST follow
`--fields` when supplied. When `--fields` is omitted (or is `*`), the
column order MUST follow the collection's `columns_order` declaration,
with any additional fields appended in alphabetical order. The `$id`
pseudo-field, when present in `--fields`, MUST appear as a column
named `$id`.

#### REQ: format-yaml-json-shape

YAML and JSON output in single-record mode MUST emit a bare mapping /
object. In set mode they MUST emit a list / array of mappings /
objects, one per record. Set-mode YAML MUST be a single document (not
multiple `---`-separated documents).

### Source selection

#### REQ: source-selection

`select` MUST accept either `--path=PATH` (local) or
`--github=OWNER/REPO[@REF]` (remote GitHub), but never both. When
neither is provided, the current working directory is used as the
local path. This MUST behave identically in both modes (single-record
and set) and MUST follow
[github-direct-access](../../github-direct-access/README.md) and
[path-targeting](../../path-targeting/README.md).

## Dependencies

- [shared-cli-flags](../../shared-cli-flags/README.md) — defines
  `--from`, `--id`, `--where`, `--order-by`, `--fields`, and the
  mode-selection rules.
- [id-flag-format](../../id-flag-format/README.md) — `--id` syntax.
- [path-targeting](../../path-targeting/README.md) — `--path` resolution.
- [github-direct-access](../../github-direct-access/README.md) —
  `--github` source.
- [output-formats](../../output-formats/README.md) — shared format
  contracts.

## Acceptance Criteria

### AC: single-record-yaml-default

**Requirements:** cli/select#req:subcommand-name, cli/select#req:single-record-shape, cli/select#req:format-flag

Given a local database with a record at `countries/ie`,
`ingitdb select --id=countries/ie` MUST write the record's fields to
stdout as a single YAML mapping and exit `0`.
`ingitdb select --id=countries/ie --format=json` MUST emit a single
JSON object (not an array).

### AC: single-record-not-found

**Requirements:** cli/select#req:single-record-not-found

Given a local database with no record at `countries/zz`,
`ingitdb select --id=countries/zz` MUST exit non-zero with a
diagnostic that names `countries/zz`. Stdout MUST be empty.

### AC: single-record-rejects-set-flags

**Requirements:** cli/select#req:single-record-rejected-flags

`ingitdb select --id=countries/ie --where='active===true'` MUST be
rejected (per `shared-cli-flags#req:where-requires-set-mode`).
`ingitdb select --id=countries/ie --order-by=name` MUST be rejected.
`ingitdb select --id=countries/ie --limit=5` MUST be rejected.
`ingitdb select --id=countries/ie --min-affected=1` MUST be rejected.
`ingitdb select --id=countries/ie --fields='$id,name'` MUST succeed and
project the requested fields onto the output record.

### AC: set-mode-csv-default

**Requirements:** cli/select#req:subcommand-name, cli/select#req:set-mode-shape, cli/select#req:format-flag, cli/select#req:format-tabular-columns

Given a collection `countries` with declared `columns_order: [name,
population]` and three records,
`ingitdb select --from=countries` MUST emit a CSV with a header row
`name,population` followed by three data rows and exit `0`.
`ingitdb select --from=countries --format=yaml` MUST emit a YAML list
of three mappings.

### AC: where-filter-and-order

**Requirements:** cli/select#req:set-mode-shape, cli/select#req:order-then-limit

Given a collection `countries` with records `{ie: pop=5}`, `{us:
pop=330}`, `{de: pop=83}`,
`ingitdb select --from=countries --where='population>10' --order-by='-population' --fields='$id,population'`
MUST emit two rows in the order `us, de` (descending by population, ie
filtered out). Adding `--limit=1` MUST emit only the `us` row.

### AC: set-mode-single-match-still-a-list

**Requirements:** cli/select#req:set-mode-shape, cli/select#req:format-yaml-json-shape

Given a collection `countries` where exactly one record satisfies the
filter,
`ingitdb select --from=countries --where='$id===ie' --format=json` MUST
emit a JSON array containing exactly one object (not a bare object).
`--format=yaml` MUST emit a YAML list of length one (not a bare
mapping). `--format=csv` MUST emit a header row plus one data row.

### AC: empty-set-result

**Requirements:** cli/select#req:set-mode-empty-result

Given a collection `countries` where no record satisfies the filter,
`ingitdb select --from=countries --where='population>100000000'` MUST
exit `0` and emit a CSV containing only the header row.
With `--format=json` it MUST emit `[]`. With `--format=yaml` it MUST
emit `[]`. With `--format=md` it MUST emit the header row only.

### AC: min-affected-fails-below-threshold

**Requirements:** cli/select#req:min-affected-flag

Given a collection `countries` where 2 records match `region==EU`,
`ingitdb select --from=countries --where='region==EU' --min-affected=3`
MUST exit non-zero with a diagnostic stating "matched 2 records,
required at least 3", and MUST NOT write any record to stdout. The
same invocation with `--min-affected=2` MUST emit both matching
records and exit `0`. Combining with `--limit=1` on a 5-record match
with `--min-affected=3` MUST still pass the threshold check (5 ≥ 3)
and then apply the limit, emitting one record.

### AC: min-affected-validation

**Requirements:** cli/select#req:min-affected-flag

`ingitdb select --from=countries --min-affected=0` MUST be rejected.
`ingitdb select --from=countries --min-affected=-1` MUST be rejected.
`ingitdb select --id=countries/ie --min-affected=1` MUST be rejected
(single-record mode).

### AC: limit-validation

**Requirements:** cli/select#req:limit-flag

`ingitdb select --from=countries --limit=-1` MUST be rejected.
`ingitdb select --from=countries --limit=0` MUST behave as if
`--limit` were omitted (no cap).
`ingitdb select --from=countries --limit=10` against a 3-record
collection MUST emit all 3 records and exit `0`.

### AC: reads-from-github

**Requirements:** cli/select#req:source-selection

`ingitdb select --github=owner/repo --id=countries/ie` MUST resolve
the record from the default branch and emit it as YAML (single-record
default). Pinning to a ref (`owner/repo@main`) MUST read from that
ref. Combining `--github` with `--path` MUST be rejected.

### AC: rejects-non-select-flags

**Requirements:** cli/select#req:subcommand-name

`ingitdb select --from=countries --into=other` MUST be rejected
(per `shared-cli-flags#req:into-flag`).
`ingitdb select --from=countries --set='active=true'` MUST be
rejected. `ingitdb select --from=countries --all` MUST be rejected.

## Outstanding Questions

- Should the `read-record` exit code (currently a generic non-zero)
  become a structured code distinct from generic flag errors? Carried
  from the original `read-record` Outstanding Question; defer to a
  cross-cutting exit-code spec if/when one exists.
- Should `--fields` accept a trailing wildcard (`--fields='$id,name,*'`)
  to mean "these first, then everything else"? Defer until a real use
  case appears.
- For set-mode YAML output, should the user be able to opt into the
  `---`-separated multi-document form (e.g. `--format=yaml-stream`)
  for streaming? Defer until streaming use case appears.

---
*This document follows the https://specscore.md/feature-specification*
