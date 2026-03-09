# Step 001 — Create empty git repo and `.ingitdb/` structure

## Purpose

Establish a clean, isolated git repository with the minimum `.ingitdb/` layout that
`ingitdb` commands require.

## Justification

No similar steps exist. This is the only step that initialises the repository;
all subsequent steps depend on a clean git repo with `.ingitdb/` in place.

## Actions

```shell
mkdir -p /tmp/ingitdb-scenario-001
cd /tmp/ingitdb-scenario-001

git init
git config user.email "test@example.com"
git config user.name "Test User"

mkdir -p .ingitdb
printf '' > .ingitdb/root-collections.yaml

git add .ingitdb
git commit -m "chore: init ingitdb repo"
```

## Assertions

### Repo has exactly one commit

**Command:**
```shell
git log --oneline
```

**Expected exit code:** `0`

**Expected output contains:**
<!-- assert:contains -->
```
chore: init ingitdb repo
```

### Working tree is clean

**Command:**
```shell
git status --short
```

**Expected exit code:** `0`

**Expected output:**
<!-- assert:exact -->
```
```

(empty — no staged or unstaged changes)

## Notes

- `ingitdb setup` is not yet implemented, so this step constructs the `.ingitdb/` structure
  manually. Update this step if `ingitdb setup` is implemented.
- `root-collections.yaml` is created empty at this stage; it is populated in step-002.
