# Feature: Insert

**Status:** Approved

## Summary

The `ingitdb insert` command creates a new record in a collection. The
target collection is given by `--into=<collection>`. The record key is
given by `--key=<value>` or by a `$id` field in the supplied data;
`--key` wins when both are present and equal, and they MUST match when
both are present. Data comes from `--data`, stdin, `--edit` (opens
`$EDITOR`), or `--empty` (record with the key only, no fields).
Insert is strict: if the record already exists, the command MUST fail.
`insert` replaces the prior `create record` command.

## Problem

`create record` today combines target-collection and record-key into a
single `--id=col/key` flag that does not echo SQL's `INSERT INTO <table>
… VALUES (…)` grammar. Markdown collections cannot supply a body via
`--data`; users have to fall back to writing the file by hand. The new
`insert` verb mirrors SQL, accepts every reasonable data source, and
makes the create-vs-update distinction explicit (insert NEVER mutates
an existing record).

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb insert`. It MUST accept the
shared flags from `shared-cli-flags` that apply: `--into`. It MUST
reject `--id`, `--from`, `--where`, `--order-by`, `--fields`, `--set`,
`--unset`, `--all`, and `--min-affected` (per `shared-cli-flags`
applicability rules).

#### REQ: into-required

`--into=<collection-id>` MUST be provided. The collection MUST exist
in `.ingitdb.yaml`; an unknown collection MUST be rejected with a
clear diagnostic.

### Record key

#### REQ: key-flag

The `--key=<value>` flag MUST take the record key as a single string.
Its value MUST conform to the record-key character rules of the target
collection's storage layout (no path separators, no leading/trailing
whitespace).

#### REQ: key-from-data-fallback

When `--key` is omitted, `insert` MUST attempt to read the record key
from a top-level `$id` field in the supplied data. If `$id` is present
and well-formed, its value MUST be used as the record key. This is
the write-side counterpart of `shared-cli-flags#req:pseudo-id-field`,
which defines `$id` as the read-side pseudo-field in `--where` and
`--fields`; both surfaces mean "the record key".

#### REQ: key-required

Exactly one effective key MUST be derivable from `--key`, from `$id` in
the data, or from both consistently. `insert` MUST be rejected when:

- `--key` is omitted AND the data has no `$id` field; OR
- both `--key` and `$id` in data are supplied but their values differ.

When both are supplied and equal, the operation MUST proceed.

#### REQ: id-field-not-stored

When `$id` appears in the supplied data, the resolved record key takes
its value from `$id`, and `$id` itself MUST NOT be stored as a data
field on the resulting record. The record key is the filesystem name;
duplicating it inside the file is not the convention.

### Data sources

#### REQ: data-source-modes

`insert` MUST accept exactly one of the following data sources per
invocation:

- `--data=<string>` — YAML or JSON literal
- `stdin` — used when stdin is not a TTY and no other source is given
- `--edit` — opens `$EDITOR` with a schema-derived template
- `--empty` — creates the record with only the key and no fields

Supplying two or more sources in the same invocation MUST be rejected.

#### REQ: no-data-error

When `--into` is provided but no data source is supplied (no `--data`,
stdin is a TTY, no `--edit`, no `--empty`), `insert` MUST be rejected
with a diagnostic that names the four available data sources.

#### REQ: data-format-parses-to-collection-format

The supplied data MUST be parsed into the target collection's record
shape:

- For a collection with `record_file.format: yaml`, `--data` and stdin
  MUST parse as YAML (which accepts JSON as a strict subset).
- For a collection with `record_file.format: json`, `--data` and stdin
  MUST parse as JSON.
- For `record_file.format: markdown`, `--data` and stdin MUST parse as
  YAML frontmatter (`---`-delimited) plus body, per the existing
  `pkg/ingitdb/markdown` parser; the body MUST be stored as the
  reserved `$content` field.
- Cross-format insert (e.g. JSON literal into a markdown collection) is
  OUT OF SCOPE for this feature.

#### REQ: edit-unchanged-template

When `--edit` is used and the user exits the editor without modifying
the schema-derived template (the saved content is byte-identical to
the template `insert` wrote into the temp file), `insert` MUST treat
this as a no-op: no record is created, and the command MUST exit
non-zero with a diagnostic explaining that the template was not
edited.

#### REQ: empty-flag-mutual-exclusion

`--empty` MUST be mutually exclusive with `--data`, stdin input, and
`--edit`. Supplying `--empty` together with any of them MUST be
rejected.

### Collision behavior

#### REQ: reject-existing-key

When the resolved record key already exists in the target collection,
`insert` MUST be rejected with a non-zero exit code and a diagnostic
that names the collection and key. The existing record MUST NOT be
mutated.

### Output and exit

#### REQ: success-output

On success, `insert` MUST exit `0`. `insert` MUST NOT write the
created record to stdout. Diagnostic and progress messages MUST go to
stderr.

### Source selection

#### REQ: source-selection

`insert` MUST accept either `--path=PATH` (local) or
`--github=OWNER/REPO[@REF]` (remote GitHub), but never both, per
[path-targeting](../../path-targeting/README.md) and
[github-direct-access](../../github-direct-access/README.md). When
neither is provided, the current working directory is used.

## Dependencies

- [shared-cli-flags](../../shared-cli-flags/README.md) — `--into`
  applicability, applicability rejections for unused flags.
- [path-targeting](../../path-targeting/README.md) — `--path` resolution.
- [github-direct-access](../../github-direct-access/README.md) —
  `--github` source.
- [markdown-insert-ux](../../../ideas/markdown-insert-ux.md) — the
  approved approach for stdin / `--edit` / markdown frontmatter
  inheritance is reused here; this feature does not contradict that
  Idea's MVP.

## Acceptance Criteria

### AC: key-via-flag

**Requirements:** cli/insert#req:subcommand-name, cli/insert#req:into-required, cli/insert#req:key-flag, cli/insert#req:data-source-modes, cli/insert#req:success-output

`ingitdb insert --into=countries --key=ie --data='{name: Ireland}'`
MUST create a record at `countries/ie` with `name: Ireland`, exit `0`,
and write nothing to stdout.

### AC: key-via-data-id

**Requirements:** cli/insert#req:key-from-data-fallback, cli/insert#req:id-field-not-stored

`ingitdb insert --into=countries --data='{$id: ie, name: Ireland}'`
(no `--key`) MUST create a record at `countries/ie` with `name:
Ireland`. The stored record MUST NOT contain a `$id` field.

### AC: key-conflict-rejected

**Requirements:** cli/insert#req:key-required

`ingitdb insert --into=countries --key=ie --data='{$id: us, name:
Ireland}'` MUST be rejected with a diagnostic that names both `ie` and
`us`. No record MUST be created.

### AC: key-required-but-missing

**Requirements:** cli/insert#req:key-required

`ingitdb insert --into=countries --data='{name: Ireland}'` (no
`--key`, no `$id` in data) MUST be rejected with a diagnostic that
recommends `--key` or a `$id` field.

### AC: key-matches-id-in-data

**Requirements:** cli/insert#req:key-required

`ingitdb insert --into=countries --key=ie --data='{$id: ie, name:
Ireland}'` MUST succeed (consistent values). The stored record MUST
NOT contain a `$id` field.

### AC: stdin-pipe

**Requirements:** cli/insert#req:data-source-modes

`echo '{name: Ireland}' | ingitdb insert --into=countries --key=ie`
MUST read the record content from stdin and create it. Stdin MUST be
treated as a data source only when it is not a TTY.

### AC: markdown-stdin

**Requirements:** cli/insert#req:data-format-parses-to-collection-format

Given a collection `posts` declared with `record_file.format:
markdown`,
`cat post.md | ingitdb insert --into=posts --key=hello` MUST parse
`---`-delimited frontmatter into structured fields and store the
remaining body under the `$content` field.

### AC: edit-mode

**Requirements:** cli/insert#req:data-source-modes, cli/insert#req:edit-unchanged-template

`ingitdb insert --into=countries --key=ie --edit` MUST open `$EDITOR`
with a schema-derived template, wait for the user to save and exit,
and then create the record from the saved content. If the user exits
without modifying the template, no record MUST be created and the
command MUST exit non-zero with an explanatory diagnostic.

### AC: empty-flag

**Requirements:** cli/insert#req:data-source-modes, cli/insert#req:empty-flag-mutual-exclusion

`ingitdb insert --into=countries --key=ie --empty` MUST create a
record at `countries/ie` with no data fields (only the key).
`ingitdb insert --into=countries --key=ie --empty --data='{name:
Ireland}'` MUST be rejected.
`ingitdb insert --into=countries --key=ie --empty` followed by piped
stdin MUST be rejected (mutual exclusion with stdin input).

### AC: no-data-source-rejected

**Requirements:** cli/insert#req:no-data-error

`ingitdb insert --into=countries --key=ie` from an interactive
terminal (TTY stdin, no `--data`, no `--edit`, no `--empty`) MUST be
rejected with a diagnostic that names all four data-source options.

### AC: existing-key-rejected

**Requirements:** cli/insert#req:reject-existing-key

Given a record already at `countries/ie`,
`ingitdb insert --into=countries --key=ie --data='{name: Ireland}'`
MUST exit non-zero with a diagnostic naming `countries/ie`. The
existing record MUST NOT be modified.

### AC: rejects-non-insert-flags

**Requirements:** cli/insert#req:subcommand-name

`ingitdb insert --into=countries --from=other --key=ie` MUST be
rejected (per `shared-cli-flags#req:from-flag`).
`ingitdb insert --into=countries --id=other/key` MUST be rejected
(per `shared-cli-flags#req:id-flag`).
`ingitdb insert --into=countries --key=ie --where='active===true'`
MUST be rejected.
`ingitdb insert --into=countries --key=ie --set='active=true'` MUST
be rejected (per `shared-cli-flags#req:set-flag-applies-to-both-modes`,
which scopes `--set` to `update` only).
`ingitdb insert --into=countries --key=ie --unset=active` MUST be
rejected (per `shared-cli-flags#req:unset-applicability`).
`ingitdb insert --into=countries --key=ie --all` MUST be rejected
(per `shared-cli-flags#req:all-flag`).
`ingitdb insert --into=countries --key=ie --data='{}' --order-by=name`
MUST be rejected (per `shared-cli-flags#req:order-by-applicability`).
`ingitdb insert --into=countries --key=ie --data='{}' --fields=name`
MUST be rejected (per `shared-cli-flags#req:fields-applicability`).
`ingitdb insert --into=countries --key=ie --data='{}' --min-affected=1`
MUST be rejected (per `shared-cli-flags#req:min-affected-applicability`).

### AC: unknown-collection-rejected

**Requirements:** cli/insert#req:into-required

`ingitdb insert --into=nonexistent --key=ie --data='{name: Ireland}'`
MUST be rejected with a diagnostic that names the unknown collection.

## Outstanding Questions

- Should `--key` accept a generated form (e.g. `--key=auto` for a UUID
  or slug from a `title` field)? Out of scope; defer until demand is
  visible.
- Should `--edit` template generation for markdown collections inherit
  exactly the convention established by
  [markdown-insert-ux](../../../ideas/markdown-insert-ux.md), including
  YAML frontmatter delimiters and ordered column placeholders? Yes,
  but the inheritance contract is owned by that Idea; this feature
  references it rather than restating.
- Cross-format insert (JSON literal into a markdown collection or vice
  versa) is OUT of MVP. A future Idea may revisit it.

---
*This document follows the https://specscore.md/feature-specification*
