---
format: https://specscore.md/scenario-specification
---

# Scenario: reject-chained-computed-reference

**Validates:** [computed-columns#req:stored-fields-only](../README.md#req-stored-fields-only)

## Steps

GIVEN a computed column greeting whose formula references another computed column full_name
WHEN the schema is loaded or validated
THEN validation fails stating a formula may reference only stored fields

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
