---
type: sidekick-seed
slug: setup-starter-config-and-overwrite-guard
captured_at: 2026-06-03T16:54:53Z
captured_by: claude
captured_during: null
trigger: explicit
status: queued
synchestra_task: null
---
# Make `setup` write a real starter config and refuse to overwrite an initialised db; reconcile the stale spec layout

Gap found verifying cli/setup (2/4 REQs). REQ:writes-starter-config wants a seeded config (rootCollections + languages) but `runSetup` (`setup.go`) writes an empty `.ingitdb/settings.yaml` with no seed (a code comment defers this). REQ:idempotent-on-empty-target wants the command to refuse an already-initialised directory and exit non-zero, but it silently overwrites via `os.WriteFile` with no guard. Also the spec text is stale: it describes a single `.ingitdb.yaml` with rootCollections/languages, while the implemented model is `.ingitdb/settings.yaml` + `.ingitdb/root_collections.yaml`. Implement the seed + overwrite guard (with tests for the cwd default and the Setup() command wiring), and update the spec to the real `.ingitdb/` layout. Blocks cli/setup → Stable.
