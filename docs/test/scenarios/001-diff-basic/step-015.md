# Step 015 — Assert `diff main --format=json` at summary depth

## Purpose

Verify the JSON output structure at `summary` depth: correct top-level fields,
correct collection counts, and absence of `records` key (which only appears at
`record` depth and below).

## Justification

Similar to **Assert diff --format=json at record depth**: both test JSON output.
Unlike the text-format depth steps, this step verifies the JSON document structure
rather than human-readable formatting. Unlike **Assert diff --format=json at record
depth**, this step specifically verifies the `summary` depth JSON — where the `records`
key must be absent — proving that depth level correctly controls JSON schema shape, not
just text verbosity.

## Actions

```shell
ingitdb diff main --path=. --format=json
```

## Assertions

### Exit code: changes found

**Expected exit code:** `1`

### Output is valid JSON

**Command:**
```shell
ingitdb diff main --path=. --format=json | jq .
```

**Expected exit code:** `0` (jq exits 0 on valid JSON)

### JSON structure — top-level fields

<!-- assert:json-path -->
- `.from` = `"main"`
- `.to` = `"HEAD"`
- `.total_commits` = `3`

### JSON structure — collections

<!-- assert:json-path -->
- `.collections | keys` = `["countries"]`
- `.collections.countries.added` = `1`
- `.collections.countries.updated` = `1`
- `.collections.countries.deleted` = `1`

### No `records` key at summary depth

<!-- assert:json-path -->
- `.collections.countries | has("records")` = `false`

**Command:**
```shell
ingitdb diff main --path=. --format=json | jq 'has("records")'
```

**Expected output:**
<!-- assert:exact -->
```
false
```
