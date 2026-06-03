# Feature: Pull Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/pull?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/pull?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/pull?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/pull?op=request-change) |
**Status:** Implementing

## Summary

The `ingitdb pull` command runs `git pull` (rebase by default, or merge), automatically resolves conflicts in generated files by regenerating them, opens an interactive TUI for source-data conflicts, rebuilds materialized views, and prints a summary of records added, updated, and deleted by the pull.

## Relationship to related commands

`pull` is an *initiator*: it drives `git pull`, then delegates conflict handling to the shared engine and adds view rebuilding plus a change summary on top.

- **[`resolve`](../resolve/README.md)** — the working-tree conflict engine `pull` delegates to (step 2–3 of the pipeline below). It auto-resolves generated-file conflicts and resolves source conflicts interactively.
- **[`rebase`](../rebase/README.md)** — the same initiator pattern for `git rebase` onto a base ref. `pull` is for syncing with a remote; `rebase` is for replaying the current branch onto an updated base.

## Problem

Pulling changes into an inGitDB working tree typically requires several manual steps: pull, regenerate views, regenerate READMEs, resolve any conflicts, and double-check the result. A single command bundles those steps into one auditable operation.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb pull`. All flags are optional.

### Flags

#### REQ: strategy-and-target

The `--strategy=rebase|merge` flag MUST control whether the pull is performed via `git pull --rebase` or `git pull --merge`; the default is `rebase`. The `--remote=REMOTE` flag MUST default to `origin`. The `--branch=BRANCH` flag MUST default to the current branch's tracking branch.

### Pipeline

#### REQ: pull-pipeline

In order, the command MUST: (1) run `git pull` with the chosen strategy, remote, and branch; (2) auto-resolve conflicts in generated files (materialized views, `README.md`) by regenerating them; (3) open an interactive TUI for conflicts in source data files; (4) rebuild materialized views and READMEs; (5) print a summary of added, updated, and deleted records.

### Exit codes

#### REQ: exit-codes

The command MUST exit `0` when all conflicts are resolved and views rebuilt successfully, `1` when unresolved conflicts remain after interactive resolution, and `2` on infrastructure errors (git missing, network failure, bad flags).

## Dependencies

- path-targeting

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/pull`):

- [`cmd/ingitdb/commands/pull.go`](../../../cmd/ingitdb/commands/pull.go) — the command, `git pull`, view rebuild, and change summary
- [`cmd/ingitdb/commands/resolve.go`](../../../cmd/ingitdb/commands/resolve.go) — `resolveWorkingTreeConflicts`, the shared conflict engine `pull` and `resolve` both call

## Acceptance Criteria

### AC: syncs-and-rebuilds

**Requirements:** cli/pull#req:subcommand-name, cli/pull#req:pull-pipeline

Given a clone that is behind its upstream by a commit adding a new record, `ingitdb pull` fast-forwards/rebases the new commit in, rebuilds materialized views, exits `0`, and prints a summary reporting one added record file.

### AC: strategy-flag

**Requirements:** cli/pull#req:strategy-and-target

`--strategy` defaults to `rebase` (`git pull --rebase`); `--strategy=merge` uses `git pull --no-rebase`; `--remote` defaults to `origin` and an omitted `--branch` uses the tracking branch. An invalid `--strategy` value is rejected before any git invocation.

### AC: delegates-conflict-resolution

**Requirements:** cli/pull#req:pull-pipeline

When the pull leaves conflicts, `pull` runs the shared working-tree engine (`resolveWorkingTreeConflicts`): generated-file (`README.md`) conflicts are auto-resolved by regeneration, source-data files get a record-aware three-way merge, and any still-unresolved source conflicts are handed to the interactive resolver (or reported), yielding a non-zero exit.

## Scope (current implementation)

Implemented: the full pipeline (pull → resolve → rebuild views → summary), `--strategy/--remote/--branch`, and a change summary at **record-file granularity** (within-file row changes for map/list layouts count as one updated file).

Deferred / inherited limitations:
- **Exit codes** are `0` (success) / non-zero (failure); the spec's distinct `1` vs `2` granularity needs main-loop exit-code support and is not yet implemented.
- **Interactive source-conflict resolution** inherits `resolve`'s current behavior — the interactive TUI is the separate, still-unbuilt [`manual-resolve`](../resolve/manual-resolve/README.md) feature, so unresolved source conflicts exit non-zero with a "not implemented yet" message.

## Open Questions

- Should the interactive TUI step be skippable via a `--no-interactive` flag for CI?
- Should the summary output be machine-readable (JSON) under a flag?

---
*This document follows the https://specscore.md/feature-specification*
