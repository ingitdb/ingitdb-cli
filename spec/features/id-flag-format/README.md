# Feature: ID Flag Format

**Status:** Implementing

## Summary

The `--id` flag, used by every CRUD command, follows the syntax `<collection-id>/<record-key>`. The collection ID is dot-separated and uses a restricted character set; the record key follows after a single `/`. When prefixes overlap, the longest matching collection ID wins.

## Problem

Without a single, sharply defined ID syntax, every CRUD command would be free to invent its own. A shared format keeps `read`, `create`, `update`, `delete` and any future single-record commands consistent for users and tooling.

## Behavior

### Syntax

#### REQ: collection-key-separator

The `--id` value MUST be of the form `<collection-id>/<record-key>`, with a single `/` separating the collection ID from the record key. Backslashes and other separators MUST NOT be accepted in place of `/`.

#### REQ: collection-id-charset

A collection ID MUST contain only alphanumeric characters and the `.` character. It MUST start and end with an alphanumeric character. The `/` character MUST NOT appear inside a collection ID; its only role inside `--id` is to separate the collection from the key.

#### REQ: longest-prefix-wins

When more than one declared collection ID is a prefix of the `--id` value, the longest matching prefix MUST be selected as the collection. The remainder of the value (after the trailing `/`) MUST be treated as the record key.

### Examples

#### REQ: example-shape

`--id=geo.nations/ie` MUST resolve to collection `geo.nations` and record key `ie`. `--id=countries/ie` MUST resolve to collection `countries` and record key `ie`.

### Applicability

#### REQ: only-singleton-collections

The `--id`-driven CRUD commands MUST only operate on collections whose `record_file.type` is `map[string]any`. They MUST reject IDs targeting collections of type `[]map[string]any` (list) or `map[string]map[string]any` (dictionary) until those layouts are explicitly supported.

## Acceptance Criteria

### AC: id-syntax-rejects-invalid

**Requirements:** id-flag-format#req:collection-key-separator, id-flag-format#req:collection-id-charset

`--id=` values that omit the `/` separator, contain disallowed characters in the collection segment, or do not start and end with alphanumeric characters MUST be rejected with a clear diagnostic.

### AC: longest-prefix-resolution

**Requirements:** id-flag-format#req:longest-prefix-wins, id-flag-format#req:example-shape

When both `geo` and `geo.nations` are declared collections, `--id=geo.nations/ie` resolves to `geo.nations`, not `geo`.

## Outstanding Questions

- Should the longest-prefix rule be configurable, or is it always implicit?
- Should `--id` validation produce a structured error code distinct from generic flag errors?

---
*This document follows the https://specscore.md/feature-specification*
