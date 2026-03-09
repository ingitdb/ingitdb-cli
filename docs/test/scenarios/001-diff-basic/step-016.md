# Step 016 — Assert `diff main --depth=record --format=json`

## Purpose

Verify that the `records` structure appears in JSON output at `record` depth with
correct paths, change-type arrays, and commit SHA arrays.

## Justification

Similar to **Assert diff --format=json at summary depth**: both test JSON output format.
Unlike that step (which verifies absence of the `records` key at summary depth), this step
verifies that `records` arrays are present and correctly structured at `record` depth —
including path values, commit SHA format, and per-change-type array lengths. The two JSON
steps together cover the full JSON schema contract.

## Actions

```shell
ingitdb diff main --path=. --depth=record --format=json
```

## Assertions

### Exit code: changes found

**Expected exit code:** `1`

### Output is valid JSON

**Command:**
```shell
ingitdb diff main --path=. --depth=record --format=json | jq .
```

**Expected exit code:** `0`

### `records` key present at record depth

<!-- assert:json-path -->
- `.collections.countries | has("records")` = `true`

### Array lengths correct

<!-- assert:json-path -->
- `.collections.countries.records.added | length` = `1`
- `.collections.countries.records.updated | length` = `1`
- `.collections.countries.records.deleted | length` = `1`

### Correct record paths

<!-- assert:json-path -->
- `.collections.countries.records.updated[0].path` contains `"ie"`
- `.collections.countries.records.deleted[0].path` contains `"fr"`
- `.collections.countries.records.added[0].path` contains `"es"`

### Commit SHA array

<!-- assert:json-path -->
- `.collections.countries.records.updated[0].commits | length` = `1`

**Command (SHA is 7-char hex):**
```shell
ingitdb diff main --path=. --depth=record --format=json \
  | jq -r '.collections.countries.records.updated[0].commits[0]' \
  | grep -E '^[0-9a-f]{7}$'
```

**Expected exit code:** `0`

### Germany not in any records array

**Command:**
```shell
ingitdb diff main --path=. --depth=record --format=json | jq -r '.. | strings' | grep -c '/de'
```

**Expected exit code:** `1` (grep exits 1 — `de` path not found)
