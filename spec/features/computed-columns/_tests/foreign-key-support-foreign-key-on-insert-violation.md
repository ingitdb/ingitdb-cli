---
format: https://specscore.md/scenario-specification
---

# Scenario: foreign-key-on-insert-violation

**Validates:** [computed-columns#req:foreign-key-support](../README.md#req-foreign-key-support)

## Steps

GIVEN a computed column owner_key with foreign_key to users whose formula yields a key absent from users
WHEN a record is inserted whose input fields make owner_key resolve to that absent key
THEN the insert fails with a referential-integrity error naming the record key and owner_key column

## Detected Surface

cli

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
