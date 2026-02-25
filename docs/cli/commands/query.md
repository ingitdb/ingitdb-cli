### ⚙️ query` — query records from a collection _(not yet implemented)_

[Source Code](../../../cmd/ingitdb/commands/query.go)


```
ingitdb query --collection=KEY [--path=PATH] [--format=json|yaml]
```

| Flag               | Required | Description                                                                |
| ------------------ | -------- | -------------------------------------------------------------------------- |
| `--collection=KEY` | yes      | Key of the collection to query.                                            |
| `--path=PATH`      | no       | Path to the database directory. Defaults to the current working directory. |
| `--format=FORMAT`  | no       | Output format: `json` (default) or `yaml`.                                 |

**Examples:**

```shell
# ⚙️ Query all records from a collection (JSON output)
ingitdb query --collection=countries.counties

# 📘 Query with YAML output
ingitdb query --collection=tasks --format=yaml

# 🔁 Query from a specific database path
ingitdb query --collection=users --path=/var/db/myapp
```

---

