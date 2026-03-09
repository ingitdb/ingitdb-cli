# Step 006 — Create feature branch

## Purpose

Branch off `main` so that subsequent changes are isolated and `ingitdb diff main`
has a meaningful comparison to make.

## Justification

No similar steps exist. `ingitdb diff main` requires at least two divergent refs to
compare; this step creates that divergence. Without a separate branch, all subsequent
mutation steps would advance `main` itself and there would be nothing to diff against.

## Actions

```shell
git checkout -b feature
```

## Assertions

### Active branch is feature

**Command:**
```shell
git branch --show-current
```

**Expected exit code:** `0`

**Expected output:**
<!-- assert:exact -->
```
feature
```

### Feature is at the same commit as main

**Command:**
```shell
git rev-list --count main..feature
```

**Expected exit code:** `0`

**Expected output:**
<!-- assert:exact -->
```
0
```

(zero commits ahead of `main` at this point)
