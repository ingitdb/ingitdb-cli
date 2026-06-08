---
type: sidekick-seed
captured_by: claude
status: promoted
---
# Implement materialize subcommands (collection/views) and the --collection / --views targeting, or reconcile the spec to the flat command

> **Promoted** to Idea [`materialize-collections-and-views`](../materialize-collections-and-views.md) on 2026-06-04. The subcommand framing was rejected in favor of a flat command with tri-state `--collections` / `--views` flags; the `cli/materialize` feature spec was rewritten accordingly.

Gap found verifying cli/materialize (1/3 REQs). The spec mandates `ingitdb materialize collection` and `ingitdb materialize views` subcommands; the code is a single flat `materialize` command (`materialize.go`) that only rebuilds views. The `--views=LIST` flag is registered (`flags.go`) but never read (dead no-op), there is no `--collection` flag, and there is no CLI path to regenerate a single collection's README (README rendering exists in `materializer/view_builder.go` but isn't wired to a `materialize collection` subcommand). Either implement the subcommands + flags (with tests) or update the spec to match the flat command. Spec Implementation list also omits `ci.go`/`flags.go`. Blocks cli/materialize → Stable.
