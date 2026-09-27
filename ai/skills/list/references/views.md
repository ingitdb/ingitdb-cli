# ingitdb list views

Enumerate materialized views in a database.

## Command

```bash
ingitdb list views [--path <dir>]
```

## When to use

- **Discovery:** Which views are materialized in this database?
- **After `validate` / `materialize`:** Confirm the expected views were built.

## Notes

- Views live under `$views/`. They're rebuilt by `ingitdb validate` and `ingitdb materialize`.
