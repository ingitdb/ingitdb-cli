# ingitdb describe collection

Print the schema (columns, types, options) of a collection.

## Command

```bash
ingitdb describe collection <collection-id> [--format <yaml|json>] [--path <dir>]
```

## Parameters

| Flag | Required | Description |
|---|---|---|
| `<collection-id>` | Yes | Dotted collection identifier (e.g., `geo.nations`). |
| `--format` | No | Output format. Default `yaml`. |
| `--path` | No | Database directory. |

## When to use

- Before writing records with `insert`/`update` — confirm column names and types.
- Schema review.

## Notes

- Read-only.
