# Feature: List-of-Records Files (YAML/JSON Sequences + JSONL)

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-format/list-of-records?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-format/list-of-records?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-format/list-of-records?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-format/list-of-records?op=request-change) |
**Status:** Approved
**Parent Feature:** [`record-format`](../README.md)
**Source Idea:** [`list-of-records-files`](../../../ideas/list-of-records-files.md)

## Summary

Make the `ListOfRecords` layout (`[]map[string]any`) a fully-wired,
first-class way to store many records in one file for **YAML** (top-level
sequence), **JSON** (top-level array), and a new **JSONL** format (one JSON
object per line) — read, write, and validate — and let
[`record-merge`](../../cli/resolve/auto-resolve/record-merge/README.md)
auto-merge such files instead of escalating them.

## Problem

`ListOfRecords` is a declared record type, but only **CSV** and **INGR** are
wired for it, each via a bespoke parser in `pkg/dalgo2ingitdb`. The general
filesystem reader (`pkg/ingitdb/materializer/records_reader_fs.go`) implements
only `MapOfRecords` and `SingleRecord`; any other type returns
`"record type %q is not supported"`. There is no parser for a top-level
YAML sequence or JSON array — `ParseRecordContent` unmarshals into
`map[string]any`, which a top-level sequence cannot satisfy. As a result a file
like:

```yaml
- name: Alex
  age: 4
- name: Bob
  age: 5
```

can be declared but not read, written, or validated. JSON arrays and JSONL
(the de-facto record-stream interchange format) have no support at all, and
record-merge escalates every YAML/JSON `ListOfRecords` conflict to manual
purely because the layout is unread.

## Behavior

### Layouts and formats

#### REQ: yaml-json-jsonl-list

The system MUST support the `ListOfRecords` (`[]map[string]any`) layout for the
`yaml`, `json`, and `jsonl` formats, in addition to the existing `csv` and
`ingr` support.

#### REQ: jsonl-format

The system MUST recognize a new record format `jsonl` (file extension
`.jsonl`), in which each non-empty line is one JSON object. Its only valid
record type MUST be `ListOfRecords`.

#### REQ: validation-accepts-list

`RecordFileDef.Validate` MUST accept the combination of `{yaml, json, jsonl}`
with `ListOfRecords`, and MUST reject `jsonl` combined with `SingleRecord` or
`MapOfRecords` with a clear error (mirroring the existing CSV restriction).

### Record identity

#### REQ: row-key-resolution

Each row's record key MUST be resolved in this order: the collection's declared
`primary_key` (joining values for a composite key), else a `$id` field, else an
`id` field. This rule MUST be defined once and reused by reading, write-side
identity, and record-merge.

#### REQ: keyless-list-rejected

A `ListOfRecords` collection whose rows carry no resolvable key (no
`primary_key`, no `$id`, no `id`) MUST fail validation with an error explaining
that list records require a key.

### Reading

#### REQ: read-sequence

The reader MUST parse a top-level YAML sequence, a top-level JSON array, or a
JSONL stream into ordered records, and yield each record with its resolved key,
preserving file order.

### Writing

#### REQ: write-sequence-ordered

The writer MUST serialize records back to the declared format preserving record
(insertion) order — newly added records are appended at the end. YAML/JSON
write a top-level sequence/array; `jsonl` writes exactly one compact JSON object
per record line.

#### REQ: deterministic-field-order

Within each record, keys MUST be emitted following the collection's
`columns_order` (remaining keys in a stable order) so that edits to one record
produce minimal Git diffs.

### Conflict resolution

#### REQ: list-auto-merge

`record-merge` MUST resolve `yaml`, `json`, and `jsonl` `ListOfRecords`
conflicts through the keyed-list three-way merge already used for CSV, rather
than escalating them to manual solely because of the layout.

## Acceptance Criteria

### AC: yaml-list-loads (verifies REQ:yaml-json-jsonl-list)

**Given** a collection whose `record_file` is `type: "[]map[string]any"`,
`format: yaml`, holding a top-level sequence of two records
**When** the database is read
**Then** both records are returned (no `"record type not supported"` error).

### AC: jsonl-requires-list (verifies REQ:jsonl-format, REQ:validation-accepts-list)

**Given** a `record_file` with `format: jsonl` and `type` of `map[string]any`
**When** the record-file definition is validated
**Then** validation fails, naming `[]map[string]any` as the required type.

### AC: yaml-list-validates (verifies REQ:validation-accepts-list)

**Given** a `record_file` with `format: yaml` and `type: "[]map[string]any"`
**When** the record-file definition is validated
**Then** validation passes.

### AC: key-from-id-field (verifies REQ:row-key-resolution)

**Given** a JSON-array collection with no declared `primary_key`, whose objects
each carry a `$id`
**When** the file is read
**Then** each record's key equals its `$id` value.

### AC: keyless-list-fails (verifies REQ:keyless-list-rejected)

**Given** a `yaml` `ListOfRecords` collection with no `primary_key` and rows
that have neither `$id` nor `id`
**When** the definition is validated
**Then** validation fails with an error that a record key is required.

### AC: read-preserves-order (verifies REQ:read-sequence)

**Given** a YAML top-level sequence of records in the order r1, r2, r3
**When** the file is read
**Then** the records are yielded in the order r1, r2, r3.

### AC: write-appends-in-order (verifies REQ:write-sequence-ordered)

**Given** a `yaml` `ListOfRecords` file holding r1, r2 to which r3 is added
**When** the file is written
**Then** the file lists r1, r2, r3 in that order.

### AC: jsonl-one-object-per-line (verifies REQ:write-sequence-ordered)

**Given** two records written to a `jsonl` collection file
**When** the file is serialized
**Then** the output has exactly two lines, each a standalone valid JSON object.

### AC: field-order-follows-columns (verifies REQ:deterministic-field-order)

**Given** a collection with `columns_order: [name, age]` and a record holding
both fields
**When** the record is written
**Then** `name` is emitted before `age`.

### AC: list-conflict-auto-merges (verifies REQ:list-auto-merge)

**Given** a `yaml` `ListOfRecords` file where two branches each appended a
record with a distinct key, producing a merge conflict
**When** `ingitdb resolve` runs
**Then** both records are present in the merged file and it is staged (no longer
reported by `git diff --name-only --diff-filter=U`).

## Out of Scope

- Refactoring the existing CSV and INGR parsers onto the shared list layer —
  deferred to a follow-up; only the shared key-resolution rule is unified now.
- Position-indexed identity — list records require a resolvable key
  (`REQ:keyless-list-rejected`).
- Streaming / large-file performance optimization — correctness first.
- Changing the default record format or layout — purely additive; `MapOfRecords`
  remains the recommended keyed multi-record YAML layout.
- Splicing a single row on update — the writer rewrites the whole file in this
  Feature (see Open Questions).

## Open Questions

- **CRUD write granularity.** Whole-file rewrite (chosen here) vs. single-row
  splice — revisit if large list files show diff noise on single-record edits.
- **YAML list vs map default.** Whether new YAML collections should ever default
  to `ListOfRecords`; for now `MapOfRecords` stays the recommended default.
- **Empty-list representation.** Confirm `[]` (JSON), an empty document (YAML),
  and an empty file (JSONL) each round-trip to an empty record set.

---
*This document follows the https://specscore.md/feature-specification*
