# Feature: Delete Records Command

**Status:** Draft

## Summary

The `ingitdb delete records --collection=ID` command bulk-deletes records that match a filter inside a single collection. Filtering is done via `--filter-name` (glob on record name) and optionally narrowed by `--in` (regex on sub-path).

## Problem

Cleaning up obsolete records — for example every record whose name matches `*old*` — is too tedious to do one record at a time and too risky to do with shell `rm`. A scoped, filter-driven bulk delete keeps the operation auditable inside a single git commit.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb delete records`. The `--collection=ID` flag is required.

### Flags

#### REQ: filter-flags

The `--filter-name=PATTERN` flag MUST accept a glob-style pattern matched against record names. The `--in=REGEXP` flag MUST further narrow the deletion to records whose sub-path matches the regex.

#### REQ: path-flag

The `--path=PATH` flag MUST select the database directory; when omitted the current working directory is used.

### Semantics

#### REQ: bounded-to-collection

The command MUST only delete records that belong to the named collection AND match the supplied filters. The collection definition MUST remain intact.

## Dependencies

- path-targeting

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should the command require at least one of `--filter-name` or `--in` to avoid degenerating into `truncate`?
- Should there be a `--dry-run` flag that lists matches without deleting?

---
*This document follows the https://specscore.md/feature-specification*
