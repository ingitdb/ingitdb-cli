# Feature: Auto-Resolve Generated Conflicts

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/auto-resolve?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/auto-resolve?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/auto-resolve?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/auto-resolve?op=request-change) |
**Status:** Implementing
**Parent Feature:** [`cli/resolve`](../README.md)

## Summary

Non-interactive resolution of merge conflicts in **generated / reproducible**
files. When a conflicted file's content is fully derivable from source records,
`resolve` regenerates it from those records and stages the result, so a human
never hand-merges a reproducible artifact.

## Problem

Generated artifacts — collection `README.md`, materialized views, data
indexes — routinely conflict during merges and rebases even when the underlying
source data does not. Hand-resolving them is pure busywork: the correct content
is simply whatever regeneration produces.

## Current Implementation

Implemented today in
[`cmd/ingitdb/commands/conflict_resolver.go`](../../../../../cmd/ingitdb/commands/conflict_resolver.go):

- Detects conflicted files in the working tree via
  `git diff --name-only --diff-filter=U`, independent of which git operation
  (rebase, merge, cherry-pick, stash pop) produced them.
- For conflicted collection **`README.md`** files, regenerates the README from
  the collection's source records (`docsbuilder.ProcessCollection`) and stages
  it with `git add`.
- Exposed both directly through `ingitdb resolve` and by `rebase`, which
  delegates to the same shared engine instead of duplicating the logic.

Only the `readme` category is wired into the conflict path so far.

## Future Vision

- Extend auto-resolution to **materialized views** and **data indexes** — the
  materializer can already regenerate them; wire them into the conflict path.
- A `--resolve=CATEGORIES` opt-in (parity with `rebase`'s flag) so callers can
  select which generated categories to auto-resolve.
- When a regenerated file still differs from both conflict sides, surface it for
  review instead of silently overwriting.

## Behavior

### REQ: auto-resolve-generated

For conflicted files that are generated / reproducible (collection `README.md`
today), the command MUST resolve them non-interactively by regenerating the file
from its source records and staging it (`git add`), without prompting the user.
This MUST apply regardless of which git operation produced the conflict.

#### AC-1: readme-conflict-regenerated-and-staged

**Given** a working tree with a merge conflict in a collection `README.md`
**When** `ingitdb resolve` runs
**Then** the README is regenerated from source records, staged, and no longer
reported by `git diff --name-only --diff-filter=U`.

#### AC-2: independent-of-git-operation

**Given** conflict markers left by a manual `git merge` (not an `ingitdb rebase`)
**When** `ingitdb resolve` runs
**Then** the generated-file conflicts are resolved the same way as for a rebase.

## Dependencies

- path-targeting

## Implementation

Source files (annotated with `// specscore: feature/cli/resolve/auto-resolve`):

- [`cmd/ingitdb/commands/conflict_resolver.go`](../../../../../cmd/ingitdb/commands/conflict_resolver.go)

## Open Questions

- Should views and indexes be auto-resolved by default, or gated behind an
  explicit `--resolve` category opt-in?

---
*This document follows the https://specscore.md/feature-specification*
