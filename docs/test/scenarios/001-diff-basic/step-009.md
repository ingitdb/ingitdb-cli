# Step 009 — Add Spain record

## Purpose

Add Spain to the `feature` branch. This creates the **added** record case for the diff.

## Justification

Similar to **Add Ireland record** and **Add France and Germany records** (all add records)
and to **Modify Ireland population** and **Delete France record** (all mutate state on the
branch). Unlike the main-branch additions (which build the baseline), this step adds a
record on the `feature` branch, producing the "added" change type in the diff. A separate
step from **Delete France record** because the two operations are independent: one proves
deletion, the other proves addition, and keeping them in separate commits gives each change
a distinct SHA for commit-annotation assertions.

## Actions

```shell
mkdir -p 'countries/$records/es'

cat > 'countries/$records/es/es.yaml' <<'EOF'
titles:
  en: Spain
population: 47450795
area_km2: 505990
currency: EUR
flag: 🇪🇸
EOF

git add 'countries/$records/es'
git commit -m "feat(countries): add Spain"
```

## Assertions

### Record is readable

**Command:**
```shell
ingitdb select --path=. --id=countries/es
```

**Expected exit code:** `0`

**Expected output contains:**
<!-- assert:contains -->
```
population: 47450795
```

### Feature is 3 commits ahead of main

**Command:**
```shell
git rev-list --count main..feature
```

**Expected exit code:** `0`

**Expected output:**
<!-- assert:exact -->
```
3
```

## Notes

After this step the full git state is:

```
main:    [init] → [schema] → [add ie] → [add fr] → [add de]   ← 5 commits
feature:                                              ↑ branched
                                         → [modify ie] → [delete fr] → [add es]
```

`ingitdb diff main` (run from `feature`) will span these 3 commits.
Assertion steps 010–018 follow.
