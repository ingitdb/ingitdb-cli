### 🔁 migrate` — migrate data between schema versions _(not yet implemented)_

[Source Code](../../../cmd/ingitdb/commands/migrate.go)


```
ingitdb migrate --from=VERSION --to=VERSION --target=TARGET \
    [--path=PATH] [--format=FORMAT] [--collections=LIST] [--output-dir=DIR]
```

| Flag                 | Required | Description                                                                                      |
| -------------------- | -------- | ------------------------------------------------------------------------------------------------ |
| `--from=VERSION`     | yes      | Source schema version.                                                                           |
| `--to=VERSION`       | yes      | Target schema version.                                                                           |
| `--target=TARGET`    | yes      | Migration target identifier.                                                                     |
| `--path=PATH`        | no       | Path to the database directory. Defaults to the current working directory.                       |
| `--format=FORMAT`    | no       | Output format for migrated records.                                                              |
| `--collections=LIST` | no       | Comma-separated list of collections to migrate. Without this flag, all collections are migrated. |
| `--output-dir=DIR`   | no       | Directory to write migrated records into.                                                        |

**Examples:**

```shell
# ⚙️ Migrate all collections from v1 to v2
ingitdb migrate --from=v1 --to=v2 --target=production

# ⚙️ Migrate specific collections only
ingitdb migrate --from=v1 --to=v2 --target=production --collections=tasks,users

# 🔁 Write migrated records to a staging directory
ingitdb migrate --from=v1 --to=v2 --target=production --output-dir=/tmp/migration
```
