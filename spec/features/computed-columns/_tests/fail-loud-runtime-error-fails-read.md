---
format: https://specscore.md/scenario-specification
---

# Scenario: runtime-error-fails-read

**Validates:** [computed-columns#req:fail-loud](../README.md#req-fail-loud)

## Steps

GIVEN a computed int column whose formula divides by a field that is zero for some record
WHEN that record is read
THEN the read aborts naming the collection, record key, and column, and no partial row is emitted

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
