# Feature: Project-Level Default Record Format

**Status:** Implemented
**Source Idea:** [`default-record-format`](../../../ideas/default-record-format.md)
**Parent Feature:** [`record-format`](../README.md)

## Summary

Add a project-level `default_record_format` field to inGitDB's existing `.ingitdb/settings.yaml` config. Centralize format resolution in a single helper so all read/write call sites consult the same fallback chain: per-collection `record_file.format` → project-level `default_record_format` → hard fallback (`yaml`). Loading rejects an unrecognized value at load time with a clear error.

The field is `omitempty`-tagged so existing projects without it keep working unchanged. The hard fallback remains `yaml` (matches the historical inGitDB convention and broadest editor/CI ecosystem familiarity).

## Problem

Today the only place a format can be set is `.collection/definition.yaml#record_file.format`, per collection. There is no project-wide default, so consumers writing inGitDB content programmatically (e.g. `datatug-cli`'s `db copy`, `dalgo2ingitdb`'s schema-file writer) have no project-driven answer to "what format do I use when the user hasn't set one for this collection." Today the answer is a hard-coded fallback in code; this Feature makes it a configurable project setting with a clear resolution order.

## Behavior

### REQ: default-record-format-field

The `config.Settings` struct in `pkg/ingitdb/config/root_config.go` MUST gain a new field:

```go
DefaultRecordFormat ingitdb.RecordFormat `yaml:"default_record_format,omitempty"`
```

The field type reuses the existing `ingitdb.RecordFormat` string-alias type. The YAML tag uses `omitempty` so existing `.ingitdb/settings.yaml` files without the field continue to load unchanged.

#### AC-1: field-exists

**Given** a Go program that imports `github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config`
**When** the program declares `var s config.Settings; _ = s.DefaultRecordFormat`
**Then** the program compiles and `s.DefaultRecordFormat` has type `ingitdb.RecordFormat`.

#### AC-2: omitempty-on-existing-files

**Given** an existing `.ingitdb/settings.yaml` whose content is `default_namespace: todo` (no `default_record_format` line)
**When** the file is loaded into `config.Settings` via the existing YAML unmarshal path
**Then** the load succeeds with no error, `s.DefaultNamespace == "todo"`, and `s.DefaultRecordFormat` is the zero value (empty `RecordFormat("")`).

#### AC-3: field-loads-from-yaml

**Given** a `.ingitdb/settings.yaml` with content `default_record_format: ingr`
**When** the file is loaded into `config.Settings`
**Then** `s.DefaultRecordFormat == ingitdb.RecordFormatINGR`.

### REQ: invalid-format-rejected

When `.ingitdb/settings.yaml` contains a `default_record_format` value that is not one of the seven supported formats (`yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`, `csv`), loading MUST fail with a typed error whose message identifies the offending value AND lists all valid options.

The validation lives in `config.Settings.Validate` (or `config.RootConfig.Validate` if the package already has one there). It runs after YAML unmarshal, so the error surfaces at load time rather than at first read.

#### AC-1: unsupported-value-rejected

**Given** a `.ingitdb/settings.yaml` with content `default_record_format: xml`
**When** the file is loaded and `Validate()` is called
**Then** the result is a non-nil error whose message contains `xml` AND lists all seven valid format values (`yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`, `csv`).

#### AC-2: empty-value-accepted

**Given** a `.ingitdb/settings.yaml` with content `default_record_format: ""`
**When** the file is loaded
**Then** the result is no error (an empty string indicates "no project default; use hard fallback").

#### AC-3: each-of-seven-accepted

**Given** seven separate `.ingitdb/settings.yaml` files, each with `default_record_format` set to one of `yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`, `csv`
**When** each is loaded and validated
**Then** all seven validations return `nil`.

### REQ: fallback-resolution

The package MUST export a single helper function (or method) that resolves the effective record format for a given collection, applying the fallback chain in this exact order:

1. If `collection != nil && collection.RecordFile != nil && collection.RecordFile.Format != ""`, return `collection.RecordFile.Format`.
2. Otherwise, if `settings != nil && settings.DefaultRecordFormat != ""`, return `settings.DefaultRecordFormat`.
3. Otherwise, return the hard fallback `ingitdb.RecordFormatYAML`.

Note: `CollectionDef.RecordFile` is `*RecordFileDef` (a nullable pointer) in the current codebase — a `nil` `RecordFile` is a legitimate runtime state that the helper MUST handle without panicking.

The helper signature is:

```go
// ResolveRecordFormat returns the effective RecordFormat for a collection,
// applying the fallback chain:
//
//   collection.RecordFile.Format → settings.DefaultRecordFormat → ingitdb.RecordFormatYAML.
//
// The helper tolerates a nil collection, a collection with a nil RecordFile,
// an empty Format string, and a nil settings — each is treated as "no
// per-tier setting" and the helper falls through to the next tier.
func ResolveRecordFormat(collection *ingitdb.CollectionDef, settings *Settings) ingitdb.RecordFormat
```

All read/write call sites that today consult `RecordFileDef.Format` directly MUST be updated to consult `ResolveRecordFormat` instead. The audit of those sites is part of plan-time work, not this spec.

#### AC-1: collection-setting-wins

**Given** a `CollectionDef` with `RecordFile.Format = RecordFormatJSON` AND a `Settings` with `DefaultRecordFormat = RecordFormatINGR`
**When** `ResolveRecordFormat(collection, settings)` is called
**Then** the result is `RecordFormatJSON`.

#### AC-2: project-default-when-collection-unset

**Given** a `CollectionDef` with empty `RecordFile.Format` AND a `Settings` with `DefaultRecordFormat = RecordFormatINGR`
**When** `ResolveRecordFormat(collection, settings)` is called
**Then** the result is `RecordFormatINGR`.

#### AC-3: hard-fallback-when-both-unset

**Given** a `CollectionDef` with empty `RecordFile.Format` AND a `Settings` with empty `DefaultRecordFormat`
**When** `ResolveRecordFormat(collection, settings)` is called
**Then** the result is `RecordFormatYAML`.

#### AC-4: nil-collection-uses-project-default

**Given** a `nil` `*CollectionDef` AND a `Settings` with `DefaultRecordFormat = RecordFormatCSV`
**When** `ResolveRecordFormat(nil, settings)` is called
**Then** the result is `RecordFormatCSV`.

#### AC-5: nil-settings-uses-hard-fallback

**Given** a `nil` `*CollectionDef` AND `nil` `*Settings`
**When** `ResolveRecordFormat(nil, nil)` is called
**Then** the result is `RecordFormatYAML`.

#### AC-6: nil-recordfile-falls-through

**Given** a non-nil `*CollectionDef` whose `RecordFile` field is `nil` AND a `Settings` with `DefaultRecordFormat = RecordFormatINGR`
**When** `ResolveRecordFormat(collection, settings)` is called
**Then** the result is `RecordFormatINGR` (the helper falls through to the project default without panicking on the nil pointer dereference).

#### AC-7: empty-format-string-falls-through

**Given** a `CollectionDef` with `RecordFile = &RecordFileDef{Format: ""}` (non-nil RecordFile but empty Format) AND a `Settings` with `DefaultRecordFormat = RecordFormatJSON`
**When** `ResolveRecordFormat(collection, settings)` is called
**Then** the result is `RecordFormatJSON`.

## Architecture

| File | Change |
|---|---|
| `pkg/ingitdb/config/root_config.go` | Extend `config.Settings` with `DefaultRecordFormat` field. Add `Settings.Validate` (or extend existing if present) with the unrecognized-value check. Add `ResolveRecordFormat(collection, settings)` helper function. |
| `pkg/ingitdb/config/root_config_test.go` | New tests for the AC contract above. |

## Testing Strategy

In-package Go tests using YAML-encoded `.ingitdb/settings.yaml` content fixtures. Validation tests cover the rejection paths; resolution tests cover the three-level fallback chain.

## Rehearse Integration

All ACs are testable via `go test ./pkg/ingitdb/config/...`. No external scaffolding needed.

## Outstanding Questions

- **Audit of existing call sites.** Plan-time work: identify every place in the codebase that currently consults `RecordFileDef.Format` directly and update each to use `ResolveRecordFormat`. The audit is plan-scope, not spec-scope, but the count matters for plan sizing.
- ~~`yml` canonicalization.~~ **Resolved:** preserve `yml` as-written. The existing reader at `pkg/dalgo2ingitdb/parse.go:23` already accepts both `RecordFormatYAML` and `RecordFormatYML` in the same switch case, so downstream code already handles both. Canonicalizing at load would force a write-time edit to user-supplied config — a worse trade than the trivial dual-accept that already exists.
- **Where does `ResolveRecordFormat` live?** Top-level in `config`, or attached to `RootConfig`/`Settings` as a method? Pick at plan time.

---
*This document follows the https://specscore.md/feature-specification*
