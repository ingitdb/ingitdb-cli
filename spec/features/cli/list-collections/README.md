# Feature: List Collections Command

**Status:** Implementing

## Summary

The `ingitdb list collections` command prints every collection ID defined in a database. It works against a local directory (`--path`) or a GitHub repository (`--github`), and supports two narrowing flags: `--in` (regex on starting-point path) and `--filter-name` (glob on collection name).

## Problem

Discovering which collections exist in a database is a precondition for almost every other operation: querying, finding, deleting, materializing. Without an explicit listing command, callers would have to parse `.ingitdb.yaml` themselves or grep the directory tree.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb list collections`. All flags are optional.

### Flags

#### REQ: source-selection

`--path=PATH` and `--github=OWNER/REPO[@REF]` MUST be mutually exclusive. When neither is given the current working directory is used.

#### REQ: in-flag

The `--in=REGEXP` flag MUST scope the listing to collections whose starting-point path matches the regular expression.

#### REQ: filter-name-flag

The `--filter-name=PATTERN` flag MUST filter collections by name using a glob-style pattern (e.g. `*city*`).

### Output

#### REQ: prints-ids

The command MUST print one collection ID per line and MUST exit `0` on success. The output MUST be deterministic given a fixed database state and flag set.

## Dependencies

- path-targeting
- github-direct-access

## Acceptance Criteria

### AC: lists-all-collections

**Requirements:** list-collections-command#req:subcommand-name, list-collections-command#req:prints-ids

`ingitdb list collections` from a directory whose `.ingitdb.yaml` declares N collections prints exactly N collection IDs to stdout, one per line, and exits `0`.

### AC: scoped-listing

**Requirements:** list-collections-command#req:in-flag, list-collections-command#req:filter-name-flag

`ingitdb list collections --in='countries/(ie|gb)' --filter-name='*city*'` returns only collections whose starting-point path matches the regex AND whose name matches the glob.

## Outstanding Questions

- Should `list collections` emit JSON when `--format=json` is passed (consistent with `read record`)?

---
*This document follows the https://specscore.md/feature-specification*
