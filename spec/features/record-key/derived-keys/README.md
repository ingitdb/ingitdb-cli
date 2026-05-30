# Feature: Derived Record Keys

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-key/derived-keys?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-key/derived-keys?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-key/derived-keys?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-key/derived-keys?op=request-change) |
**Status:** Approved
**Date:** 2026-05-30
**Owner:** alexander.trakhimenok@gmail.com
**Source Idea:** [`derived-record-keys`](../../../ideas/derived-record-keys.md)
**Parent Feature:** [`record-key`](../README.md)
**Supersedes:** —
**Grade:** A

## Summary

Schema authors can declare that the canonical record key for a single-record
collection is computed from fields inside the record. inGitDB validates that the
filename key matches the parsed record content and lets single-record `insert`
derive the key from supplied data when `--key` is omitted.

## Problem

Single-record collections currently treat the filename key and the record fields
as separate facts. That works for arbitrary keys, but it is brittle for semantic
document and log collections where filenames are intentionally reproducible from
record content, such as `{timestamp}-{agent}-{category}-{project}.md`.

Without a schema-level derived-key contract, users must duplicate the same facts
in the filename and the record body, CI cannot detect drift mechanically, and
`insert` cannot create the correct path without asking the user to type the
derived key manually.

## Behavior

### Schema

#### REQ: derived-key-schema

Collections MAY declare a `record_key` block alongside `record_file`. The block
MUST contain a `template` string and a `fields` map keyed by record field name.
Every placeholder in `template` MUST be one of the field names declared in
`fields`, and every field named in `fields` MUST be a declared collection column.

Example:

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

#### REQ: single-record-only

`record_key` MUST be valid only for collections whose `record_file.type` is
`map[string]any`. `ListOfRecords` and `MapOfRecords` collections MUST reject
`record_key` during definition validation.

#### REQ: storage-template-unchanged

Derived keys MUST compute the canonical record key only. The storage path
continues to be produced by substituting that key into `record_file.name` through
the existing `{key}` placeholder; `record_file.name` MUST NOT interpolate record
fields directly.

### Key resolution

#### REQ: shared-derived-key-resolver

The system MUST provide one shared derived-key resolver used by validation,
single-record insert, and read/select identity surfaces. Given a collection
definition and parsed record data, the resolver MUST produce either one key
string or a structured error that names the missing/invalid field.

#### REQ: supported-transforms

The MVP transform vocabulary MUST support:

- raw string formatting for fields with no transform, accepting string values only;
- `type: datetime` with an explicit Go-style `output_format`, accepting parsed
  `time.Time` values and RFC3339/RFC3339Nano string values;
- `transform: slug`, implemented through one shared helper that wraps
  `github.com/gosimple/slug`, accepting string values only.

Unknown field options, unknown transforms, malformed datetime layouts, and
invalid runtime values for raw, datetime, or slug fields MUST be rejected with
diagnostics that name the field and the expected value kind.

#### REQ: missing-derivation-fields

When a record cannot provide a field needed by `record_key.template`, validation
MUST fail. If the field's column is required, the failure MAY be reported through
the existing required-column diagnostic; otherwise the derived-key resolver MUST
report that the record cannot produce its key because the field is missing.

### Validation

#### REQ: validate-derived-key-matches-filename

For each existing single-record file in a derived-key collection, validation MUST
parse the record content, derive the expected key, compare it with the actual
filename key, and fail when they differ. The diagnostic MUST name the record path,
actual key, expected key, and derivation fields used.

#### REQ: invalid-derived-key-config

Definition validation MUST reject malformed `record_key` configuration before any
record files are parsed. Invalid configuration includes an empty template,
placeholders with no matching field entry, field entries that do not correspond
to declared columns, use on non-single-record layouts, unsupported transforms,
and datetime fields without an `output_format`.

### Insert

#### REQ: insert-derives-key

In single-record mode, when `--key` is omitted and the target collection declares
`record_key`, `insert` MUST derive the effective key from the supplied record
data. The derived key participates in the existing strict insert behavior:
existing records with the same key MUST be rejected and no existing file may be
mutated.

#### REQ: insert-key-must-match-derived-key

In single-record mode, when both `--key` and derivable record fields are supplied
for a derived-key collection, `insert` MUST proceed only when `--key` equals the
derived key. A mismatch MUST be rejected before writing any file, with a
diagnostic that names the supplied key and derived key.

#### REQ: id-field-precedence-unchanged

For collections without `record_key`, the existing single-record key resolution
remains unchanged: `insert --key` wins when it matches `$id`, `$id` is used when
`--key` is omitted, and a missing effective key is rejected.

### Read and select identity

#### REQ: canonical-key-surface

Derived-key collections MUST continue exposing the canonical record key through
the existing record-key surface (`$id`/key). The system MUST NOT add a second
derived-key-specific pseudo-field.

## Acceptance Criteria

### AC: schema-accepts-field-map (verifies REQ:derived-key-schema)

**Given** a single-record collection with a `record_key` template and a `fields`
map keyed by declared column names
**When** the collection definition is validated
**Then** validation succeeds.

### AC: schema-rejects-placeholder-without-field (verifies REQ:derived-key-schema, REQ:invalid-derived-key-config)

**Given** a collection whose `record_key.template` contains `{agent}` but
`record_key.fields` has no `agent` entry
**When** the collection definition is validated
**Then** validation fails with a diagnostic naming the `agent` placeholder.

### AC: schema-rejects-list-layout (verifies REQ:single-record-only)

**Given** a collection with `record_file.type: "[]map[string]any"` and a
`record_key` block
**When** the collection definition is validated
**Then** validation fails with a diagnostic that derived keys require
`map[string]any`.

### AC: schema-rejects-empty-template (verifies REQ:derived-key-schema, REQ:invalid-derived-key-config)

**Given** a single-record collection with `record_key.template: ""`
**When** the collection definition is validated
**Then** validation fails with a diagnostic naming the empty `record_key.template`.

### AC: schema-rejects-field-without-column (verifies REQ:derived-key-schema, REQ:invalid-derived-key-config)

**Given** a collection whose `record_key.fields` map contains `agent`, but the
collection has no `agent` column
**When** the collection definition is validated
**Then** validation fails with a diagnostic naming the undeclared `agent` field.

### AC: schema-rejects-datetime-without-output-format (verifies REQ:supported-transforms, REQ:invalid-derived-key-config)

**Given** a collection whose `record_key.fields.timestamp.type` is `datetime`,
but the field has no `output_format`
**When** the collection definition is validated
**Then** validation fails with a diagnostic naming the missing
`timestamp.output_format`.

### AC: storage-template-keeps-key-placeholder (verifies REQ:storage-template-unchanged)

**Given** a derived-key collection with `record_file.name: "{key}.md"` and a
record whose fields derive `expected-key`
**When** the record path is resolved
**Then** the path is resolved by substituting `expected-key` for `{key}` in
`record_file.name`, not by interpolating record fields into `record_file.name`.

### AC: resolver-formats-motivating-key (verifies REQ:shared-derived-key-resolver, REQ:supported-transforms)

**Given** parsed record data with timestamp `2026-05-30T12:10:00Z`, agent
`Go Expert`, category `Deep Work`, and project `inGitDB CLI`
**When** the derived-key resolver applies template
`{timestamp}-{agent}-{category}-{project}`
**Then** it returns `2026-05-30T12-10-00-go-expert-deep-work-ingitdb-cli`.

### AC: schema-rejects-unsupported-transform (verifies REQ:supported-transforms, REQ:invalid-derived-key-config)

**Given** a derived-key collection whose `record_key.fields.agent.transform` is
`upper-snake`
**When** the collection definition is validated
**Then** validation fails with a diagnostic naming the unsupported `agent`
transform.

### AC: resolver-rejects-invalid-runtime-value (verifies REQ:shared-derived-key-resolver, REQ:supported-transforms)

**Given** parsed record data where `timestamp` is `not-a-date` and
`record_key.fields.timestamp.type` is `datetime`
**When** the derived-key resolver computes the key
**Then** it fails with a diagnostic naming `timestamp` and the expected datetime
value kind.

### AC: validation-detects-filename-drift (verifies REQ:validate-derived-key-matches-filename)

**Given** a derived-key Markdown record stored at
`$records/wrong-key.md` whose frontmatter derives
`2026-05-30T12-10-00-go-expert-deep-work-ingitdb-cli`
**When** validation runs
**Then** validation fails with a diagnostic naming the path, `wrong-key`, the
derived key, and the fields used to derive it.

### AC: validation-rejects-missing-optional-field (verifies REQ:missing-derivation-fields)

**Given** a derived-key collection whose template references optional column
`project`, and a record missing `project`
**When** validation runs
**Then** validation fails with a derived-key diagnostic naming the missing
`project` field.

### AC: insert-omitted-key-uses-derived-key (verifies REQ:insert-derives-key)

**Given** a derived-key Markdown collection and insert data containing every
field needed by `record_key.template`
**When** `ingitdb insert --into=agent-log --data=<record>` runs without `--key`
**Then** the created file path uses the key derived from the record fields.

### AC: insert-missing-derived-field-does-not-write (verifies REQ:insert-derives-key, REQ:missing-derivation-fields)

**Given** a derived-key collection and insert data missing a field required by
`record_key.template`
**When** `ingitdb insert --into=agent-log --data=<record>` runs without `--key`
**Then** the command fails before writing a file and reports the missing
derivation field.

### AC: insert-supplied-key-must-match (verifies REQ:insert-key-must-match-derived-key)

**Given** a derived-key collection and insert data whose fields derive key
`expected-key`
**When** `ingitdb insert --into=agent-log --key=wrong-key --data=<record>` runs
**Then** the command fails before writing a file and reports both `wrong-key` and
`expected-key`.

### AC: non-derived-insert-stays-compatible (verifies REQ:id-field-precedence-unchanged)

**Given** a collection without `record_key`
**When** single-record `insert` resolves its effective key
**Then** the existing `--key` and `$id` behavior is unchanged.

### AC: select-exposes-derived-key-as-id (verifies REQ:canonical-key-surface)

**Given** a valid derived-key collection with a record whose filename key matches
its derived key
**When** `ingitdb select --from=<collection> --fields=$id` runs
**Then** `$id` is the canonical record key from the filename and no additional
derived-key pseudo-field is required.

## Architecture

- Add a `RecordKeyDef`-style schema type under `pkg/ingitdb` and attach it to
  `CollectionDef` as `record_key`.
- Add a shared derived-key resolver close to the core schema/parsing layer so
  validators and write paths call the same logic.
- Wrap `github.com/gosimple/slug` in one in-repo helper before using the `slug`
  transform. Resolver branches and CLI code should depend on the wrapper, not on
  the third-party package directly.
- Integrate validation after record content is parsed and before a record is
  accepted as valid.
- Integrate single-record insert at the effective-key resolution step, after data
  parsing and before collision checks or file writes.

## Rehearse Integration

No Rehearse stubs are scaffolded yet. Every acceptance criterion above has a
clear Go unit, validator fixture, or CLI test surface; implementation should add
those tests directly in the existing Go test suites.

## Out of Scope

- Arbitrary expression languages or custom user code in key derivation.
- Automatic migration or renaming of existing files whose keys drift.
- `ListOfRecords` row identity changes; that follow-up is captured as
  [`explore-derived-keys-for-list-of-records-row-identity`](../../../ideas/seeds/explore-derived-keys-for-list-of-records-row-identity.md).
- Batch-mode `insert --format=...` derivation; this Feature covers
  single-record insert only.
- Direct record-field interpolation inside `record_file.name`.

## Assumption Carryover

| Idea assumption | Feature treatment |
|---|---|
| A derived key can be computed after Markdown/YAML parsing without changing single-record layout. | Carried into REQ:shared-derived-key-resolver, REQ:storage-template-unchanged, and validation ACs. |
| The transform vocabulary can stay small for the motivating case. | Carried into REQ:supported-transforms; only raw string formatting, datetime formatting, and slug are included. |
| Validation can report drift precisely enough for CI users to fix by hand. | Carried into REQ:validate-derived-key-matches-filename and AC:validation-detects-filename-drift. |
| `insert --key` can remain strict while allowing derived keys. | Carried into REQ:insert-derives-key and REQ:insert-key-must-match-derived-key. |
| More transforms may be wanted later. | Deferred until additional schemas justify expanding the vocabulary. |

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
