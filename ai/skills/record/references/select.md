# ingitdb select

Read one record by ID, or filter records from a collection with a `--where` expression.

## Command

```bash
# By ID
ingitdb select --path <dir> --id <collection-id>/<record-key> [--format <yaml|json>]

# By filter
ingitdb select --from <collection-id> --where '<expr>' [--format <yaml|json>]

# Remote (read-only public repos need no token)
ingitdb select --remote <host/owner/repo[@ref]> --id <collection-id>/<record-key>
```

## Parameters

| Flag | Required | Description |
|---|---|---|
| `--id` | One of | Single-record fetch. `<collection-id>/<record-key>`. |
| `--from` | One of | Collection to filter. |
| `--where` | If `--from` | Filter expression. Operators: `==`, `===`, `!=`, `!==`, `>=`, `<=`, `>`, `<`. |
| `--format` | No | `yaml` (default) or `json`. |
| `--path` | One of | Local database. |
| `--remote` | One of | Remote repo. Append `@<ref>` to pin a branch/tag/SHA. |
| `--token` | If private | `--token` flag or host-derived env var (e.g., `GITHUB_TOKEN`). |

## Examples

```bash
ingitdb select --path=. --id=geo.nations/ie
ingitdb select --from=geo.nations --where='population>1000000'
ingitdb select --remote=github.com/owner/repo@main --id=todo.tags/active
```

## Notes

- Read-only.
- Filter expressions are evaluated server-side for remote queries when supported, else client-side after a list.
