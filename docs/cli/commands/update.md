### 🔹 update record` — update fields of an existing record

[Source Code](../../../cmd/ingitdb/commands/update.go)


```
ingitdb update record --id=ID --set=YAML [--path=PATH]
ingitdb update record --id=ID --set=YAML --remote=HOST/OWNER/REPO[@REF] [--token=TOKEN]
```

Updates fields of an existing record using patch semantics: only the fields listed in `--set`
are changed; all other fields are preserved.

| Flag                        | Required | Description                                                                             |
| --------------------------- | -------- | --------------------------------------------------------------------------------------- |
| `--id=ID`                   | yes      | Record ID as `collection/path/key`.                                                     |
| `--set=YAML`                | yes      | Fields to patch as YAML or JSON (e.g. `'{capital: Dublin}'`).                           |
| `--path=PATH`               | no       | Path to the local database directory. Defaults to the current working directory.        |
| `--remote=HOST/OWNER/REPO[@REF]` | no       | Remote Git repository (e.g. `github.com/owner/repo`). Mutually exclusive with `--path`.   |
| `--token=TOKEN`             | no       | Personal access token. Falls back to host-derived env vars (e.g. `GITHUB_TOKEN`). Required for `--remote` writes. |

**Examples:**

```shell
# 📘 Patch a record locally
ingitdb update record --id=countries/ie --set='{capital: Dublin}'

# 🐙 Patch a record in a GitHub repository
export GITHUB_TOKEN=ghp_...
ingitdb update record --remote=github.com/myorg/mydb --id=countries/ie \
  --set='{capital: Dublin, population: 5100000}'
```

---

