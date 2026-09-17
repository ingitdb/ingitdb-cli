---
format: https://specscore.md/features-index-specification
---

# Features

This directory tracks the SpecScore feature specifications for the **ingitdb-cli** repository. Each feature describes the externally observable behavior of a single CLI command or a cross-cutting concern (flag conventions, output formats, ID syntax). Storage format and collection schema definitions live in the [ingitdb-specs](https://github.com/ingitdb/ingitdb-specs) repository and are referenced from here when needed.

## Index

| Feature | Status | Description |
|---|---|---|
| [Record Format Extensions](record-format/README.md) | Stable | Three small, additive extensions to inGitDB's existing record-format machinery: (1) **CSV** as a seventh first-class format alongside the existing six (`yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`); (2) a **project-level `default_record_format` config field** in `.ingitdb/settings.yaml` with a centralized fallback resolver (collection → project → hard YAML default); (3) a **`--default-format` CLI flag** on `ingitdb setup`. The collection-level `--format` flag is reserved for a future `ingitdb create-collection` command and explicitly deferred from this Feature batch. |
| [cli/version](cli/version/README.md) | Implementing | `ingitdb version` — print build version, commit hash, and date. |
| [cli/validate](cli/validate/README.md) | Implementing | `ingitdb validate` — check schema and records against `.ingitdb.yaml`. |
| [cli/select](cli/select/README.md) | Implementing | `ingitdb select` — read a single record (`--id`) or query a set of records (`--from`/`--where`). |
| [cli/insert](cli/insert/README.md) | Implementing | `ingitdb insert` — create a new record (`--into`/`--key`). |
| [cli/update](cli/update/README.md) | Implementing | `ingitdb update` — patch fields of one or more records. |
| [cli/delete](cli/delete/README.md) | Implementing | `ingitdb delete` — delete records by ID or by `--from`/`--where`. |
| [cli/drop](cli/drop/README.md) | Implementing | `ingitdb drop` — drop a collection or view. |
| [cli/list-collections](cli/list-collections/README.md) | Implementing | `ingitdb list collections` — list collection IDs. |
| [cli/list-views](cli/list-views/README.md) | Implementing | `ingitdb list views` — list views as `collectionID/viewName`. |
| [cli/rebase](cli/rebase/README.md) | Implementing | `ingitdb rebase` — rebase with auto-resolution of generated-file conflicts. |
| [cli/read-record](cli/read-record/README.md) | Superseded by [cli/select](cli/select/README.md) | `ingitdb read record` (removed). |
| [cli/create-record](cli/create-record/README.md) | Superseded by [cli/insert](cli/insert/README.md) | `ingitdb create record` (removed). |
| [cli/update-record](cli/update-record/README.md) | Superseded by [cli/update](cli/update/README.md) | `ingitdb update record` (removed). |
| [cli/delete-record](cli/delete-record/README.md) | Superseded by [cli/delete](cli/delete/README.md) | `ingitdb delete record` (removed). |
| [cli/delete-records](cli/delete-records/README.md) | Superseded by [cli/delete](cli/delete/README.md) | `ingitdb delete records` (removed). |
| [cli/delete-collection](cli/delete-collection/README.md) | Superseded by [cli/drop](cli/drop/README.md) | `ingitdb delete collection` (removed). |
| [cli/query](cli/query/README.md) | Superseded by [cli/select](cli/select/README.md) | `ingitdb query` (removed). |
| [cli/materialize](cli/materialize/README.md) | Draft | `ingitdb materialize` — build materialized views and READMEs. |
| [cli/diff](cli/diff/README.md) | Draft | `ingitdb diff` — record-level diff between two git refs. |
| [cli/pull](cli/pull/README.md) | Draft | `ingitdb pull` — pull, auto-resolve, and rebuild views. |
| [cli/watch](cli/watch/README.md) | Draft | `ingitdb watch` — stream record change events to stdout. |
| [cli/serve](cli/serve/README.md) | Draft | `ingitdb serve` — MCP, HTTP API, and file-watcher servers. |
| [cli/resolve](cli/resolve/README.md) | Draft | `ingitdb resolve` — interactive merge-conflict TUI. |
| [cli/setup](cli/setup/README.md) | Draft | `ingitdb setup` — initialise a new database directory. |
| [cli/demo](cli/demo/README.md) | Draft | `ingitdb demo install` — create the TODO demo database shared with OpenVaultDB. |
| [cli/find](cli/find/README.md) | Withdrawn — moved to DataTug CLI | `ingitdb find` (not implemented). |
| [cli/truncate](cli/truncate/README.md) | Withdrawn — use `delete --all` | `ingitdb truncate` (not implemented). |
| [cli/migrate](cli/migrate/README.md) | Withdrawn (deferred) | `ingitdb migrate` (not implemented). |
| [ID Flag Format](id-flag-format/README.md) | Stable | The `--id` flag, used by every CRUD command, follows the syntax `<collection-id>/<record-key>`. The collection ID is dot-separated and uses a restricted character set; the record key follows after a single `/`. When prefixes overlap, the longest matching collection ID wins. |
| [output-formats](output-formats/README.md) | Stable | Cross-cutting `--format=yaml|json` flag and YAML default. |
| [Path Targeting](path-targeting/README.md) | Stable | The `--path=PATH` flag selects the local database directory that a command operates on. When omitted, commands default to the current working directory. `--path` is mutually exclusive with `--remote` on every command that supports both: a single invocation targets either a local directory or a remote repository, never both. |
| [Remote Repository Access](remote-repo-access/README.md) | Stable | Several commands accept `--remote=<value>` as a substitute for `--path` to read or write records directly against a remote Git hosting service (GitHub, GitLab, Bitbucket, and self-hosted instances) over the service's REST API, without a local clone. Authentication is provided via `--token` or host-derived environment variables. Each successful write operation MUST result in exactly one commit in the remote repository. |
| [shared-cli-flags](shared-cli-flags/README.md) | Single source of truth for the CLI flag grammar shared across select, insert, update, delete, and drop verbs: --from, --into, --where, --set, --id, --all, --order-by, --fields. Defines parsing rules, operator semantics (==, ===, !=, !==, >=, <=, >, <), value-type model, and flag mutual-exclusion rules. |
| [CLI Features](cli/README.md) | Unknown | TODO: Add description. |
| [dbschema + ddl + ConcurrencyAware Coverage for inGitDB](dalgo2ingitdb-dbschema-ddl-coverage/README.md) | Stable | `dalgo2ingitdb` is the DALgo driver for the inGitDB project format — a git repository where each collection is a directory of files (yaml/json/markdown/toml/ingr/csv) with schema declared in `.collection/definition.yaml`. Today the package provides `dal.DB` read/write for records (Get, Set, Delete, queries) via `CollectionForKey` and format-aware parsers, but does NOT implement the schema-management capability interfaces shipped in `dal-go/dalgo`. |
| [DALgo Referential Integrity](dalgo2ingitdb-referential-integrity/README.md) | Stable | DALgo adapter callers get write-time referential integrity for schemas that already declare `foreign_key` on collection columns. `Set`, `Insert`, and `Update` reject non-empty references to missing parent records, and `Delete` rejects removing a parent record while child records still point at it. |
| [Record Key](record-key/README.md) | Draft | Record-key Features define how inGitDB resolves, validates, and exposes the canonical identity of a record across schema validation, reads, writes, and CLI commands. |
| [Computed Columns (Inline Starlark Formulas)](computed-columns/README.md) | Stable | Lets a schema author add an inline Starlark formula to a collection column so its value is computed from the record's other fields at read time, rather than stored. It serves schema authors who want derived values (full names, labels, simple arithmetic) that stay in sync with their source fields and never drift in git. |
| [Computed Columns via dalgo (lazy delegation)](computed-columns-via-dalgo/README.md) | Stable | Moves computed-column (FORMULA) value computation out of ingitdb's eager read pipeline and onto dalgo's `recordset.Evaluator` contract (dalgo v0.46.0). ingitdb keeps owning the Starlark language and schema validation, but delegates the *computation wiring* to dalgo so computed values are resolved **lazily, per accessed column**, through `recordset.Row`. It serves ingitdb users by avoiding needless formula evaluation — a computed column is only computed when a consumer actually reads it. |
| [TUI lazy computed-cell evaluation](tui-lazy-computed-cells/README.md) | Stable | Makes the TUI collection screen evaluate a computed (FORMULA) column only for the cells it actually paints — the visible row × column window — so hidden and off-viewport computed columns are never evaluated. It serves ingitdb users browsing collections with expensive computed columns by keeping the screen responsive regardless of row count. |
| [Subcollection Addressing](subcollection-addressing/README.md) | Draft | `ingitdb select --from` and the terminal UI reach the records of a subcollection, not only of a root collection. `--from=lists/to-buy/items` returns the `items` of the list record `to-buy`, with every set-mode flag working as it does for a root collection, and the terminal UI lets a person open a record's declared subcollection and go back with Esc. |

## Feature Summaries

### cli/version
Prints build version, commit hash, and build date to stdout. The simplest CLI command and a smoke test for the binary.

### cli/validate
Validates the `.ingitdb.yaml` definition and every record file against its collection schema. Supports `--only=definition|records` for partial passes and `--from-commit`/`--to-commit` for fast CI mode that only checks files changed in a commit range.

### cli/select
Reads a single record by `--id` (yaml default) or queries a set of records from a collection via `--from` with optional `--where`/`--order-by`/`--fields`/`--limit` (csv default). Replaces the legacy `read record` and `query` commands.

### cli/insert
Creates a new record in a collection using `--into=COLLECTION` and `--key=KEY` (or `$id` in the supplied data). Accepts data via `--data`, stdin, or `--edit`. Fails when the key already exists. Replaces the legacy `create record` command.

### cli/update
Applies patch-style updates: `--set` adds/changes fields, `--unset` removes fields. Works in single-record mode (`--id`) or set mode (`--from` + `--where`/`--all`). Replaces the legacy `update record` command.

### cli/delete
Deletes records in single-record mode (`--id`) or set mode (`--from` + `--where`/`--all`). For `SingleRecord` collections the record file is removed; for `MapOfIDRecords` collections only the matching key is removed. Replaces the legacy `delete record` and `delete records` commands.

### cli/drop
Drops schema objects: `drop collection <name>` and `drop view <name>`. Removes both the schema entry and any associated data directory in a single git commit. `--if-exists` for idempotence; `--cascade` to drop dependents. Replaces the legacy `delete collection` and `delete view` commands.

### cli/list-collections
Lists collection IDs from a local DB or a GitHub repository, with optional `--in` regex scoping and `--filter-name` glob filtering.

### cli/list-views
Lists views across all collections (recursing into subcollections) as sorted `collectionID/viewName` identifiers, with optional `--in` regex scoping (on the owning collection path) and `--filter-name` glob filtering (on the bare view name).

### cli/rebase
Runs `git rebase` on top of a base ref and auto-resolves conflicts in generated files (collection `README.md`, materialized views, indexes) when the user opts in via `--resolve`.

### cli/materialize
Renders generated artifacts: collection `README.md` files and materialized view files under `$views/`.

### cli/diff
Reports inGitDB record-level changes between two git refs at configurable depth (summary, record, fields, full) and exits non-zero when changes exist for use as a CI guard.

### cli/pull
Wraps `git pull` and follows it with automatic conflict resolution for generated files, an interactive TUI for source-data conflicts, and a view rebuild.

### cli/watch
Watches the database directory and streams structured add/update/delete events for every record change to stdout in either text or JSON format.

### cli/serve
Hosts one or more long-running services in a single process: the MCP server, the HTTP API, and the file watcher.

### cli/resolve
Opens an interactive TUI for resolving merge conflicts in inGitDB record files.

### cli/setup
Initialises a new inGitDB database directory with a starter `.ingitdb.yaml` and the expected layout.

### cli/demo
Creates the TODO demo (To buy and To watch lists, defined once in `ingitdb-go` and shared with OpenVaultDB) as a new Git repository, and prints next steps for the TUI, `select` and OpenVaultDB's web TODO app.

### id-flag-format
Defines the `--id=<collection-id>/<record-key>` syntax used by every CRUD command, including the longest-prefix-match rule and the allowed character set for collection IDs.

### output-formats
Defines the `--format=yaml|json` flag, the YAML default, and the contract that read-style commands obey.

### path-targeting
Defines the `--path` flag, its default of the current working directory, and its mutual exclusivity with `--remote`.

### remote-repo-access

Defines the `--remote=<URL>` flag for direct access to remote Git hosting services (GitHub, GitLab, Bitbucket, and self-hosted instances), with built-in provider inference, `--provider` override for unknown hosts, host-derived token environment variables, and the one-commit-per-write rule.

### subcollection-addressing

Lets `select --from` take a subcollection path (`lists/to-buy/items`) with every set-mode flag, and lets the terminal UI open a record's declared subcollection and return with Esc. Record paths stay the storage driver's.

### dalgo2ingitdb-referential-integrity

Enforces existing `foreign_key` metadata in DALgo write transactions. Child writes fail when non-empty FK values point at missing parent records, and parent deletes fail while child records still reference them.

## Open Questions

- None at this time.

---
*This document follows the https://specscore.md/feature-specification*

---
*This document follows the https://specscore.md/features-index-specification*
