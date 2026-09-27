# ingitdb update

Update fields of an existing record. Patch semantics — only listed fields change; unlisted fields are preserved.

## Command

```bash
ingitdb update --path <dir> --id <collection-id>/<record-key> --set '<yaml-or-json>'
```

## Parameters

| Flag | Required | Description |
|---|---|---|
| `--id` | Yes | `<collection-id>/<record-key>`. |
| `--set` | Yes | Map of field → new value. Only listed fields are written. |
| `--path` / `--remote` | One of | Local or remote target. |
| `--token` | If write+remote | Required for any write operation. |

## Example

```bash
ingitdb update --path=. --id=geo.nations/ie --set='{title: "Ireland, Republic of"}'
```

## Notes

- Fails if the record does not exist. Use [`insert`](insert.md) to create.
- Patch — does NOT delete keys not listed in `--set`.
