### 🔹 validate` — validate database schema and data

[Source Code](../../cmd/ingitdb/commands/validate.go)


```
ingitdb validate [--path=PATH] [--from-commit=SHA] [--to-commit=SHA]
```

| Flag                | Description                                                                |
| ------------------- | -------------------------------------------------------------------------- |
| `--path=PATH`       | Path to the database directory. Defaults to the current working directory. |
| `--from-commit=SHA` | Validate only records changed since this commit.                           |
| `--to-commit=SHA`   | Validate only records up to this commit.                                   |

Reads `.ingitdb.yaml`, checks that every record file matches its collection schema, and reports any violations to stderr. With `--from-commit` / `--to-commit`, only files changed in that commit range are checked (see [Validator docs](components/validator/README.md)).

Exit code is `0` on success, non-zero on any validation error.

**Examples:**

```shell
# 📘 Validate the current directory
ingitdb validate

# 🔁 Validate a specific path
ingitdb validate --path=/path/to/your/db

# 🔁 Fast CI mode: validate only records changed between two commits
ingitdb validate --from-commit=abc1234 --to-commit=def5678
```

---

