# Feature: Read Record Command

**Status:** Implementing

## Summary

The `ingitdb read record` command reads a single record by `--id` and writes it to stdout in YAML (default) or JSON. It works against a local database directory (`--path`) or directly against a GitHub repository (`--github`).

## Problem

Inspecting a single record is a fundamental data-access operation. Users need a uniform way to fetch one record without writing custom YAML/JSON parsing code, regardless of whether the database lives on a local clone or in a remote GitHub repository.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb read record`. The `--id` flag is required; all others are optional.

### Flags

#### REQ: id-required

The `--id=<collection-id>/<record-key>` flag MUST be provided. Its value MUST follow the syntax defined by [id-flag-format](../id-flag-format/README.md).

#### REQ: source-selection

The command MUST accept either `--path=PATH` (local directory) or `--github=OWNER/REPO[@REF]` (remote GitHub repository), but never both. When neither is given, the current working directory is used as the local path.

#### REQ: format-flag

The `--format=FORMAT` flag MUST accept `yaml` or `json`. When omitted, the output format MUST be `yaml`.

### Output

#### REQ: writes-to-stdout

The command MUST write the resolved record to stdout in the requested format and exit `0`. If the record cannot be located it MUST exit non-zero with a diagnostic message.

## Dependencies

- id-flag-format
- output-formats
- path-targeting
- github-direct-access

## Acceptance Criteria

### AC: reads-local-record

**Requirements:** read-record-command#req:subcommand-name, read-record-command#req:id-required, read-record-command#req:source-selection, read-record-command#req:writes-to-stdout

Given a local database with a record at `geo.nations/ie`, `ingitdb read record --id=geo.nations/ie` writes the record's fields to stdout as YAML and exits `0`. Adding `--format=json` switches the output to JSON.

### AC: reads-from-github

**Requirements:** read-record-command#req:source-selection, read-record-command#req:format-flag

`ingitdb read record --github=owner/repo --id=countries/ie` resolves the record from the default branch of the given repository without requiring a local clone. Pinning to a ref (`owner/repo@main`) reads from that ref instead.

## Outstanding Questions

- Should the command emit a structured "not found" error code distinct from generic failures?

---
*This document follows the https://specscore.md/feature-specification*
