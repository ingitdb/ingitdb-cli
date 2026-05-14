# Feature: Setup Command

**Status:** Draft

## Summary

The `ingitdb setup` command initialises a new inGitDB database directory by writing a starter `.ingitdb.yaml` and the expected directory layout. It accepts `--path=PATH` to target a directory other than the current one.

## Problem

The first-run experience for inGitDB requires a small but specific set of files and conventions. Asking new users to copy them by hand from documentation produces inconsistent layouts and friction on the very first command they run.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb setup`. All flags are optional.

### Flags

#### REQ: path-flag

The `--path=PATH` flag MUST select the directory to initialise; when omitted the current working directory is used.

### Behavior

#### REQ: writes-starter-config

The command MUST write a starter `.ingitdb.yaml` (with at least `rootCollections` and `languages` keys) and create any directories required by the layout. It MUST refuse to overwrite an existing `.ingitdb.yaml` unless explicitly instructed to do so.

#### REQ: idempotent-on-empty-target

When run against an already-initialised directory, the command MUST exit non-zero with a clear message rather than silently mutating the existing setup.

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/setup`):

- [`cmd/ingitdb/commands/setup.go`](../../../cmd/ingitdb/commands/setup.go)

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should `setup` support a `--template=NAME` flag to scaffold from canned starter projects?
- Should `setup` initialise a git repository if the target is not already one?

---
*This document follows the https://specscore.md/feature-specification*
