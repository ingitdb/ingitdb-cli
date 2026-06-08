---
format: https://specscore.md/scenario-specification
---

# Scenario: reject-stored-computed-value

**Validates:** [computed-columns#req:reject-stored-value](../README.md#req-reject-stored-value)

## Steps

GIVEN a computed column full_name
WHEN an insert or a record file under validation supplies a full_name value
THEN the operation fails naming the collection, record key, and full_name column

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
