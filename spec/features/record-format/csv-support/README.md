# Feature: CSV Format Support

**Status:** Implemented
**Source Idea:** [`default-record-format`](../../../ideas/default-record-format.md)
**Parent Feature:** [`record-format`](../README.md)

## Summary

Add CSV as the seventh first-class record format alongside the existing six (`yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`). Use Go's standard `encoding/csv` for serialization. The CSV file shape is: **header row in schema-defined column order**, then one data row per record. CSV is restricted to `RecordType: ListOfRecords` (analogous to the existing INGR restriction); the writer fails cleanly with a typed error when asked to serialize a record with nested or array-valued fields.

## Problem

CSV is the universal tabular format — Excel, Pandas, `awk`/`sed`, every data tool. inGitDB users with flat-schema collections (no nested data) have a clear use case: serialize records as CSV to integrate with analytics pipelines. Today inGitDB has no CSV support at all.

## Behavior

### REQ: csv-record-format-constant

The `pkg/ingitdb` package MUST export a new `RecordFormat` constant `RecordFormatCSV` with the string value `"csv"`. The constant lives alongside the existing six in `pkg/ingitdb/constants.go`.

#### AC-1: constant-exists

**Given** a Go program that imports `github.com/ingitdb/ingitdb-cli/pkg/ingitdb`
**When** the program references `ingitdb.RecordFormatCSV`
**Then** the program compiles and `ingitdb.RecordFormatCSV == ingitdb.RecordFormat("csv")`.

### REQ: csv-record-type-restriction

`pkg/ingitdb/record_file_def.go` MUST validate that a collection declared with `format: csv` has `RecordType: ListOfRecords` (not `SingleRecord` and not `MapOfRecords`). The validation lives in `RecordFileDef.Validate` alongside the existing INGR restriction (currently lines 68–70 for INGR).

#### AC-1: csv-rejects-single-record

**Given** a `RecordFileDef{Format: RecordFormatCSV, RecordType: SingleRecord}` value
**When** `Validate()` is called
**Then** the result is a non-nil error whose message mentions both `csv` and `SingleRecord`, listing `ListOfRecords` as the only supported `RecordType` for CSV.

#### AC-2: csv-rejects-map-of-records

**Given** a `RecordFileDef{Format: RecordFormatCSV, RecordType: MapOfRecords}` value
**When** `Validate()` is called
**Then** the result is a non-nil error similar to AC-1 with `MapOfRecords` named.

#### AC-3: csv-accepts-list-of-records

**Given** a `RecordFileDef{Format: RecordFormatCSV, RecordType: ListOfRecords}` value
**When** `Validate()` is called
**Then** the result is `nil` (CSV with the supported RecordType validates cleanly).

### REQ: csv-read-path

CSV reading requires schema access (to validate the header row and to map columns to fields). The existing schema-agnostic `ParseRecordContent(content, format)` cannot serve CSV — it has no `*CollectionDef` parameter. The CSV read path MUST therefore be routed through (or modeled on) the existing schema-aware helper `ParseRecordContentForCollection(content, colDef *ingitdb.CollectionDef)` that already handles the markdown case. Concretely, `ParseRecordContentForCollection` gains a `case ingitdb.RecordFormatCSV:` branch alongside its existing markdown branch and its delegation to `ParseRecordContent` for the other formats.

The CSV branch MUST read the input as RFC 4180 CSV using Go's `encoding/csv`. The first row is treated as a header row (column names); subsequent rows are records.

The reader MUST validate that the header row matches the collection's schema, which it accesses via `colDef.ColumnsOrder` (the declared column order). When the header has extra columns, missing columns, or columns in a different order than `ColumnsOrder`, the reader returns a typed error naming the mismatch.

#### AC-1: csv-read-roundtrip-list-of-records

**Given** a CSV file with a header row `id,email,age` and two data rows `1,alice@example.com,30` and `2,bob@example.com,25`, AND a collection schema declaring columns `id`, `email`, `age` in that order
**When** the reader parses the file via the CSV path
**Then** the result is a slice of two record maps with the correct field-name → value bindings.

#### AC-2: csv-read-rejects-mismatched-header

**Given** a CSV file with header `id,email` AND a collection schema declaring columns `id`, `email`, `age`
**When** the reader parses the file
**Then** the result is a non-nil error whose message identifies the missing column (`age`).

#### AC-3: csv-read-rejects-reordered-header

**Given** a CSV file with header `email,id,age` AND a collection schema declaring columns `id`, `email`, `age` in that order
**When** the reader parses the file
**Then** the result is a non-nil error identifying the order mismatch.

### REQ: csv-write-path

CSV writing requires schema access (to emit the header row in declared column order and to enforce column-set match). The existing schema-agnostic `marshalForFormat(value, format)` and `encodeRecordContent(data, format)` therefore CANNOT serve CSV — they have no `*CollectionDef` parameter. The CSV write path MUST be routed through a new schema-aware writer helper that accepts `(value, colDef *ingitdb.CollectionDef)` (mirroring the read side's `ParseRecordContentForCollection` pattern). The new helper handles the `RecordFormatCSV` case directly and delegates to the existing schema-agnostic writers for the other six formats.

The writer MUST serialize the input as RFC 4180 CSV using Go's `encoding/csv`. The header row is emitted first, with column names in `colDef.ColumnsOrder` order. Each subsequent line is one record's values, with each cell looked up by column name in the same order. Strings containing commas, quotes, or newlines are quoted per RFC 4180 (Go's `encoding/csv` handles this natively). Row order in the output preserves the input slice's iteration order — CSV writing accepts only `[]map[string]any` (a list of records), not `map[string]map[string]any` (a keyed-by-id map), because the latter has no deterministic iteration order.

#### AC-1: csv-write-header-in-schema-order

**Given** a collection with schema columns declared in order `id`, `name`, `email` AND records to write
**When** the writer serializes the records as CSV
**Then** the first line of the output is exactly `id,name,email\n`.

#### AC-2: csv-write-roundtrip-byte-stability

**Given** an input `[]map[string]any` of two flat records with primitive values AND a collection schema with three columns declared in order `id`, `name`, `email`
**When** the writer serializes them as CSV against that schema
**Then** the bytes produced are deterministic across multiple invocations: header row is `id,name,email`; data rows are emitted in the slice's iteration order; column values within each row are emitted in schema order (not map iteration order).

#### AC-3: csv-write-rejects-keyed-input

**Given** an input of type `map[string]map[string]any` (records keyed by id) AND a flat-schema collection
**When** the CSV writer is invoked
**Then** the writer returns a typed error indicating that CSV accepts only `[]map[string]any` (a list), not a keyed map — because map iteration order is non-deterministic in Go and CSV row order matters.

### REQ: csv-nested-field-error

When the CSV writer encounters a record containing a nested object (e.g. `map[string]any`) or an array-valued field (`[]any`, slice of structs, etc.) as a column value, it MUST fail with a typed error rather than coerce silently. The error identifies the offending field name and the record's position in the input.

#### AC-1: csv-write-rejects-nested-object

**Given** an input record where field `address` has value `map[string]any{"city": "Berlin"}`
**When** the writer attempts to serialize the record as CSV
**Then** the result is a non-nil error whose message identifies the field name `address` and the constraint ("CSV does not support nested or array-valued fields").

#### AC-2: csv-write-rejects-array-field

**Given** an input record where field `tags` has value `[]any{"a", "b", "c"}`
**When** the writer attempts to serialize the record as CSV
**Then** the result is a non-nil error similar to AC-1 with `tags` named.

## Architecture

| File | Change |
|---|---|
| `pkg/ingitdb/constants.go` | Add `RecordFormatCSV RecordFormat = "csv"` constant. |
| `pkg/ingitdb/record_file_def.go` | Extend `RecordFileDef.Validate` with CSV→ListOfRecords restriction (mirrors INGR validation at lines 68–70). |
| `pkg/dalgo2ingitdb/parse.go` | Add `case ingitdb.RecordFormatCSV:` to the existing schema-aware `ParseRecordContentForCollection` (read path; markdown is the existing precedent). Add a new schema-aware writer helper that accepts `(*CollectionDef, value)` and dispatches CSV directly, delegating to the existing `marshalForFormat` for the other six formats. Header validation logic for read. |
| `pkg/dalgo2ghingitdb/tx_readwrite.go` | Route CSV writes through the new schema-aware writer helper (the GitHub backend's current `encodeRecordContent` is also schema-agnostic and must either be extended to accept `*CollectionDef` or wrapped in a schema-aware caller; pick at plan time). |
| `pkg/dalgo2ingitdb/parse_test.go` | New CSV-specific tests covering all ACs above. |

## Testing Strategy

In-package Go tests using `encoding/csv`-produced fixtures. Two fixture flavors: flat-schema collection (works), nested-schema collection (writer errors). Header-validation tests use mismatched-header CSV strings.

## Rehearse Integration

All ACs are testable via `go test ./pkg/dalgo2ingitdb/... ./pkg/ingitdb/...`. No external scaffolding needed.

## Outstanding Questions

- **Empty/null cell handling.** Go's `encoding/csv` writes an empty string for empty cells. Does inGitDB's read path treat `""` as the empty string OR as a null/missing value? Decide at plan time; document in godoc on the CSV read path.
- **Boolean serialization.** CSV has no native boolean type. Write as `true`/`false`? `1`/`0`? Empty for false? Go convention is `true`/`false` text — defaulting to that unless a contrary precedent exists in the codebase.
- **Numeric precision.** Floating-point round-trip via CSV is lossy at the limits of float64 representation. Acceptable for MVP; document.
- **UTF-8 BOM.** Some tools (older Excel) expect a BOM at the file start. Go's `encoding/csv` doesn't emit one. Skip BOM in MVP; revisit if a user complains.

---
*This document follows the https://specscore.md/feature-specification*
