# Features

This directory tracks the SpecScore feature specifications for the **ingitdb-cli** repository. Each feature describes the externally observable behavior of a single CLI command or a cross-cutting concern (flag conventions, output formats, ID syntax). Storage format and collection schema definitions live in the [ingitdb-specs](https://github.com/ingitdb/ingitdb-specs) repository and are referenced from here when needed.

## Index

| Feature | Status | Description |
|---|---|---|
| [record-format](record-format/README.md) | Implemented | Umbrella for record-format extensions: CSV support, project-level `default_record_format` config, `--default-format` CLI flag on `ingitdb setup`. Additive on top of the existing six-format machinery (yaml/yml/json/markdown/toml/ingr). |
| [cli/version](cli/version/README.md) | Implementing | `ingitdb version` — print build version, commit hash, and date. |
| [cli/validate](cli/validate/README.md) | Implementing | `ingitdb validate` — check schema and records against `.ingitdb.yaml`. |
| [cli/select](cli/select/README.md) | Implementing | `ingitdb select` — read a single record (`--id`) or query a set of records (`--from`/`--where`). |
| [cli/insert](cli/insert/README.md) | Implementing | `ingitdb insert` — create a new record (`--into`/`--key`). |
| [cli/update](cli/update/README.md) | Implementing | `ingitdb update` — patch fields of one or more records. |
| [cli/delete](cli/delete/README.md) | Implementing | `ingitdb delete` — delete records by ID or by `--from`/`--where`. |
| [cli/drop](cli/drop/README.md) | Implementing | `ingitdb drop` — drop a collection or view. |
| [cli/list-collections](cli/list-collections/README.md) | Implementing | `ingitdb list collections` — list collection IDs. |
| [cli/rebase](cli/rebase/README.md) | Implementing | `ingitdb rebase` — rebase with auto-resolution of generated-file conflicts. |
| [cli/find](cli/find/README.md) | Draft | `ingitdb find` — search records by substring, regex, or exact value. |
| [cli/truncate](cli/truncate/README.md) | Draft | `ingitdb truncate` — remove all records from a collection. |
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
| [cli/migrate](cli/migrate/README.md) | Draft | `ingitdb migrate` — migrate records between schema versions. |
| [id-flag-format](id-flag-format/README.md) | Implementing | Cross-cutting `--id=<collection-id>/<record-key>` syntax. |
| [output-formats](output-formats/README.md) | Implementing | Cross-cutting `--format=yaml|json` flag and YAML default. |
| [path-targeting](path-targeting/README.md) | Implementing | Cross-cutting `--path` flag and its relation to `--remote`. |
| [remote-repo-access](remote-repo-access/README.md) | Implementing | Cross-cutting `--remote=<URL>` flag, provider dispatch, and token resolution for remote Git hosting services. |
| [shared-cli-flags](shared-cli-flags/README.md) | Single source of truth for the CLI flag grammar shared across select, insert, update, delete, and drop verbs: --from, --into, --where, --set, --id, --all, --order-by, --fields. Defines parsing rules, operator semantics (==, ===, !=, !==, >=, <=, >, <), value-type model, and flag mutual-exclusion rules. |

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

### cli/rebase
Runs `git rebase` on top of a base ref and auto-resolves conflicts in generated files (collection `README.md`, materialized views, indexes) when the user opts in via `--resolve`.

### cli/find
Searches record fields by substring, regex, or exact match. Supports field whitelisting, sub-path scoping, and a result limit.

### cli/truncate
Removes every record from a collection while preserving the collection definition.

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

### output-formats
Defines the `--format=yaml|json` flag, the YAML default, and the contract that read-style commands obey.

### path-targeting
Defines the `--path` flag, its default of the current working directory, and its mutual exclusivity with `--remote`.

### remote-repo-access
Defines the `--remote=<URL>` flag for direct access to remote Git hosting services (GitHub, GitLab, Bitbucket, and self-hosted instances), with built-in provider inference, `--provider` override for unknown hosts, host-derived token environment variables, and the one-commit-per-write rule.

## Outstanding Questions

- None at this time.

---
*This document follows the https://specscore.md/feature-specification*

---
*This document follows the https://specscore.md/features-index-specification*
