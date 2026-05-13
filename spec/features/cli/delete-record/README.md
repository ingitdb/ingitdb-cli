# Feature: Delete Record Command

**Status:** Superseded by [spec/features/cli/delete/](../delete/README.md). The `ingitdb delete record` command has been removed; use `ingitdb delete --id=...` instead. This document is preserved as a historical record.

## Summary

The `ingitdb delete record` command removes a single record by `--id` from a local database directory or from a remote Git repository. For `SingleRecord` collections the entire record file is removed; for `MapOfIDRecords` collections only the matching key is removed from the shared map file.

## Problem

Deleting a record from an inGitDB database requires understanding the collection's storage layout: is each record its own file, or are records keyed entries inside a shared file? A dedicated `delete record` command shields callers from that detail and produces the right on-disk change for both layouts.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb delete record`. The `--id` flag is required.

### Flags

#### REQ: id-required

`--id=<collection-id>/<record-key>` MUST identify the record to delete and MUST follow the syntax defined by [id-flag-format](../../id-flag-format/README.md).

#### REQ: source-selection

`--path=PATH` and `--remote=HOST/OWNER/REPO[@REF]` MUST be mutually exclusive. When neither is given the current working directory is used.

### Semantics

#### REQ: storage-layout-aware

For collections whose `record_file.type` is `SingleRecord`, the command MUST delete the on-disk file backing the record. For collections whose `record_file.type` is `MapOfIDRecords`, the command MUST remove the matching key from the shared map file while leaving sibling keys intact.

#### REQ: remote-write-requires-token

For `--remote` writes, a token MUST be supplied via `--token` or a host-derived environment variable (e.g. `GITHUB_TOKEN` for `github.com`). Each successful delete MUST produce exactly one commit in the remote repository.

## Dependencies

- id-flag-format
- path-targeting
- remote-repo-access

## Acceptance Criteria

### AC: deletes-single-record-file

**Requirements:** cli/delete-record#req:subcommand-name, cli/delete-record#req:id-required, cli/delete-record#req:storage-layout-aware

Given a `SingleRecord` collection containing the record `countries/ie`, running `ingitdb delete record --id=countries/ie` removes the corresponding file and exits `0`. Re-running the command exits non-zero because the record no longer exists.

### AC: deletes-key-from-map

**Requirements:** cli/delete-record#req:storage-layout-aware

Given a `MapOfIDRecords` collection whose shared file contains keys `ie` and `gb`, deleting `--id=collection/ie` removes the `ie` key from the file but leaves `gb` and any other keys untouched.

## Outstanding Questions

- None at this time.

---
*This document follows the https://specscore.md/feature-specification*
