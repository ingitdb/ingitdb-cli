---
name: validate
description: Validate an inGitDB database — check schema definitions, record conformance, and rebuild materialized views. Use when running CI checks, after schema edits, or before committing record changes.
user-invocable: true
---

# inGitDB validate

This skill wraps the `ingitdb validate` command — the structured pass over a database directory that checks collection definitions, record conformance, and rebuilds the `$views/` materialized output.

## Pre-flight check

Before running any `ingitdb` command, verify the CLI is installed:

```bash
command -v ingitdb >/dev/null 2>&1
```

If this check fails (exit `127` / `command not found`), stop and tell the user exactly:

> The `ingitdb` CLI is not installed. Either:
> - invoke `/ingitdb:install` to see install options, or
> - install directly per <https://github.com/ingitdb/ingitdb-cli#installation>.
>
> Then retry your command.

Do not proceed with the original command until `ingitdb version` succeeds.

## Pick a verb

| You need to… | Read |
|---|---|
| Validate a full database directory (schema + records + rebuild views) | [references/full.md](references/full.md) |
| Validate only collection definitions | [references/definitions.md](references/definitions.md) |
| Validate only records (skip schema) | [references/records.md](references/records.md) |
| Validate only the diff between two commits (fast CI mode) | [references/diff.md](references/diff.md) |
