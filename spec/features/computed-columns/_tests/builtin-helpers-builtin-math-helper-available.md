# Scenario: builtin-math-helper-available

**Validates:** [computed-columns#req:builtin-helpers](../README.md#req-builtin-helpers)

## Steps

GIVEN an int column rounded with formula 'round(score)' and a record with score 4.6
WHEN the record is read
THEN rounded equals integer 5

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
