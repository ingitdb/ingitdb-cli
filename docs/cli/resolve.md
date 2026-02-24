### 🔹 resolve` — resolve merge conflicts in database files _(not yet implemented)_

[Source Code](../../cmd/ingitdb/commands/resolve.go)


```
ingitdb resolve [--path=PATH] [--file=FILE]
```

| Flag          | Description                                                                               |
| ------------- | ----------------------------------------------------------------------------------------- |
| `--path=PATH` | Path to the database directory. Defaults to the current working directory.                |
| `--file=FILE` | Specific conflict file to resolve. Without this flag, all conflicted files are processed. |

**Examples:**

```shell
# 📘 Interactively resolve all conflicted files
ingitdb resolve

# 📘 Resolve a single conflicted file
ingitdb resolve --file=countries/ie/counties/dublin.yaml
```

---

