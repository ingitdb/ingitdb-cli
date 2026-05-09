# Feature: Validate Command

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

## Dependencies

- path-targeting

## Acceptance Criteria

### AC: full-validation-by-default

**Requirements:** validate-command#req:subcommand-name, validate-command#req:only-flag, validate-command#req:per-collection-summary, validate-command#req:exit-code

Running `ingitdb validate` with no flags from a directory containing a valid `.ingitdb.yaml` validates both definitions and records, prints a summary line per collection, and exits `0`. Introducing a single record that violates its schema causes the command to exit non-zero.

### AC: scoped-validation

**Requirements:** validate-command#req:only-flag, validate-command#req:commit-range

`ingitdb validate --only=records --from-commit=A --to-commit=B` checks only records that changed between commits `A` and `B` and skips definition validation. Records outside the commit range are not opened.

## Outstanding Questions

- Should `--only=definition` ignore record files entirely, or only skip schema enforcement on them?

---
*This document follows the https://specscore.md/feature-specification*
