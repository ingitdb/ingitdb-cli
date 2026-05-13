### `update` — patch fields of one or more records

[Source Code](../../../cmd/ingitdb/commands/update_new.go)

Two modes:

- **single-record mode** — `--id=COLLECTION/KEY` updates a single record.
- **set mode** — `--from=COLLECTION` with `--where=EXPR` (or `--all`) updates every matching
  record in the collection.

Patch semantics: only fields listed in `--set` are changed; `--unset` removes the listed
fields; every other field is preserved.

```
ingitdb update --id=ID --set=YAML [--unset=FIELDS] [--path=PATH]
ingitdb update --from=COLLECTION (--where=EXPR ... | --all) --set=YAML [--unset=FIELDS] [--path=PATH]
```

| Flag                             | Required           | Description                                                                                  |
| -------------------------------- | ------------------ | -------------------------------------------------------------------------------------------- |
| `--id=ID`                        | single-record mode | Record ID as `collection/key`.                                                               |
| `--from=COLLECTION`              | set mode           | Target collection.                                                                           |
| `--where=EXPR`                   | set mode           | Filter expression; repeatable for AND. Required in set mode unless `--all` is given.         |
| `--all`                          | set mode           | Apply to every record in the collection. Mutually exclusive with `--where`.                  |
| `--set=YAML`                     | yes                | Fields to patch as YAML or JSON (e.g. `'{capital: Dublin}'`).                                |
| `--unset=FIELDS`                 | no                 | Comma-separated field names to remove.                                                       |
| `--require-match`                | no                 | In set mode, exit non-zero when zero records match.                                          |
| `--path=PATH`                    | no                 | Local database directory. Defaults to current directory.                                     |
| `--remote=HOST/OWNER/REPO[@REF]` | no                 | Remote Git repository. Mutually exclusive with `--path`.                                     |
| `--token=TOKEN`                  | no                 | Personal access token. Required for `--remote` writes.                                       |

**Examples:**

```shell
# Patch one record locally
ingitdb update --id=countries/ie --set='{capital: Dublin}'

# Patch one record in a GitHub repository
export GITHUB_TOKEN=ghp_...
ingitdb update --remote=github.com/myorg/mydb --id=countries/ie \
  --set='{capital: Dublin, population: 5100000}'

# Bulk-update every matching record
ingitdb update --from=countries --where='continent==Europe' --set='{region: EU}'
```

---
