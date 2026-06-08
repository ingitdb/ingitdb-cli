---
format: https://specscore.md/idea-specification
status: Approved
---

# Idea: Derived Record Keys from Record Fields

**Status:** Approved
**Date:** 2026-05-30
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How Might We let schema authors declare that a single-record file key is derived from record fields, so filenames, record contents, insert behavior, and validation stay in sync?

## Context

Triggered by [GitHub issue #66](https://github.com/ingitdb/ingitdb-cli/issues/66): `project-hub` agent-log records need filenames like
`{timestamp}-{agent}-{category}-{project}.md` derived from Markdown frontmatter
fields, but today `record_file.name` only knows `{key}`.

The current model has two separate facts that users must keep aligned by hand:

- the record key, which becomes the `{key}` part of the single-record filename;
- the record fields, such as `timestamp`, `agent`, `category`, and `project`.

That split is fine when the key is arbitrary or externally assigned. It breaks
down for log-like and document-like collections where the filename is deliberately
semantic and should be reproducible from the record itself. The target user is a
schema author who wants Git-friendly, human-readable filenames without relying on
README-only conventions. Success means CI validation can catch drift, `insert`
can create the right path without duplicate key entry, and `select` keeps exposing
the canonical key consistently as record identity.

## Recommended Direction

Add a collection-level derived record key definition that turns selected record
fields into the canonical record key through explicit, machine-readable
transforms. The key should remain the record identity; this Idea only defines how
that identity can be computed from record content when a collection opts in.

The strongest shape is a small declarative schema, not hooks or arbitrary code:

```yaml
record_key:
  template: "{timestamp}-{agent}-{category}-{project}"
  fields:
    timestamp:
      type: datetime
      output_format: "2006-01-02T15-04-05"
    agent:
      transform: slug
    category:
      transform: slug
    project:
      transform: slug
```

Use a map keyed by field name as shown above. The exact option names can change
at Feature-spec time, but the contract should be constrained: named fields, a
template, and a small transform vocabulary (`datetime` formatting and slug
normalization are enough for the motivating case). A search found no slug helper
inside `ingitdb-cli`, but sibling projects provide prior art:
`specscore-cli/pkg/slug.IssueSlug` is dependency-free, while Sneat services use
`github.com/gosimple/slug`. Use `github.com/gosimple/slug` for the MVP unless
Feature-spec review finds a compatibility issue; route every derived-key slug
transform through one helper so validation, insert, and read/select identity
surfaces do not grow three subtly different interpretations of the same
convention.

MVP pressure should be validation-first: prove that the schema can recompute the
expected key from parsed record data and produce a precise error when
`$records/<actual>.md` does not match. Once that invariant exists, `insert` can
derive the key when `--key` is omitted. `select` should continue exposing the
record key through the existing `$id`/key surface; it does not need a second,
derived-key-specific pseudo-field.

## Alternatives Considered

- **Keep conventions in README files.** Rejected — this is the status quo, and it
  asks users and CI to trust prose for something the schema can validate
  mechanically. It also leaves insert unable to generate the path.
- **Let `record_file.name` interpolate arbitrary record fields directly.** Rejected
  as the primary design — it looks simple but hides transform rules inside a file
  path template and makes validation errors harder to explain. Keep
  `record_file.name: "{key}.md"` as storage layout, and define how `{key}` is
  derived separately.
- **Require users to keep supplying `--key`.** Rejected — it catches fewer mistakes
  than validation and preserves duplicate entry of the same facts. It is still a
  useful override/error-check path when supplied alongside derivable fields.
- **Use custom hooks or scripts for key generation.** Rejected for MVP — hooks are
  powerful but non-portable and difficult to run safely in CI, remote operations,
  and GitHub Actions. A tiny declarative transform set covers the first real use
  case with much lower risk.
- **Broaden this into every kind of identity resolution.** Rejected — list row keys,
  `primary_key`, `$id`, and `id` fallback are already covered by adjacent
  record-format work. This Idea is specifically about single-record file keys
  derived from fields inside that record.

## MVP Scope

A two-week slice that proves one job: a single-record Markdown/YAML collection can
declare a derived key template, and inGitDB can enforce that filename identity
matches parsed record content.

The MVP should support the motivating template:
`{timestamp}-{agent}-{category}-{project}`. It should parse existing records,
format datetimes with an explicit Go-style output format, slugify selected string
fields, compare the derived key to the actual `{key}` filename, and emit a
validation error that names the record path, expected key, actual key, and fields
used. `insert` may omit `--key` when all derived-key fields are present; if
`--key` is also supplied, it must be accepted only when it matches the derived
key.

## Not Doing (and Why)

- Arbitrary expression language — too much power before the core invariant is proven
- Automatic migration or renaming of existing files — validation should identify drift before tooling mutates Git history
- List-of-records row identity changes — captured as a sidekick seed for separate evaluation

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | A derived key can be computed after record parsing for Markdown and YAML without changing the on-disk single-record layout. | Add resolver tests that parse Markdown frontmatter and YAML records, derive the same key, and leave `record_file.name: "{key}.md"` unchanged. |
| Must-be-true | The transform vocabulary can stay intentionally small while covering the motivating case. | Validate `datetime` output formatting and string slugification against the issue #66 example; reject unknown transforms with clear schema errors. |
| Must-be-true | Validation can report key drift precisely enough for CI users to fix the file by hand. | Create a mismatched filename fixture and assert the diagnostic includes path, expected key, actual key, and source fields. |
| Should-be-true | Letting `insert` omit `--key` improves UX without weakening strict insert semantics. | Test insert with derivable fields, with missing derivation fields, with matching `--key`, and with both `--key` and derived key present but different. |
| Might-be-true | Users will want additional transforms such as lower-case, trim, replacement maps, or field omission. | Defer until at least two real schemas need them; collect examples before expanding the transform grammar. |


## SpecScore Integration

- **New Features this would create:** likely a `record-key/derived-keys` Feature
  covering schema shape, resolver behavior, validation, and insert integration.
- **Existing Features affected:**
  - [`cli/insert`](../features/cli/insert/README.md) — `--key` becomes optional
    when a collection can derive the key from supplied fields; supplied `--key`
    must agree with the derived value.
  - [`record-format/list-of-records`](../features/record-format/list-of-records/README.md)
    — related identity work, but list-row key derivation stays out of scope.
- **Dependencies:** existing single-record parsing for YAML/Markdown,
  `record_file.name` `{key}` behavior, and shared CLI exposure of record identity
  as `$id`/key.

## Resolved Decisions

- `record_key.fields` should be a map keyed by field name.
- Missing derivation fields should fail schema validation when the corresponding
  columns are required; otherwise they should fail record validation when a record
  cannot produce its derived key.
- Slug normalization should use `github.com/gosimple/slug` for the MVP unless
  Feature-spec review finds a compatibility issue; no helper currently exists in
  `ingitdb-cli`, so wrap the dependency behind one shared helper instead of
  calling it directly from resolver branches.
- `insert --key` should be allowed for derived-key collections only when the
  supplied key matches the value generated from record fields.
- Derived keys should stay limited to single-record layouts in this Idea.
  List-of-records row identity is captured separately as a sidekick seed.

## Open Questions

None at this time.

## Sidekick Seeds Generated

- [explore-derived-keys-for-list-of-records-row-identity](seeds/explore-derived-keys-for-list-of-records-row-identity.md) — captured 2026-05-30 by specstudio:ideate

---
*This document follows the https://specscore.md/idea-specification*
