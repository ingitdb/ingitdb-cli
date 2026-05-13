# Feature: Update Record Command

**Status:** Superseded by [spec/features/cli/update/](../update/README.md). The `ingitdb update record` command has been renamed to `ingitdb update`. This document is preserved as a historical record.

## Summary

The `ingitdb update record` command applies patch-style updates to an existing record: only the fields listed in `--set` change; every other field is preserved. The command works against a local path or a remote Git repository.

## Problem

Most edits to a record touch one or two fields. A full-replace API forces callers to first read the record, mutate it, and write the whole document back — a race-prone, error-prone pattern. Patch semantics keep updates surgical and make scripts safe even when the schema gains new fields.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb update record`. The `--id` and `--set` flags are required.

### Flags

#### REQ: id-and-set-required

`--id=<collection-id>/<record-key>` and `--set=YAML` MUST both be supplied. `--set` carries the fields to patch as YAML or JSON (e.g. `'{capital: Dublin}'`).

#### REQ: source-selection

`--path=PATH` and `--remote=HOST/OWNER/REPO[@REF]` MUST be mutually exclusive. When neither is given the current working directory is used.

### Semantics

#### REQ: patch-semantics

The command MUST apply the fields in `--set` as a shallow patch onto the existing record. Fields not mentioned in `--set` MUST remain untouched. The command MUST fail when the target record does not exist.

#### REQ: remote-write-requires-token

For `--remote` writes, a token MUST be supplied via `--token` or a host-derived environment variable (e.g. `GITHUB_TOKEN` for `github.com`), and each successful update MUST produce exactly one commit in the remote repository.

## Dependencies

- id-flag-format
- path-targeting
- remote-repo-access

## Acceptance Criteria

### AC: patches-existing-record

**Requirements:** cli/update-record#req:subcommand-name, cli/update-record#req:id-and-set-required, cli/update-record#req:patch-semantics

Given a record `{name: Ireland, population: 5000000}`, running `ingitdb update record --id=countries/ie --set='{capital: Dublin}'` produces a record `{name: Ireland, population: 5000000, capital: Dublin}` and exits `0`. Updating a non-existent record exits non-zero.

### AC: remote-update-creates-one-commit

**Requirements:** cli/update-record#req:source-selection, cli/update-record#req:remote-write-requires-token

With a valid token, `ingitdb update record --remote=github.com/owner/repo --id=countries/ie --set='{capital: Dublin}'` produces exactly one commit in `owner/repo` whose diff is limited to the patched fields.

## Outstanding Questions

- Should the patch be deep-merged into nested maps, or remain shallow at the top level?

---
*This document follows the https://specscore.md/feature-specification*
