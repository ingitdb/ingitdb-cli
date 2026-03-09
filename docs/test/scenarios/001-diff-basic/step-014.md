# Step 014 — Assert `diff main --collection=countries` scoping

## Purpose

Verify that `--collection=countries` limits output to the `countries` collection
without error, and that the result matches the unscoped output (since only one
collection exists in this scenario).

## Justification

Unlike the four `--depth` steps (**Assert diff — summary depth** through **Assert diff at
full depth**), this step tests a different axis: the `--collection` scope filter rather
than output verbosity. It verifies that the flag is parsed and applied without error, and
that it does not suppress the exit code — a concern specific to filtering that no depth
step covers.

## Actions

```shell
ingitdb diff main --path=. --collection=countries
```

## Assertions

### Exit code: changes found

**Expected exit code:** `1`

### Output matches unscoped summary

**Expected output contains:**
<!-- assert:contains -->
```
  countries               1        1        1
```

### No error output

**Command:**
```shell
ingitdb diff main --path=. --collection=countries 2>&1 | grep -i error
```

**Expected exit code:** `1` (grep exits 1 — no error lines)

### Non-existent collection

**Command:**
```shell
ingitdb diff main --path=. --collection=nonexistent
echo "exit: $?"
```

> **Open**: Expected exit code for an unknown collection key is unspecified.
> Candidates:
> - `0` — collection not found, no changes to report (treat as empty scope)
> - `2` — invalid argument error
>
> Update this step once the spec or implementation decides.

## Notes

- In a multi-collection database, `--collection=countries` must suppress all other
  collections from the output. That case is not covered by this scenario (only one
  collection exists here) and belongs in a future scenario.
