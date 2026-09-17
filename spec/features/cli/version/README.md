---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Version Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/version?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/version?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/version?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/version?op=request-change) |
**Status:** Stable
**Source Ideas:** —

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

### Machine-readable output

#### REQ: json-flag

The command MUST accept a `--json` flag that prints exactly one JSON object
to stdout and nothing else, exiting `0`, per the fleet-wide `version --json`
contract (cli-install#req:version-json-contract in `strongo/cli-helpers`):
string keys `name` (the binary name, `ingitdb`), `version` (bare, no leading
`v`; `dev` when undetermined), `commit` (full SHA, `+dirty` suffix when the
build tree was modified; `""` when unknown), `date` (RFC 3339; `""` when
unknown), and `date_source` (`build` when link-time stamped, `commit` when
read from `vcs.time`, `""` when unknown). `--json` is the fleet-wide probe
flag independent of any other output-format flag; it MUST NOT perform
network I/O, write files, or emit telemetry, so probing an installed
`ingitdb` is safe to repeat (cli-install#req:version-json-side-effect-free).
The writer and any reader share one exported Go type,
`github.com/strongo/buildinfo.VersionJSON`.

### Exit code

#### REQ: exit-zero

The command MUST exit with status `0` whenever the binary runs to completion. There is no error path beyond crashes in the runtime itself.

## Implementation

`ingitdb` wires this command through
`github.com/strongo/buildinfo/fangcmd.Wire` (`cmd/ingitdb/main.go`), which
adds the `version` subcommand — including `--json` — from
`github.com/strongo/buildinfo/cobracmd.VersionCommand`, the same
implementation point every fleet CLI wired through `buildinfo` shares
(annotated `// specscore: feature/cli/version` is not applicable here: the
command body itself lives upstream in `strongo/buildinfo`, not in this
repository).

## Acceptance Criteria

### AC: version-prints-three-fields

**Requirements:** cli/version#req:subcommand-name, cli/version#req:prints-build-info

Running `ingitdb version` prints a single output containing the build version string, the commit hash, and the build date. A reader can identify each of the three fields without consulting external documentation.

### AC: version-json-contract

**Requirements:** cli/version#req:json-flag

**Given** an `ingitdb` build with release ldflags and, separately, a plain `go build`
**When** `ingitdb version --json` runs with no network access
**Then** stdout is exactly one JSON object whose `name` is `"ingitdb"`, `date_source` is `"build"` for the stamped build and `"commit"` or `""` for the plain build, and no network connection, telemetry event, or file write occurs.

### AC: version-exits-zero

**Requirements:** cli/version#req:exit-zero

Running `ingitdb version` always exits with status `0` on a working install. CI scripts can use the command as a smoke test by checking only the exit code.

## Open Questions

- Should the command include the Go runtime version and OS/arch (as `go version` does)?

---
*This document follows the https://specscore.md/feature-specification*
