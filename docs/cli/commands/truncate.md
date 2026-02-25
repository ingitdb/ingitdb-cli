### ⚙️ truncate` — remove all records from a collection _(not yet implemented)_

[Source Code](../../../cmd/ingitdb/commands/truncate.go)


```
ingitdb truncate --collection=ID [--path=PATH]
```

Removes every record file from the specified collection, leaving the collection definition intact.

| Flag              | Required | Description                                                                |
| ----------------- | -------- | -------------------------------------------------------------------------- |
| `--collection=ID` | yes      | Collection id to truncate (e.g. `countries.counties.dublin`).              |
| `--path=PATH`     | no       | Path to the database directory. Defaults to the current working directory. |

**Example:**

```shell
ingitdb truncate --collection=countries.counties
```

---

