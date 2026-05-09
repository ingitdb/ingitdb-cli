# Feature: Query Command

**Status:** Draft

## Summary

The `ingitdb query -c=COLLECTION` command queries records from a single collection with optional filtering (`--where`), sorting (`--order-by`), field projection (`--fields`/`-f`), and pluggable output formats (`--format=csv|json|yaml|md`). CSV is the default.

## Problem

Reading records one at a time with `read record` does not scale to "give me every country with population > 1,000,000". A first-class query command makes ad-hoc data exploration feel like a SQL-lite over a git repository, without standing up a server.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb query`. The `--collection`/`-c` flag is required.

### Flags

#### REQ: fields-projection

The `--fields`/`-f` flag MUST accept `*` (all fields, default), `$id` (the record key), or a comma-separated list of field names.

#### REQ: where-filter

The `--where`/`-w` flag MUST accept comparison expressions of the form `field<op>value` where `<op>` is one of `>=`, `<=`, `>`, `<`, `==`, `=` (with `=` treated as `==`). The flag MUST be repeatable; multiple occurrences combine with logical AND. Numeric values containing commas (e.g. `1,000,000`) MUST be parsed by stripping the commas.

#### REQ: order-by

The `--order-by` flag MUST accept a comma-separated list of field names; a leading `-` reverses the sort for that field.

#### REQ: format-flag

The `--format` flag MUST accept `csv` (default, with header row), `json`, `yaml`, and `md` (Markdown table).

## Dependencies

- path-targeting

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should `--where` support `OR` and parentheses in addition to repeated AND clauses?
- Should the command support cross-collection joins, or remain single-collection only?

---
*This document follows the https://specscore.md/feature-specification*
