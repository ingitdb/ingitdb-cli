# Scenario: builtin-string-helper-available

**Validates:** [computed-columns#req:builtin-helpers](../README.md#req-builtin-helpers)

## Steps

GIVEN a string column display with formula 'first_name.strip().upper()' and a record with first_name ' ada '
WHEN the record is read
THEN display equals "ADA"

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
