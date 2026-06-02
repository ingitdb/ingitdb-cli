# Rehearse Scenarios: Computed Columns

**Status:** Draft
**Date:** 2026-06-02
**Owner:** alexander.trakhimenok@gmail.com

Pending Rehearse Scenario stubs for the [computed-columns](../README.md) Feature.
One stub per acceptance criterion; each carries a `## TODO` checklist until wired up.

## Contents

| Scenario | Verifies REQ |
|----------|--------------|
| [formula-declared-and-computed](declare-formula-formula-declared-and-computed.md) | declare-formula, evaluate-on-read |
| [formula-syntax-error](declare-formula-formula-syntax-error.md) | declare-formula |
| [reject-stored-computed-value](reject-stored-value-reject-stored-computed-value.md) | reject-stored-value |
| [reject-chained-computed-reference](stored-fields-only-reject-chained-computed-reference.md) | stored-fields-only |
| [deterministic-evaluation](sandboxed-deterministic-deterministic-evaluation.md) | sandboxed-deterministic |
| [type-coercion-success](coerce-to-type-type-coercion-success.md) | coerce-to-type |
| [unsupported-type-rejected](coerce-to-type-unsupported-type-rejected.md) | coerce-to-type |
| [runtime-error-fails-read](fail-loud-runtime-error-fails-read.md) | fail-loud |
| [filter-on-computed-column](usable-in-filter-and-sort-filter-on-computed-column.md) | usable-in-filter-and-sort |
| [order-by-computed-column](usable-in-filter-and-sort-order-by-computed-column.md) | usable-in-filter-and-sort |
| [foreign-key-on-insert-violation](foreign-key-support-foreign-key-on-insert-violation.md) | foreign-key-support |
| [foreign-key-revalidates-on-input-change](foreign-key-revalidate-on-input-change-foreign-key-revalidates-on-input-change.md) | foreign-key-revalidate-on-input-change |
| [foreign-key-parent-delete-detected](foreign-key-parent-side-foreign-key-parent-delete-detected.md) | foreign-key-parent-side |
| [foreign-key-parent-rename-detected](foreign-key-parent-side-foreign-key-parent-rename-detected.md) | foreign-key-parent-side |
| [builtin-string-helper-available](builtin-helpers-builtin-string-helper-available.md) | builtin-helpers |
| [builtin-math-helper-available](builtin-helpers-builtin-math-helper-available.md) | builtin-helpers |

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/scenarios-index-specification*
