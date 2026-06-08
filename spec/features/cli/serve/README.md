---
format: https://specscore.md/feature-specification
status: Withdrawn — the `serve` command and its HTTP API / MCP gateway were removed as a still-born, datatug-overlapping surface (see [docs/adr/0001-remove-serve-command.md](../../../../docs/adr/0001-remove-serve-command.md)). This document is preserved as a historical record of the original design.
---

# Feature: Serve Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/serve?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/serve?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/serve?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/serve?op=request-change) |
**Status:** Withdrawn — the `serve` command and its HTTP API / MCP gateway were removed as a still-born, datatug-overlapping surface (see [docs/adr/0001-remove-serve-command.md](../../../../docs/adr/0001-remove-serve-command.md)). This document is preserved as a historical record of the original design.
**Source Ideas:** —

> **Withdrawn & removed.** The `serve` command and its HTTP API / MCP gateway
> implementation were removed as a still-born, datatug-overlapping surface. For
> programmatic access, import the `pkg/dalgo2ghingitdb` driver directly; cross-source
> serving belongs to [DataTug](https://github.com/datatug/datatug-cli) (which already
> consumes inGitDB as a backend). See
> [`docs/adr/0001-remove-serve-command.md`](../../../../docs/adr/0001-remove-serve-command.md).
> The implementation is preserved in git history (last present at commit `184a40e`).
> This document is kept as a record of the original design.

## Summary

The `ingitdb serve` command starts one or more long-running services in a single process: the MCP (Model Context Protocol) server, the HTTP API server, and the file watcher. Services are enabled à la carte via `--mcp`, `--http`, and `--watcher`; at least one MUST be set.

## Problem

Different consumers of inGitDB need different protocols: AI agents speak MCP, classic clients want HTTP, and local tooling reacts to the file watcher. Running each in its own process multiplies operational complexity. `ingitdb serve` lets a single binary host any combination of services on demand.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb serve`. At least one of `--mcp`, `--http`, or `--watcher` MUST be provided; the command MUST fail when none are set.

### Flags

#### REQ: service-flags

`--mcp` MUST enable the MCP server. `--http` MUST enable the HTTP API server. `--watcher` MUST enable the file watcher. Multiple flags MAY be combined.

#### REQ: path-flag

The `--path=PATH` flag MUST select the database directory; when omitted the current working directory is used.

### Lifecycle

#### REQ: foreground-process

The command MUST run in the foreground and host every requested service inside the same process until interrupted. It MUST NOT silently drop a service or restart itself.

## Dependencies

- path-targeting

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/serve`):

- [`cmd/ingitdb/commands/serve.go`](../../../cmd/ingitdb/commands/serve.go)
- [`cmd/ingitdb/commands/serve_http.go`](../../../cmd/ingitdb/commands/serve_http.go)
- [`cmd/ingitdb/commands/serve_mcp.go`](../../../cmd/ingitdb/commands/serve_mcp.go)

## Acceptance Criteria

Not defined yet.

## Open Questions

- Acceptance criteria not yet defined for this feature.
- Should the HTTP and MCP servers expose the same operations behind two protocol facades, or have distinct surfaces?
- Should `serve` support a config file in addition to flags?

---
*This document follows the https://specscore.md/feature-specification*
