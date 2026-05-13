# Step 008 — Delete France record

## Purpose

Delete France from the `feature` branch. This creates the **deleted** record case for the diff.

## Justification

Similar to **Modify Ireland population** and **Add Spain record**: all three mutate state
on the `feature` branch. Unlike **Modify Ireland population** (which edits a field) and
**Add Spain record** (which creates a new record), this step removes an existing record
entirely, producing the "deleted" change type in the diff. All three change types
(updated, deleted, added) are needed to fully exercise every code path in `ingitdb diff`.

## Actions

```shell
ingitdb delete --path=. --id=countries/fr

# If ingitdb delete does not auto-stage the removal, stage it manually:
# git add -u 'countries/$records/fr'

git commit -m "chore(countries): remove France"
```

## Assertions

### Record no longer readable

**Command:**
```shell
ingitdb select --path=. --id=countries/fr
```

**Expected exit code:** `2` (record not found / error)

### File no longer exists

**Command:**
```shell
test -f 'countries/$records/fr/fr.yaml' && echo "exists" || echo "gone"
```

**Expected exit code:** `1` (the `test` command fails because file is absent)

**Expected output:**
<!-- assert:exact -->
```
gone
```

## Notes

- If `ingitdb delete` performs `git rm` internally, no manual staging is needed.
  If it only deletes the file from disk, add `git add -u` before committing.
  The correct behaviour should be documented in [`docs/cli/commands/delete.md`](../../../cli/commands/delete.md).
