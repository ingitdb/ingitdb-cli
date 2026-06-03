# Scenario: visible-computed-error-non-fatal

**Validates:** [tui-lazy-computed-cells#req:visible-error-non-fatal](../README.md#req-visible-error-non-fatal)

## Steps

GIVEN a computed column whose formula raises, positioned within the visible window
WHEN the collection screen renders
THEN the erroring cell shows a bounded error indicator and the screen renders other cells without crashing

## Detected Surface

pure-fn

## TODO

- [ ] Pick Rehearse driver
- [ ] Wire up fixtures
- [ ] Implement assertion

---
*This document follows the https://specscore.md/scenario-specification*
