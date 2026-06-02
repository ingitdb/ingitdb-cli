# Scenario: filter-on-computed-column

**Validates:** [computed-columns#req:usable-in-filter-and-sort](../README.md#req-usable-in-filter-and-sort)

## Steps

GIVEN the people collection with computed full_name
WHEN select is run with --where 'full_name == "Ada Lovelace"'
THEN only records whose computed full_name equals "Ada Lovelace" are returned

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
