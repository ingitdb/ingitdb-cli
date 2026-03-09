# Step 017 — Assert exit code 0 — no changes

## Purpose

Verify that `ingitdb diff` exits `0` (not `1`) when the two refs are identical.
This validates the CI guard pattern: exit `0` means "safe to proceed".

## Justification

Similar to **Assert exit code 1 — CI guard pattern**: both test exit codes. Unlike that
step (which verifies exit `1` when changes are present), this step verifies the complement
case — exit `0` when no changes exist. Both sides of the exit-code contract must be tested;
a command that always exits `1` would pass all the "changes found" assertions but break
every CI guard that expects `0` on a clean branch.

## Actions

```shell
git checkout main
ingitdb diff main --path=.
echo "exit: $?"
git checkout feature
```

## Assertions

### Exit code is 0

**Expected exit code:** `0`

Quoting the spec:
> Exit code `0`: Diff completed; no changes found.

### Output reflects no changes

**Expected output contains:**
<!-- assert:contains -->
```
  countries               0        0        0
```

Or the output is empty / shows a "no changes" message — either is acceptable.
The critical assertion is the exit code.

### Return to feature branch

After the assertion, verify we are back on `feature`:

**Command:**
```shell
git branch --show-current
```

**Expected output:**
<!-- assert:exact -->
```
feature
```

## Notes

- Comparing `main` against itself (`ingitdb diff main` while on `main`) is the
  simplest way to produce a zero-change diff without adding new commits.
- An equivalent form: `ingitdb diff main..main --path=.`
