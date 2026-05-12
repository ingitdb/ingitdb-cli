# Feature: Create Record Command

**Status:** Implementing

## Summary

The `ingitdb create record` command creates a new record in a `map[string]any` collection. The record's collection and key are taken from `--id`; its fields come from `--data` as YAML or JSON. The command works against a local path or a GitHub repository; remote writes require an authentication token.

## Problem

Users adding data to an inGitDB database should not have to hand-write the on-disk YAML/JSON in the exact location and shape the validator expects. A dedicated `create record` command encapsulates the placement, encoding, and (for GitHub) the commit creation in a single invocation.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb create record`. The `--id` and `--data` flags are required.

### Flags

#### REQ: id-and-data-required

`--id=<collection-id>/<record-key>` and `--data=YAML` MUST both be supplied. `--data` accepts inline YAML or JSON describing the record's fields (e.g. `'{name: Ireland}'`).

#### REQ: source-selection

`--path=PATH` and `--github=OWNER/REPO[@REF]` MUST be mutually exclusive. When neither is given the current working directory is used.

### Semantics

#### REQ: fails-if-exists

The command MUST fail when a record with the same key already exists in the target collection. It MUST NOT silently overwrite existing data.

#### REQ: github-write-requires-token

For `--github` writes, an authentication token MUST be supplied via `--token` or the `GITHUB_TOKEN` environment variable. Each successful create MUST result in exactly one commit in the remote repository (see [github-direct-access](../../github-direct-access/README.md)).

## Dependencies

- id-flag-format
- path-targeting
- github-direct-access

## Acceptance Criteria

### AC: creates-local-record

**Requirements:** cli/create-record#req:subcommand-name, cli/create-record#req:id-and-data-required, cli/create-record#req:fails-if-exists

`ingitdb create record --id=countries/ie --data='{name: Ireland}'` writes a new record file in the `countries` collection and exits `0`. Re-running the same command (without first deleting the record) exits non-zero.

### AC: creates-github-record-with-token

**Requirements:** cli/create-record#req:source-selection, cli/create-record#req:github-write-requires-token

With `GITHUB_TOKEN` set, `ingitdb create record --github=owner/repo --id=countries/ie --data='{name: Ireland}'` creates one commit in `owner/repo` containing the new record file. Without a token the command exits non-zero before any network request that would require authentication.

## Outstanding Questions

- Should `create record` support `[]map[string]any` and `map[string]map[string]any` collection types, or remain limited to `map[string]any`?

---
*This document follows the https://specscore.md/feature-specification*
