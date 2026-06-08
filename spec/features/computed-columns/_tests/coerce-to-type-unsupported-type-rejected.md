---
format: https://specscore.md/scenario-specification
---

# Scenario: unsupported-type-rejected

**Validates:** [computed-columns#req:coerce-to-type](../README.md#req-coerce-to-type)

## Steps

GIVEN a computed column declared with type datetime
WHEN the schema is loaded or validated
THEN validation fails because computed columns support only string, int, float, bool, and any

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
