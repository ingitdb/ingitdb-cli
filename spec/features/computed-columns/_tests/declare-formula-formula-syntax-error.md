---
format: https://specscore.md/scenario-specification
---

# Scenario: formula-syntax-error

**Validates:** [computed-columns#req:declare-formula](../README.md#req-declare-formula)

## Steps

GIVEN a column declaring formula 'first_name +' which is not a complete Starlark expression
WHEN the schema is loaded or validated
THEN validation fails naming the collection and the full_name column

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
