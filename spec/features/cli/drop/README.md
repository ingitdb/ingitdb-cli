# Feature: Drop

**Status:** Approved

## Summary

The `ingitdb drop` command removes schema objects from the database.
Two kinds are supported today: `drop collection <name>` and
`drop view <name>`. The kind is a positional subcommand chosen first;
the object name is the next positional argument. Each successful
invocation removes both the schema entry in `.ingitdb.yaml` and any
associated data directory in a single git commit, mirroring SQL's
`DROP TABLE` / `DROP VIEW`. `--if-exists` makes the operation
idempotent; `--cascade` also drops dependents. `drop` replaces and
consolidates the prior `delete collection` and `delete view` commands.

## Problem

The two predecessor commands (`delete collection`, `delete view`) used
`delete` for both record removal and schema-object removal — making
`delete` semantically overloaded. SQL distinguishes `DELETE` (rows)
from `DROP` (schema). The redesign moves all record removal to
`delete` and consolidates schema-object removal under `drop`, with a
positional kind discriminator (`collection` / `view`) that scales
cleanly to future kinds (`index`, `materialized-view`) via cobra
subcommands and tab-completion.

## Behavior

### Invocation

#### REQ: subcommand-shape

The command MUST be invoked as `ingitdb drop <kind> <name>`, where
`<kind>` is the positional subcommand selector and `<name>` is the
positional name of the object to drop. Today `<kind>` MUST be one of
`collection` or `view`. Unknown kinds MUST be rejected with a
diagnostic that lists the supported kinds.

#### REQ: name-positional

The object name MUST be supplied positionally (e.g.
`drop collection cities`). It MUST NOT be supplied via a `--name` or
similar flag. The name MUST follow the collection ID character rules
defined by [id-flag-format#req:collection-id-charset](../../id-flag-format/README.md)
(alphanumeric and `.`, starting and ending with alphanumeric, no `/`).
View names follow the same character rules.

#### REQ: shared-flag-rejections

`drop` MUST reject every shared-flag that targets records or
collections by query: `--id`, `--from`, `--into`, `--where`, `--set`,
`--unset`, `--all`, `--min-affected`, `--order-by`, `--fields` (per
the applicability rules in `shared-cli-flags`).

### Drop semantics

#### REQ: drop-collection-removes-schema-and-data

`drop collection <name>` MUST:

1. Remove the collection's entry from `.ingitdb.yaml`.
2. Remove the collection's data directory and every record file
   inside it.
3. Commit both changes in a single git commit on the local working
   tree (or, with `--github`, a single remote commit).

Partial drops (schema-only or data-only) MUST NOT happen; either
both succeed or neither does.

#### REQ: drop-view-removes-schema-and-materialized-files

`drop view <name>` MUST:

1. Remove the view's entry from `.ingitdb.yaml`.
2. Remove any materialized output files produced by that view.
3. Commit both changes in a single git commit.

The source records (records in the underlying collections that the
view reads from) MUST NOT be touched.

### Idempotency

#### REQ: missing-target-error-by-default

When `<name>` does not exist as the named `<kind>`, `drop` MUST exit
non-zero with a diagnostic that names the missing kind and name.
No write MUST occur.

#### REQ: if-exists-flag

The `--if-exists` boolean flag MUST suppress the missing-target
error. When supplied and the target does not exist, `drop` MUST exit
`0` and write nothing to the database. When supplied and the target
exists, `drop` MUST proceed normally.

### Dependents and cascade

#### REQ: dependents-error-by-default

When the target has dependents (e.g. a view's definition references
a collection being dropped, or a materialized-view depends on the
view being dropped), `drop` MUST exit non-zero by default with a
diagnostic that names each dependent. No write MUST occur.

#### REQ: cascade-flag

The `--cascade` boolean flag MUST opt the invocation into dropping
every dependent of the target as part of the same operation. All
dropped objects (the named target plus every transitively dependent
schema object and any associated data) MUST be removed in a single
git commit. The commit message serves as the audit trail of which
objects were cascaded; per `req:success-output`, stdout stays silent
on success.

### Output and exit

#### REQ: success-output

On success, `drop` MUST exit `0` and write nothing to stdout.
Diagnostic and progress messages MUST go to stderr.

### Source selection

#### REQ: source-selection

`drop` MUST accept either `--path=PATH` (local) or
`--github=OWNER/REPO[@REF]` (remote GitHub), but never both, per
[path-targeting](../../path-targeting/README.md) and
[github-direct-access](../../github-direct-access/README.md). When
neither is provided, the current working directory is used.

#### REQ: github-write-requires-token

For `--github` writes, a token MUST be supplied via `--token` or the
`GITHUB_TOKEN` environment variable. Each successful invocation MUST
produce exactly one commit in the remote repository, even when
`--cascade` drops multiple objects.

## Dependencies

- [shared-cli-flags](../../shared-cli-flags/README.md) — applicability
  rejections for the flags `drop` does NOT accept.
- [path-targeting](../../path-targeting/README.md) — `--path`.
- [github-direct-access](../../github-direct-access/README.md) —
  `--github`.

## Acceptance Criteria

### AC: drop-collection-success

**Requirements:** cli/drop#req:subcommand-shape, cli/drop#req:name-positional, cli/drop#req:drop-collection-removes-schema-and-data, cli/drop#req:success-output, cli/drop#req:github-write-requires-token

Given a database where `.ingitdb.yaml` declares collection `cities`
with 12 records under `cities/`,
`ingitdb drop collection cities` MUST remove the `cities` entry from
`.ingitdb.yaml`, delete the `cities/` directory and all 12 record
files, and produce one git commit. The command MUST exit `0` and
write nothing to stdout. Targeting the same operation against a
GitHub repository MUST produce exactly one remote commit.

### AC: drop-view-success

**Requirements:** cli/drop#req:drop-view-removes-schema-and-materialized-files

Given a database where `.ingitdb.yaml` declares a view
`active_cities` with materialized output at `views/active_cities/`,
`ingitdb drop view active_cities` MUST remove the view's entry and
its materialized files in one commit. The records in the underlying
collection (`cities`) MUST remain untouched.

### AC: unknown-kind-rejected

**Requirements:** cli/drop#req:subcommand-shape

`ingitdb drop schema cities` MUST be rejected with a diagnostic
listing the supported kinds (`collection`, `view`). `ingitdb drop`
(no kind, no name) MUST also be rejected, as MUST `ingitdb drop
collection` (no name).

### AC: name-as-flag-rejected

**Requirements:** cli/drop#req:name-positional

`ingitdb drop collection --name=cities` MUST be rejected — the name
is positional, not a flag.

### AC: missing-target-fails-by-default

**Requirements:** cli/drop#req:missing-target-error-by-default

`ingitdb drop collection mars_cities` against a database without a
`mars_cities` collection MUST exit non-zero with a diagnostic naming
the kind (`collection`) and name (`mars_cities`). `.ingitdb.yaml`
MUST NOT be modified.

### AC: if-exists-idempotent

**Requirements:** cli/drop#req:if-exists-flag

`ingitdb drop collection mars_cities --if-exists` against a database
without a `mars_cities` collection MUST exit `0` and perform no
write. Repeating the same invocation MUST also exit `0` (idempotent).
With an existing collection, `ingitdb drop collection cities
--if-exists` MUST behave identically to `ingitdb drop collection
cities` (success, schema + data removed, one commit).

### AC: dependent-blocks-drop

**Requirements:** cli/drop#req:dependents-error-by-default

Given a database where view `active_cities` references collection
`cities`, `ingitdb drop collection cities` MUST exit non-zero with a
diagnostic naming `active_cities` as the blocking dependent.
`.ingitdb.yaml` MUST NOT be modified.

### AC: cascade-drops-dependents

**Requirements:** cli/drop#req:cascade-flag, cli/drop#req:github-write-requires-token

Given the same database as the previous AC,
`ingitdb drop collection cities --cascade` MUST drop both `cities`
(schema entry + data) and `active_cities` (schema entry + materialized
files) in a single git commit. The command MUST exit `0` and write
nothing to stdout. Targeting GitHub MUST produce exactly one remote
commit covering all dropped objects.

### AC: cascade-without-dependents

**Requirements:** cli/drop#req:cascade-flag

`ingitdb drop collection cities --cascade` against a database where
`cities` has NO dependents MUST behave identically to `ingitdb drop
collection cities` (success, schema + data removed, one commit).
`--cascade` MUST NOT error when there is nothing to cascade.

### AC: rejects-shared-flags

**Requirements:** cli/drop#req:shared-flag-rejections

`ingitdb drop collection cities --id=cities/ie` MUST be rejected
(per `shared-cli-flags#req:id-flag`).
`ingitdb drop collection cities --from=other` MUST be rejected
(per `shared-cli-flags#req:from-flag`).
`ingitdb drop collection cities --into=other` MUST be rejected
(per `shared-cli-flags#req:into-flag`).
`ingitdb drop collection cities --where='active===true'` MUST be
rejected.
`ingitdb drop collection cities --set='active=false'` MUST be
rejected.
`ingitdb drop collection cities --unset=active` MUST be rejected
(per `shared-cli-flags#req:unset-applicability`).
`ingitdb drop collection cities --all` MUST be rejected (per
`shared-cli-flags#req:all-flag`).
`ingitdb drop collection cities --order-by=name` MUST be rejected
(per `shared-cli-flags#req:order-by-applicability`).
`ingitdb drop collection cities --fields=name` MUST be rejected
(per `shared-cli-flags#req:fields-applicability`).
`ingitdb drop collection cities --min-affected=1` MUST be rejected
(per `shared-cli-flags#req:min-affected-applicability`).

## Outstanding Questions

- Should `drop` support additional kinds in MVP (`drop index <name>`,
  `drop materialized-view <name>`), or are `collection` and `view`
  enough? The Idea cited `index` and `materialized-view` as
  motivation for the positional-subcommand shape, but the spec only
  commits to the current two kinds; expansion is additive.
- Should the dropped objects' final state appear as a single commit
  with a structured commit message (e.g. listing each dropped object
  on its own line for cascade), or a single one-line subject?
  Implementation detail; defer to plan time.
- Does `--cascade` need a dry-run mode (`--cascade --dry-run`) that
  lists what would be dropped without doing it? Defer until a real
  use case appears; for now `select`-based inspection of the
  dependency graph is the workaround.

---
*This document follows the https://specscore.md/feature-specification*
