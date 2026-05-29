# Feature: Resolve Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve?op=request-change) |
**Status:** Draft

## Summary

The `ingitdb resolve` command is the working-tree conflict engine for inGitDB. It resolves merge conflicts that any git operation — `git rebase`, `git merge`, `git cherry-pick`, `git stash pop` — has left in the working tree (files containing conflict markers), independent of how those conflicts arose.

It handles two classes of conflict differently:

- **Generated / reproducible files** (collection `README.md`, materialized views, data indexes) are auto-resolved non-interactively by regenerating them from the source records and staging them.
- **Source-data files** (hand-edited record YAML/JSON) are resolved through an interactive, record-aware terminal UI.

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

### Behavior

#### REQ: auto-resolve-generated

For conflicted files that are generated / reproducible (collection `README.md`, materialized views, data indexes), the command MUST resolve them non-interactively by regenerating the file from the source records and staging it (`git add`), without prompting the user. This applies regardless of which git operation produced the conflict.

#### REQ: tui-loop

For conflicted source-data files, the command MUST run an interactive TUI that presents conflicts and accepts the user's choice for each. After each file is fully resolved the command MUST stage it (`git add`) and proceed to the next file. The command MUST exit `0` when all targeted files are resolved and non-zero when the user aborts or a file remains unresolved.

## Dependencies

- path-targeting

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/resolve`):

- [`cmd/ingitdb/commands/resolve.go`](../../../cmd/ingitdb/commands/resolve.go)

## Acceptance Criteria

Not defined yet.

## Open Questions

- Acceptance criteria not yet defined for this feature.
- Should `resolve` support a non-interactive `--strategy=ours|theirs` for scripting?
- How should the TUI present conflicts in `MapOfIDRecords` files where the conflict is at the key level rather than the field level?

---
*This document follows the https://specscore.md/feature-specification*
