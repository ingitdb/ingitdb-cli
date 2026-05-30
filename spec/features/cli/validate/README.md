# Feature: Validate Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/validate?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/validate?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/validate?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/validate?op=request-change) |
**Status:** Implementing

## Summary

The `ingitdb validate` command reads `.ingitdb.yaml` and verifies that the collection definitions and every record file conform to the declared schema. It supports partial validation (`--only=definition` or `--only=records`) and a fast CI mode that limits the check to files changed within a git commit range.

## Problem

inGitDB stores data as plain YAML or JSON files in a git repository. Without a dedicated validator, schema drift is only discovered when a downstream consumer breaks at runtime. `ingitdb validate` makes correctness a first-class, pre-commit / pre-merge concern with a clear non-zero exit code on any violation.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb validate`. All flags are optional.

### Flags

#### REQ: path-flag

The `--path=PATH` flag MUST set the database directory to validate. When omitted, the command MUST default to the current working directory.

#### REQ: only-flag

The `--only=VALUE` flag MUST accept the values `definition` and `records`. When set to `definition` only the collection definitions in `.ingitdb.yaml` are validated; when set to `records` only the record files are validated. When the flag is omitted both are validated in a single pass.

#### REQ: commit-range

The `--from-commit=SHA` and `--to-commit=SHA` flags MUST scope record validation to files changed between the two commits. They MAY be combined with `--only=records`. When neither flag is provided, all record files in the working tree are checked.

### Output and exit code

#### REQ: per-collection-summary

For each collection, the command MUST print a summary line reporting how many records were validated and how many were valid (e.g. "All 42 records are valid for collection: users" or "38 out of 42 records are valid for collection: users").

#### REQ: exit-code

The command MUST exit with status `0` when validation passes and a non-zero status when any validation error is detected.

#### REQ: record-file-parsing

Record validation MUST parse every candidate record file using the collection's declared `record_file.format` and `record_file.type` before counting it as valid. Unparsable files, including malformed Markdown frontmatter and invalid YAML, JSON, TOML, CSV, JSONL, or INGR content, MUST be reported as validation errors with the offending record path and a useful parse error.

## Dependencies

- path-targeting

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/validate`):

- [`cmd/ingitdb/commands/validate.go`](../../../cmd/ingitdb/commands/validate.go)
- [`pkg/ingitdb/validator/def_validator.go`](../../../pkg/ingitdb/validator/def_validator.go)
- [`pkg/ingitdb/validator/subscribers_validator.go`](../../../pkg/ingitdb/validator/subscribers_validator.go)

## Acceptance Criteria

### AC: full-validation-by-default

**Requirements:** cli/validate#req:subcommand-name, cli/validate#req:only-flag, cli/validate#req:per-collection-summary, cli/validate#req:exit-code

Running `ingitdb validate` with no flags from a directory containing a valid `.ingitdb.yaml` validates both definitions and records, prints a summary line per collection, and exits `0`. Introducing a single record that violates its schema causes the command to exit non-zero.

### AC: scoped-validation

**Requirements:** cli/validate#req:only-flag, cli/validate#req:commit-range

`ingitdb validate --only=records --from-commit=A --to-commit=B` checks only records that changed between commits `A` and `B` and skips definition validation. Records outside the commit range are not opened.

### AC: malformed-record-file

**Requirements:** cli/validate#req:only-flag, cli/validate#req:record-file-parsing, cli/validate#req:exit-code

Given a Markdown-backed collection, `ingitdb validate --only=records` MUST parse each `*.md` record file. If a record contains malformed YAML frontmatter, the command exits non-zero and the error output includes the record path plus the Markdown/YAML parse error.

## Open Questions

- Should `--only=definition` ignore record files entirely, or only skip schema enforcement on them?

---
*This document follows the https://specscore.md/feature-specification*
