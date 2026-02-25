### 🧾 materialize` — build generated files from records _(not yet implemented)_

[Source Code](../../../cmd/ingitdb/commands/materialize.go)

Top-level command with subcommands for materializing views and README files.

#### 🔸 materialize collection`

```
ingitdb materialize collection [--collection=ID] [--path=PATH]
```

Renders the `README.md` file for a collection. If the generated content differs from the existing file, it is automatically updated. See [Collection README Builder](../components/readme-builders/collection.md) for details on what is included in the generated README.

| Flag              | Description                                                                                             |
| ----------------- | ------------------------------------------------------------------------------------------------------- |
| `--collection=ID` | Collection ID to materialize (e.g. `countries`). If omitted, READMEs for all collections are processed. |
| `--path=PATH`     | Path to the database directory. Defaults to the current working directory.                              |

#### 🔸 materialize views`

```
ingitdb materialize views [--path=PATH] [--views=VIEW1,VIEW2,...]
```

| Flag           | Description                                                                                       |
| -------------- | ------------------------------------------------------------------------------------------------- |
| `--path=PATH`  | Path to the database directory. Defaults to the current working directory.                        |
| `--views=LIST` | Comma-separated list of view names to materialize. Without this flag, all views are materialized. |

Output is written into the `$views/` directory defined in `.ingitdb.yaml`.

**Examples:**

```shell
# 🧾 Rebuild all views
ingitdb materialize

# 🔁 Rebuild specific views only
ingitdb materialize --views=by_status,by_assignee

# 🔁 Rebuild views for a database at a specific path
ingitdb materialize --path=/var/db/myapp
```

---
