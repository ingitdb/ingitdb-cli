# Step 011 — Assert `diff main --depth=record`

## Purpose

Verify that record-level output shows one line per changed record with the correct
change-type symbol, path, and a commit annotation. Germany must not appear.

## Justification

Similar to **Assert diff — summary depth**, **Assert diff at fields depth**, and
**Assert diff at full depth**: all test `--depth` variants of the same diff. Unlike
**Assert diff — summary depth** (counts only, no record lines), this step verifies that
individual records appear with their change-type symbols (`+`, `~`, `-`) and commit-hash
annotations. Unlike **Assert diff at fields depth** and **Assert diff at full depth**, it
does not show field names or values — only record identity.

## Actions

```shell
ingitdb diff main --path=. --depth=record
```

## Assertions

### Exit code: changes found

**Expected exit code:** `1`

### countries section heading present

**Expected output contains:**
<!-- assert:contains -->
```
  countries
```

### Ireland appears as updated

**Expected output contains:**
<!-- assert:contains -->
```
    ~ countries/
```

The line must contain `ie` and `[1 commit:` followed by a 7-character hex SHA.

### France appears as deleted

**Expected output contains:**
<!-- assert:contains -->
```
    - countries/
```

The line must contain `fr` and `[1 commit:`.

### Spain appears as added

**Expected output contains:**
<!-- assert:contains -->
```
    + countries/
```

The line must contain `es` and `[1 commit:`.

### Germany does not appear

**Command:**
```shell
ingitdb diff main --path=. --depth=record | grep -c '/de'
```

**Expected exit code:** `1` (grep exits 1 when no matches found)

### Commit SHAs are 7-character hex

Each record line's `[N commit: SHA]` annotation must match the pattern `[0-9a-f]{7}`.

## Notes

- The exact path format (`countries/ie` vs. `countries/$records/ie/ie.yaml`) is an open
  question — see scenario README. Update this step once the implementation decides.
- Symbols: `+` added, `~` updated, `-` deleted — per the diff spec.
