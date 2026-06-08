---
format: https://specscore.md/scenario-specification
---

# Scenario: foreign-key-revalidates-on-input-change

**Validates:** [computed-columns#req:foreign-key-revalidate-on-input-change](../README.md#req-foreign-key-revalidate-on-input-change)

## Steps

GIVEN a record whose computed owner_key currently resolves to an existing users record
WHEN an update changes an input field so owner_key would resolve to a non-existent users record
THEN the update fails with a referential-integrity error even though owner_key was not directly written

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
