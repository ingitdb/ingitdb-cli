# Features

This directory tracks the SpecScore feature specifications for the **ingitdb-cli** repository. Each feature describes the externally observable behavior of a single CLI command or a cross-cutting concern (flag conventions, output formats, ID syntax). Storage format and collection schema definitions live in the [ingitdb-specs](https://github.com/ingitdb/ingitdb-specs) repository and are referenced from here when needed.

## Index

| Feature | Status | Description |
|---|---|---|
| [cli/version](cli/version/README.md) | Implementing | `ingitdb version` — print build version, commit hash, and date. |
| [cli/validate](cli/validate/README.md) | Implementing | `ingitdb validate` — check schema and records against `.ingitdb.yaml`. |
| [cli/read-record](cli/read-record/README.md) | Implementing | `ingitdb read record` — read a single record by ID. |
| [cli/create-record](cli/create-record/README.md) | Implementing | `ingitdb create record` — create a new record. |
| [cli/update-record](cli/update-record/README.md) | Implementing | `ingitdb update record` — patch fields of an existing record. |
| [cli/delete-record](cli/delete-record/README.md) | Implementing | `ingitdb delete record` — delete a single record by ID. |
| [cli/list-collections](cli/list-collections/README.md) | Implementing | `ingitdb list collections` — list collection IDs. |
| [cli/rebase](cli/rebase/README.md) | Implementing | `ingitdb rebase` — rebase with auto-resolution of generated-file conflicts. |
| [cli/find](cli/find/README.md) | Draft | `ingitdb find` — search records by substring, regex, or exact value. |
| [cli/truncate](cli/truncate/README.md) | Draft | `ingitdb truncate` — remove all records from a collection. |
| [cli/delete-collection](cli/delete-collection/README.md) | Draft | `ingitdb delete collection` — remove a collection and its records. |
| [cli/delete-records](cli/delete-records/README.md) | Draft | `ingitdb delete records` — remove records matching a filter. |
| [cli/query](cli/query/README.md) | Draft | `ingitdb query` — query and format records from a collection. |
| [cli/materialize](cli/materialize/README.md) | Draft | `ingitdb materialize` — build materialized views and READMEs. |
| [cli/diff](cli/diff/README.md) | Draft | `ingitdb diff` — record-level diff between two git refs. |
| [cli/pull](cli/pull/README.md) | Draft | `ingitdb pull` — pull, auto-resolve, and rebuild views. |
| [cli/watch](cli/watch/README.md) | Draft | `ingitdb watch` — stream record change events to stdout. |
| [cli/serve](cli/serve/README.md) | Draft | `ingitdb serve` — MCP, HTTP API, and file-watcher servers. |
| [cli/resolve](cli/resolve/README.md) | Draft | `ingitdb resolve` — interactive merge-conflict TUI. |
| [cli/setup](cli/setup/README.md) | Draft | `ingitdb setup` — initialise a new database directory. |
| [cli/migrate](cli/migrate/README.md) | Draft | `ingitdb migrate` — migrate records between schema versions. |
| [id-flag-format](id-flag-format/README.md) | Implementing | Cross-cutting `--id=<collection-id>/<record-key>` syntax. |
| [github-direct-access](github-direct-access/README.md) | Implementing | Cross-cutting `--github=owner/repo[@ref]` flag and token handling. |
| [output-formats](output-formats/README.md) | Implementing | Cross-cutting `--format=yaml|json` flag and YAML default. |
| [path-targeting](path-targeting/README.md) | Implementing | Cross-cutting `--path` flag and its relation to `--github`. |

## Feature Summaries

### cli/version
Prints build version, commit hash, and build date to stdout. The simplest CLI command and a smoke test for the binary.

### cli/validate
Validates the `.ingitdb.yaml` definition and every record file against its collection schema. Supports `--only=definition|records` for partial passes and `--from-commit`/`--to-commit` for fast CI mode that only checks files changed in a commit range.

### cli/read-record
Reads a single record by `--id` from a local path or directly from a GitHub repository, formatting the result as YAML (default) or JSON.

### cli/create-record
Creates a new record in a `map[string]any` collection. Fails when a record with the same key already exists. Works against a local path or a GitHub repository (writes require a token).

### cli/update-record
Updates an existing record using patch semantics: only fields supplied in `--set` change; all others are preserved.

### cli/delete-record
Deletes a single record by ID. For `SingleRecord` collections the record file is removed; for `MapOfIDRecords` collections the key is removed from the shared map file.

### cli/list-collections
Lists collection IDs from a local DB or a GitHub repository, with optional `--in` regex scoping and `--filter-name` glob filtering.

### cli/rebase
Runs `git rebase` on top of a base ref and auto-resolves conflicts in generated files (collection `README.md`, materialized views, indexes) when the user opts in via `--resolve`.

### cli/find
Searches record fields by substring, regex, or exact match. Supports field whitelisting, sub-path scoping, and a result limit.

### cli/truncate
Removes every record from a collection while preserving the collection definition.

### cli/delete-collection
Deletes a collection definition and every record file that belongs to it.

### cli/delete-records
Bulk-deletes records that match a glob pattern within a collection, optionally scoped by an `--in` sub-path regex.

### cli/query
Queries records from a single collection with `--where` filters, `--order-by` sorting, and pluggable output formats (CSV, JSON, YAML, Markdown).

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

### cli/migrate
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
