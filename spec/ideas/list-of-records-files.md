# Idea: List-of-Records Files: Top-Level YAML/JSON Sequences + JSONL

**Status:** Draft
**Date:** 2026-05-29
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** extends:default-record-format, depends_on:batch-insert

## Problem Statement

How might we let a single file hold many records as a top-level YAML/JSON
sequence (and as JSON Lines), with full read/write/validate support, so
multi-record files are not limited to the ID-keyed map layout?

## Context

inGitDB declares three record layouts in `pkg/ingitdb/record_file_def.go`:

- `SingleRecord` — `map[string]any` (one record per file).
- `MapOfRecords` — `map[$record_id]…` (many records, keyed by ID).
- `ListOfRecords` — `[]map[string]any` (many records as a top-level sequence).

`ListOfRecords` is a declared type and **passes validation** for non-CSV
formats, but it is only actually wired for **CSV** and **INGR** today, each via
its own parser in `pkg/dalgo2ingitdb` (`parseCSVForCollection`,
`parseINGRAsMap`). The general filesystem reader
`pkg/ingitdb/materializer/records_reader_fs.go` implements **only**
`MapOfRecords` and `SingleRecord`; any other type hits
`"record type %q is not supported"`. There is no parser for a top-level
YAML/JSON sequence — `ParseRecordContent` unmarshals into `map[string]any`,
which a top-level `- name: Alex` sequence cannot satisfy.

So a file like:

```yaml
- name: Alex
  age: 4
- name: Bob
  age: 5
```

is **not a usable layout today**: it can be declared but not read, written, or
validated. For YAML, the supported multi-record layout is `MapOfRecords`
(ID-keyed map).

Two adjacent capabilities already exist and should be reused: batch parsers in
`pkg/dalgo2ingitdb/batch_parsers.go` already implement **`ParseBatchJSONL`**,
`ParseBatchYAMLStream`, and key resolution (`$id`/`id`/key column) for bulk
import — but those feed the insert path, not the on-disk record-file layout.
The [`record-merge`](../features/cli/resolve/auto-resolve/record-merge/README.md)
feature also already keys list rows by primary key / `$id` / `id`, and currently
**escalates YAML/JSON `ListOfRecords` conflicts to manual** because the layout
is unread — closing this gap would let those auto-merge too.

The trigger: users naturally expect a YAML/JSON array file to "just work" for
many records, and JSONL is the de-facto interchange format for record streams
(logs, exports, LLM datasets). Supporting them makes inGitDB a better citizen
for data-pipeline and append-heavy workflows.

## Recommended Direction

Make `ListOfRecords` a first-class, fully-wired layout for sequence formats, and
add **JSONL** (`json-lines`) as a new record format whose natural layout is
`ListOfRecords`.

**Read.** Add a list parser alongside the existing map parser in
`pkg/dalgo2ingitdb` — e.g. `ParseListOfRecordsContent(content, format)` returning
`[]map[string]any` — that handles YAML top-level sequences, JSON top-level
arrays, and JSONL (one JSON object per line, reusing `ParseBatchJSONL`). Wire a
`ListOfRecords` case into `records_reader_fs.go` that calls it and yields each
row as a record. Identity for each row follows the CSV/INGR precedent: declared
`primary_key`, else a `$id`/`id` field — needed so the reader can produce keyed
`IRecordEntry` values and so CRUD can address individual rows.

**Write.** Add the symmetric encoder (`EncodeListOfRecordsContent`) and route it
through `EncodeRecordContentForCollection`, preserving record order. JSONL writes
one compact JSON object per line; YAML/JSON write a top-level
sequence/array with deterministic key ordering per `columns_order`.

**Validate.** Relax/confirm `RecordFileDef.Validate` so YAML/JSON/JSONL +
`ListOfRecords` is explicitly allowed, and add `RecordFormatJSONL` with its
natural `ListOfRecords` restriction (mirroring CSV). Validate that list rows
carry a resolvable key when the collection needs per-record addressing.

**Record-merge.** Replace the `default → escalate` branch for YAML/JSON/JSONL
lists in `pkg/ingitdb/recordmerge/bridge.go` with the keyed-list merge already
used for CSV, so these conflicts auto-merge.

This is layered and additive: the three existing wired paths (map YAML/JSON,
single-record, CSV/INGR lists) are untouched; we fill in the unimplemented
YAML/JSON/JSONL list paths and add one new format.

**Unify the list family.** CSV and INGR are already `ListOfRecords`
implementations, but each is a bespoke parser. The clean target is a single
list-of-records layer — one reader/writer dispatch plus one shared
key-resolution rule (`primary_key` → `$id` → `id`) — with **per-format codecs**
(CSV, INGR, YAML-sequence, JSON-array, JSONL) plugging in. CSV and INGR should
be refactored onto this shared layer so identity and dispatch live in one place;
because they are already shipped, that refactor is behavior-preserving and can
land incrementally (add the new codecs against the shared layer first, migrate
CSV/INGR after). The end state: every `ListOfRecords` format reads, writes,
validates, and merges through the same code path.

## Alternatives Considered

- **Leave `ListOfRecords` CSV/INGR-only; tell users to use `MapOfRecords` for
  YAML/JSON.** Rejected — array files are a natural, widely-expected shape, and
  JSONL has no map equivalent at all. The asymmetry ("the type exists but only
  for two formats") is a footgun: configs validate but silently don't work.
- **Support JSONL only, skip YAML/JSON sequences.** Rejected — the read/write
  plumbing for a top-level sequence is shared across all three; doing JSONL
  alone leaves the original YAML-array question unanswered for marginal savings.
- **Auto-derive a synthetic key from row position/index.** Rejected as the
  primary identity scheme — position is unstable across edits and breaks
  three-way merge and CRUD addressing. Key on `primary_key`/`$id`/`id` like
  CSV/INGR; position-only files can still be read but are flagged as
  not-individually-addressable (see Open Questions).
- **Treat list files as opaque text (no record semantics).** Rejected —
  defeats the purpose; inGitDB's value is record-aware read/write/validate/merge.

## MVP Scope

Make a YAML top-level sequence and a JSONL file each round-trip as records
through inGitDB — read, write, validate — and auto-merge in record-merge.

1. `ParseListOfRecordsContent` + `EncodeListOfRecordsContent` in
   `pkg/dalgo2ingitdb` for YAML, JSON, and JSONL (reusing `ParseBatchJSONL`).
2. `RecordFormatJSONL` constant + `RecordFileDef.Validate` rules
   (JSONL ⇒ `ListOfRecords`; YAML/JSON/JSONL + `ListOfRecords` allowed).
3. `ListOfRecords` case in `records_reader_fs.go`, keyed by
   `primary_key`/`$id`/`id`.
4. Record-merge: YAML/JSON/JSONL lists merge via the existing keyed-list path
   instead of escalating.

Verification: round-trip a demo collection of ~5 records as (a) a YAML
sequence and (b) a JSONL file through create/insert/read; confirm a disjoint
two-user append auto-merges via `ingitdb resolve`; confirm deterministic output
ordering.

## Not Doing (and Why)

- Position-indexed identity as the default — unstable across edits; key on
  `primary_key`/`$id`/`id` instead.
- Streaming/huge-file optimization — correctness first; large-file performance is
  a later concern.
- Nested/array-valued field restrictions for JSONL/YAML/JSON — unlike CSV these
  formats represent nesting natively, so no flattening constraint applies.
- Changing the default record format or layout — this is purely additive; YAML
  `MapOfRecords` remains the recommended keyed multi-record layout.
- Converting existing `MapOfRecords` files to sequences automatically — a
  separate one-shot migration concern if ever wanted.

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | A top-level YAML sequence and JSON array can be parsed into `[]map[string]any` without disturbing the existing `map[string]any` parse path. | Add a dedicated list parser; confirm the map parser and its callers are unchanged. |
| Must-be-true | Each list row can be assigned a stable key from `primary_key`/`$id`/`id`, sufficient for CRUD addressing and three-way merge. | Reuse the CSV/INGR/`batch_parsers` key-resolution logic; test files with and without a key field. |
| Must-be-true | `records_reader_fs.go` can gain a `ListOfRecords` case without breaking the two existing cases. | Add the case; run the full reader/materializer test suite. |
| Should-be-true | JSONL read/write reuses `ParseBatchJSONL` cleanly and round-trips byte-stably enough for Git diffs. | Round-trip a JSONL file; diff before/after; confirm one-object-per-line, deterministic key order. |
| Should-be-true | Deterministic output ordering (rows in record order, keys per `columns_order`) keeps Git diffs minimal. | Edit one row; confirm only that row's lines change in `git diff`. |
| Might-be-true | Users want individually-addressable records in array files (vs. treating the whole file as a blob). | Instrument/ask after MVP; if files are append-only logs, addressing may matter less. |

## SpecScore Integration

- **New Features this would create (likely under a `record-format/` or
  `record-layout/` umbrella in `spec/features/`):**
  - List-of-records read/write for YAML, JSON, and JSONL in `pkg/dalgo2ingitdb`
    + the `records_reader_fs.go` `ListOfRecords` case.
  - `RecordFormatJSONL` constant + validation rules.
- **Existing Features affected:**
  - [`cli/resolve/auto-resolve/record-merge`](../features/cli/resolve/auto-resolve/record-merge/README.md)
    — its YAML/JSON list `default → escalate` branch becomes a real keyed-list
    merge; INGR/CSV list handling is the existing precedent to mirror.
  - [`default-record-format`](default-record-format.md) — JSONL joins the
    format menu and the `--format` flag's accepted values.
- **Dependencies:** Go stdlib (`encoding/json`, `gopkg.in/yaml.v3`); existing
  `batch_parsers.go` (`ParseBatchJSONL`, key resolution). No new external deps.

## Open Questions

- **Key field for list rows.** Standardize on `primary_key` → `$id` → `id`
  precedence (matching CSV/INGR/batch)? What is the behavior for a list file with
  no resolvable key — read-only blob, or reject at validation?
- **JSONL format name and extension.** Constant value `json-lines` vs `jsonl`;
  file extension `.jsonl` vs `.ndjson`.
- **YAML/JSON list vs map for the same data.** Should a collection be allowed to
  switch a YAML file between `MapOfRecords` and `ListOfRecords`, and is there a
  recommended default for new YAML collections?
- **Ordering semantics.** Is list order user-meaningful (preserve insertion
  order) or normalized (sort by key)? Affects diffs and merge output.
- **CRUD write granularity.** When updating one record in a list file, rewrite
  the whole file (simple, larger diffs) or splice the single row?
- **Empty list representation.** `[]` for JSON, empty document for YAML, empty
  file for JSONL — confirm each round-trips to an empty record set.
- **Refactor CSV/INGR onto the shared layer now, or later?** Both already ship.
  Migrating them in this effort removes duplicate key-resolution/dispatch logic
  but touches working code; deferring keeps the change purely additive. Decide
  at specify/plan time (likely: add new codecs first, migrate CSV/INGR in a
  follow-up task within the same feature).

---
*This document follows the https://specscore.md/idea-specification*
