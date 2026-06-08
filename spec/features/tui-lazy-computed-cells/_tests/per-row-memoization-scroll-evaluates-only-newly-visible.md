---
format: https://specscore.md/scenario-specification
---

# Scenario: scroll-evaluates-only-newly-visible

**Validates:** [tui-lazy-computed-cells#req:per-row-memoization](../README.md#req-per-row-memoization)

## Steps

GIVEN the same collection rendered once at the top
WHEN the user scrolls so a previously off-viewport record becomes visible and a previously visible record is repainted
THEN the newly-visible record's computed cell is evaluated and the repainted record's is not evaluated again

## Detected Surface

pure-fn

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
