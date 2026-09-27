---
name: describe
description: Describe a schema object in an inGitDB database — a collection (columns, types, indexes) or a view (definition). Use to understand structure before reading or writing records.
user-invocable: true
---

# inGitDB describe

Wraps `ingitdb describe <kind> <name>` — print the schema of a collection or view.

## Pre-flight check

```bash
command -v ingitdb >/dev/null 2>&1
```

On miss, invoke `/ingitdb:install` and stop until `ingitdb version` succeeds.

## Pick a verb

| You need to… | Read |
|---|---|
| Describe a collection (columns, types, options) | [references/collection.md](references/collection.md) |
| Describe a view (definition, source collections) | [references/view.md](references/view.md) |
