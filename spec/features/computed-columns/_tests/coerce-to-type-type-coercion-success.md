---
format: https://specscore.md/scenario-specification
---

# Scenario: type-coercion-success

**Validates:** [computed-columns#req:coerce-to-type](../README.md#req-coerce-to-type)

## Steps

GIVEN an int column total with formula 'qty * price' and a record with qty 3 and price 4
WHEN the record is read
THEN total equals integer 12

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
