# Plan: DALgo Referential Integrity

**Status:** Implemented
**Source Feature:** dalgo2ingitdb-referential-integrity
**Date:** 2026-05-31
**Owner:** @trakhimenok
**Supersedes:** —
**Mode:** full

## Summary

Implement DALgo write-time referential integrity for existing `foreign_key` metadata. The work is split into shared child-write validation, complete write-operation wiring, nullable/required value semantics, and restrict-delete protection.

## Approach

Build one reusable referential-integrity validation path inside `pkg/dalgo2ingitdb`, then call it from each mutating operation before file writes happen. Delete protection comes after child-write validation because it needs the same schema traversal but scans reverse references instead of resolving parent records.

## Tasks

### Task 1: Add child-write FK target validation

**Verifies:** dalgo2ingitdb-referential-integrity#ac:insert-valid-parent-succeeds, dalgo2ingitdb-referential-integrity#ac:insert-missing-parent-fails, dalgo2ingitdb-referential-integrity#ac:invalid-fk-target-collection-fails
**Status:** done
**Depends-On:** —

Add DALgo fixture coverage for parent and child collections with `foreign_key` metadata. Implement shared validation that resolves referenced collections from the loaded definition, confirms non-empty FK values point at existing parent record keys, rejects invalid referenced collection declarations as configuration errors, and runs before `Insert` persists data.

### Task 2: Wire Update and Set through the same validation

**Verifies:** dalgo2ingitdb-referential-integrity#ac:update-missing-parent-fails, dalgo2ingitdb-referential-integrity#ac:set-missing-parent-fails
**Status:** done
**Depends-On:** 3

Call the shared FK validation from `Update` and `Set` before any record mutation is written. Add regression tests proving failed `Update` and `Set` calls return deterministic errors and leave existing or absent child records unchanged.

### Task 3: Preserve optional and required FK value semantics

**Verifies:** dalgo2ingitdb-referential-integrity#ac:optional-empty-fk-is-allowed, dalgo2ingitdb-referential-integrity#ac:required-empty-fk-still-fails
**Status:** done
**Depends-On:** 1

Define the empty-value boundary for FK validation so missing, nil, and empty-string values are skipped for non-required FK columns. Keep existing required-column behavior authoritative for required FK columns, and add tests for both optional and required empty values.

### Task 4: Enforce restrict-delete parent protection

**Verifies:** dalgo2ingitdb-referential-integrity#ac:delete-referenced-parent-fails, dalgo2ingitdb-referential-integrity#ac:delete-unreferenced-parent-succeeds
**Status:** done
**Depends-On:** 2

Before deleting a parent record, scan collections with FK columns that reference the parent collection. Fail deletes that have at least one matching child reference with an error that identifies the parent and blocking child record, and preserve existing delete behavior when no child reference matches.

## Open Questions

- Should the implementation introduce exported sentinel errors for FK failures now, or keep them as deterministic wrapped errors until callers need type assertions?

---
*This document follows the https://specscore.md/plan-specification*
