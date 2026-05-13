### `drop` — remove schema objects (collections and views)

[Source Code](../../../cmd/ingitdb/commands/drop.go)

```
ingitdb drop collection <name> [--if-exists] [--cascade] [--path=PATH]
ingitdb drop view <name>       [--if-exists] [--cascade] [--path=PATH]
```

Removes a schema object — both its entry in `.ingitdb.yaml` and any associated data directory
or materialized output — in a single git commit. Replaces the legacy `delete collection` and
`delete view` subcommands.

| Subcommand          | Description                                                                            |
| ------------------- | -------------------------------------------------------------------------------------- |
| `drop collection`   | Drops a collection definition and removes every record file that belongs to it.        |
| `drop view`         | Drops a view definition and removes its materialised output files.                     |

| Flag                             | Required | Description                                                                              |
| -------------------------------- | -------- | ---------------------------------------------------------------------------------------- |
| `--if-exists`                    | no       | Make the operation idempotent — exit successfully when the target does not exist.        |
| `--cascade`                      | no       | Also drop dependent objects (e.g. views that read from a dropped collection).            |
| `--path=PATH`                    | no       | Local database directory. Defaults to current directory.                                 |
| `--remote=HOST/OWNER/REPO[@REF]` | no       | Remote Git repository. Mutually exclusive with `--path`.                                 |
| `--token=TOKEN`                  | no       | Personal access token. Required for `--remote` writes.                                   |

**Examples:**

```shell
# Drop a collection (errors if it does not exist)
ingitdb drop collection countries.archive

# Drop a collection idempotently
ingitdb drop collection countries.archive --if-exists

# Drop a view and any dependent views
ingitdb drop view by_status --cascade

# Drop in a GitHub repository
export GITHUB_TOKEN=ghp_...
ingitdb drop collection countries.archive --remote=github.com/myorg/mydb
```

---
