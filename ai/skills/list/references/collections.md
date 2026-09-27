# ingitdb list collections

Enumerate collections in a database. Filter by parent path or by name pattern.

## Command

```bash
ingitdb list collections [--in <regex>] [--filter-name <glob>] [--path <dir>]
```

## Parameters

| Flag | Required | Description |
|---|---|---|
| `--in` | No | Regular expression matching parent paths (e.g., `'countries/(ie|gb)'`). |
| `--filter-name` | No | Glob pattern on the collection name (e.g., `'*city*'`). |
| `--path` | No | Database directory. Defaults to current directory. |

## When to use

- **Discovery:** What collections exist?
- **Targeted exploration:** Find collections matching a pattern before describing or querying.
- **Pipe target:** Default text output is one collection per line, ready for `grep`, `fzf`, `xargs`.

## Examples

```bash
# List everything
ingitdb list collections

# Collections nested under specific countries
ingitdb list collections --in='countries/(ie|gb)'

# Name contains "city"
ingitdb list collections --filter-name='*city*'
```

## Notes

- Read-only. Never mutates state.
