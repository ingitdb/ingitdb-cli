# Feature: Diff Command

**Status:** Draft

## Summary

The `ingitdb diff` command compares two git refs and reports inGitDB record-level changes — added, updated, and deleted records grouped by collection. It supports four detail levels (`summary`, `record`, `fields`, `full`) and four output formats (`text`, `json`, `yaml`, `toml`). Exit code `1` on changes makes the command suitable as a CI guard.

## Problem

`git diff` operates on raw text and cannot distinguish a meaningful field change from a YAML reformat. Reviewers and CI need a tool that speaks records, fields, and collections natively, with enough depth control to summarize a release or audit a single field.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb diff`. The first positional argument MAY be `<ref>` (compare ref against HEAD), `<ref>..<ref>` (compare two refs), or omitted (working tree against HEAD).

### Scoping flags

#### REQ: scoping-flags

`--collection=KEY` and `--view=VIEW_KEY` MUST be mutually exclusive and limit the diff to a single collection or view. With `--view`, `--view-mode=output|source` MUST select between diffing the generated output file (`output`, default) or the underlying source records (`source`). `--path-filter=PATTERN` MUST further narrow by record path prefix or glob.

### Detail control

#### REQ: depth

The `--depth=summary|record|fields|full` flag MUST control output detail: `summary` prints per-collection counts; `record` prints one line per record with commit count and short hashes; `fields` adds the list of changed field names; `full` adds before/after values for each changed field. The default MUST be `summary`.

#### REQ: format

The `--format=text|json|yaml|toml` flag MUST select the output format. The default MUST be `text`.

### Exit codes

#### REQ: exit-codes

The command MUST exit `0` when no changes are found, `1` when changes are found, and `2` on infrastructure errors (bad flags, git failures, etc.).

## Dependencies

- path-targeting

## Acceptance Criteria

Not defined yet.

## Outstanding Questions

- Acceptance criteria not yet defined for this feature.
- Should `--depth=full` redact secrets the way some logging frameworks do?
- Should the JSON output schema be versioned and documented as a stable contract?

---
*This document follows the https://specscore.md/feature-specification*
