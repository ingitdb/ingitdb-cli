# Step 003 — Add Ireland record and commit

## Purpose

Add the first record to `main`. Ireland will later be modified on the `feature` branch,
making it appear as an **updated** record in the diff.

## Justification

Similar to **Add France and Germany records**: both add records to the `countries` collection.
Unlike that step (which adds to an already non-empty collection), this step adds the very
first record, proving the empty→non-empty transition. It also establishes Ireland as the
record that will later be modified, so a single-record commit keeps its commit SHA isolated
and unambiguous in `--depth=record` assertions.

## Actions

```shell
mkdir -p 'countries/$records/ie'

cat > 'countries/$records/ie/ie.yaml' <<'EOF'
titles:
  en: Ireland
population: 5123536
area_km2: 70273
currency: EUR
flag: 🇮🇪
EOF

git add 'countries/$records'
git commit -m "feat(countries): add Ireland"
```

## Assertions

### Record is readable

**Command:**
```shell
ingitdb read record --path=. --id=countries/ie
```

**Expected exit code:** `0`

**Expected output contains:**
<!-- assert:contains -->
```
population: 5123536
```

**Expected output contains:**
<!-- assert:contains -->
```
titles:
  en: Ireland
```

## Notes

- The `$records` directory name starts with `$`, which requires quoting in shell.
  All commands in this scenario single-quote path segments containing `$records`.
- `population: 5123536` is the baseline value. Step-007 changes it to `5200000`.
