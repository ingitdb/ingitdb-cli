---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Manual (Interactive) Conflict Resolution

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/manual-resolve?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/manual-resolve?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/manual-resolve?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/manual-resolve?op=request-change) |
**Status:** Draft
**Source Ideas:** —
**Parent Feature:** [`cli/resolve`](../README.md)

## Summary

Interactive, record-aware resolution of **source-data** conflicts — the
hand-edited record files (YAML/JSON/…) whose merge conflicts need a human
decision and cannot be regenerated from anything. The envisioned UI presents
each conflict field-by-field and lets the user pick a winner per field, instead
of editing raw `<<<<<<<`/`=======`/`>>>>>>>` markers.

## Problem

Merge conflicts in record files are visually noisy in `git mergetool`: the user
sees raw text hunks, not records. A record-aware TUI can show the two sides
structured by field and let the user choose per field, which is faster and less
error-prone than manual marker surgery.

## Current Implementation

This subfeature is **not implemented yet**. What exists today is a faithful
placeholder so the user is never left guessing:

- When `ingitdb resolve` finishes the [auto-resolve](../auto-resolve/README.md)
  pass and source-data conflicts remain, on a terminal it launches a small
  placeholder TUI screen
  ([`cmd/ingitdb/tui/conflicts_screen.go`](../../../../../cmd/ingitdb/tui/conflicts_screen.go))
  that shows a title, lists the conflicted files, describes the envisioned
  per-field resolution UI, and states **"Sorry, not implemented yet."** It quits
  on `q` / `esc` / `enter` / `ctrl+c`.
- Off a terminal (CI, scripts), the same information is printed to stderr.
- Either way the command exits non-zero, because the conflicts remain
  unresolved.

## Future Vision

- A per-conflict screen that parses each side of the record and presents the
  differing **fields**, letting the user pick ours/theirs (or edit) per field.
- Stage each fully-resolved file (`git add`) and advance to the next; exit `0`
  only when every targeted file is resolved, non-zero if the user aborts.
- Handle `MapOfIDRecords` files where the conflict is at the **key** level
  (record added/removed) rather than the field level.
- An optional non-interactive `--strategy=ours|theirs` for scripting/CI.

## Naming: why "manual", not "interactive"

This subfeature is named `manual-resolve` (not `interactive-resolve`)
deliberately:

- **It is the clean antonym of [`auto-resolve`](../auto-resolve/README.md).**
  The defining axis between the two subfeatures is *who decides the outcome* —
  the tool (`auto`) vs a human (`manual`). "Interactive" describes a *UX*, not
  the absence of automation, so it doesn't pair cleanly with "auto".
- **It stays consistent with the planned `--strategy=ours|theirs`.** That option
  is human-directed but *non-interactive*; calling the subfeature "interactive"
  would make a non-interactive flag inside it self-contradictory. Both the TUI
  and `--strategy` are "manual" (human-chosen), just different UX modes.
- **It matches git's conflict-resolution vocabulary.** Git reserves
  "interactive" for the `-i` family (`rebase -i`, `add -i`) — commit/hunk
  editing, not conflict resolution — and describes resolving conflicts as doing
  it *manually* (vs a merge tool).

"Interactive" is the right word for the *mode/flag* within this subfeature (the
TUI is the interactive mode), not for the subfeature itself.

## Behavior

### REQ: placeholder-until-implemented

Until interactive resolution is implemented, when source-data conflicts remain
after auto-resolve the command MUST inform the user — via a TUI screen on a
terminal, or stderr text otherwise — with a title, the list of conflicted files,
a description of the envisioned UI, and a "not implemented yet" message, and MUST
exit non-zero.

#### AC-1: terminal-shows-placeholder

**Given** unresolved source-data conflicts and a terminal
**When** `ingitdb resolve` runs
**Then** the interactive placeholder screen is launched and the command exits
non-zero with a "not implemented yet" error.

#### AC-2: non-terminal-prints-placeholder

**Given** unresolved source-data conflicts and no terminal (e.g. CI)
**When** `ingitdb resolve` runs
**Then** the placeholder title, conflicted files, and "not implemented yet"
notice are printed to stderr and the command exits non-zero.

### REQ: tui-loop

When implemented, the command MUST run an interactive TUI that presents each
source-data conflict and accepts the user's choice. After a file is fully
resolved it MUST be staged (`git add`); the command MUST exit `0` when all
targeted files are resolved and non-zero when the user aborts or a file remains
unresolved.

## Dependencies

- path-targeting

## Implementation

Source files (annotated with `// specscore: feature/cli/resolve/manual-resolve`):

- [`cmd/ingitdb/tui/conflicts_screen.go`](../../../../../cmd/ingitdb/tui/conflicts_screen.go)
- [`cmd/ingitdb/commands/resolve.go`](../../../../../cmd/ingitdb/commands/resolve.go) (`reportSourceConflicts`)

## Open Questions

- How should the TUI present conflicts in `MapOfIDRecords` files where the
  conflict is at the key level rather than the field level?
- Should a non-interactive `--strategy=ours|theirs` be offered for scripting?

---
*This document follows the https://specscore.md/feature-specification*
