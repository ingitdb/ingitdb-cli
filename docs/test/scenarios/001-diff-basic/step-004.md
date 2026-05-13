# Step 004 — Add France and Germany records

## Purpose

Add the remaining two baseline records to `main` in a single commit. France will be
deleted on the `feature` branch (deleted record case); Germany will remain untouched
(unchanged record case — must never appear in diff output).

Adding both here avoids a repetitive "add another record" step that would prove nothing
beyond what step-003 already established.

## Justification

Similar to **Add Ireland record**: both add records to the `countries` collection.
Unlike that step (which adds the first record to prove the empty→non-empty transition),
this step batches two records in one commit because adding to a non-empty collection
was already proven. France and Germany serve distinct roles in later diff assertions
(France will be deleted; Germany will remain untouched), but neither role requires a
separate commit.

## Actions

```shell
mkdir -p 'countries/$records/fr' 'countries/$records/de'

cat > 'countries/$records/fr/fr.yaml' <<'EOF'
titles:
  en: France
population: 67750000
area_km2: 643801
currency: EUR
flag: 🇫🇷
EOF

cat > 'countries/$records/de/de.yaml' <<'EOF'
titles:
  en: Germany
population: 83369843
area_km2: 357022
currency: EUR
flag: 🇩🇪
EOF

git add 'countries/$records'
git commit -m "feat(countries): add France and Germany"
```

## Assertions

### Both records are readable

**Command:**
```shell
ingitdb select --path=. --id=countries/fr
```

**Expected exit code:** `0`

**Expected output contains:**
<!-- assert:contains -->
```
population: 67750000
```

**Command:**
```shell
ingitdb select --path=. --id=countries/de
```

**Expected exit code:** `0`

**Expected output contains:**
<!-- assert:contains -->
```
population: 83369843
```

## Notes

- `population: 67750000` for France is the value that must appear in `--depth=full`
  output (step-013) as the "before" value of a deleted record.
- Germany's values never appear in any diff output — its presence here is solely to
  verify that unchanged records are silently omitted.
