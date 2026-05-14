# Feature: Insert

> [View in SpecStudio](https://specstudio.synchestra.io/project/features?id=ingitdb-cli@ingitdb@github.com&path=spec%2Ffeatures%2Fcli%2Finsert) — graph, discussions, approvals

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

When `--format=<jsonl|yaml|ingr|csv>` is supplied, `insert` switches to
**batch mode**: it reads a multi-record stream from stdin, derives the
key for each record from its own `$id` (or, for CSV, the resolved key
column), and inserts all records atomically inside a single
read-write transaction. The stream wire format is independent of the
target collection's on-disk storage format — any of the four batch
formats may persist into a collection whose `record_file.format` is
`yaml`, `json`, `markdown`, `ingr`, or `csv`.

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
applicability rules). Batch mode (`req:batch-format-flag`) carves out
one narrow exception: `--fields` is permitted, with new semantics,
when `--format=csv`; see `req:batch-csv-fields-flag`. `--key-column`
is a batch-CSV-only flag introduced by `req:batch-csv-key-resolution`
and is rejected outside batch CSV mode.

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

When `--into` is provided in **single-record mode** (i.e. `--format`
is absent) but no data source is supplied (no `--data`, stdin is a
TTY, no `--edit`, no `--empty`), `insert` MUST be rejected with a
diagnostic that names the four available data sources. In batch mode
the corresponding stdin requirement is governed by
`req:batch-stdin-required`.

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
`--remote=HOST/OWNER/REPO[@REF]` (remote Git repository), but never
both, per [path-targeting](../../path-targeting/README.md) and
[remote-repo-access](../../remote-repo-access/README.md). When
neither is provided, the current working directory is used.

### Batch mode

#### REQ: batch-format-flag

`insert` MUST accept `--format=<value>` where `<value>` is one of
`jsonl`, `yaml`, `ingr`, or `csv`. Supplying `--format` switches
`insert` into **batch mode**: stdin MUST be read as a multi-record
stream parsed with the selected parser. `--format` has no default;
when omitted, `insert` operates in single-record mode (the existing
behavior). Any other `--format` value MUST be rejected with a
diagnostic listing the four supported batch formats. Markdown is
explicitly NOT a supported stream format (it is supported on the
**storage** side; see `req:batch-storage-format-independence`).

#### REQ: batch-single-record-flag-exclusion

In batch mode (`--format` present), `--data`, `--edit`, `--empty`, and
`--key` MUST all be rejected. `--data`/`--edit`/`--empty` are
single-record data sources; the batch data source is always stdin.
`--key` is rejected because one CLI value cannot address many records;
each record carries its own key (`req:batch-per-record-key`).

#### REQ: batch-stdin-required

In batch mode, stdin MUST NOT be a TTY. When `--format` is supplied
and stdin is a TTY, `insert` MUST be rejected with a diagnostic
instructing the user to pipe input.

#### REQ: batch-per-record-key

In batch mode, every record in the stream MUST carry its own key.
For `jsonl`, `yaml`, and `ingr` formats the key MUST come from the
top-level `$id` field of the record. For `csv`, see
`req:batch-csv-key-resolution`. A record without a resolvable key
MUST cause the entire batch to be rejected, with a diagnostic that
names the offending record's position (line number for `jsonl`/`csv`,
document index for `yaml`/`ingr`). The key field MUST NOT be stored
as a data field on the resulting record, consistent with
`req:id-field-not-stored`.

#### REQ: batch-csv-key-resolution

For `--format=csv`, the record key MUST be resolved in this
precedence order:

1. If `--key-column=<name>` is supplied, the column named `<name>`
   (in the header row or in `--fields`) MUST be the key. The column
   MUST exist; if it does not, `insert` MUST be rejected before any
   records are read.
2. Else if a column literally named `$id` is present, it MUST be the
   key.
3. Else if a column literally named `id` is present, it MUST be the
   key (auto-mapped to the record key, equivalent to `$id`).
4. Otherwise, `insert` MUST be rejected with a diagnostic explaining
   that no key column was found and suggesting `--key-column`, `$id`,
   or `id`.

When both `$id` AND `id` columns are present in the same CSV (and
`--key-column` is not supplied), `$id` MUST win per the precedence
list above. The `id` column MUST then be treated as an ordinary data
field and stored on the record under the name `id`.

The resolved key column's value MUST NOT be stored as a data field on
the resulting record.

Record positions in batch diagnostics are 1-based: line 1 is the
first line of stdin (for `jsonl` and `csv`); document 1 is the first
document in the stream (for `yaml` and `ingr`). For `csv` with a
header row, line 2 is the first data record. When `--fields` is
supplied (no header), line 1 is the first data record.

#### REQ: batch-csv-fields-flag

For `--format=csv`, `insert` MUST accept an optional `--fields=<list>`
flag whose value is a comma-separated list of column names. When
`--fields` is supplied, it MUST override the CSV header row (or drive
parsing when the input has no header row); the first line of stdin is
treated as data, not as a header. `--fields` MUST be rejected when
`--format` is not `csv`.

#### REQ: batch-atomic

In batch mode, all parsed records MUST be inserted inside a single
read-write transaction. If any single record fails **before commit**
for any reason — parse error, missing key, schema violation, key
collision with an existing record, key collision with an earlier
record in the same batch, individual write failure, filesystem error
(disk full, permission denied), or commit failure itself — the entire
batch MUST be rolled back: no record from the batch MUST land in the
collection, and `insert` MUST exit non-zero with a diagnostic that
names the offending record's position (where applicable) and the
failure reason.

#### REQ: batch-post-commit-failure

If view materialization (`req:batch-view-materialization`) fails
**after** the transaction has committed, the inserted records MUST
remain on disk (rollback is no longer possible) and `insert` MUST
exit non-zero with a diagnostic stating that records were inserted
successfully but view materialization failed, and naming the failing
view. The diagnostic MUST be distinguishable from a pre-commit
failure so that scripts can tell the two cases apart.

#### REQ: batch-duplicate-keys-in-stream

If two records in the same batch share a resolved key, the batch
MUST be rejected per `req:batch-atomic`, with a diagnostic that names
both record positions and the conflicting key.

#### REQ: batch-empty-stream

An empty stream (zero records on stdin) in batch mode MUST be treated
as a successful no-op: `insert` MUST exit `0` and SHOULD write a
"0 records inserted" diagnostic to stderr.

#### REQ: batch-storage-format-independence

The `--format` flag describes only the stdin wire format. The on-disk
representation of each inserted record is governed by the target
collection's `record_file.format` setting. Any of the four batch
stream formats MUST correctly persist into a collection whose storage
format is `yaml`, `json`, `markdown`, `ingr`, or `csv`. In
particular, for a `record_file.format: markdown` collection, each
record in the batch MUST land as a `<key>.md` file with YAML
frontmatter built from the record's structured fields and a body
sourced from the reserved `$content` field (matching the
single-record behavior in `req:data-format-parses-to-collection-format`).
Records that do not supply `$content` for a markdown-stored
collection MUST persist with an empty body.

#### REQ: batch-view-materialization

In batch mode, local view materialization MUST run exactly once,
after the transaction commits. Per-record incremental materialization
is NOT permitted in batch mode.

## Dependencies

- [shared-cli-flags](../../shared-cli-flags/README.md) — `--into`
  applicability, applicability rejections for unused flags.
- [path-targeting](../../path-targeting/README.md) — `--path` resolution.
- [remote-repo-access](../../remote-repo-access/README.md) —
  `--remote` source.
- [markdown-insert-ux](../../../ideas/markdown-insert-ux.md) — the
  approved approach for stdin / `--edit` / markdown frontmatter
  inheritance is reused here; this feature does not contradict that
  Idea's MVP.
- [batch-insert](../../../ideas/batch-insert.md) — source Idea for
  the batch-mode requirements (`req:batch-*`). All four supported
  stream formats (`jsonl`, `yaml`, `ingr`, `csv`) and the
  atomic-transaction guarantee originate there.
- [record-format](../../record-format/README.md) — the per-collection
  `record_file.format` setting that batch mode reads through when
  persisting records (`req:batch-storage-format-independence`).

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/insert`):

- [`cmd/ingitdb/commands/insert.go`](../../../cmd/ingitdb/commands/insert.go)
- [`cmd/ingitdb/commands/insert_batch.go`](../../../cmd/ingitdb/commands/insert_batch.go)
- [`cmd/ingitdb/commands/insert_context.go`](../../../cmd/ingitdb/commands/insert_context.go)

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

### AC: batch-jsonl-basic

**Requirements:** cli/insert#req:batch-format-flag, cli/insert#req:batch-per-record-key, cli/insert#req:batch-atomic, cli/insert#req:success-output

Given a YAML-stored collection `countries`, when the user runs:

```
printf '{"$id":"ie","name":"Ireland"}\n{"$id":"fr","name":"France"}\n' \
  | ingitdb insert --into=countries --format=jsonl
```

`insert` MUST create `countries/ie` and `countries/fr` atomically,
exit `0`, and write nothing to stdout. Neither stored record MUST
contain a `$id` field.

### AC: batch-yaml-stream

**Requirements:** cli/insert#req:batch-format-flag, cli/insert#req:batch-per-record-key

Given a YAML-stored collection `countries`, when stdin is a YAML
multi-document stream (`---`-separated) of two records each carrying
`$id`, `ingitdb insert --into=countries --format=yaml` MUST insert
both records atomically.

### AC: batch-ingr-stream

**Requirements:** cli/insert#req:batch-format-flag, cli/insert#req:batch-per-record-key

Given a YAML-stored collection `countries`, when stdin is a
multi-record `ingr` stream of two records each carrying `$id`,
`ingitdb insert --into=countries --format=ingr` MUST insert both
records atomically.

### AC: batch-csv-id-column

**Requirements:** cli/insert#req:batch-csv-key-resolution

Given a YAML-stored collection `countries` and stdin:

```
$id,name,population
ie,Ireland,5
fr,France,68
```

`ingitdb insert --into=countries --format=csv` MUST create
`countries/ie` and `countries/fr` from the `name` and `population`
columns. The stored records MUST NOT contain a `$id` field.

### AC: batch-csv-id-auto-mapped

**Requirements:** cli/insert#req:batch-csv-key-resolution

Given stdin:

```
id,name
ie,Ireland
fr,France
```

(no `$id` column, just `id`), `ingitdb insert --into=countries
--format=csv` MUST use the `id` column as the record key
(auto-mapped). The stored records MUST NOT contain an `id` field.

### AC: batch-csv-key-column-override

**Requirements:** cli/insert#req:batch-csv-key-resolution

Given stdin:

```
external_id,name
ie,Ireland
fr,France
```

`ingitdb insert --into=countries --format=csv --key-column=external_id`
MUST use the `external_id` column as the record key. The stored
records MUST NOT contain an `external_id` field. If `--key-column`
names a column that is not in the header, `insert` MUST be rejected
before reading any records.

### AC: batch-csv-fields-no-header

**Requirements:** cli/insert#req:batch-csv-fields-flag, cli/insert#req:batch-csv-key-resolution

Given stdin with no header row:

```
ie,Ireland
fr,France
```

`ingitdb insert --into=countries --format=csv --fields='$id,name'`
MUST treat the first line as data and use the supplied field names
for column mapping. `$id` MUST resolve as the record key per
`req:batch-csv-key-resolution`.

### AC: batch-csv-fields-only-with-csv

**Requirements:** cli/insert#req:batch-csv-fields-flag

`ingitdb insert --into=countries --format=jsonl --fields='$id,name'`
MUST be rejected with a diagnostic stating that `--fields` is valid
only with `--format=csv`.

### AC: batch-single-record-flags-rejected

**Requirements:** cli/insert#req:batch-single-record-flag-exclusion

Each of the following MUST be rejected with a diagnostic naming the
offending single-record flag:

- `ingitdb insert --into=countries --format=jsonl --data='{$id: ie}'`
- `ingitdb insert --into=countries --format=jsonl --edit`
- `ingitdb insert --into=countries --format=jsonl --empty`
- `ingitdb insert --into=countries --format=jsonl --key=ie`

### AC: batch-stdin-tty-rejected

**Requirements:** cli/insert#req:batch-stdin-required

`ingitdb insert --into=countries --format=jsonl` invoked from an
interactive terminal (TTY stdin) MUST be rejected with a diagnostic
instructing the user to pipe input.

### AC: batch-missing-key-rejected

**Requirements:** cli/insert#req:batch-per-record-key, cli/insert#req:batch-atomic

Given stdin:

```
{"$id":"ie","name":"Ireland"}
{"name":"France"}
```

`ingitdb insert --into=countries --format=jsonl` MUST be rejected
with a diagnostic that names line 2 as missing `$id`. The
`countries/ie` record from line 1 MUST NOT exist on disk after the
command returns.

### AC: batch-duplicate-key-in-stream-rejected

**Requirements:** cli/insert#req:batch-duplicate-keys-in-stream, cli/insert#req:batch-atomic

Given stdin with two records sharing `$id: ie`,
`ingitdb insert --into=countries --format=jsonl` MUST be rejected
with a diagnostic that names both record positions and the
conflicting key. No record MUST be written.

### AC: batch-collision-with-existing-record

**Requirements:** cli/insert#req:reject-existing-key, cli/insert#req:batch-atomic

Given an existing record at `countries/ie`, when the user pipes:

```
{"$id":"fr","name":"France"}
{"$id":"ie","name":"Ireland"}
```

`ingitdb insert --into=countries --format=jsonl` MUST be rejected
with a diagnostic naming `countries/ie` and line 2. The
`countries/fr` record from line 1 MUST NOT exist on disk after the
command returns; the existing `countries/ie` MUST NOT be mutated.

### AC: batch-empty-stream-succeeds

**Requirements:** cli/insert#req:batch-empty-stream

`printf '' | ingitdb insert --into=countries --format=jsonl` MUST
exit `0`. A "0 records inserted" diagnostic MAY be written to
stderr. The collection MUST be unchanged.

### AC: batch-cross-format-jsonl-to-markdown

**Requirements:** cli/insert#req:batch-storage-format-independence

Given a collection `posts` declared with `record_file.format:
markdown`, when the user pipes:

```
{"$id":"hello","title":"Hello","$content":"# Hi\n\nFirst post."}
{"$id":"world","title":"World","$content":"Second post."}
```

`ingitdb insert --into=posts --format=jsonl` MUST create
`posts/hello.md` and `posts/world.md`, each with YAML frontmatter
containing `title:` (and any other structured fields) and a body
sourced from `$content`. Neither file MUST contain a `$id` or
`$content` field in its frontmatter.

### AC: batch-cross-format-csv-to-markdown

**Requirements:** cli/insert#req:batch-storage-format-independence, cli/insert#req:batch-csv-key-resolution

Given a `record_file.format: markdown` collection `posts` and stdin
(values are real newlines per RFC 4180 CSV quoting, NOT literal `\n`
escape sequences):

```
$id,title,$content
hello,Hello,First post body.
world,World,Second post body.
```

`ingitdb insert --into=posts --format=csv` MUST create
`posts/hello.md` and `posts/world.md` with frontmatter from the
`title` column and body from the `$content` column.

### AC: batch-cross-format-yaml-to-markdown

**Requirements:** cli/insert#req:batch-storage-format-independence

Given a `record_file.format: markdown` collection `posts` and a YAML
multi-document stream on stdin with two records each carrying `$id`,
`title`, and `$content`, `ingitdb insert --into=posts --format=yaml`
MUST create one `<key>.md` per document, each with YAML frontmatter
derived from the structured fields and a body sourced from
`$content`.

### AC: batch-cross-format-ingr-to-markdown

**Requirements:** cli/insert#req:batch-storage-format-independence

Given a `record_file.format: markdown` collection `posts` and a
multi-record `ingr` stream on stdin with two records each carrying
`$id`, `title`, and `$content`, `ingitdb insert --into=posts
--format=ingr` MUST create one `<key>.md` per record, each with YAML
frontmatter derived from the structured fields and a body sourced
from `$content`.

### AC: batch-csv-both-id-and-id-columns

**Requirements:** cli/insert#req:batch-csv-key-resolution

Given stdin:

```
$id,id,name
ie,IE-001,Ireland
fr,FR-002,France
```

`ingitdb insert --into=countries --format=csv` (no `--key-column`)
MUST use the `$id` column as the record key. The stored
`countries/ie` record MUST contain `id: IE-001` and `name: Ireland`
as data fields. The stored `countries/fr` record MUST contain
`id: FR-002` and `name: France`.

### AC: batch-post-commit-view-failure

**Requirements:** cli/insert#req:batch-post-commit-failure

Given a collection with at least one local materialized view whose
materialization is engineered to fail (e.g. the view targets a write
path that is read-only), batch-inserting records via
`ingitdb insert --into=countries --format=jsonl` MUST result in:

- the inserted records being present on disk (the transaction has
  already committed); and
- a non-zero exit code with a diagnostic that distinguishes the
  failure from a pre-commit rollback by stating that records were
  inserted but view materialization for `<view-name>` failed.

### AC: key-column-rejected-without-batch-csv

**Requirements:** cli/insert#req:subcommand-name, cli/insert#req:batch-csv-key-resolution

`ingitdb insert --into=countries --key=ie --data='{}' --key-column=external_id`
MUST be rejected (no `--format=csv`).
`ingitdb insert --into=countries --format=jsonl --key-column=external_id`
MUST be rejected (`--format` is not `csv`).

### AC: batch-invalid-format-value-rejected

**Requirements:** cli/insert#req:batch-format-flag

`ingitdb insert --into=countries --format=xml` MUST be rejected with
a diagnostic that lists the four supported values: `jsonl`, `yaml`,
`ingr`, `csv`. `ingitdb insert --into=posts --format=markdown` MUST
likewise be rejected.

### AC: batch-view-materialization-once

**Requirements:** cli/insert#req:batch-view-materialization

Given a collection with at least one local materialized view, batch
inserting N records via `--format=jsonl` MUST trigger view
materialization exactly once (after the transaction commits), not N
times.

## Outstanding Questions

### Resolved during this Feature spec

- **CSV `$id` column naming** (carried from `batch-insert` Idea):
  resolved as a three-tier precedence — `--key-column` overrides
  everything; otherwise a literal `$id` header column wins; otherwise
  a literal `id` header column auto-maps to the record key. See
  `req:batch-csv-key-resolution`.
- **Empty batch handling** (carried from `batch-insert` Idea):
  resolved in favour of "succeed silently with exit 0" over
  "reject as scripting bug." Rationale: upstream filters that yield
  zero rows are a legitimate pipeline outcome, and a non-zero exit
  for an empty batch would force every caller to special-case the
  zero-row condition. See `req:batch-empty-stream`. Revisit if real
  users hit silent-no-op surprise in practice.

### Deferred

- Should `--key` accept a generated form (e.g. `--key=auto` for a UUID
  or slug from a `title` field)? Out of scope; defer until demand is
  visible.
- Should `--edit` template generation for markdown collections inherit
  exactly the convention established by
  [markdown-insert-ux](../../../ideas/markdown-insert-ux.md), including
  YAML frontmatter delimiters and ordered column placeholders? Yes,
  but the inheritance contract is owned by that Idea; this feature
  references it rather than restating.
- Cross-format insert in **single-record** mode (JSON literal into a
  markdown collection or vice versa) is OUT of MVP. Cross-format
  insert in **batch** mode is IN scope (per
  `req:batch-storage-format-independence`); the single-record
  asymmetry is intentional for MVP and may be revisited.
- Is a max-batch-size guardrail desirable (e.g. reject batches >
  100k records to prevent OOM, overridable via `--max-records`)?
  Deferred from the `batch-insert` Idea; ship without a guardrail and
  revisit if users hit OOM in practice.
- Should `update` and `delete` learn analogous `--format=<…>` batch
  modes once `insert` ships? Deferred; out of scope for this Feature.

---
*This document follows the https://specscore.md/feature-specification*
