# Feature: Version Command

**Status:** Implementing

## Summary

The `ingitdb version` command prints the build version, the commit hash from which the binary was built, and the build date. It is the simplest CLI command and serves as a smoke test that the binary is installed and runnable.

## Problem

Users, package maintainers, and CI pipelines need a deterministic way to identify which build of `ingitdb` is on the `PATH`. Without a `version` command, debugging a problem reported against "the CLI" requires guessing or reading shell history.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb version`. It takes no arguments and no flags.

### Output

#### REQ: prints-build-info

The command MUST print the build version, the commit hash, and the build date. The exact formatting is implementation-defined but MUST include all three fields and MUST be human-readable on a terminal.

#### REQ: stdout-only

Build information MUST be written so that it can be captured by piping the command's output. The command MUST NOT require a TTY.

### Exit code

#### REQ: exit-zero

The command MUST exit with status `0` whenever the binary runs to completion. There is no error path beyond crashes in the runtime itself.

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/version`):

- [`cmd/ingitdb/commands/version.go`](../../../cmd/ingitdb/commands/version.go)

## Acceptance Criteria

### AC: version-prints-three-fields

**Requirements:** cli/version#req:subcommand-name, cli/version#req:prints-build-info

Running `ingitdb version` prints a single output containing the build version string, the commit hash, and the build date. A reader can identify each of the three fields without consulting external documentation.

### AC: version-exits-zero

**Requirements:** cli/version#req:exit-zero

Running `ingitdb version` always exits with status `0` on a working install. CI scripts can use the command as a smoke test by checking only the exit code.

## Outstanding Questions

- Should `version` accept a `--format=json` flag for machine-readable output?
- Should the command include the Go runtime version and OS/arch (as `go version` does)?

---
*This document follows the https://specscore.md/feature-specification*
