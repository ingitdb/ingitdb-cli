### 🔹 delete` — delete database objects

[Source Code](../../../cmd/ingitdb/commands/delete.go)


Top-level command with four subcommands. `delete record` is implemented; the rest are planned.

#### 🔸 delete record`

```
ingitdb delete record --id=ID [--path=PATH]
ingitdb delete record --id=ID --remote=HOST/OWNER/REPO[@REF] [--token=TOKEN]
```

Deletes a single record by ID. For `SingleRecord` collections, the record file is removed. For
`MapOfIDRecords` collections, the key is removed from the shared map file.

| Flag                        | Required | Description                                                                             |
| --------------------------- | -------- | --------------------------------------------------------------------------------------- |
| `--id=ID`                   | yes      | Record ID as `collection/path/key`.                                                     |
| `--path=PATH`               | no       | Path to the local database directory. Defaults to the current working directory.        |
| `--remote=HOST/OWNER/REPO[@REF]` | no       | Remote Git repository (e.g. `github.com/owner/repo`). Mutually exclusive with `--path`.   |
| `--token=TOKEN`             | no       | Personal access token. Falls back to host-derived env vars (e.g. `GITHUB_TOKEN`). Required for `--remote` writes. |

**Examples:**

```shell
# 📘 Delete a record locally
ingitdb delete record --id=countries/ie

# 🐙 Delete a record in a GitHub repository
export GITHUB_TOKEN=ghp_...
ingitdb delete record --remote=github.com/myorg/mydb --id=countries/ie
```

The following subcommands are planned but not yet implemented:

#### ⚙️ delete collection`

```
ingitdb delete collection --collection=ID [--path=PATH]
```

Deletes a collection definition and all of its record files.

| Flag              | Required | Description                                                                |
| ----------------- | -------- | -------------------------------------------------------------------------- |
| `--collection=ID` | yes      | Collection id to delete (e.g. `countries.counties`).                       |
| `--path=PATH`     | no       | Path to the database directory. Defaults to the current working directory. |

**Example:**

```shell
ingitdb delete collection --collection=countries.counties.dublin
```

#### 🔸 delete view`

```
ingitdb delete view --view=ID [--path=PATH]
```

Deletes a view definition and removes its materialised output files.

| Flag          | Required | Description                                                                |
| ------------- | -------- | -------------------------------------------------------------------------- |
| `--view=ID`   | yes      | View id to delete.                                                         |
| `--path=PATH` | no       | Path to the database directory. Defaults to the current working directory. |

**Example:**

```shell
ingitdb delete view --view=by_status
```

#### 🔸 delete records`

```
ingitdb delete records --collection=ID [--path=PATH] [--in=REGEXP] [--filter-name=PATTERN]
```

Deletes individual records from a collection. Use `--in` and `--filter-name` to scope which records are removed.

| Flag                    | Required | Description                                                                |
| ----------------------- | -------- | -------------------------------------------------------------------------- |
| `--collection=ID`       | yes      | Collection to delete records from.                                         |
| `--path=PATH`           | no       | Path to the database directory. Defaults to the current working directory. |
| `--in=REGEXP`           | no       | Regular expression scoping deletion to a sub-path.                         |
| `--filter-name=PATTERN` | no       | Glob-style pattern to match record names to delete.                        |

**Example:**

```shell
ingitdb delete records --collection=countries.counties --filter-name='*old*'
```

---

