### `insert` — create a new record

[Source Code](../../../cmd/ingitdb/commands/insert.go)

```
ingitdb insert --into=COLLECTION --key=KEY --data=YAML [--path=PATH]
ingitdb insert --into=COLLECTION --key=KEY --data=YAML --remote=HOST/OWNER/REPO[@REF] [--token=TOKEN]
```

Creates a new record in `--into=COLLECTION` with key `--key=KEY`. Fails if a record with the
same key already exists. The key may also be supplied inside `--data` via the `$id` field as a
fallback when `--key` is omitted.

| Flag                             | Required | Description                                                                                                       |
| -------------------------------- | -------- | ----------------------------------------------------------------------------------------------------------------- |
| `--into=COLLECTION`              | yes      | Target collection ID (e.g. `countries`).                                                                          |
| `--key=KEY`                      | no       | Record key. If omitted, `--data` must include `$id`.                                                              |
| `--data=YAML`                    | no       | Record fields as YAML or JSON (e.g. `'{name: Ireland}'`). May also be piped via stdin or supplied via `--edit`.   |
| `--path=PATH`                    | no       | Path to the local database directory. Defaults to the current working directory.                                  |
| `--remote=HOST/OWNER/REPO[@REF]` | no       | Remote Git repository (e.g. `github.com/owner/repo`). Mutually exclusive with `--path`.                           |
| `--token=TOKEN`                  | no       | Personal access token. Falls back to host-derived env vars (e.g. `GITHUB_TOKEN`). Required for `--remote` writes. |

**Examples:**

```shell
# Insert a record locally
ingitdb insert --into=countries --key=ie --data='{name: Ireland}'

# Insert a record in a GitHub repository
export GITHUB_TOKEN=ghp_...
ingitdb insert --remote=github.com/myorg/mydb --into=countries --key=ie \
  --data='{name: Ireland, capital: Dublin, population: 5000000}'
```

---
