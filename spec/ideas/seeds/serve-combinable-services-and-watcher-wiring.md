---
type: sidekick-seed
slug: serve-combinable-services-and-watcher-wiring
captured_at: 2026-06-03T16:54:53Z
captured_by: claude
captured_during: null
trigger: explicit
status: queued
synchestra_task: null
---
# Make `serve` run combinable services and wire --watcher

Gap found verifying cli/serve (2/4 REQs). REQ:service-flags says multiple of `--mcp`/`--http`/`--watcher` MAY be combined and REQ:foreground-process says all requested services run in one process, but `serve.go` uses an if/else that runs only the first match (mcp > http): `--mcp --http` together runs only MCP. The `--watcher` flag is registered but never read (`GetBool("watcher")` absent), so `--watcher` alone falls through to the "no server mode" error; `pkg/watcher` exists but is unreferenced by serve. Run requested services concurrently in one process, wire the watcher, and add tests for combinations + watcher. Spec Implementation list omits `pkg/watcher`. Blocks cli/serve → Stable.
