# Scenario: foreign-key-parent-rename-detected

**Validates:** [computed-columns#req:foreign-key-parent-side](../README.md#req-foreign-key-parent-side)

## Steps

GIVEN a users record referenced by some things record's computed owner_key
WHEN that users record's key is renamed so the old key no longer exists
THEN the rename fails with a referential-integrity error in the reference-error shape naming things, the referencing key, owner_key, and users

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
