# Feature: Delete Collection Command

**Status:** Draft

## Summary

The `ingitdb delete collection --collection=ID` command removes a collection definition and every record file that belongs to it. Unlike `truncate`, this command leaves no trace of the collection in `.ingitdb.yaml`.

## Problem

Retiring a collection is a destructive operation that touches the definition, the data directory, and any generated artifacts (READMEs, materialized views). A dedicated command performs all of these consistently in one pass.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb delete collection`. The `--collection=ID` flag is required.

### Semantics

#### REQ: removes-definition-and-records

The command MUST remove the collection's definition from `.ingitdb.yaml` AND every record file that belongs to it. It SHOULD also remove any generated artifacts (e.g. the collection's `README.md`) that would otherwise become orphaned.

#### REQ: path-flag

The `--path=PATH` flag MUST select the database directory; when omitted the current working directory is used.

## Dependencies

- path-targeting

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should the command refuse to delete a collection that other collections or views reference?
- Should `delete collection` support `--github`?

---
*This document follows the https://specscore.md/feature-specification*
