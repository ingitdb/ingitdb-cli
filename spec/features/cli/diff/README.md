# Feature: Diff Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/diff?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/diff?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/diff?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/diff?op=request-change) |
**Status:** Implementing

## Summary

The `ingitdb diff` command compares two git refs and reports inGitDB record-level changes — added, updated, and deleted records grouped by collection. It supports four detail levels (`summary`, `record`, `fields`, `full`) and four output formats (`text`, `json`, `yaml`, `toml`). Exit code `1` on changes makes the command suitable as a CI guard.

## Problem

`git diff` operates on raw text and cannot distinguish a meaningful field change from a YAML reformat. Reviewers and CI need a tool that speaks records, fields, and collections natively, with enough depth control to summarize a release or audit a single field.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb diff`. The first positional argument MAY be `<ref>` (compare ref against HEAD), `<ref>..<ref>` (compare two refs), or omitted (working tree against HEAD).

### Scoping flags

#### REQ: scoping-flags

`--collection=KEY` and `--view=VIEW_KEY` MUST be mutually exclusive and limit the diff to a single collection or view. With `--view`, `--view-mode=output|source` MUST select between diffing the generated output file (`output`, default) or the underlying source records (`source`). `--path-filter=PATTERN` MUST further narrow by record path prefix or glob.

### Detail control

#### REQ: depth

The `--depth=summary|record|fields|full` flag MUST control output detail: `summary` prints per-collection counts; `record` prints one line per record with commit count and short hashes; `fields` adds the list of changed field names; `full` adds before/after values for each changed field. The default MUST be `summary`.

#### REQ: format

The `--format=text|json|yaml|toml` flag MUST select the output format. The default MUST be `text`.

### Exit codes

#### REQ: exit-codes

The command MUST exit `0` when no changes are found, `1` when changes are found, and `2` on infrastructure errors (bad flags, git failures, etc.).

## Dependencies

- path-targeting

## Implementation

Source files (annotated with `// specscore: feature/cli/diff`):

- [`cmd/ingitdb/commands/diff.go`](../../../cmd/ingitdb/commands/diff.go)

Reuses `pkg/ingitdb/gitdiff` (changed-file listing) and
`pkg/ingitdb/datavalidator.CollectionForRecordFile` (file→collection mapping).

## Acceptance Criteria

### AC: refs-and-record-changes

**Requirements:** cli/diff#req:subcommand-name, cli/diff#req:depth

`ingitdb diff <a>..<b>` reports added/updated/deleted records grouped by
collection. A bare `<ref>` compares that ref against HEAD; an omitted argument
compares the working tree against HEAD. `--depth=summary` (default) prints
per-collection counts; `record` lists one line per changed record; `fields`
adds changed field names; `full` adds before/after values per field.

### AC: format

**Requirements:** cli/diff#req:format

`--format=text|json|yaml|toml` (default `text`) renders the same depth-scoped
result in the chosen format; the structured formats round-trip
(e.g. JSON parses back into the record-change list).

### AC: scoping

**Requirements:** cli/diff#req:scoping-flags

`--collection=KEY` limits the diff to one collection; `--path-filter=PATTERN`
narrows by record-path prefix/glob.

### AC: exit-codes

**Requirements:** cli/diff#req:exit-codes

The command exits `0` when no record changes are found and `1` when changes
are found (suitable as a CI guard).

## Scope (current implementation)

Implemented: all three ref forms; `summary`/`record`/`fields`/`full` depth;
`text`/`json`/`yaml`/`toml` format; `--collection` and `--path-filter`; exit
`0`/`1`.

Deferred / not yet implemented:
- **`--view` / `--view-mode`** — diffing a view's generated output or source
  records (`--view` currently returns a clear "not yet implemented" error).
- **`record` depth commit metadata** — per-record commit count and short
  hashes are not yet shown (only the change kind).
- **Exit code `2`** — infrastructure errors currently exit `1` (non-zero), not
  a distinct `2`; that granularity needs main-loop exit-code support.

## Open Questions

- Should `--depth=full` redact secrets the way some logging frameworks do?
- Should the JSON output schema be versioned and documented as a stable contract?

---
*This document follows the https://specscore.md/feature-specification*
