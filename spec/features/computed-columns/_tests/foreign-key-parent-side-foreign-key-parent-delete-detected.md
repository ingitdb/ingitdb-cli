---
format: https://specscore.md/scenario-specification
---

# Scenario: foreign-key-parent-delete-detected

**Validates:** [computed-columns#req:foreign-key-parent-side](../README.md#req-foreign-key-parent-side)

## Steps

GIVEN a users record referenced by some record's computed owner_key
WHEN that users record is deleted
THEN the delete fails with a referential-integrity error naming the referencing collection, referencing record key, and owner_key column

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
