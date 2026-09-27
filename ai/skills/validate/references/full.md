# ingitdb validate — full pass

Run the complete validation pipeline against a database directory: schema definitions, every record's conformance to its collection schema, and a rebuild of materialized views.

**CLI reference:** [`ingitdb-cli` README — Quick start](https://github.com/ingitdb/ingitdb-cli#-quick-start)

## When to use

- **Local sanity check:** Catch schema or record errors before committing.
- **CI gate on every push:** Block merges when records violate their schema.
- **Post-schema-edit verification:** Confirm existing records still match an updated definition.

## Command

```bash
ingitdb validate [--path <dir>]
```

## Parameters

| Flag | Required | Description |
|---|---|---|
| `--path` | No | Database directory. Defaults to current working directory. |

## Examples

```bash
# Validate the current directory
ingitdb validate

# Validate a specific database
ingitdb validate --path=/path/to/your/db
```

## Notes

- Validation rebuilds the `$views/` directory in the same pass.
- A non-zero exit means at least one violation was reported on stdout/stderr — surface the message to the user verbatim.
