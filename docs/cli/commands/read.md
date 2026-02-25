### 🔹 read record` — read a single record

[Source Code](../../../cmd/ingitdb/commands/read.go)

```
ingitdb read record --id=ID [--path=PATH] [--format=yaml|json]
ingitdb read record --id=ID --github=OWNER/REPO[@REF] [--token=TOKEN] [--format=yaml|json]
```

Reads a single record by ID and writes it to stdout.

| Flag                        | Required | Description                                                                              |
| --------------------------- | -------- | ---------------------------------------------------------------------------------------- |
| `--id=ID`                   | yes      | Record ID as `collection/path/key` (e.g. `countries/ie`).                                |
| `--path=PATH`               | no       | Path to the local database directory. Defaults to the current working directory.         |
| `--github=OWNER/REPO[@REF]` | no       | GitHub repository as `owner/repo` or `owner/repo@ref`. Mutually exclusive with `--path`. |
| `--token=TOKEN`             | no       | GitHub personal access token. Falls back to `GITHUB_TOKEN` env var.                      |
| `--format=FORMAT`           | no       | Output format: `yaml` (default) or `json`.                                               |

**Examples:**

```shell
# 📘 Read from the current directory
ingitdb read record --id=countries/ie

# 🔁 Read from a specific local path
ingitdb read record --path=/var/db/myapp --id=countries/ie

# 🐙 Read from a public GitHub repository
ingitdb read record --github=ingitdb/ingitdb-cli --id=todo.tags/active

# 🔁 Read from a specific branch on GitHub, output as JSON
ingitdb read record --github=ingitdb/ingitdb-cli@main --id=todo.tags/active --format=json

# 🐙 Read from a private GitHub repository
export GITHUB_TOKEN=ghp_...
ingitdb read record --github=myorg/private-db --id=users/alice
```

See [GitHub Direct Access](../../features/github-direct-access.md) for more detail.

See [Read Examples](read-examples.md) for more usage examples.

---
