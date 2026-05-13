# Step 007 — Modify Ireland population

## Purpose

Update a single field (`population`) in the Ireland record on the `feature` branch.
This creates the **updated** record case for the diff. Only `population` changes —
all other fields remain identical — which validates that `--depth=fields` and
`--depth=full` report only the changed field.

## Justification

Similar to **Delete France record** and **Add Spain record**: all three mutate state on
the `feature` branch. Unlike **Delete France record** (which removes a record entirely)
and **Add Spain record** (which creates a new one), this step edits a single field of an
existing record, producing the "updated" change type in the diff. Changing only
`population` — while leaving all other fields identical — is deliberate: it makes the
`--depth=fields` and `--depth=full` assertions deterministic (only one field must appear).

## Actions

```shell
cat > 'countries/$records/ie/ie.yaml' <<'EOF'
titles:
  en: Ireland
population: 5200000
area_km2: 70273
currency: EUR
flag: 🇮🇪
EOF

git add 'countries/$records/ie/ie.yaml'
git commit -m "fix(countries): update Ireland population"
```

## Assertions

### Updated value is readable

**Command:**
```shell
ingitdb select --path=. --id=countries/ie
```

**Expected exit code:** `0`

**Expected output contains:**
<!-- assert:contains -->
```
population: 5200000
```

### Other fields unchanged

The output must also contain:

<!-- assert:contains -->
```
area_km2: 70273
```

## Notes

- Before: `population: 5123536`
- After: `population: 5200000`

These are the values that must appear in `--depth=full` output in step-013.
