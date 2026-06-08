---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Resolve Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve?op=request-change) |
**Status:** Stable
**Source Ideas:** —

## Summary

The `ingitdb resolve` command is the working-tree conflict engine for inGitDB. It resolves merge conflicts that any git operation — `git rebase`, `git merge`, `git cherry-pick`, `git stash pop` — has left in the working tree (files containing conflict markers), independent of how those conflicts arose.

It handles two classes of conflict differently:

- **Generated / reproducible files** (collection `README.md`, materialized views, data indexes) are auto-resolved non-interactively by regenerating them from the source records and staging them.
- **Source-data files** (hand-edited record YAML/JSON) are first run through a record-aware three-way merge that auto-resolves logically non-conflicting changes (e.g. records added with distinct IDs); only the genuinely contested ones fall through to an interactive, record-aware terminal UI.

With `--file=FILE` it targets a single conflicted file; without it, every conflicted file in the database is processed in turn.

## Relationship to related commands

`resolve` is the shared conflict-resolution engine; the other commands *initiate* a git operation and then delegate to it:

- **[`rebase`](../rebase/README.md)** — runs `git rebase` onto a base ref, then calls `resolve` to handle any resulting conflicts. Use `rebase` when you want inGitDB to drive the rebase; use `resolve` when conflict markers already exist from a rebase/merge you ran yourself.
- **[`pull`](../pull/README.md)** — runs `git pull`, then calls `resolve`, then rebuilds views and prints a record-change summary.

## Problem

Merge conflicts in YAML or JSON record files are visually noisy in `git mergetool`. A record-aware TUI can present the two sides field-by-field and let the user pick a winner per field instead of per text region. Conflicts in generated files are pure busywork because the content is reproducible from source — they should be resolved automatically.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb resolve`. All flags are optional.

### Flags

#### REQ: file-and-path

The `--file=FILE` flag MUST scope the operation to a single conflicted file. When omitted the command MUST iterate through every conflicted file in the database. The `--path=PATH` flag MUST select the database directory; when omitted the current working directory is used.

### Conflict handling

`resolve` routes each conflicted file to one of two subfeatures by conflict
class. See each subfeature for its requirements, current state, and roadmap.

## Subfeatures

- **[auto-resolve](auto-resolve/README.md)** — non-interactive regeneration of
  conflicted generated / reproducible files (collection `README.md` today;
  materialized views and data indexes planned). **Status: Implementing** —
  README auto-resolution is implemented and shared with `rebase`.
- **[manual-resolve](manual-resolve/README.md)** — interactive, record-aware
  resolution of source-data conflicts that need a human decision.
  **Status: Draft** — currently a placeholder screen that describes the
  envisioned UI and reports that it is not implemented yet.

After the auto-resolve pass, any remaining (source-data) conflicts are handed
to manual-resolve. The command exits `0` only when every targeted conflict is
resolved, and non-zero otherwise.

## Dependencies

- path-targeting

## Implementation

Source files implementing the shared command shell (annotated with
`// specscore: feature/cli/resolve`):

- [`cmd/ingitdb/commands/resolve.go`](../../../../cmd/ingitdb/commands/resolve.go)

Per-subfeature implementation is listed in each subfeature's spec.

## Open Questions

- Should `resolve` support a non-interactive `--strategy=ours|theirs` for
  scripting (applies mainly to manual-resolve)?

---
*This document follows the https://specscore.md/feature-specification*
