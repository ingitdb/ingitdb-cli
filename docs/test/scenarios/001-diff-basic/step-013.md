# Step 013 — Assert `diff main --depth=full`

## Purpose

Verify that before/after field values are shown for every changed field. The Ireland
`population` change is fully deterministic (known before and after values), making
this a strong correctness assertion.

## Justification

Similar to **Assert diff — summary depth**, **Assert diff at record depth**, and
**Assert diff at fields depth**: all test `--depth` variants. Unlike **Assert diff at
fields depth** (field names only), this step verifies that before/after values are
rendered. The Ireland `population` change (`5123536 → 5200000`) is the only assertion in
the scenario with fully deterministic before/after values, making this the strongest
correctness check of the four depth levels.

## Actions

```shell
ingitdb diff main --path=. --depth=full
```

## Assertions

### Exit code: changes found

**Expected exit code:** `1`

### Ireland shows exact before→after values

**Expected output contains:**
<!-- assert:contains -->
```
        population:  5123536  →  5200000
```

### Added record (Spain) shows null→value

For a new record, the "before" value is absent. The output under Spain's fields must
show the "after" values. Null representation for added records:

**Expected output contains:**
<!-- assert:contains -->
```
        population:  (none)  →  47450795
```

> **Open**: The null token `(none)` is provisional. Update this step to match the
> implementation's chosen representation (alternatives: `null`, `—`, empty string).

### Deleted record (France) shows value→null

For a deleted record, the "after" value is absent:

**Expected output contains:**
<!-- assert:contains -->
```
        population:  67750000  →  (none)
```

> **Open**: Same null token caveat as above.

### Unchanged fields do not appear under Ireland

**Command:**
```shell
ingitdb diff main --path=. --depth=full | grep -A10 '/ie' | grep -c 'area_km2'
```

**Expected exit code:** `1` (grep exits 1 — `area_km2` must not appear under Ireland)

## Notes

- The `→` separator character is from the diff spec text-format examples.
  If the implementation uses a different separator (e.g. `->`), update this step.
