# Step 018 — Assert exit code 1 — CI guard pattern

## Purpose

Verify the CI guard idiom: `ingitdb diff` exits `1` when changes are found, causing
a `&&`-chained command to be skipped. This is the primary use case for `ingitdb diff`
in CI pipelines.

## Justification

Similar to **Assert exit code 0 — no changes**: both test exit codes. Unlike that step
(which verifies the no-changes case in isolation), this step verifies the CI guard idiom
end-to-end — that exit `1` correctly short-circuits a `&&` chain in a shell script. The
distinction matters because a correct exit code is necessary but not sufficient; the
shell integration must also behave as expected for `ingitdb diff` to be usable in CI
pipelines.

## Actions

```shell
ingitdb diff main --path=. --collection=countries && echo "No changes" || echo "Changes detected"
```

## Assertions

### Exit code is 1

**Expected exit code of `ingitdb diff`:** `1`

### CI guard fires

**Expected terminal output:**
<!-- assert:exact -->
```
Changes detected
```

(The `&&` short-circuits so "No changes" is never printed; `||` catches the non-zero exit.)

### Scoped check also exits 1

`--collection` must not suppress the exit code:

**Command:**
```shell
ingitdb diff main --path=. --collection=countries
echo "exit: $?"
```

**Expected output contains:**
<!-- assert:contains -->
```
exit: 1
```

## Notes

Exit code summary per spec:

| Code | Meaning |
|------|---------|
| `0`  | Diff completed; no changes found |
| `1`  | Diff completed; one or more changes found |
| `2`  | Infrastructure error (git not found, invalid ref, bad flags) |

Typical CI guard:
```shell
ingitdb diff origin/main --collection=countries && echo "No schema drift"
```

This step concludes scenario 001. Clean up with:
```shell
cd /
rm -rf /tmp/ingitdb-scenario-001
```
