# Feature: Materialize Command

**Status:** Draft

## Summary

The `ingitdb materialize` command builds generated artifacts from records: collection `README.md` files (`materialize collection`) and materialized view files under `$views/` (`materialize views`). Both subcommands accept `--path` and may target a subset of inputs.

## Problem

inGitDB derives several artifacts from its source records — view outputs and per-collection READMEs. Keeping those derived files up to date by hand is impossible at scale. A dedicated `materialize` command makes regeneration a deliberate, scriptable step that integrates with pull, rebase, and CI.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb materialize collection` or `ingitdb materialize views`. Each subcommand accepts an optional `--path=PATH` flag.

### materialize collection

#### REQ: collection-readme

`ingitdb materialize collection [--collection=ID]` MUST regenerate the `README.md` file for the named collection, or for every collection when `--collection` is omitted. The file MUST only be written when its content differs from what is already on disk.

### materialize views

#### REQ: views-output

`ingitdb materialize views [--views=LIST] [--records-delimiter=N]` MUST regenerate every materialized view (or only those listed in `--views`) into the `$views/` directory configured for each view. The `--records-delimiter` flag MUST control INGR record-delimiter behavior with values `1` (enabled), `-1` (disabled), and `0` / omitted (use the view/project default; the application default is `1`).

## Dependencies

- path-targeting

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should `materialize` support `--remote` to write generated files back to a remote repository in a single commit?
- Should there be a `materialize all` that runs both subcommands in dependency order?

---
*This document follows the https://specscore.md/feature-specification*
