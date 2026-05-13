# Idea: Batch Insert from Stdin

**Status:** Approved
**Date:** 2026-05-13
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we let users stream many records into `ingitdb insert` from stdin so migration scripts and ETL pipelines can land bulk data in one atomic command?

## Context

The current `ingitdb insert` command (spec/features/cli/insert/) accepts exactly one record per invocation from `--data`, stdin, `--edit`, or `--empty`. For migrations and pipelines this is painful: thousands of fork+exec invocations, no cross-record atomicity, no way to roll back a partially-applied import. The related `markdown-insert-ux` idea explicitly deferred bulk import as a separate concern — this is that ticket. The codebase already has the `ingr` record format with a `RecordsDelimiter` notion (see materialize tests) and per-collection `format` settings, so multi-record framing is partially solved at the storage layer; only the CLI input path is missing.

## Recommended Direction

Teach `ingitdb insert` to read a multi-record stream from stdin when a new `--format=<jsonl|yaml|ingr|csv>` flag is supplied. The flag selects the stream parser:

- `jsonl` — one JSON object per line (NDJSON).
- `yaml` — a YAML multi-document stream (`---`-separated docs).
- `ingr` — INGR's native multi-record framing.
- `csv` — header row maps to fields by default; `--fields=$id,title,priority,…` MAY be supplied to override the header → field mapping (or to drive parsing when the CSV has no header row).

Each record MUST carry its own `$id` (record key), since one `--key` flag cannot address many records. For CSV, `$id` MUST appear in the header or in `--fields`. The target collection comes from a single `--into=<collection>`. The whole batch is wrapped in one `RunReadwriteTransaction` — if any record fails (parse error, key collision, schema violation), every prior record in the batch is rolled back and the command exits non-zero with a diagnostic that names the offending record (line number for jsonl/csv, document index for yaml/ingr). `--data`, `--edit`, and `--empty` remain valid only for single-record mode and MUST be rejected when `--format` is set. This keeps the verb (`insert`), the existing flag grammar (`--into`, `$id`), and the strict no-overwrite semantics intact; batch is a stream-parser variant of the same command, not a new subcommand.

**Stream-format / storage-format independence.** The `--format` flag describes only the wire format on stdin. The on-disk representation of each record is governed by the target collection's `format:` setting in `.ingitdb.yaml` (yaml, json, markdown, ingr, csv). Feeding `--format=jsonl` into a collection whose storage `format: markdown` MUST produce one well-formed `<key>.md` file per record (frontmatter from the record's structured fields, body from a designated body field such as `$content`). Same for the other three input formats. Markdown is supported on the storage side, never on the stdin side.

## Alternatives Considered

- **New `bulk-insert` subcommand** — clean discovery surface, but duplicates every flag (`--into`, key resolution, view materialization) of `insert` and forces users to learn a second verb for the same intent. Rejected: a `--format` flag is a smaller surface change with identical semantics.
- **Auto-detect stream format from content sniffing** — zero flags, "just works" for `[{...}]` vs `---\n...` vs leading-`{`. Rejected: ambiguity at the boundaries (JSON-array-of-one vs single JSON object; CSV vs single-line YAML) leaks into error messages and creates a brittle contract. Explicit `--format` is the SQL convention and the user picked it.
- **Per-record collection routing via `$collection` pseudo-field** — one stream → many collections, attractive for whole-DB dumps. Rejected for MVP: every record now needs two pseudo-fields (`$id` + `$collection`), atomicity across heterogeneous collections complicates the transaction story, and migration scripts can re-run with different `--into` values cheaply. Revisit on demand.
- **Shell-loop with single-record `insert`** — `xargs -n1 ingitdb insert …`. Already possible today. Rejected as a solution: thousands of fork+exec cycles, no atomicity, partial state on failure — the exact problem this Idea exists to solve.

## MVP Scope

`cat records.jsonl | ingitdb insert --into=blog.posts --format=jsonl` reads N JSON objects from stdin, each carrying `$id` and field values, and inserts all N atomically into the `blog.posts` collection. Any failure (parse, duplicate key, validation) rolls back the entire batch and exits non-zero with a diagnostic that names the offending record. Same behaviour for `--format=yaml`, `--format=ingr`, and `--format=csv` (with `--fields` when no header row or when column ordering needs to be overridden). Local view materialization happens once after the commit.

The MVP MUST also demonstrate cross-format persistence: feeding `--format=jsonl` into a collection whose storage `format: markdown` MUST produce one `<key>.md` per record with frontmatter + body. An integration test against a markdown-stored collection is part of MVP acceptance.

## Not Doing (and Why)

- Per-record collection routing ($collection pseudo-field) — single `--into` is sufficient for MVP; revisit on demand
- Continue-on-error / stop-on-first-error modes — atomic is the only mode; selectable error semantics deferred until real users ask
- Markdown as a **stdin stream format** — markdown record framing in a stream is ambiguous (frontmatter `---` collides with YAML doc delimiter). Markdown-stored collections are fully supported on the persistence side via `--format=jsonl|yaml|ingr|csv`; the stream format is independent of the collection's on-disk format.
- File-based bulk import (--from-files=*.md) — stdin is the contract; file iteration is a shell-script concern
- Upsert / replace-on-conflict — insert is strict per the existing feature spec; bulk upsert is a separate verb
- Progress reporting / streaming feedback — atomic batches commit once; per-record progress is meaningless under all-or-nothing semantics

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | A single `RunReadwriteTransaction` can hold N pending inserts and roll back cleanly on any per-record failure (parse, key collision, validation) | unit test that injects a mid-batch failure and asserts zero records landed on disk |
| Must-be-true | Stream parsers for `jsonl`/`yaml`/`ingr`/`csv` can surface a per-record location (line number for jsonl/csv, doc index for yaml/ingr) in error messages | parser unit tests on malformed inputs |
| Must-be-true | A stream in any supported `--format` lands correctly in a markdown-stored collection: each record becomes a `<key>.md` file with frontmatter (structured fields) + body (designated body field, e.g. `$content`) | integration test: define a `format: markdown` collection, pipe a 3-record jsonl batch in, assert three `<key>.md` files exist with correct frontmatter and body content, and that the transaction is atomic (force a mid-batch failure and assert zero files written) |
| Should-be-true | Typical batch sizes (≤10k records) fit comfortably in memory; we can buffer parsed records before commit without a streaming-commit story | benchmark with 10k synthetic records on a representative repo |
| Should-be-true | CSV with a header row is the dominant case; `--fields` is a needed-but-secondary escape hatch (no-header CSV, column reorder, `$id` not first column) | survey the migration scripts we expect to onboard |
| Might-be-true | Users will want batch on `update` and `delete` next (same `--format` story applied to other verbs) | defer; revisit after `insert` ships |
| Might-be-true | `--format` should default to the collection's storage format when stdin is non-TTY and `--data`/`--edit`/`--empty` are all absent | defer; explicit `--format` is the MVP contract |


## SpecScore Integration

- **New Features this would create:** none — this is an enhancement to [`cli/insert`](../features/cli/insert/README.md). May warrant a sub-feature spec for the stream-format grammar.
- **Existing Features affected:** [`cli/insert`](../features/cli/insert/README.md) (adds `--format`, defines mutual exclusion with `--data`/`--edit`/`--empty`); [`shared-cli-flags`](../features/shared-cli-flags/README.md) (`$id` pseudo-field semantics carry into the stream); possibly [`output-formats`](../features/output-formats/README.md) for the shared `--format` registry.
- **Dependencies:** none blocking; the `ingr` format's `RecordsDelimiter` story should be confirmed stable before MVP.

## Outstanding Questions

- Should `--format` default to the collection's storage format when stdin is non-TTY and no other data source is supplied, or always be explicit?
- For CSV with a header row, how is `$id` named in the header — literal `$id`, or by configurable alias?
- Should empty batches (zero records on stdin) succeed silently with "0 inserted" or be rejected as a likely scripting bug?
- Is there a max-batch-size guardrail we want (e.g., reject > 100k records to prevent OOM) or do we trust the user?
- Does materialization of local views need to be deferred until after the entire batch commits, or run incrementally? (The Recommended Direction says "once after the commit" — confirm this is acceptable for views that watch large collections.)

---
*This document follows the https://specscore.md/idea-specification*
