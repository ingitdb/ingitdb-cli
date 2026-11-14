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
- Records are checked in root collections and in subcollections at any depth.
- On success the output has one line per collection: root collections first, then every
  subcollection by its full schema path, with counts added up over all parents, then a total:

  ```
  All 4 records are valid for collection: spaces
  All 2 records are valid for collection: spaces/ext/days
  Validated 104 records in 49 collections (2 root collections, 47 subcollections)
  ```

  A line reading `All 0 records` for a collection you expected to hold data means no record files
  were found where that collection's records live; check the path before trusting the pass.
- A subcollection's records live where the definition puts them: in a directory named by the
  parent key, inside the parent's records directory (`spaces/$records/<key>/ext/` by default,
  `spaces/<key>/ext/` with `records_dir: .`). Files found at `spaces/<key>/ext/` while `spaces`
  keeps its records in `$records` (where older storage drivers wrote them) are an error that names
  the found path, the file count and the expected path. Fix it by moving the files, or by setting
  `record_file.records_dir: "."` on the parent and moving its records out of `$records/`.
- A `WARNING:` line (for example a parent directory that is a symbolic link leading outside the
  database, whose records are not read) does not change the exit code; report it to the user.
- A non-zero exit means at least one violation was reported on stdout/stderr — surface the message to the user verbatim.
