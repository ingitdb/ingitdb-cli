# Feature: Resolve Command

**Status:** Draft

## Summary

The `ingitdb resolve` command opens an interactive terminal UI for resolving merge conflicts in inGitDB record files. With `--file=FILE` it targets a single conflicted file; without it, every conflicted file in the database is processed in turn.

## Problem

Merge conflicts in YAML or JSON record files are visually noisy in `git mergetool`. A record-aware TUI can present the two sides field-by-field and let the user pick a winner per field instead of per text region.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb resolve`. All flags are optional.

### Flags

#### REQ: file-and-path

The `--file=FILE` flag MUST scope the operation to a single conflicted file. When omitted the command MUST iterate through every conflicted file in the database. The `--path=PATH` flag MUST select the database directory; when omitted the current working directory is used.

### Behavior

#### REQ: tui-loop

The command MUST run an interactive TUI that presents conflicts and accepts the user's choice for each. After each file is fully resolved the command MUST stage it (`git add`) and proceed to the next file. The command MUST exit `0` when all targeted files are resolved and non-zero when the user aborts or a file remains unresolved.

## Dependencies

- path-targeting

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should `resolve` support a non-interactive `--strategy=ours|theirs` for scripting?
- How should the TUI present conflicts in `MapOfIDRecords` files where the conflict is at the key level rather than the field level?

---
*This document follows the https://specscore.md/feature-specification*
