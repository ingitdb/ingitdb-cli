---
format: https://specscore.md/scenario-specification
---

# Scenario: off-viewport-error-never-evaluated

**Validates:** [tui-lazy-computed-cells#req:render-time-computed-eval](../README.md#req-render-time-computed-eval)

## Steps

GIVEN a computed column whose formula raises, positioned only on off-viewport records
WHEN the collection screen renders
THEN the screen renders normally and the erroring computed column is never evaluated

## Detected Surface

pure-fn

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
