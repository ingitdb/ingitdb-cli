### 🔹 watch` — watch database for changes _(not yet implemented)_

[Source Code](../../cmd/ingitdb/commands/watch.go)


```
ingitdb watch [--path=PATH] [--format=text|json]
```

| Flag              | Description                                                                |
| ----------------- | -------------------------------------------------------------------------- |
| `--path=PATH`     | Path to the database directory. Defaults to the current working directory. |
| `--format=FORMAT` | Output format: `text` (default) or `json`.                                 |

Watches the database directory for file-system changes and writes a structured event to **stdout** for every record that is added, updated, or deleted. Runs in the foreground until interrupted.

**Examples:**

```shell
# 📘 Watch the current directory, text output
ingitdb watch

# 🔁 Watch a specific database path with JSON output (pipe-friendly)
ingitdb watch --path=/var/db/myapp --format=json
```

**Text output example:**

```
Record /countries/gb/cities/london: added
Record /countries/gb/cities/london: 2 fields updated: {population: 9000000, area: 1572}
Record /countries/gb/cities/london: deleted
```

**JSON output example (`--format=json`):**

```json
{"type":"added","record":"/countries/gb/cities/london"}
{"type":"updated","record":"/countries/gb/cities/london","fields":{"population":9000000,"area":1572}}
{"type":"deleted","record":"/countries/gb/cities/london"}
```

---

