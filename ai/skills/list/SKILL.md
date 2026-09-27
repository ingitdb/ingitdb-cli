---
name: list
description: List database objects in an inGitDB database — collections (and their nested paths) or views. Use to discover what's in a database before querying or describing individual objects.
user-invocable: true
---

# inGitDB list

Wraps `ingitdb list` — enumerate database objects (collections or views).

## Pre-flight check

Verify the CLI is installed:

```bash
command -v ingitdb >/dev/null 2>&1
```

On miss, instruct the user to invoke `/ingitdb:install` or install per <https://github.com/ingitdb/ingitdb-cli#installation>, then retry.

## Pick a verb

| You need to… | Read |
|---|---|
| List collections | [references/collections.md](references/collections.md) |
| List views | [references/views.md](references/views.md) |
