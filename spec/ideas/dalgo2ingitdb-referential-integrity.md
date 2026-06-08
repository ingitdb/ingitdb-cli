---
format: https://specscore.md/idea-specification
status: Approved
---

# Idea: DALgo referential integrity on writes

**Status:** Approved
**Date:** 2026-05-30
**Owner:** @trakhimenok
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we let DALgo callers preserve inGitDB foreign-key relationships during writes, without turning the filesystem driver into a full relational database?

## Context

pkg/ingitdb/ColumnDef already exposes a foreign_key field, and pkg/dalgo2ingitdb read-write transactions currently persist Set, Insert, Update, and Delete operations without validating parent existence or child references.

## Recommended Direction

Add runtime referential guardrails inside `pkg/dalgo2ingitdb` write transactions. `Set`, `Insert`, and `Update` should validate non-empty foreign-key values against the referenced collection before persisting a record; `Delete` should fail with a clear error when another record still points at the target.

Keep the first version deliberately conservative: treat missing, nil, or empty FK values as allowed unless the column is also required, use restrict semantics for delete, and surface deterministic errors to DALgo callers. Do not add cascade behavior, schema redesign, or repository-wide validation in the MVP; those are separate ideas once the runtime contract proves useful.

## Alternatives Considered

Validation-only reporting would catch existing broken references but would not protect the write path that creates them. That is useful later for audits, but it misses the user's stated insert/update/delete pain.

Materialized FK indexes could make delete checks faster, especially for large repositories, but they add an optimization dependency before the correctness contract exists. The first version can use straightforward collection scans and leave indexing as a follow-up if performance data justifies it.

A richer schema language with per-FK actions would make constraints more expressive, but it risks designing cascade/set-null semantics before inGitDB has proven the simpler restrict behavior. The existing `foreign_key` field is enough to validate the first useful case.

## MVP Scope

A focused DALgo transaction change that prevents new broken references and blocks deletion of referenced records for schemas that already declare column-level foreign_key metadata.

## Not Doing (and Why)

- Cascade or set-null delete actions — restrict semantics are safer and smaller for the first contract
- Repository-wide FK audit command — this Idea targets DALgo runtime writes, not offline validation
- Redesigning the schema language — the existing foreign_key field is enough to validate the first useful path

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | `ColumnDef.ForeignKey` values reliably identify the referenced collection whose record key should match the FK field value. | Add tests using a schema with parent and child collections; verify valid writes pass and missing parent keys fail. |
| Should-be-true | Delete-time referrer scans are acceptable for the expected DALgo-backed repository sizes. | Benchmark or fixture-test deletes with several child collections before adding index-backed lookup work. |
| Might-be-true | DALgo callers expect constraint failures to be ordinary returned errors, not panics or partial transaction writes. | Review existing DALgo error patterns and assert failed writes leave the filesystem unchanged. |


## SpecScore Integration

- **New Features this would create:** A DALgo write-time referential-integrity feature under `spec/features/`.
- **Existing Features affected:** `spec/features/dalgo2ingitdb-dbschema-ddl-coverage/README.md`
- **Dependencies:** none

## Open Questions

- Should FK error messages include the child collection, field, and offending value in a stable format for callers to assert against?

---
*This document follows the https://specscore.md/idea-specification*
