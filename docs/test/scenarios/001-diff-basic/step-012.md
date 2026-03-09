# Step 012 — Assert `diff main --depth=fields`

## Purpose

Verify that field names are listed under each changed record. For Ireland (updated),
only `population` must appear — not `area_km2`, `currency`, or `flag`, which were unchanged.

## Justification

Similar to **Assert diff — summary depth**, **Assert diff at record depth**, and
**Assert diff at full depth**: all test `--depth` variants. Unlike **Assert diff at
record depth** (record lines only), this step verifies that field names appear under
each record. Unlike **Assert diff at full depth** (which shows values), this step shows
names only — making it the right place to assert that `area_km2`, `currency`, and `flag`
are absent under Ireland, since those fields did not change.

## Actions

```shell
ingitdb diff main --path=. --depth=fields
```

## Assertions

### Exit code: changes found

**Expected exit code:** `1`

### Ireland shows only the changed field

The output under the Ireland record must contain `population`:

**Expected output contains:**
<!-- assert:contains -->
```
        population
```

### Ireland does not show unchanged fields

**Command:**
```shell
ingitdb diff main --path=. --depth=fields | grep -A5 '/ie' | grep -c 'area_km2'
```

**Expected exit code:** `1` (grep exits 1 — `area_km2` must not appear under Ireland)

Same check for `currency` and `flag`.

### Spain (added) shows all its fields

Since Spain is a new record, all its fields must be listed. The output under the Spain
record must contain at least:

**Expected output contains:**
<!-- assert:contains -->
```
        titles
```

**Expected output contains:**
<!-- assert:contains -->
```
        population
```

### France (deleted) shows all its fields

Same as Spain — all fields of the deleted record should appear in the fields list.

## Notes

- The indentation in the spec is 8 spaces for field names under a record line.
  Actual indentation may vary; the key assertion is presence of the field name in
  proximity to the record path.
