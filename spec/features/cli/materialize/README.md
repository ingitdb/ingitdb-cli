# Feature: Materialize Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/materialize?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/materialize?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/materialize?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/materialize?op=request-change) |
**Status:** Approved
**Source Ideas:** materialize-collections-and-views
**Supersedes:** —
**Grade:** A

## Summary

The `ingitdb materialize` command regenerates the derived artifacts inGitDB
builds from its source records: per-collection `README.md` files and
materialized view files under each view's configured `$views/` directory.

It is a **single flat command** with two selector flags, `--collections`
and `--views`. Each flag is tri-state: absent, present-without-value
(= every artifact of that type), or present-with-a-list (= only the named
ones). Running `ingitdb materialize` with no flags regenerates everything.
Files are written only when their content differs from what is already on
disk, so repeated runs are idempotent and produce minimal diffs.
`materialize --collections` becomes the single supported way to regenerate
collection READMEs, superseding the deprecated `docs update` command.

## Problem

inGitDB derives several artifacts from its source records — collection
READMEs and view outputs. Keeping those up to date by hand is impossible at
scale. A dedicated `materialize` command makes regeneration a deliberate,
scriptable step that integrates with pull, rebase, and CI.

Two problems motivated this design. First, README generation and view
generation were split across two unrelated commands — `docs update
--collection=GLOB` and `materialize` (views only) — with no way to
regenerate both, or an arbitrary subset, in one run. Second, an earlier
`materialize` design used positional subcommands (`materialize collection`,
`materialize views`) plus a `--views=LIST` flag. That shape was rejected
because it was internally inconsistent (singular `collection` vs plural
`views`) and produced redundant invocations like `materialize views
--views=v1`, where the word "views" appears twice meaning two different
things. With only two artifact types, two well-named flags on a flat
command are shorter, composable, and remove the redundancy — and let a
single command own all derived-artifact regeneration.

## Behavior

### Invocation

#### REQ: flat-command

The command MUST be invoked as `ingitdb materialize`. It MUST NOT use
positional subcommands. Artifact type is chosen by the `--collections`
and/or `--views` flags, never by a positional argument.

#### REQ: bare-command-materializes-all

`ingitdb materialize` with neither `--collections` nor `--views` supplied
MUST regenerate every artifact of both types: all collection READMEs and
all materialized views.

### Selection

#### REQ: collections-flag

`--collections` MUST select collection-README regeneration:

- present without a value (`--collections`) MUST target every collection.
- present with a value (`--collections=PATTERNS`) MUST target only the
  collections whose IDs match `PATTERNS`.

`PATTERNS` is a list of glob patterns separated by `,` (canonical) or `;`.
Glob semantics MUST match those already accepted by `docs update
--collection` (e.g. `agile.teams/*`, `agile.teams/**`).

When `--collections` is absent, no collection README MUST be written
(unless `--views` is also absent, per `req:bare-command-materializes-all`).

#### REQ: views-flag

`--views` MUST select materialized-view regeneration:

- present without a value (`--views`) MUST target every view.
- present with a value (`--views=PATTERNS`) MUST target only the views
  whose names match `PATTERNS`.

`PATTERNS` is a list of glob patterns separated by `,` (canonical) or `;`,
with the same glob semantics as `--collections`.

When `--views` is absent, no view MUST be written (unless `--collections`
is also absent, per `req:bare-command-materializes-all`).

#### REQ: combined-selection

`--collections` and `--views` MUST be combinable in a single invocation,
each independently in any of its three states. For example,
`materialize --views=v1 --collections=c1,c2` MUST regenerate only view
`v1` and the READMEs of collections `c1` and `c2`.

#### REQ: equals-syntax-for-values

Because each selector flag is tri-state (the bare flag means "all"),
a list value MUST be attached with `=` (`--views=v1,v2`). The
space-separated form (`--views v1,v2`) is NOT supported: with a bare flag
carrying an implicit "all" default, `v1,v2` would be parsed as a
positional argument rather than the flag's value. Help text MUST document
the `=` requirement.

### Consolidation

#### REQ: supersedes-docs-update

`materialize --collections` MUST be the single supported way to regenerate
collection READMEs, routing through the same `docsbuilder` logic that
`ingitdb docs update --collection` uses today. The `docs update` command
MUST be marked deprecated, and its help text MUST direct users to
`ingitdb materialize --collections`.

### Output

#### REQ: write-only-on-change

Each regenerated file MUST be written only when its rendered content
differs from the file already on disk. Unchanged files MUST be left
untouched (no rewrite, no mtime churn, no spurious git diff).

#### REQ: records-delimiter

`--records-delimiter=N` MUST control the INGR record-delimiter behavior of
**view** output only, with values `1` (enabled), `-1` (disabled), and `0`
/ omitted (use the view/project default; the application default is `1`).
When the invocation regenerates no views (e.g. `--collections` only), the
flag MUST have no effect.

#### REQ: success-output

On success `materialize` MUST exit `0`. A summary of how many files were
created, updated, deleted, and left unchanged MUST be written to stderr.
stdout MUST stay silent so the command composes in scripts and pipelines.

### Source selection

#### REQ: source-selection

`materialize` MUST accept `--path=PATH` to locate the database directory,
per [path-targeting](../../path-targeting/README.md). When `--path` is
omitted, the current working directory is used.

## Dependencies

- [path-targeting](../../path-targeting/README.md) — `--path`.

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/materialize`):

- [`cmd/ingitdb/commands/materialize.go`](../../../cmd/ingitdb/commands/materialize.go)
- [`cmd/ingitdb/commands/view_builder_helper.go`](../../../cmd/ingitdb/commands/view_builder_helper.go)
- [`cmd/ingitdb/commands/flags.go`](../../../cmd/ingitdb/commands/flags.go)
- [`cmd/ingitdb/commands/docs.go`](../../../cmd/ingitdb/commands/docs.go)
- [`cmd/ingitdb/commands/docs_update.go`](../../../cmd/ingitdb/commands/docs_update.go)
- [`pkg/ingitdb/materializer/view_builder.go`](../../../pkg/ingitdb/materializer/view_builder.go)
- [`pkg/ingitdb/docsbuilder/update.go`](../../../pkg/ingitdb/docsbuilder/update.go)
- [`pkg/ingitdb/docsbuilder/collection_readme.go`](../../../pkg/ingitdb/docsbuilder/collection_readme.go)

## Acceptance Criteria

### AC: bare-materializes-everything

**Requirements:** cli/materialize#req:flat-command, cli/materialize#req:bare-command-materializes-all, cli/materialize#req:write-only-on-change, cli/materialize#req:success-output

Given a database with collections `cities` and `teams` and a materialized
view `active_cities`, `ingitdb materialize` (no flags) MUST regenerate the
README of every collection and rebuild every view. Files whose content is
unchanged MUST NOT be rewritten. The command MUST exit `0`, print a
created/updated/deleted/unchanged summary to stderr, and write nothing to
stdout.

### AC: views-only-all

**Requirements:** cli/materialize#req:views-flag

`ingitdb materialize --views` MUST rebuild every materialized view and MUST
NOT write any collection README.

### AC: collections-only-all

**Requirements:** cli/materialize#req:collections-flag

`ingitdb materialize --collections` MUST regenerate every collection README
and MUST NOT write any view file.

### AC: views-subset

**Requirements:** cli/materialize#req:views-flag

Given views `active_cities` and `large_cities`,
`ingitdb materialize --views=active_cities` MUST rebuild only
`active_cities`. `large_cities` and all collection READMEs MUST be left
untouched.

### AC: collections-glob-targeting

**Requirements:** cli/materialize#req:collections-flag

Given nested collections `agile.teams/alpha` and `agile.teams/beta`,
`ingitdb materialize --collections='agile.teams/**'` MUST regenerate the
READMEs of both. `ingitdb materialize --collections='cities;teams'`
(semicolon-separated) MUST target collections `cities` and `teams`.

### AC: combined-subset

**Requirements:** cli/materialize#req:combined-selection

`ingitdb materialize --views=active_cities --collections=cities,teams` MUST
rebuild only view `active_cities` and only the READMEs of collections
`cities` and `teams`. No other artifact MUST be written.

### AC: equals-syntax-required

**Requirements:** cli/materialize#req:equals-syntax-for-values

`ingitdb materialize --views=active_cities` MUST target view
`active_cities`. The space-separated form
`ingitdb materialize --views active_cities` MUST NOT bind `active_cities`
to `--views` (it is treated as a positional argument and rejected); the
bare `--views` form means "all views".

### AC: docs-update-deprecated

**Requirements:** cli/materialize#req:supersedes-docs-update

`ingitdb docs update --help` MUST display a deprecation notice directing
users to `ingitdb materialize --collections`. For the same glob,
`ingitdb materialize --collections=GLOB` MUST produce the identical README
output that `ingitdb docs update --collection=GLOB` produced.

### AC: idempotent-second-run

**Requirements:** cli/materialize#req:write-only-on-change

Running `ingitdb materialize` twice in a row MUST result in zero files
written on the second run (all reported as unchanged), because no source
records changed between runs.

### AC: records-delimiter-applies-to-views

**Requirements:** cli/materialize#req:records-delimiter

`ingitdb materialize --views --records-delimiter=-1` MUST disable the
`#-` record delimiter in regenerated INGR view output.
`ingitdb materialize --collections --records-delimiter=-1` MUST behave
identically to `ingitdb materialize --collections` (the flag has no effect
when no view is regenerated).

## Open Questions

- Should `docs update` be removed outright, or kept as a thin deprecated
  alias that forwards to `materialize --collections` for one release before
  removal? Decide at plan time; grep CI configs and docs for existing
  `docs update` usage before choosing hard removal.
- Should `materialize` support `--remote` to write generated files back to
  a remote repository in a single commit (as `drop` does)? Deferred to a
  follow-up after the local MVP lands.

---
*This document follows the https://specscore.md/feature-specification*
