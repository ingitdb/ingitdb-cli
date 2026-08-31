### 🔹 validate` — validate database schema and data

[Source Code](../../../cmd/ingitdb/commands/validate.go)


```
ingitdb validate [--path=PATH] [--only=definition|records] [--from-commit=SHA] [--to-commit=SHA] [--safe-diagnostics]
```

| Flag                | Description                                                                |
| ------------------- | -------------------------------------------------------------------------- |
| `--path=PATH`       | Path to the database directory. Defaults to the current working directory. |
| `--only=VALUE`      | Validate only `definition` or `records`. Omit to validate both.            |
| `--from-commit=SHA` | Validate only records changed since this commit.                           |
| `--to-commit=SHA`   | Validate only records up to this commit.                                   |
| `--safe-diagnostics` | Report repository-relative finding identities and constraint classes without record values. |

Validates the database schema and records in the `.ingitdb.yaml` file. By default, checks both
the collection definitions and every record against its schema. Use `--only` to validate just
the definitions or just the records. With `--from-commit` / `--to-commit`, only files changed
in that commit range are checked (see [Validator docs](components/validator/README.md)).

Exit code is `0` on success and `2` when validation completes with invalid
repository definitions or records. Command configuration, startup, and other
runtime failures exit `1`, so automation can distinguish invalid data
from a validator that could not complete. Validation messages report record
counts per collection (e.g., "All 42 records are valid for collection: users"
or "38 out of 42 records are valid for collection: users").

The CLI process boundary recovers unexpected panics as generic runtime failures
with exit code `1`; panic payloads and stack traces are not printed because they
may contain repository values.

**Examples:**

```shell
# 📘 Validate the current directory (schema + records)
ingitdb validate

# 📘 Validate a specific path
ingitdb validate --path=/path/to/your/db

# 🔍 Validate only collection definitions
ingitdb validate --only=definition

# 🔍 Validate only records (skip schema validation)
ingitdb validate --only=records

# 🔁 Fast CI mode: validate only records changed between two commits
ingitdb validate --from-commit=abc1234 --to-commit=def5678

# 🔁 Validate records changed in a commit range (skip schema validation)
ingitdb validate --only=records --from-commit=abc1234 --to-commit=def5678

# 🔒 CI-safe full validation without record values in diagnostics
ingitdb validate --safe-diagnostics
```

---
