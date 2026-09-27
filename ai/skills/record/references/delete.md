# ingitdb delete

Delete a single record by ID, or delete records from a collection matching a `--where` filter.

## Command

```bash
# Single record
ingitdb delete --path <dir> --id <collection-id>/<record-key>

# Bulk by filter
ingitdb delete --from <collection-id> --where '<expr>'
```

## Parameters

| Flag | Required | Description |
|---|---|---|
| `--id` | One of | Single-record delete. |
| `--from` + `--where` | One of | Bulk delete by filter. Same operator set as [`select`](select.md). |
| `--path` / `--remote` | One of | Local or remote target. |
| `--token` | If remote | Required for write to remote. |

## Examples

```bash
ingitdb delete --path=. --id=geo.nations/ie
ingitdb delete --from=countries.counties --where='status==archived'
```

## Notes

- Destructive. There is no `--dry-run` flag today — prefer `select` with the same `--where` first to inspect what would be deleted.
- To drop an entire collection, use `ingitdb drop collection <id>` instead.
