### `select` — read a single record or query a set of records

[Source Code](../../../cmd/ingitdb/commands/select.go)

Two modes, both implemented by the same `select` verb:

- **single-record mode** — use `--id=COLLECTION/KEY` to read one record (replaces the legacy
  `read record` command). Default output format: `yaml`.
- **set mode** — use `--from=COLLECTION` with optional `--where`, `--order-by`, `--fields`,
  `--limit` (replaces the legacy `query` command). Default output format: `csv`.

```
ingitdb select --id=ID [--path=PATH] [--format=yaml|json]
ingitdb select --from=COLLECTION [--where=EXPR ...] [--order-by=FIELDS] [--fields=FIELDS] [--limit=N] [--format=csv|json|yaml|md] [--path=PATH]
```

| Flag                             | Required           | Description                                                                                                |
| -------------------------------- | ------------------ | ---------------------------------------------------------------------------------------------------------- |
| `--id=ID`                        | single-record mode | Record ID as `collection/key` (e.g. `countries/ie`).                                                       |
| `--from=COLLECTION`              | set mode           | Collection ID to query.                                                                                    |
| `--where=EXPR`                   | no                 | Filter expression; repeatable for AND. See operators below.                                                |
| `--order-by=FIELDS`              | no                 | Comma-separated fields; prefix `-` = descending (e.g. `-population`).                                      |
| `--fields=FIELDS`                | no                 | `*` = all (default), `$id` = record key only, or a comma list (e.g. `$id,name,population`).                |
| `--limit=N`                      | no                 | Maximum number of records to return in set mode.                                                           |
| `--format=FORMAT`                | no                 | `yaml` (default for single record), `csv` (default for set), `json`, `md`.                                 |
| `--path=PATH`                    | no                 | Local database directory. Defaults to current directory.                                                   |
| `--remote=HOST/OWNER/REPO[@REF]` | no                 | Remote Git repository. Mutually exclusive with `--path`.                                                   |
| `--token=TOKEN`                  | no                 | Personal access token; falls back to host-derived env vars (e.g. `GITHUB_TOKEN`).                          |

**Operators in `--where`:** `==`, `===`, `!=`, `!==`, `>=`, `<=`, `>`, `<`. There is **no**
`LIKE` and **no** `IN`.

**Number formatting:** commas are stripped before parsing (e.g. `1,000,000` → `1000000`).

**Examples — single-record mode:**

```shell
# Read from current directory (yaml)
ingitdb select --id=countries/ie

# Read from a specific local path as JSON
ingitdb select --path=/var/db/myapp --id=countries/ie --format=json

# Read from a public GitHub repository
ingitdb select --remote=github.com/ingitdb/ingitdb-cli --id=todo.tags/active

# Read from a specific branch
ingitdb select --remote=github.com/ingitdb/ingitdb-cli@main --id=todo.tags/active

# Read from a private repo
export GITHUB_TOKEN=ghp_...
ingitdb select --remote=github.com/myorg/private-db --id=users/alice
```

**Examples — set mode:**

```shell
# All record keys
ingitdb select --from=countries --fields='$id'

# Specific fields as CSV
ingitdb select --from=countries --fields='$id,currency,flag'

# JSON output
ingitdb select --from=countries --fields='$id,currency,flag' --format=json

# Markdown table
ingitdb select --from=countries --fields='$id,currency,flag' --format=md

# Filter records where population > 1,000,000
ingitdb select --from=countries --fields='$id' --where='population>1,000,000'

# Sort by population descending
ingitdb select --from=countries --fields='$id,population' --order-by='-population'

# Multiple WHERE conditions (AND)
ingitdb select --from=countries --fields='$id' \
  --where='population>50,000,000' --where='population<300,000,000'
```

See [Remote Repository Access](../../features/remote-repo-access.md) for more detail on
`--remote` and authentication.

---
