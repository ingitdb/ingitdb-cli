# Scenario: off-viewport-rows-not-evaluated

**Validates:** [tui-lazy-computed-cells#req:render-time-computed-eval](../README.md#req-render-time-computed-eval)

## Steps

GIVEN a collection with a computed column bound to a counting evaluator and more records than fit the visible row window of height V
WHEN the collection screen renders without scrolling
THEN the evaluator is invoked only for the V visible records and zero times for off-viewport records

## Detected Surface

pure-fn

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
