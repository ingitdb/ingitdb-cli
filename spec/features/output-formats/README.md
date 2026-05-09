# Feature: Output Formats

**Status:** Implementing

## Summary

Read-style commands accept a `--format=FORMAT` flag that selects the encoding of their output. The two universally supported values are `yaml` (default) and `json`. Individual commands MAY support additional formats (CSV, Markdown, TOML) but MUST always honor `yaml` and `json` with the same default behavior.

## Problem

Without a shared output-format convention, every command would invent its own flag name and default. A common contract makes scripts and pipelines composable: a user who knows `--format=json` for one command knows it for all of them.

## Behavior

### Flag

#### REQ: format-flag-name

When a command emits structured data, the flag controlling its encoding MUST be `--format=FORMAT`. Short forms (e.g. `-f`) MAY be added for ergonomic reasons but MUST NOT replace `--format`.

#### REQ: yaml-default

When `--format` is omitted, the output MUST be YAML. Commands MUST NOT silently switch the default to JSON based on a TTY check or environment variable.

#### REQ: json-supported

Every command that supports `--format` MUST accept `json` as a value and produce output that is valid JSON suitable for piping to `jq` or similar tools.

### Extensions

#### REQ: command-specific-formats

A command MAY support additional formats (e.g. `csv`, `md`, `toml`) beyond `yaml` and `json` when they make domain sense. Such formats MUST be documented in the command's own feature spec.

## Acceptance Criteria

### AC: yaml-is-default

**Requirements:** output-formats#req:format-flag-name, output-formats#req:yaml-default

Running any read-style command without `--format` produces YAML output. The output is identical whether or not stdout is a TTY.

### AC: json-mode-is-pipe-friendly

**Requirements:** output-formats#req:json-supported

Adding `--format=json` to a read-style command produces output that parses cleanly as JSON. No log lines or progress output leak into stdout.

## Outstanding Questions

- Should multi-record commands emit JSON arrays, NDJSON, or both behind a flag?

---
*This document follows the https://specscore.md/feature-specification*
