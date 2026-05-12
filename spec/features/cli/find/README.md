# Feature: Find Command

**Status:** Draft

## Summary

The `ingitdb find` command searches record fields for a substring (`--substr`), regex (`--re`), or exact value (`--exact`). Searches MAY be scoped to specific fields (`--fields`), to a sub-path (`--in`), and capped by `--limit`. At least one search flag is required.

## Problem

Users browsing or auditing an inGitDB database need a quick "grep across records" tool that understands the record-file layout. A plain `grep` on the working tree does not respect collection boundaries, leaks YAML structure into matches, and cannot scope by field.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb find`. At least one of `--substr`, `--re`, or `--exact` MUST be provided. When more than one is provided they MUST be combined with logical OR.

### Flags

#### REQ: scoping-flags

The `--in=REGEXP` flag MUST scope the search to records whose path matches the regex. The `--fields=LIST` flag MUST limit matching to the named fields; when omitted all fields are searched. The `--limit=N` flag MUST cap the number of returned records.

### Output

#### REQ: prints-matches

The command MUST print one matching record per output unit (line or block) and MUST exit `0` whether or not matches were found, unless an internal error occurred.

## Dependencies

- path-targeting

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should `find` accept `--format=yaml|json` for machine-readable output?
- When multiple search flags are provided, is OR the right default, or should they AND together?

---
*This document follows the https://specscore.md/feature-specification*
