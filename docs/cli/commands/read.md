### 🔹 read record` — read a single record

[Source Code](../../../cmd/ingitdb/commands/read.go)

```
ingitdb read record --id=ID [--path=PATH] [--format=yaml|json]
ingitdb read record --id=ID --remote=HOST/OWNER/REPO[@REF] [--token=TOKEN] [--format=yaml|json]
```

Reads a single record by ID and writes it to stdout.

| Flag                        | Required | Description                                                                              |
| --------------------------- | -------- | ---------------------------------------------------------------------------------------- |
| `--id=ID`                   | yes      | Record ID as `collection/path/key` (e.g. `countries/ie`).                                |
| `--path=PATH`               | no       | Path to the local database directory. Defaults to the current working directory.         |
| `--remote=HOST/OWNER/REPO[@REF]` | no       | Remote Git repository (e.g. `github.com/owner/repo` or `github.com/owner/repo@ref`). Mutually exclusive with `--path`. |
| `--token=TOKEN`             | no       | Personal access token for the remote provider. Falls back to host-derived env vars (e.g. `GITHUB_TOKEN` for `github.com`). |
| `--format=FORMAT`           | no       | Output format: `yaml` (default) or `json`.                                               |

**Examples:**

```shell
# 📘 Read from the current directory
ingitdb read record --id=countries/ie

# 🔁 Read from a specific local path
ingitdb read record --path=/var/db/myapp --id=countries/ie

# 🐙 Read from a public GitHub repository
ingitdb read record --remote=github.com/ingitdb/ingitdb-cli --id=todo.tags/active

# 🔁 Read from a specific branch on GitHub, output as JSON
ingitdb read record --remote=github.com/ingitdb/ingitdb-cli@main --id=todo.tags/active --format=json

# 🐙 Read from a private GitHub repository
export GITHUB_TOKEN=ghp_...
ingitdb read record --remote=github.com/myorg/private-db --id=users/alice
```

See [Remote Repository Access](../../features/remote-repo-access.md) for more detail.

See [Read Examples](read-examples.md) for more usage examples.

---
