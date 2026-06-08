---
format: https://specscore.md/scenario-specification
---

# Scenario: stored-locale-discovery-unchanged

**Validates:** [tui-lazy-computed-cells#req:stored-columns-unchanged](../README.md#req-stored-columns-unchanged)

## Steps

GIVEN a collection with an L10N stored column whose locale keys appear only on off-viewport records
WHEN the locale dropdown is built
THEN every locale across all records appears, exactly as before this Feature

## Detected Surface

pure-fn

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
