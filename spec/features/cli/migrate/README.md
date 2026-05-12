# Feature: Migrate Command

**Status:** Draft

## Summary

The `ingitdb migrate` command transforms records from one schema version to another for a named target. `--from`, `--to`, and `--target` are required; `--collections`, `--format`, and `--output-dir` allow narrowing scope and redirecting output.

## Problem

Schema evolution in long-lived inGitDB databases requires structured, repeatable record transformations. A dedicated `migrate` command keeps the rules in one place rather than scattered across one-off scripts.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb migrate`. The `--from=VERSION`, `--to=VERSION`, and `--target=TARGET` flags MUST all be supplied.

### Flags

#### REQ: scoping-flags

The `--collections=LIST` flag MUST limit the migration to the named collections; when omitted every collection is migrated. The `--output-dir=DIR` flag MUST redirect migrated records to the named directory instead of overwriting in place. The `--format=FORMAT` flag MUST select the output format for migrated records. The `--path=PATH` flag MUST select the database directory; when omitted the current working directory is used.

### Behavior

#### REQ: from-to-determines-transform

The pair `(--from, --to)` MUST determine the transformation applied to each record. The command MUST fail clearly when no migration is registered for the requested pair and target.

## Dependencies

- path-targeting

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- How are migration rules registered — code, config file, or external plugin?
- Should `migrate` support a multi-step path (e.g. v1 -> v2 -> v3 in one invocation)?

---
*This document follows the https://specscore.md/feature-specification*
