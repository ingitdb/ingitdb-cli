# Scenario: formula-declared-and-computed

**Validates:** [computed-columns#req:declare-formula](../README.md#req-declare-formula)

## Steps

GIVEN a people collection with a string column full_name and formula 'first_name + " " + last_name'
WHEN a record with first_name Ada and last_name Lovelace is read via select
THEN the returned full_name equals "Ada Lovelace"

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
