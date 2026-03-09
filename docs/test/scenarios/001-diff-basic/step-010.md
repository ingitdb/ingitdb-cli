# Step 010 — Assert `diff main` — summary depth (text, default)

## Purpose

Verify the default text output shows correct added/updated/deleted counts per collection,
and that the header identifies the two refs being compared.

## Justification

Similar to **Assert diff at record depth**, **Assert diff at fields depth**, and
**Assert diff at full depth**: all four run `ingitdb diff main` and verify its output.
Unlike those steps, this step exercises the default `--depth=summary` behaviour —
aggregate counts per collection, no per-record lines. It is the first and simplest diff
assertion and sets the baseline expectation before deeper output modes are tested.

## Actions

```shell
ingitdb diff main --path=.
```

## Assertions

### Exit code: changes found

**Expected exit code:** `1`

### Header identifies comparison

**Expected output contains:**
<!-- assert:contains -->
```
Comparing main..HEAD
```

### Summary table is correct

**Expected output contains:**
<!-- assert:contains -->
```
  Collection          Added  Updated  Deleted
  ─────────────────── ─────  ───────  ───────
  countries               1        1        1
```

### Germany does not appear

Germany was not changed and must not appear in the output at all.

**Command:**
```shell
ingitdb diff main --path=. | grep -c 'de\b'
```

**Expected exit code:** `1` (grep exits 1 when no matches found)

## Notes

- The header line's commit count should show `3 commits` (the three commits on `feature`
  ahead of `main`). Exact wording may vary.
- Column alignment in the table may vary by implementation; the key assertion is the
  presence of `countries` with counts `1  1  1`.
