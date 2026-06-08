---
format: https://specscore.md/feature-specification
status: Withdrawn — superseded by `delete --from=COLLECTION --all`. A standalone `truncate` verb would duplicate `delete`'s set-mode semantics for no benefit, so the SQL `delete --all` path covers this use case. This document is preserved as a historical record of the original design.
---

# Feature: Truncate Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/truncate?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/truncate?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/truncate?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/truncate?op=request-change) |
**Status:** Withdrawn — superseded by `delete --from=COLLECTION --all`. A standalone `truncate` verb would duplicate `delete`'s set-mode semantics for no benefit, so the SQL `delete --all` path covers this use case. This document is preserved as a historical record of the original design.
**Source Ideas:** —

## Summary

The `ingitdb truncate --collection=ID` command removes every record from a collection while leaving the collection definition in `.ingitdb.yaml` intact. After truncation the collection still exists; it just has zero records.

## Problem

Resetting a collection's data without forgetting its schema is a recurring operation in tests, demos, and seed-data refreshes. Without a truncate command, users must hand-delete record files and risk also deleting the collection definition or missing nested files.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb truncate`. The `--collection=ID` flag is required.

### Semantics

#### REQ: removes-records-only

The command MUST delete every record file or map entry that belongs to the named collection. It MUST NOT remove the collection's definition entry from `.ingitdb.yaml` and MUST NOT touch other collections.

#### REQ: path-flag

The `--path=PATH` flag MUST select the database directory; when omitted the current working directory is used.

## Dependencies

- path-targeting

## Acceptance Criteria

Not defined yet.

## Open Questions

- Acceptance criteria not yet defined for this feature.
- Should `truncate` require an interactive confirmation, or rely on the user's git history for safety?
- Should `--remote` be supported for remote truncation given that it could produce a single very large commit?

---
*This document follows the https://specscore.md/feature-specification*
