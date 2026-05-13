### 🔹 create record` — create a new record

[Source Code](../../../cmd/ingitdb/commands/create.go)


```
ingitdb create record --id=ID --data=YAML [--path=PATH]
ingitdb create record --id=ID --data=YAML --remote=HOST/OWNER/REPO[@REF] [--token=TOKEN]
```

Creates a new record. Fails if a record with the same key already exists in the collection.

| Flag                        | Required | Description                                                                             |
| --------------------------- | -------- | --------------------------------------------------------------------------------------- |
| `--id=ID`                   | yes      | Record ID as `collection/path/key`.                                                     |
| `--data=YAML`               | yes      | Record fields as YAML or JSON (e.g. `'{name: Ireland}'`).                               |
| `--path=PATH`               | no       | Path to the local database directory. Defaults to the current working directory.        |
| `--remote=HOST/OWNER/REPO[@REF]` | no       | Remote Git repository (e.g. `github.com/owner/repo`). Mutually exclusive with `--path`.   |
| `--token=TOKEN`             | no       | Personal access token. Falls back to host-derived env vars (e.g. `GITHUB_TOKEN`). Required for `--remote` writes. |

**Examples:**

```shell
# 📘 Create a record locally
ingitdb create record --id=countries/ie --data='{name: Ireland}'

# 🐙 Create a record in a GitHub repository
export GITHUB_TOKEN=ghp_...
ingitdb create record --remote=github.com/myorg/mydb --id=countries/ie \
  --data='{name: Ireland, capital: Dublin, population: 5000000}'
```

---

