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
| `--group` | No | With `--from`: read every instance of a subcollection (see below). |
| `--where` | If `--from` | Filter expression. Operators: `==`, `===`, `!=`, `!==`, `>=`, `<=`, `>`, `<`. |
| `--format` | No | `yaml` (default) or `json`. |
| `--path` | One of | Local database. |
| `--remote` | One of | Remote repo. Append `@<ref>` to pin a branch/tag/SHA. |
| `--token` | If private | `--token` flag or host-derived env var (e.g., `GITHUB_TOKEN`). |

With `--group`, `--from` is a schema path of collection ids only (`spaces/ext/days`, the full
path `ingitdb validate` prints) and the command reads every instance of that subcollection, like a
Firestore collection-group query; `$id` is then each record's full path
(`spaces/k3v9q/ext/calendarius/days/2026-11-24`). Local databases only.

## Examples

```bash
ingitdb select --path=. --id=geo.nations/ie
ingitdb select --from=geo.nations --where='population>1000000'
ingitdb select --remote=github.com/owner/repo@main --id=todo.tags/active
```

## Notes

- Read-only.
- Filter expressions are evaluated server-side for remote queries when supported, else client-side after a list.
