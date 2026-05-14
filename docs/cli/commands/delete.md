### `delete` — remove one or more records

[Source Code](../../../cmd/ingitdb/commands/delete.go)

Two modes:

- **single-record mode** — `--id=COLLECTION/KEY` deletes one record.
- **set mode** — `--from=COLLECTION` with `--where=EXPR` (or `--all`) deletes every matching
  record in the collection.

For `SingleRecord` collections the record file is removed. For `MapOfIDRecords` collections
the key is removed from the shared map file.

```
ingitdb delete --id=ID [--path=PATH]
ingitdb delete --from=COLLECTION (--where=EXPR ... | --all) [--path=PATH]
ingitdb delete --id=ID --remote=HOST/OWNER/REPO[@REF] [--token=TOKEN]
```

| Flag                             | Required           | Description                                                                                  |
| -------------------------------- | ------------------ | -------------------------------------------------------------------------------------------- |
| `--id=ID`                        | single-record mode | Record ID as `collection/key`.                                                               |
| `--from=COLLECTION`              | set mode           | Target collection.                                                                           |
| `--where=EXPR`                   | set mode           | Filter expression; repeatable for AND. Required in set mode unless `--all` is given.         |
| `--all`                          | set mode           | Match every record in the collection. Mutually exclusive with `--where`.                     |
| `--min-affected=N`               | no                 | Exit non-zero when fewer than N records were deleted.                                        |
| `--path=PATH`                    | no                 | Local database directory. Defaults to current directory.                                     |
| `--remote=HOST/OWNER/REPO[@REF]` | no                 | Remote Git repository. Mutually exclusive with `--path`.                                     |
| `--token=TOKEN`                  | no                 | Personal access token. Required for `--remote` writes.                                       |

To remove an entire collection or view, see [`drop`](drop.md). To empty a collection while
preserving its definition, use `delete --from=COLLECTION --all`.

**Examples:**

```shell
# Delete one record locally
ingitdb delete --id=countries/ie

# Delete one record in a GitHub repository
export GITHUB_TOKEN=ghp_...
ingitdb delete --remote=github.com/myorg/mydb --id=countries/ie

# Bulk-delete records matching a filter
ingitdb delete --from=countries --where='population<100,000'

# Delete every record in a collection
ingitdb delete --from=countries.archive --all
```

---
