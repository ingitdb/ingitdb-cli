# Features

This directory tracks the SpecScore feature specifications for the **ingitdb-cli** repository. Each feature describes the externally observable behavior of a single CLI command or a cross-cutting concern (flag conventions, output formats, ID syntax). Storage format and collection schema definitions live in the [ingitdb-specs](https://github.com/ingitdb/ingitdb-specs) repository and are referenced from here when needed.

## Index

| Feature | Status | Description |
|---|---|---|
| [version-command](version-command/README.md) | Implementing | `ingitdb version` — print build version, commit hash, and date. |
| [validate-command](validate-command/README.md) | Implementing | `ingitdb validate` — check schema and records against `.ingitdb.yaml`. |
| [read-record-command](read-record-command/README.md) | Implementing | `ingitdb read record` — read a single record by ID. |
| [create-record-command](create-record-command/README.md) | Implementing | `ingitdb create record` — create a new record. |
| [update-record-command](update-record-command/README.md) | Implementing | `ingitdb update record` — patch fields of an existing record. |
| [delete-record-command](delete-record-command/README.md) | Implementing | `ingitdb delete record` — delete a single record by ID. |
| [list-collections-command](list-collections-command/README.md) | Implementing | `ingitdb list collections` — list collection IDs. |
| [rebase-command](rebase-command/README.md) | Implementing | `ingitdb rebase` — rebase with auto-resolution of generated-file conflicts. |
| [find-command](find-command/README.md) | Draft | `ingitdb find` — search records by substring, regex, or exact value. |
| [truncate-command](truncate-command/README.md) | Draft | `ingitdb truncate` — remove all records from a collection. |
| [delete-collection-command](delete-collection-command/README.md) | Draft | `ingitdb delete collection` — remove a collection and its records. |
| [delete-records-command](delete-records-command/README.md) | Draft | `ingitdb delete records` — remove records matching a filter. |
| [query-command](query-command/README.md) | Draft | `ingitdb query` — query and format records from a collection. |
| [materialize-command](materialize-command/README.md) | Draft | `ingitdb materialize` — build materialized views and READMEs. |
| [diff-command](diff-command/README.md) | Draft | `ingitdb diff` — record-level diff between two git refs. |
| [pull-command](pull-command/README.md) | Draft | `ingitdb pull` — pull, auto-resolve, and rebuild views. |
| [watch-command](watch-command/README.md) | Draft | `ingitdb watch` — stream record change events to stdout. |
| [serve-command](serve-command/README.md) | Draft | `ingitdb serve` — MCP, HTTP API, and file-watcher servers. |
| [resolve-command](resolve-command/README.md) | Draft | `ingitdb resolve` — interactive merge-conflict TUI. |
| [setup-command](setup-command/README.md) | Draft | `ingitdb setup` — initialise a new database directory. |
| [migrate-command](migrate-command/README.md) | Draft | `ingitdb migrate` — migrate records between schema versions. |
| [id-flag-format](id-flag-format/README.md) | Implementing | Cross-cutting `--id=<collection-id>/<record-key>` syntax. |
| [github-direct-access](github-direct-access/README.md) | Implementing | Cross-cutting `--github=owner/repo[@ref]` flag and token handling. |
| [output-formats](output-formats/README.md) | Implementing | Cross-cutting `--format=yaml|json` flag and YAML default. |
| [path-targeting](path-targeting/README.md) | Implementing | Cross-cutting `--path` flag and its relation to `--github`. |

## Feature Summaries

### version-command
Prints build version, commit hash, and build date to stdout. The simplest CLI command and a smoke test for the binary.

### validate-command
Validates the `.ingitdb.yaml` definition and every record file against its collection schema. Supports `--only=definition|records` for partial passes and `--from-commit`/`--to-commit` for fast CI mode that only checks files changed in a commit range.

### read-record-command
Reads a single record by `--id` from a local path or directly from a GitHub repository, formatting the result as YAML (default) or JSON.

### create-record-command
Creates a new record in a `map[string]any` collection. Fails when a record with the same key already exists. Works against a local path or a GitHub repository (writes require a token).

### update-record-command
Updates an existing record using patch semantics: only fields supplied in `--set` change; all others are preserved.

### delete-record-command
Deletes a single record by ID. For `SingleRecord` collections the record file is removed; for `MapOfIDRecords` collections the key is removed from the shared map file.

### list-collections-command
Lists collection IDs from a local DB or a GitHub repository, with optional `--in` regex scoping and `--filter-name` glob filtering.

### rebase-command
Runs `git rebase` on top of a base ref and auto-resolves conflicts in generated files (collection `README.md`, materialized views, indexes) when the user opts in via `--resolve`.

### find-command
Searches record fields by substring, regex, or exact match. Supports field whitelisting, sub-path scoping, and a result limit.

### truncate-command
Removes every record from a collection while preserving the collection definition.

### delete-collection-command
Deletes a collection definition and every record file that belongs to it.

### delete-records-command
Bulk-deletes records that match a glob pattern within a collection, optionally scoped by an `--in` sub-path regex.

### query-command
Queries records from a single collection with `--where` filters, `--order-by` sorting, and pluggable output formats (CSV, JSON, YAML, Markdown).

### materialize-command
Renders generated artifacts: collection `README.md` files and materialized view files under `$views/`.

### diff-command
Reports inGitDB record-level changes between two git refs at configurable depth (summary, record, fields, full) and exits non-zero when changes exist for use as a CI guard.

### pull-command
Wraps `git pull` and follows it with automatic conflict resolution for generated files, an interactive TUI for source-data conflicts, and a view rebuild.

### watch-command
Watches the database directory and streams structured add/update/delete events for every record change to stdout in either text or JSON format.

### serve-command
Hosts one or more long-running services in a single process: the MCP server, the HTTP API, and the file watcher.

### resolve-command
Opens an interactive TUI for resolving merge conflicts in inGitDB record files.

### setup-command
Initialises a new inGitDB database directory with a starter `.ingitdb.yaml` and the expected layout.

### migrate-command
Migrates records from one schema version to another for a named target.

### id-flag-format
Defines the `--id=<collection-id>/<record-key>` syntax used by every CRUD command, including the longest-prefix-match rule and the allowed character set for collection IDs.

### github-direct-access
Defines the `--github=owner/repo[@ref]` flag, token resolution via `--token` and `GITHUB_TOKEN`, and the one-commit-per-write rule for remote operations.

### output-formats
Defines the `--format=yaml|json` flag, the YAML default, and the contract that read-style commands obey.

### path-targeting
Defines the `--path` flag, its default of the current working directory, and its mutual exclusivity with `--github`.

## Outstanding Questions

- None at this time.

---
*This document follows the https://specscore.md/feature-specification*
