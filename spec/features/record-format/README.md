# Feature: Record Format Extensions

**Status:** Implemented
**Source Idea:** [`default-record-format`](../../ideas/default-record-format.md)

## Summary

Three small, additive extensions to inGitDB's existing record-format machinery: (1) **CSV** as a seventh first-class format alongside the existing six (`yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`); (2) a **project-level `default_record_format` config field** in `.ingitdb/settings.yaml` with a centralized fallback resolver (collection → project → hard YAML default); (3) a **`--default-format` CLI flag** on `ingitdb setup`. The collection-level `--format` flag is reserved for a future `ingitdb create-collection` command and explicitly deferred from this Feature batch.

This umbrella does NOT ship the existing six formats — those already exist in code (`pkg/ingitdb/constants.go`, `pkg/dalgo2ingitdb/parse.go`, `pkg/dalgo2ghingitdb/tx_readwrite.go`). The three child Features are surgical additions.

## Problem

inGitDB today supports six record formats but the choice is **only** per-collection. There is no project-level default, no `--format` flag on creation commands, and no CSV support. The DALgo schema-modification work (now Implemented in `dal-go/dalgo`) and the cross-engine `db copy` consumer in `datatug-cli` will start writing inGitDB content programmatically; both need a defensible answer to "what format do I write when the user hasn't told me." Today the answer is a hard-coded fallback in code; this Feature makes the fallback a configurable project setting with a clear resolution order.

## Children

Listed in **dependency order** (implement bottom-up):

| Feature | Summary |
|---|---|
| [csv-support/](csv-support/README.md) | New `RecordFormatCSV` constant + schema-aware read/write paths in `pkg/dalgo2ingitdb/parse.go` (extending the existing `ParseRecordContentForCollection` pattern that markdown already uses) and `pkg/dalgo2ghingitdb/tx_readwrite.go`. Header row in schema-defined column order. Restricted to `RecordType: ListOfRecords` (stricter than INGR — INGR currently allows `MapOfRecords` too). Writer fails cleanly on nested/array-valued fields. |
| [project-default/](project-default/README.md) | New `DefaultRecordFormat` field on `config.Settings` (YAML tag `default_record_format,omitempty`) loaded from `.ingitdb/settings.yaml`. Centralized fallback resolver: per-collection `record_file.format` → project-level `default_record_format` → hard fallback (`yaml`). Validation rejects unrecognized values at load time. |
| [cli-default-format-flag/](cli-default-format-flag/README.md) | New `--default-format=FORMAT` flag on `ingitdb setup`. Populates `.ingitdb/settings.yaml#default_record_format`. Validates against the seven supported formats; exits non-zero on unsupported value. |

## Non-Goals

Inherited from the source Idea, reinforced at Feature-spec time:

- **Adding a collection-level `--format` flag** — naming convention reserved (`--format` on a future `ingitdb create-collection` command) but explicitly out of scope for this Feature batch. No `create-collection` command exists today.
- **Adding an `ingitdb create-collection` command** — deferred to a separate future Idea.
- **Re-specifying the six existing formats** — `yaml`, `yml`, `json`, `markdown`, `toml`, `ingr` are already implemented and working. Documenting them in spec form is a useful follow-up but out of scope here.
- **Migrating existing collections from one format to another** — out of scope; conversion is a separate one-shot command if users want it.
- **Removing or deprecating any of the seven formats** — all seven (six existing + CSV) remain first-class.
- **Defining the INGR, TOML, or CSV formats themselves** — external specifications; this Feature consumes them via existing dependencies (`github.com/ingr-io/ingr-go`, `github.com/pelletier/go-toml/v2`, Go stdlib `encoding/csv`).
- **Per-record format overrides via CLI** — readers already tolerate mixed-format projects; no new CLI surface to author that way.
- **Subdirectory-level format defaults** — covered by per-collection settings; subdirectory granularity is not added.
- **Promoting any non-YAML format as the hard fallback** — YAML stays as the hard default when no project setting exists.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  ingitdb setup --default-format=<f>  (cli-default-format-flag)│
│        │                                                    │
│        ▼ writes                                             │
│  .ingitdb/settings.yaml                                     │
│     default_record_format: <f>      (project-default field) │
│                                                             │
│  At read/write time, the centralized resolver picks:        │
│     1. collection's .collection/definition.yaml             │
│        #record_file.format                  (existing)      │
│     2. .ingitdb/settings.yaml                               │
│        #default_record_format     (project-default field)   │
│     3. yaml          (hard fallback per project-default)    │
│                                                             │
│  The resolved format value drives reader/writer dispatch    │
│  in pkg/dalgo2ingitdb/parse.go (already format-aware for    │
│  the 6 existing formats; csv-support adds the 7th case).    │
└─────────────────────────────────────────────────────────────┘
```

## Files Touched (Across Children)

| File | Touched by |
|---|---|
| `pkg/ingitdb/constants.go` | `csv-support` (add `RecordFormatCSV` constant) |
| `pkg/ingitdb/record_file_def.go` | `csv-support` (CSV `RecordType` validation, analogous to existing INGR validation at lines 68–70) |
| `pkg/dalgo2ingitdb/parse.go` | `csv-support` (CSV read/write cases in `ParseRecordContent` and `marshalForFormat`) |
| `pkg/dalgo2ghingitdb/tx_readwrite.go` | `csv-support` (CSV case in `encodeRecordContent`) |
| `pkg/ingitdb/config/root_config.go` | `project-default` (extend `config.Settings` struct + add fallback resolver) |
| `cmd/ingitdb/commands/setup.go` (or wherever the setup command lives) | `cli-default-format-flag` (add `--default-format` flag + validation + write to `.ingitdb/settings.yaml`) |

## Testing Strategy

Each child Feature scopes its own Go tests against the specific files it touches. Cross-Feature behavior (the fallback resolver consuming the `csv-support`-added `RecordFormatCSV`) is exercised at integration level by a round-trip test in `project-default`.

## Rehearse Integration

All ACs are testable via `go test ./...` + the existing `ingitdb` CLI test suite. No external scaffolding needed.

## Assumption Carryover

From the source Idea:

| Idea assumption | Status |
|---|---|
| Must-be-true: Adding `RecordFormat` field with `omitempty` to `config.Settings` doesn't break existing projects | Carried; validated by `project-default` Feature ACs. |
| Must-be-true: Centralized fallback resolver is implementable without order-of-operations bugs | Carried; validated by `project-default` REQ:fallback-resolution. |
| Must-be-true: Go stdlib `encoding/csv` is sufficient for CSV needs | Carried; validated by `csv-support` round-trip AC. |
| Must-be-true: CSV's `RecordType: ListOfRecords` constraint validates at load + write time | Carried; encoded in `csv-support` REQ:csv-record-type-restriction. |
| Must-be-true: CSV writer cleanly refuses nested/array-valued fields | Carried; encoded in `csv-support` REQ:csv-nested-field-error. |
| Should-be-true: CSV header row in schema order is useful for Excel/Pandas users | Carried; encoded in `csv-support` REQ:csv-write-header-row. |

## Outstanding Questions

- **`ingitdb setup` Feature is in Draft** with the current setup spec mentioning `.ingitdb.yaml` (single file). The actual codebase uses `.ingitdb/settings.yaml` (directory + file). The `cli-default-format-flag` Feature MUST write to the codebase-real path. Recommend reconciling the `cli/setup` Feature spec at the same time but out of scope here.
- **`yml` vs `yaml`** — these are aliases today (both map to YAML parsing). When `default_record_format: yml` is written to `.ingitdb/settings.yaml`, does the validator accept it or canonicalize to `yaml`? Resolved in `project-default` Feature.

---
*This document follows the https://specscore.md/feature-specification*
