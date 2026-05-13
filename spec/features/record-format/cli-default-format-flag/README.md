# Feature: `ingitdb setup --default-format` Flag

**Status:** Approved
**Source Idea:** [`default-record-format`](../../../ideas/default-record-format.md)
**Parent Feature:** [`record-format`](../README.md)

## Summary

Add a `--default-format=FORMAT` flag to the `ingitdb setup` command. The flag accepts any of the seven supported record formats (`yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`, `csv`); its value is written to `.ingitdb/settings.yaml` under the new `default_record_format` field (defined by the sibling [`project-default`](../project-default/README.md) Feature). An unsupported value causes the command to exit non-zero with a clear error listing valid options.

A collection-level `--format` flag is **out of scope** for this Feature batch (naming convention reserved for a future `ingitdb create-collection` command).

## Problem

Users running `ingitdb setup` today have no way to express "I want all new collections in this project to default to format X" — the project-level setting only exists if hand-edited into `.ingitdb/settings.yaml` after the fact. A `--default-format` flag lets the user lock in the project's preferred format at initialization time.

## Behavior

### REQ: default-format-flag

The `ingitdb setup` command MUST accept a `--default-format=FORMAT` flag. The flag is optional. When provided, the flag's value is written to `.ingitdb/settings.yaml` under the `default_record_format` field. When omitted, the field is not written (so the project relies on the hard YAML fallback per the `project-default` Feature's resolution chain).

#### AC-1: flag-writes-settings-yaml

**Given** an empty directory
**When** the user runs `ingitdb setup --default-format=ingr` in that directory
**Then** the resulting `.ingitdb/settings.yaml` file contains a line `default_record_format: ingr` (or an equivalent valid YAML representation that round-trips to the same value).

#### AC-2: flag-omitted-leaves-field-unset

**Given** an empty directory
**When** the user runs `ingitdb setup` (no `--default-format` argument)
**Then** the resulting `.ingitdb/settings.yaml` either omits the `default_record_format` key entirely OR sets it to an empty string. Either form must round-trip to `ResolveRecordFormat` returning `RecordFormatYAML` (per the `project-default` Feature's hard-fallback contract).

### REQ: flag-validation

The `--default-format` flag value MUST be validated against the seven supported format strings (`yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`, `csv`). An unsupported value causes the command to exit non-zero **without** writing any files, and to emit a clear error message that:

1. Names the offending value.
2. Lists the seven valid options (or a clear reference to where the valid options are documented).

#### AC-1: unsupported-value-rejected

**Given** an empty directory
**When** the user runs `ingitdb setup --default-format=xml`
**Then** the command exits with non-zero status, no `.ingitdb/` directory is created, and stderr contains both the substring `xml` and all seven valid format names (`yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`, `csv`).

#### AC-2: case-sensitivity

**Given** an empty directory
**When** the user runs `ingitdb setup --default-format=YAML` (uppercase)
**Then** the command either accepts the value (canonicalizing to lowercase `yaml` on write) OR rejects it with a clear case-sensitivity error. The plan decides which; this AC documents that the behavior must be one or the other deterministically — silent acceptance with lowercase storage is fine, but the contract MUST NOT be "sometimes accept, sometimes reject."

#### AC-3: empty-value-rejected

**Given** an empty directory
**When** the user runs `ingitdb setup --default-format=` (empty value)
**Then** the command exits with non-zero status and emits an error indicating `--default-format` requires a non-empty value.

### REQ: flag-value-survives-existing-content

When `ingitdb setup` is run against a directory that already has a partial `.ingitdb/settings.yaml` (e.g. `default_namespace: todo` exists but `default_record_format` does not), the `--default-format` flag's value MUST be added without overwriting the existing fields.

Note: the broader question of what `ingitdb setup` does against an already-initialized directory is governed by the existing [`setup` Feature](../../cli/setup/README.md) REQ:idempotent-on-empty-target. This Feature does NOT change that behavior; it only specifies that, if `setup` does proceed to write, the `--default-format` value is preserved alongside any existing fields.

#### AC-1: preserves-existing-fields

**Given** a directory with `.ingitdb/settings.yaml` containing `default_namespace: todo` and no `default_record_format`
**When** `ingitdb setup` is run with `--default-format=ingr` AND the existing `setup` command's idempotent-on-empty-target REQ permits the write (e.g. via a future `--force` flag or because `setup` writes a fresh file alongside merging)
**Then** the resulting `.ingitdb/settings.yaml` contains BOTH `default_namespace: todo` AND `default_record_format: ingr` — neither field is lost.

## Architecture

| File | Change |
|---|---|
| `cmd/ingitdb/commands/setup.go` (or wherever the `setup` command is registered) | Add `--default-format` flag definition; validate the value; pass it through to the settings-writing logic. |
| `cmd/ingitdb/commands/setup_test.go` (or equivalent) | New tests covering all ACs above. |

The `cli-default-format-flag` Feature depends on the `project-default` Feature having shipped the `DefaultRecordFormat` field on `config.Settings` — the flag's value is written via the same path that `project-default` defined. Implement `project-default` first.

## Testing Strategy

Integration-style tests against a temp directory: run the `setup` command's underlying function with various `--default-format` values, assert the resulting `.ingitdb/settings.yaml` contents.

## Rehearse Integration

ACs are testable via `go test ./cmd/ingitdb/commands/...`. No external scaffolding needed.

## Outstanding Questions

- **Existing `cli/setup` Feature spec mentions `.ingitdb.yaml`** (a single file) while the actual codebase uses `.ingitdb/settings.yaml` (directory + file). This Feature's contract assumes the codebase reality. The `cli/setup` Feature spec needs reconciliation in a separate task. Flag at plan time.
- **Case sensitivity of the flag value.** AC-2 in REQ:flag-validation captures the contract that the behavior MUST be deterministic; the plan picks "accept and lowercase" vs "reject" based on usability preference.
- **Conflict with future `--format` flag.** When `ingitdb create-collection --format=...` is eventually added (out of this Feature's scope), the naming will naturally distinguish project-level (`--default-format`) from collection-level (`--format`). Document this naming convention in the `setup` command's godoc.

---
*This document follows the https://specscore.md/feature-specification*
