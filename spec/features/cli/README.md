---
format: https://specscore.md/feature-specification
---

# CLI Features

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli?op=request-change) |
This directory contains feature specifications for individual `ingitdb` CLI subcommands.
Each subdirectory corresponds to one command. Older verb-noun specs (e.g. `create-record`)
are kept as historical records and are marked **Superseded** in the index below.

Many of these commands are also exposed to AI agents as Claude Code skills in the
[ingitdb-ai-skills](https://github.com/ingitdb/ingitdb-ai-skills) plugin, which wraps the
CLI surface; keep those skills in sync when a command's flags or behavior change here.

## Contents

| Child | Description |
|---|---|
| [select](select/README.md) | The `select` verb queries records from a single collection. Two modes: single-record (`--id`) and set (`--from` + optional `--where`/`--order-by`/`--fields`/`--limit`). Output format defaults to yaml in single-record mode and csv in set mode; `--format` overrides. Replaces `read-record` and `query`. |
| [insert](insert/README.md) | The `insert` verb creates a new record in a collection. Uses `--into` for the target collection and `--key` for the record key (or `$id` in the data as fallback). Accepts `--data`, stdin, `--edit`, or `--empty` as the data source. Rejects when the key already exists. Replaces `create-record`. |
| [update](update/README.md) | The `update` verb applies patch-style changes to records: `--set` adds/changes fields, `--unset` removes fields. Two modes inherited from shared-cli-flags: single-record (`--id`) and set (`--from` + `--where`/`--all`). Shallow patch semantics. Silent on success. `--require-match` opts into non-zero exit when set mode finds zero records. Renames `update-record`. |
| [delete](delete/README.md) | The `delete` verb removes records from a collection. Two modes inherited from shared-cli-flags: single-record (`--id`) and set (`--from` + `--where`/`--all`). `--min-affected=N` opts into non-zero exit when fewer than N records are deleted. Silent on success. Replaces `delete-record` and `delete-records`. |
| [drop](drop/README.md) | The `drop` verb removes schema objects from the database. Two kinds today: `drop collection <name>` and `drop view <name>`. Removes both the schema entry in `.ingitdb.yaml` and any associated data directory in a single git commit. `--if-exists` makes the operation idempotent; `--cascade` also drops dependents. Replaces `delete-collection` and `delete-view`. |
| [describe](describe/README.md) | The `describe` verb (alias `desc`) prints the full definition of a single collection or view. |
| [demo](demo/README.md) | The `demo install` verb creates the TODO demo (To buy and To watch lists, shared with OpenVaultDB's demo) as a new Git-backed database and prints how to browse it in the TUI, query it with `select`, and points to OpenVaultDB's web TODO app, which keeps its own copy. |
| [create-record](create-record/README.md) | Historical: `ingitdb create record` (removed). Superseded by `insert`. |
| [delete-collection](delete-collection/README.md) | Historical: `ingitdb delete collection` (removed). Superseded by `drop collection`. |
| [delete-record](delete-record/README.md) | Historical: `ingitdb delete record` (removed). Superseded by `delete --id`. |
| [delete-records](delete-records/README.md) | Historical: `ingitdb delete records` (removed). Superseded by `delete --from`. |
| [diff](diff/README.md) | The `diff` verb reports record-level changes between two git refs, grouped by collection, at four detail levels. |
| [find](find/README.md) | Withdrawn: record search moved to DataTug CLI. |
| [list-collections](list-collections/README.md) | The `list collections` verb prints every collection ID, locally or from a remote repository, with `--in` and `--filter-name` narrowing. |
| [list-views](list-views/README.md) | The `list views` verb prints every view as `collectionID/viewName`, with `--in` and `--filter-name` narrowing. |
| [materialize](materialize/README.md) | The `materialize` verb regenerates collection READMEs and materialized views, selected by `--collections` and `--views`. |
| [migrate](migrate/README.md) | Withdrawn (deferred): schema-version record migration. |
| [pull](pull/README.md) | The `pull` verb runs `git pull`, auto-resolves generated-file conflicts, rebuilds views and summarises record changes. |
| [query](query/README.md) | Historical: `ingitdb query` (removed). Superseded by `select --from`. |
| [read-record](read-record/README.md) | Historical: `ingitdb read record` (removed). Superseded by `select --id`. |
| [rebase](rebase/README.md) | The `rebase` verb runs `git rebase` and auto-resolves conflicts in generated files. |
| [resolve](resolve/README.md) | The `resolve` verb resolves working-tree merge conflicts: generated files automatically, source data interactively. |
| [self-update](self-update/README.md) | The `self-update` verb updates a manually installed binary in place, or prints the package-manager upgrade command. |
| [serve](serve/README.md) | Withdrawn: the MCP/HTTP `serve` command was removed (ADR 0001). |
| [setup](setup/README.md) | The `setup` verb initialises a new database directory. |
| [truncate](truncate/README.md) | Withdrawn: use `delete --from=COLLECTION --all`. |
| [update-record](update-record/README.md) | Historical: `ingitdb update record` (removed). Renamed to `update`. |
| [validate](validate/README.md) | The `validate` verb checks collection definitions and record files against their schema, fully or for a commit range. |
| [version](version/README.md) | The `version` verb prints build version, commit hash and build date. |
| [watch](watch/README.md) | Withdrawn (deferred): stream record change events to stdout. |

## Index

| Feature | Status | Command |
|---------|--------|---------|
| [version](version/README.md) | Implementing | `ingitdb version` |
| [self-update](self-update/README.md) | Implementing | `ingitdb self-update` |
| [validate](validate/README.md) | Implementing | `ingitdb validate` |
| [select](select/README.md) | Implementing | `ingitdb select` |
| [insert](insert/README.md) | Implementing | `ingitdb insert` |
| [update](update/README.md) | Implementing | `ingitdb update` |
| [delete](delete/README.md) | Implementing | `ingitdb delete` |
| [drop](drop/README.md) | Implementing | `ingitdb drop` |
| [list-collections](list-collections/README.md) | Implementing | `ingitdb list collections` |
| [list-views](list-views/README.md) | Implementing | `ingitdb list views` |
| [rebase](rebase/README.md) | Implementing | `ingitdb rebase` |
| [materialize](materialize/README.md) | Draft | `ingitdb materialize` |
| [diff](diff/README.md) | Draft | `ingitdb diff` |
| [pull](pull/README.md) | Draft | `ingitdb pull` |
| [watch](watch/README.md) | Withdrawn (deferred) | `ingitdb watch` (not implemented) |
| [serve](serve/README.md) | Withdrawn — removed from CLI (ADR 0001) | `ingitdb serve` (removed) |
| [resolve](resolve/README.md) | Draft | `ingitdb resolve` |
| [setup](setup/README.md) | Draft | `ingitdb setup` |
| [demo](demo/README.md) | Implementing | `ingitdb demo install` |
| [find](find/README.md) | Withdrawn — moved to DataTug CLI | `ingitdb find` (not implemented) |
| [truncate](truncate/README.md) | Withdrawn — use `delete --all` | `ingitdb truncate` (not implemented) |
| [migrate](migrate/README.md) | Withdrawn (deferred) | `ingitdb migrate` (not implemented) |
| [read-record](read-record/README.md) | Superseded by [select](select/README.md) | `ingitdb read record` (removed) |
| [create-record](create-record/README.md) | Superseded by [insert](insert/README.md) | `ingitdb create record` (removed) |
| [update-record](update-record/README.md) | Superseded by [update](update/README.md) | `ingitdb update record` (removed) |
| [delete-record](delete-record/README.md) | Superseded by [delete](delete/README.md) | `ingitdb delete record` (removed) |
| [delete-records](delete-records/README.md) | Superseded by [delete](delete/README.md) | `ingitdb delete records` (removed) |
| [delete-collection](delete-collection/README.md) | Superseded by [drop](drop/README.md) | `ingitdb delete collection` (removed) |
| [query](query/README.md) | Superseded by [select](select/README.md) | `ingitdb query` (removed) |

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
