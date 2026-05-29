# Feature: Delete Records Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/delete-records?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/delete-records?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/delete-records?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/delete-records?op=request-change) |
**Status:** Superseded by [spec/features/cli/delete/](../delete/README.md). The `ingitdb delete records` command has been replaced by `ingitdb delete --from=... --where=...` (or `--all`). This document is preserved as a historical record.

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

## Open Questions

- Acceptance criteria not yet defined for this feature.
- Should the command require at least one of `--filter-name` or `--in` to avoid degenerating into `truncate`?
- Should there be a `--dry-run` flag that lists matches without deleting?

---
*This document follows the https://specscore.md/feature-specification*
