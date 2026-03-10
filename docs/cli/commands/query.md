### `query` — query records from a collection

[Source Code](../../../cmd/ingitdb/commands/query.go)

```
ingitdb query -c=COLLECTION [-f=FIELDS] [--where=EXPR] [--order-by=FIELDS] [--format=csv|json|yaml|md] [--path=PATH]
```

| Flag | Alias | Required | Description |
|---|---|---|---|
| `--collection` | `-c` | yes | Collection ID to query |
| `--fields` | `-f` | no | `*` = all (default), `$id` = record key, `field1,field2` = specific fields |
| `--where` | `-w` | no | Filter expression: `field>value`, `field==value`, etc. Repeatable for AND. |
| `--order-by` | | no | Comma-separated fields; prefix `-` = descending |
| `--format` | | no | `csv` (default, with header), `json`, `yaml`, `md` (markdown table) |
| `--path` | | no | DB directory (default: current directory) |

**Operators in `--where`:** `>=`, `<=`, `>`, `<`, `==`, `=` (`=` is treated as `==`)

**Number formatting:** commas are stripped before parsing (e.g. `1,000,000` → `1000000`)

**Examples:**

```shell
# Query all records, show only the record key
ingitdb query -c=countries --fields='$id'

# Query all fields
ingitdb query -c=countries -f='*'

# Query specific fields as CSV
ingitdb query -c=countries --fields='$id,currency,flag'

# JSON output
ingitdb query -c=countries -f='$id,currency,flag' --format=json

# YAML output
ingitdb query -c=countries -f='$id,currency,flag' --format=yaml

# Markdown table
ingitdb query -c=countries -f='$id,currency,flag' --format=md

# Filter records where population > 1000000
ingitdb query -c=countries -f='$id' --where='population>1000000'

# Filter with thousands-separator in value
ingitdb query -c=countries -f='$id' --where='population>1,000,000'

# Sort by population descending
ingitdb query -c=countries -f='$id,population' --order-by='-population'

# Multiple WHERE conditions (AND logic)
ingitdb query -c=countries -f='$id' --where='population>50000000' --where='population<300000000'

# Query from a specific database path
ingitdb query -c=users --path=/var/db/myapp
```

---
