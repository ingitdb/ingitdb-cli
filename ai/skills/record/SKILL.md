---
name: record
description: Create, read, update, and delete records in an inGitDB collection. Use for SQL-style INSERT, SELECT, UPDATE, and DELETE against `map[string]any` collections, locally or against a remote Git repo.
user-invocable: true
---

# inGitDB record CRUD

Wraps the four record-level commands in the `ingitdb` CLI: `insert`, `select`, `update`, `delete`. All four target `map[string]any` collections (i.e., collections whose `record_file.type` is `map[string]any`).

## Pre-flight check

```bash
command -v ingitdb >/dev/null 2>&1
```

On miss, invoke `/ingitdb:install` and stop until `ingitdb version` succeeds.

## ID format

Record IDs follow `<collection-id>/<record-key>`. Collection IDs allow alphanumeric and `.`; `/` separates the collection from the record key.

Example: `geo.nations/ie` → collection `geo.nations`, record key `ie`.

## Local vs. remote

Every record command accepts `--path <dir>` (local clone) **or** `--remote <host/owner/repo[@ref]>` (no clone, via REST API). Public repos need no token for reads; writes always require one (via `GITHUB_TOKEN` env var or `--token` flag).

## Pick a verb

| You need to… | Read |
|---|---|
| Insert a new record | [references/insert.md](references/insert.md) |
| Select records (by ID or by `--where` filter) | [references/select.md](references/select.md) |
| Update fields of an existing record | [references/update.md](references/update.md) |
| Delete a record (by ID or by `--where` filter) | [references/delete.md](references/delete.md) |
