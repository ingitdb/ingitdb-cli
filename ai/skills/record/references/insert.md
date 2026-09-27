# ingitdb insert

Create a new record in a `map[string]any` collection.

## Command

```bash
# Local
ingitdb insert --path <dir> --id <collection-id>/<record-key> --data '<yaml-or-json>'

# Remote (write requires token)
ingitdb insert --remote <host/owner/repo> --id <collection-id>/<record-key> --data '<yaml-or-json>'
```

## Parameters

| Flag | Required | Description |
|---|---|---|
| `--id` | Yes | `<collection-id>/<record-key>`. Collection IDs allow alphanumeric and `.`. |
| `--data` | Yes | Record content as YAML or JSON map literal. |
| `--path` | One of | Local database directory. |
| `--remote` | One of | Remote repo (e.g., `github.com/owner/repo[@ref]`). |
| `--token` | If write+remote | Token for remote write. Or set `GITHUB_TOKEN` env var. |

## Example

```bash
ingitdb insert --path=. --id=geo.nations/ie --data='{title: "Ireland"}'
```

## Notes

- Fails if the record key already exists. Use [`update`](update.md) to modify an existing record.
- The collection must be `record_file.type: "map[string]any"`. Other collection types are not supported by record CRUD.
