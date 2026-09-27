# ingitdb validate — definitions only

Validate only the collection definitions (schema YAML files), skipping per-record checks. Faster than a full pass when you've only changed schema files.

## Command

```bash
ingitdb validate --only=definition [--path <dir>]
```

## When to use

- Editing collection schemas without touching records.
- Quick syntax check on schema YAML.

## Notes

- This is a subset of [`validate full`](full.md) — same exit-code contract.
