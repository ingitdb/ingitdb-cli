---
format: https://specscore.md/scenario-specification
---

# Scenario: order-by-computed-column

**Validates:** [computed-columns#req:usable-in-filter-and-sort](../README.md#req-usable-in-filter-and-sort)

## Steps

GIVEN the people collection with computed full_name
WHEN select is run with order_by full_name asc
THEN the returned records are ordered by their computed full_name

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
