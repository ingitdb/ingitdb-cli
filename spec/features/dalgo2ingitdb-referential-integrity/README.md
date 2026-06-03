# Feature: DALgo Referential Integrity

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/dalgo2ingitdb-referential-integrity?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/dalgo2ingitdb-referential-integrity?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/dalgo2ingitdb-referential-integrity?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/dalgo2ingitdb-referential-integrity?op=request-change) |
**Status:** Stable
**Date:** 2026-05-30
**Owner:** @trakhimenok
**Source Idea:** [`dalgo2ingitdb-referential-integrity`](../../ideas/dalgo2ingitdb-referential-integrity.md)
**Supersedes:** —
**Grade:** A

## Summary

DALgo adapter callers get write-time referential integrity for schemas that already declare `foreign_key` on collection columns. `Set`, `Insert`, and `Update` reject non-empty references to missing parent records, and `Delete` rejects removing a parent record while child records still point at it.

## Problem

`pkg/ingitdb` already carries column-level `foreign_key` metadata, but `pkg/dalgo2ingitdb` read-write transactions currently persist records without enforcing it. A caller can create dangling references by inserting or updating a child row, or by deleting a parent row that is still referenced elsewhere. Those failures are expensive to discover later because the filesystem state already looks successful to the DALgo caller.

## Behavior

### Write-time foreign-key validation

DALgo write transactions enforce the existing `ColumnDef.ForeignKey` metadata without changing the schema language.

#### REQ: validate-fk-target-on-write

For every `Set`, `Insert`, and `Update` that writes a record in a collection with one or more foreign-key columns, the adapter MUST validate each non-empty foreign-key value before mutating storage. A non-empty value is valid only when the referenced collection exists in the loaded definition and contains a record whose key equals the foreign-key value.

#### REQ: set-is-covered

`Set` MUST receive the same referential-integrity checks as `Insert` and `Update`, because `Set` can either create a new child record or overwrite an existing child record.

#### REQ: optional-fk-values

Missing, nil, and empty-string foreign-key values MUST be allowed when the foreign-key column is not otherwise required by the schema. Existing required-column validation MUST continue to reject missing, nil, or empty values for required columns before or alongside foreign-key validation.

#### REQ: invalid-fk-declaration

When a foreign-key column names a referenced collection that does not exist in the loaded definition, affected `Set`, `Insert`, and `Update` operations MUST fail with a configuration error before mutating storage.

### Restrict delete

Deletes use conservative restrict semantics for parent records. The MVP does not cascade, set child values to null, or repair children automatically.

#### REQ: restrict-delete-when-referenced

Before deleting a record, the adapter MUST scan collections whose columns declare `foreign_key` pointing at the target record's collection. If any child record has a non-empty foreign-key value equal to the target key, `Delete` MUST fail and leave the parent record in place.

#### REQ: delete-unreferenced-parent

Deleting a parent record with no matching child references MUST continue to behave like the existing DALgo delete path.

### Failure behavior

Referential-integrity failures are ordinary DALgo operation failures, not panics and not partial transaction commits.

#### REQ: deterministic-errors

Every referential-integrity failure MUST return a deterministic error that names the operation, target collection, relevant field, referenced collection, and offending key when those facts are known. Delete failures MUST also identify at least one child collection and child key that blocks the delete.

#### REQ: no-partial-write

When referential-integrity validation fails, the adapter MUST leave all files that would have been mutated by the failed operation unchanged.

## Acceptance Criteria

### AC: insert-valid-parent-succeeds (verifies REQ:validate-fk-target-on-write)

**Given** a child collection has a foreign-key column pointing at a parent collection and the parent record exists
**When** a DALgo caller inserts a child record whose foreign-key value equals the parent key
**Then** the insert succeeds and the child record is persisted.

### AC: insert-missing-parent-fails (verifies REQ:validate-fk-target-on-write, REQ:deterministic-errors, REQ:no-partial-write)

**Given** a child collection has a foreign-key column pointing at a parent collection and no parent record exists for key `missing-parent`
**When** a DALgo caller inserts a child record with that foreign-key value
**Then** the insert fails with an error naming the child collection, field, parent collection, and `missing-parent`, and no child record file is created.

### AC: update-missing-parent-fails (verifies REQ:validate-fk-target-on-write, REQ:deterministic-errors, REQ:no-partial-write)

**Given** an existing child record currently points at a valid parent record
**When** a DALgo caller updates the child record so its foreign-key value points at a missing parent key
**Then** the update fails with a deterministic referential-integrity error and the existing child record remains unchanged.

### AC: set-missing-parent-fails (verifies REQ:set-is-covered, REQ:validate-fk-target-on-write, REQ:no-partial-write)

**Given** a child collection has a foreign-key column pointing at a parent collection
**When** a DALgo caller calls `Set` with a child record whose foreign-key value points at a missing parent key
**Then** `Set` fails before creating or overwriting the child record.

### AC: optional-empty-fk-is-allowed (verifies REQ:optional-fk-values)

**Given** a child collection has a non-required foreign-key column
**When** a DALgo caller inserts or updates a child record with that field missing, nil, or an empty string
**Then** foreign-key validation does not reject the operation because of that field value.

### AC: required-empty-fk-still-fails (verifies REQ:optional-fk-values)

**Given** a child collection has a required foreign-key column
**When** a DALgo caller writes a child record with that field missing, nil, or an empty string
**Then** the write fails and no record mutation is persisted.

### AC: invalid-fk-target-collection-fails (verifies REQ:invalid-fk-declaration, REQ:deterministic-errors, REQ:no-partial-write)

**Given** a collection schema declares `foreign_key: missing_collection` on one of its columns
**When** a DALgo caller writes a record with a non-empty value for that column
**Then** the operation fails with a configuration error naming `missing_collection`, and no record mutation is persisted.

### AC: delete-referenced-parent-fails (verifies REQ:restrict-delete-when-referenced, REQ:deterministic-errors, REQ:no-partial-write)

**Given** a child record has a foreign-key value equal to parent key `p1`
**When** a DALgo caller deletes parent record `p1`
**Then** the delete fails with an error naming the parent collection, `p1`, and at least one blocking child collection and key, and the parent file remains in place.

### AC: delete-unreferenced-parent-succeeds (verifies REQ:delete-unreferenced-parent)

**Given** parent record `p2` exists and no child record has a foreign-key value equal to `p2`
**When** a DALgo caller deletes parent record `p2`
**Then** the delete succeeds with the existing DALgo delete behavior.

## Architecture

- Add the referential-integrity checks inside `pkg/dalgo2ingitdb` read-write transaction logic, close to the current `Set`, `Insert`, `Update`, and `Delete` mutation paths.
- Reuse the loaded inGitDB definition and existing record loading helpers; do not add a new schema field or a parallel constraint model.
- Resolve foreign-key metadata from collection columns. A child column with `foreign_key: parent_collection` references records in `parent_collection` by record key.
- For the MVP, delete protection may scan child collections directly. Index-backed lookup and materialized FK views are deferred until there is measured need.

## Data Flow

For `Set`, `Insert`, and `Update`, the adapter resolves the would-be child record, inspects its collection's foreign-key columns, skips optional empty values, checks each non-empty value against the referenced parent collection, and only then writes the record. For `Delete`, the adapter identifies child collections that reference the target collection, scans child records for matching non-empty values, and only deletes the parent when no match exists.

## Error Handling and Failure Modes

Foreign-key target misses, invalid foreign-key declarations, and restrict-delete conflicts return errors to the DALgo caller. They must not panic, must not be logged as success-shaped warnings, and must not leave partially written record files behind.

## Testing Strategy

Implementation should add Go tests under `pkg/dalgo2ingitdb` using parent/child fixture collections. Tests should cover successful writes, missing-parent failures for `Insert`, `Update`, and `Set`, optional empty values, required empty values, invalid referenced collections, referenced-parent delete failure, unreferenced-parent delete success, and unchanged files after every rejected write.

## Rehearse Integration

No Rehearse stubs are scaffolded. Every acceptance criterion above has a direct Go test surface in `pkg/dalgo2ingitdb`, and implementation should add those tests in the existing Go test suites.

## Out of Scope

- Cascade delete, set-null, or other automatic child mutation policies.
- Repository-wide offline FK audit or validation commands.
- Redesigning `ColumnDef.ForeignKey` into richer constraint metadata.
- Materialized or persisted FK indexes for delete checks.
- Enforcing referential integrity in non-DALgo write paths.

## Assumption Carryover

| Idea assumption | Feature treatment |
|---|---|
| `ColumnDef.ForeignKey` values reliably identify the referenced collection whose record key should match the FK field value. | Carried into REQ:validate-fk-target-on-write, REQ:invalid-fk-declaration, and the parent/child ACs. |
| Delete-time referrer scans are acceptable for expected DALgo-backed repository sizes. | Carried into REQ:restrict-delete-when-referenced and Architecture; indexing is out of scope until performance data says otherwise. |
| DALgo callers expect constraint failures to be ordinary returned errors, not panics or partial transaction writes. | Carried into REQ:deterministic-errors and REQ:no-partial-write. |

## Open Questions

- Should FK error messages become a stable public error type in addition to deterministic text?

---
*This document follows the https://specscore.md/feature-specification*
