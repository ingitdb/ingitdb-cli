# Scenario 001 — diff basic

## Goal

Verify that `ingitdb diff main` correctly reports record-level changes between a base
branch and the current HEAD across all depth levels (`summary`, `record`, `fields`, `full`),
output formats (`text`, `json`), collection scoping, and exit codes.

## Status

**Draft** — `ingitdb diff` is not yet implemented. These steps serve as acceptance criteria.

## Database

Single `countries` collection. Schema mirrors the `countries` collection from
the [demo-ingitdb](https://github.com/ingitdb/demo-ingitdb) `geo` module.

Records committed to `main`:

| Key | name       | population | area_km2 | currency |
|-----|------------|------------|----------|----------|
| ie  | Ireland    | 5 123 536  | 70 273   | EUR      |
| fr  | France     | 67 750 000 | 643 801  | EUR      |
| de  | Germany    | 83 369 843 | 357 022  | EUR      |

Changes on `feature` branch:

| Key | Change   | What changed            |
|-----|----------|-------------------------|
| ie  | updated  | population: 5 200 000   |
| fr  | deleted  | —                       |
| es  | added    | Spain, 47 450 795       |
| de  | —        | unchanged (must not appear in diff) |

## Git State

```
main:    [init] → [schema] → [add ie] → [add fr] → [add de]
                                                         ↑
feature: (branched here) → [modify ie] → [delete fr] → [add es]
```

`ingitdb diff main` is run from `feature` HEAD. The diff spans 3 commits.

## Open Questions

1. **Record path format in output**: The spec examples show logical paths like `countries/ie`.
   The filesystem path is `countries/$records/ie/ie.yaml`. Step-011 uses the filesystem path —
   update once the implementation decides which form to display.
2. **Null representation in `--depth=full`**: The JSON spec uses `null`; the text format is
   unspecified. Step-013 pins this as `(none)` — update if the implementation differs.
3. **`--collection=nonexistent` exit code**: Step-014 asks whether this exits `0` or `2`.
4. **`ingitdb delete` staging**: Does it `git rm` or only delete from disk? Step-008 notes the conditional.

## Steps

| Step | Title |
|------|-------|
| [001](step-001.md) | Create empty git repo and `.ingitdb/` structure |
| [002](step-002.md) | Register countries collection |
| [003](step-003.md) | Add Ireland record and commit |
| [004](step-004.md) | Add France and Germany records (batch) |
| [006](step-006.md) | Create feature branch |
| [007](step-007.md) | Modify Ireland population |
| [008](step-008.md) | Delete France record |
| [009](step-009.md) | Add Spain record |
| [010](step-010.md) | Assert `diff main` — summary depth (text) |
| [011](step-011.md) | Assert `diff main --depth=record` |
| [012](step-012.md) | Assert `diff main --depth=fields` |
| [013](step-013.md) | Assert `diff main --depth=full` |
| [014](step-014.md) | Assert `diff main --collection=countries` scoping |
| [015](step-015.md) | Assert `diff main --format=json` at summary depth |
| [016](step-016.md) | Assert `diff main --depth=record --format=json` |
| [017](step-017.md) | Assert exit code 0 — no changes |
| [018](step-018.md) | Assert exit code 1 — CI guard pattern |

## Related

- Feature spec: [`ingitdb-specs/docs/features/diff.md`](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/features/diff.md)
- CLI reference: [`docs/cli/commands/diff.md`](../../cli/commands/diff.md)
- Schema reference: `countries/.collection/definition.yaml` from the
  [demo-ingitdb](https://github.com/ingitdb/demo-ingitdb) `geo` module.
