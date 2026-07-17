---
format: https://specscore.md/feature-specification
status: Implementing
---

# Feature: List Views Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/list-views?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/list-views?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/list-views?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/list-views?op=request-change) |
**Status:** Implementing
**Source Ideas:** —

## Summary

The `ingitdb list views` command prints every view defined in a database, one per line, as a qualified `collectionID/viewName` identifier. It works against a local directory (`--path`) and supports two narrowing flags: `--in` (regex on the owning collection's starting-point path) and `--filter-name` (glob on the bare view name).

## Problem

A collection can declare any number of views (materializable projections such as `status_{status}`). Before this command, `ingitdb list views` returned `not yet implemented`, so the only way to discover which views exist — a precondition for `materialize --views=<name>` — was to grep the `.collection/views/` directories by hand. Listing views is the natural companion to `list collections`.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb list views`. All flags are optional. When no `--path` is given the current working directory is used.

### Enumeration

#### REQ: qualified-view-ids

The command MUST enumerate views across every collection, recursing into subcollections, and MUST print each as a `collectionID/viewName` identifier so that identically-named views on different collections remain distinct. Output MUST be sorted ascending and deterministic given a fixed database state and flag set.

### Flags

#### REQ: in-flag

The `--in=REGEXP` flag MUST scope the listing to views whose owning collection's starting-point path matches the regular expression.

#### REQ: filter-name-flag

The `--filter-name=PATTERN` flag MUST filter views by their bare (unqualified) name using a glob-style pattern (e.g. `status_*`). `--in` and `--filter-name` combine with AND.

## Dependencies

- path-targeting

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/list-collections`, which registers the shared
`cmd/ingitdb/commands/list.go` file that hosts both `list` subcommands):

- [`cmd/ingitdb/commands/list.go`](../../../cmd/ingitdb/commands/list.go)

## Acceptance Criteria

### AC: lists-all-views

**Requirements:** cli/list-views#req:subcommand-name, cli/list-views#req:qualified-view-ids

Given a database whose collections (including subcollections) declare N views in total, when `ingitdb list views` runs, then it prints exactly N `collectionID/viewName` lines to stdout in ascending order and exits `0`.

### AC: scoped-listing

**Requirements:** cli/list-views#req:in-flag, cli/list-views#req:filter-name-flag

Given views spread across several collections, when `ingitdb list views --in='/todo/tasks$' --filter-name='by_*'` runs, then only views whose owning collection path matches the regex AND whose bare name matches the glob are printed.

## Open Questions

- Should `list views` gain a `--remote` source like `list collections`, reading view definitions over the GitHub file reader?

---
*This document follows the https://specscore.md/feature-specification*
