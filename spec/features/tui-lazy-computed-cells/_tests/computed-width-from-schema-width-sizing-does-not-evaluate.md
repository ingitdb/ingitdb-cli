---
format: https://specscore.md/scenario-specification
---

# Scenario: width-sizing-does-not-evaluate

**Validates:** [tui-lazy-computed-cells#req:computed-width-from-schema](../README.md#req-computed-width-from-schema)

## Steps

GIVEN a collection with a computed column bound to a counting evaluator
WHEN the screen computes column widths
THEN the evaluator is not invoked and the computed width equals the header/declared-type width

## Detected Surface

pure-fn

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
