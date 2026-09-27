# ingitdb validate — records only

Validate only the records against existing collection schemas. Skips re-validating the schemas themselves.

## Command

```bash
ingitdb validate --only=records [--path <dir>]
```

## When to use

- Bulk record import or edit, when the schema is already trusted.
- Investigating which records violate the schema after a known-good schema pass.

## Notes

- Records-only validation does NOT verify schema syntax — pair with [`validate definitions`](definitions.md) if you've also edited schemas.
