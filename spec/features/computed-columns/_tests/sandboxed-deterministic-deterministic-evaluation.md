---
format: https://specscore.md/scenario-specification
---

# Scenario: deterministic-evaluation

**Validates:** [computed-columns#req:sandboxed-deterministic](../README.md#req-sandboxed-deterministic)

## Steps

GIVEN any record and a computed column
WHEN the formula is evaluated twice for the same input
THEN both evaluations return byte-identical output and no network, filesystem, clock, or randomness is reachable

## Detected Surface

pure-function

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
