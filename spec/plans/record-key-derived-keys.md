# Plan: Derived Record Keys

**Status:** Approved
**Source Feature:** record-key/derived-keys
**Date:** 2026-05-30
**Owner:** @trakhimenok
**Supersedes:** —

## Summary

Implement derived record keys for single-record collections by adding schema support for `record_key`, a shared key resolver, and integrations in validation, insert, and select/read identity surfaces. The plan keeps storage templates based on `{key}.md` while deriving that key from configured record fields.

## Approach

Build the feature from schema boundaries inward: first define and validate the `record_key` configuration, then centralize derivation and transforms in a reusable resolver, then wire that resolver into existing file validation and CLI write/read paths. No acceptance criteria are deferred.

## Tasks

### Task 1: Add record key schema and base validation

**Verifies:** record-key/derived-keys#ac:schema-accepts-field-map, record-key/derived-keys#ac:schema-rejects-placeholder-without-field, record-key/derived-keys#ac:schema-rejects-list-layout, record-key/derived-keys#ac:schema-rejects-empty-template, record-key/derived-keys#ac:schema-rejects-field-without-column

Add the `record_key` definition model with `template` and `fields` keyed by field name, including validation that every placeholder resolves to a configured field and every configured field resolves to a collection column. Reject empty templates and non-single-record collection layouts so unsupported row-identity semantics stay out of the MVP.

### Task 2: Implement shared derived key resolver and transforms

**Verifies:** record-key/derived-keys#ac:schema-rejects-datetime-without-output-format, record-key/derived-keys#ac:schema-rejects-unsupported-transform, record-key/derived-keys#ac:resolver-formats-motivating-key, record-key/derived-keys#ac:resolver-rejects-invalid-runtime-value

Create a reusable resolver that derives keys from parsed record values, supports raw string, datetime output formatting, and slug transforms, and returns structured errors that name the failing field and path context supplied by callers. Validate transform configuration during schema load so unsupported transforms and incomplete datetime settings fail before runtime.

### Task 3: Validate existing files against derived keys

**Verifies:** record-key/derived-keys#ac:validation-detects-filename-drift, record-key/derived-keys#ac:validation-rejects-missing-optional-field

Wire the shared resolver into database validation so existing single-record files are checked against the expected key derived from their parsed content. Report diagnostics containing the file path, actual filename key, expected derived key, and source fields, and treat missing optional-but-derived fields as validation errors.

### Task 4: Derive keys during insert writes

**Verifies:** record-key/derived-keys#ac:insert-omitted-key-uses-derived-key, record-key/derived-keys#ac:insert-missing-derived-field-does-not-write, record-key/derived-keys#ac:insert-supplied-key-must-match

Update `insert` so derived-key collections may omit `--key`, deriving the target key from the submitted record before any file is written. If `--key` is supplied, compare it with the derived key and fail before writing on mismatch; also fail before writing when any required derivation input is absent or invalid.

### Task 5: Preserve non-derived behavior and storage template contract

**Verifies:** record-key/derived-keys#ac:storage-template-keeps-key-placeholder, record-key/derived-keys#ac:non-derived-insert-stays-compatible

Keep `record_file.name` interpolation centered on `{key}` and feed it the already-resolved key, rather than teaching storage templates to interpolate record fields directly. Add regression coverage proving collections without `record_key` retain existing explicit-key insert behavior.

### Task 6: Preserve key identity on read and select surfaces

**Verifies:** record-key/derived-keys#ac:select-exposes-derived-key-as-id

Ensure read/select output continues to expose the filename key as the record identity, so `$id` and equivalent record identifiers match the derived filename key for derived-key collections. Add coverage for records inserted via derived keys and records already present on disk.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/plan-specification*
