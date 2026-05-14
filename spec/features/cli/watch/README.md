# Feature: Watch Command

**Status:** Draft

## Summary

The `ingitdb watch` command monitors a database directory for file-system changes and writes one structured event per record change (added, updated, deleted) to stdout. It runs in the foreground until interrupted and supports `--format=text|json` for human or pipe-friendly output.

## Problem

Tools that react to data changes — deployments, view rebuilds, AI agents, dashboards — need a stream they can subscribe to. Re-implementing file-system watching in every consumer is duplicative; a single CLI that emits canonical record events keeps every consumer consistent.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb watch`. All flags are optional.

### Flags

#### REQ: path-and-format

The `--path=PATH` flag MUST select the database directory (default: current directory). The `--format=FORMAT` flag MUST accept `text` (default, human-friendly) or `json` (one event per line, pipe-friendly).

### Events

#### REQ: event-types

For every record change the command MUST emit exactly one event whose `type` is one of `added`, `updated`, or `deleted`. `updated` events MUST include the changed field names (and, in JSON mode, their new values).

### Lifecycle

#### REQ: foreground-until-interrupted

The command MUST run in the foreground and continue emitting events until the process receives an interrupt (e.g. SIGINT). It MUST NOT daemonize itself.

## Dependencies

- path-targeting

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/watch`):

- [`cmd/ingitdb/commands/watch.go`](../../../cmd/ingitdb/commands/watch.go)

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should `watch` support filtering by collection or path prefix at the source?
- How should the command behave on rename or move events?

---
*This document follows the https://specscore.md/feature-specification*
