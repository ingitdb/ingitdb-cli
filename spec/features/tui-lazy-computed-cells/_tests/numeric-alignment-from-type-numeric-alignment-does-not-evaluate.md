# Scenario: numeric-alignment-does-not-evaluate

**Validates:** [tui-lazy-computed-cells#req:numeric-alignment-from-type](../README.md#req-numeric-alignment-from-type)

## Steps

GIVEN an int computed column bound to a counting evaluator
WHEN the screen determines column alignment
THEN the column is right-aligned and the evaluator is not invoked

## Detected Surface

pure-fn

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
