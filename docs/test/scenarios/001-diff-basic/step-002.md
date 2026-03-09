# Step 002 — Register countries collection

## Purpose

Create the `countries` collection schema and register it in `root-collections.yaml`.

## Justification

No similar steps exist. This is the only step that registers a collection schema;
it proves that `ingitdb validate` and `ingitdb list collections` work against a fresh,
empty collection before any records are added.

## Actions

```shell
mkdir -p countries/.collection

cat > countries/.collection/definition.yaml <<'EOF'
titles:
  en: Countries
record_file:
  name: "{key}/{key}.yaml"
  type: "map[string]any"
  format: yaml
columns:
  titles:
    type: "map[locale]string"
    required: true
  population:
    type: number
  area_km2:
    type: number
  currency:
    type: string
  flag:
    type: string
default_view: {}
EOF

printf 'countries: countries\n' > .ingitdb/root-collections.yaml

git add countries .ingitdb/root-collections.yaml
git commit -m "feat: add countries collection schema"
```

## Assertions

### Schema is valid

**Command:**
```shell
ingitdb validate --path=.
```

**Expected exit code:** `0`

### Collection is listed

**Command:**
```shell
ingitdb list collections --path=.
```

**Expected exit code:** `0`

**Expected output contains:**
<!-- assert:contains -->
```
countries
```

### No records yet

The collection exists but has no records. The list output shows `countries` with zero records
(exact format depends on implementation; the collection key must appear and no record paths).

## Notes

- Schema is identical to `demo-dbs/test-db/countries/.collection/definition.yaml`.
- Record path pattern is `{key}/{key}.yaml` inside a `$records/` directory.
