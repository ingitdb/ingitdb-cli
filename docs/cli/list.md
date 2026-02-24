### 🔹 list` — list database objects

[Source Code](../../cmd/ingitdb/commands/list.go)


Top-level command with three subcommands. Shared flags on every subcommand:

| Flag                    | Description                                                                                            |
| ----------------------- | ------------------------------------------------------------------------------------------------------ |
| `--path=PATH`           | Path to the database directory. Defaults to the current working directory.                             |
| `--in=REGEXP`           | Regular expression that matches the starting-point path. Only objects under matching paths are listed. |
| `--filter-name=PATTERN` | Glob-style pattern to filter by name (e.g. `*substr*`).                                                |

#### ⚙️ list collections`

```
ingitdb list collections [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
ingitdb list collections --github=OWNER/REPO[@REF] [--token=TOKEN]
```

Lists all collection IDs defined in the database.

| Flag                        | Required | Description                                                                                     |
| --------------------------- | -------- | ----------------------------------------------------------------------------------------------- |
| `--path=PATH`               | no       | Path to the local database directory. Defaults to the current working directory.                |
| `--github=OWNER/REPO[@REF]` | no       | GitHub repository as `owner/repo` or `owner/repo@ref`. Mutually exclusive with `--path`.        |
| `--token=TOKEN`             | no       | GitHub personal access token. Falls back to `GITHUB_TOKEN` env var. Required for private repos. |
| `--in=REGEXP`               | no       | Regular expression that matches the starting-point path.                                        |
| `--filter-name=PATTERN`     | no       | Glob-style pattern to filter collection names (e.g. `*city*`).                                  |

**Examples:**

```shell
# ⚙️ List all collections in the current directory
ingitdb list collections

# ⚙️ List collections from a GitHub repository (no token needed for public repos)
ingitdb list collections --github=ingitdb/ingitdb-cli

# 🔁 Pin to a specific branch or tag
ingitdb list collections --github=ingitdb/ingitdb-cli@main

# 📘 Private repository
export GITHUB_TOKEN=ghp_...
ingitdb list collections --github=myorg/private-db

# ⚙️ Local: list collections nested under a matching path
ingitdb list collections --in='countries/(ie|gb)'

# ⚙️ Local: list collections whose name contains "city"
ingitdb list collections --filter-name='*city*'
```

#### 🔸 list view`

```
ingitdb list view [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
```

Lists all view definitions in the database.

**Examples:**

```shell
# 🧾 List all views
ingitdb list view

# 🔁 List views under a specific path
ingitdb list view --in='countries/.*'
```

#### 🔸 list subscribers`

```
ingitdb list subscribers [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
```

Lists all subscriber definitions in the database.

**Examples:**

```shell
# 📘 List all subscribers
ingitdb list subscribers

# 📘 List subscribers filtered by name
ingitdb list subscribers --filter-name='*webhook*'
```

---

