# Feature: Pull Command

**Status:** Draft

## Summary

The `ingitdb pull` command runs `git pull` (rebase by default, or merge), automatically resolves conflicts in generated files by regenerating them, opens an interactive TUI for source-data conflicts, rebuilds materialized views, and prints a summary of records added, updated, and deleted by the pull.

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

- [`cmd/ingitdb/commands/pull.go`](../../../cmd/ingitdb/commands/pull.go)

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should the interactive TUI step be skippable via a `--no-interactive` flag for CI?
- Should the summary output be machine-readable (JSON) under a flag?

---
*This document follows the https://specscore.md/feature-specification*
