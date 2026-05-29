# Plan: List-of-Records Files (YAML/JSON Sequences + JSONL)

**Status:** Approved
**Source Feature:** record-format/list-of-records
**Date:** 2026-05-29
**Owner:** Alexander Trakhimenok
**Supersedes:** —

## Summary

Decomposes the approved `record-format/list-of-records` Feature into five
dependency-ordered tasks: a JSONL format + validation, a list parser with the
shared key-resolution rule, reader wiring, a list writer, and record-merge
wiring — so YAML sequences, JSON arrays, and JSONL files read, write, validate,
and auto-merge.

## Approach

Tasks are vertical slices ordered so each builds on the last. Task 1 adds the
`jsonl` format and the `RecordFileDef.Validate` combos (no data needed). Task 2
adds the list parser for yaml/json/jsonl plus the single key-resolution rule
(`primary_key` → `$id` → `id`) that the reader, writer, and merge all reuse.
Task 3 wires the `ListOfRecords` case into the filesystem reader, yielding
ordered keyed records and rejecting keyless lists. Task 4 adds the symmetric
writer (insertion order, JSONL one-object-per-line, field order per
`columns_order`). Task 5 replaces record-merge's YAML/JSON/JSONL list escalation
with the keyed-list merge. No ACs are deferred.

## Tasks

### Task 1: JSONL format and record-type validation

**Verifies:** record-format/list-of-records#ac:jsonl-requires-list, record-format/list-of-records#ac:yaml-list-validates

Add the `RecordFormatJSONL` constant (`jsonl`, extension `.jsonl`) and extend
`RecordFileDef.Validate` so `{yaml, json, jsonl}` + `ListOfRecords` is accepted
and `jsonl` with `SingleRecord`/`MapOfRecords` is rejected with a clear error
(mirroring the CSV restriction).

### Task 2: List parser and shared key resolution

**Verifies:** record-format/list-of-records#ac:key-from-id-field

Add `ParseListOfRecordsContent` in `pkg/dalgo2ingitdb` for YAML top-level
sequences, JSON arrays, and JSONL (reusing `ParseBatchJSONL`), plus a single
key-resolution helper (`primary_key` → `$id` → `id`, joining composite keys)
that the reader, writer-side identity, and record-merge all consume.

### Task 3: Reader wiring for the list layout

**Verifies:** record-format/list-of-records#ac:yaml-list-loads, record-format/list-of-records#ac:read-preserves-order, record-format/list-of-records#ac:keyless-list-fails

Add the `ListOfRecords` case to `pkg/ingitdb/materializer/records_reader_fs.go`,
yielding each parsed row as a keyed record in file order, and reject (at
read/validate time) a list whose rows have no resolvable key.

### Task 4: List writer

**Verifies:** record-format/list-of-records#ac:write-appends-in-order, record-format/list-of-records#ac:jsonl-one-object-per-line, record-format/list-of-records#ac:field-order-follows-columns

Add `EncodeListOfRecordsContent` and route it through
`EncodeRecordContentForCollection`: preserve record (insertion) order, emit
keys per `columns_order`, and write one compact JSON object per line for
`jsonl` (top-level sequence/array for yaml/json).

### Task 5: Record-merge wiring for list formats

**Verifies:** record-format/list-of-records#ac:list-conflict-auto-merges

Replace the YAML/JSON/JSONL `default → escalate` branch in
`pkg/ingitdb/recordmerge/bridge.go` with the keyed-list three-way merge already
used for CSV, so these conflicts auto-merge instead of escalating.

## Open Questions

- `record-merge`'s list serialization for the new formats must round-trip
  through the Task 4 writer; confirm the merge path and the CRUD write path
  share one encoder rather than diverging.

---
*This document follows the https://specscore.md/plan-specification*
