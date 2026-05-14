# Feature: Describe Command

**Status:** Approved

## Summary

The `ingitdb describe` command (alias `desc`) prints the full definition of a single
schema object — a `collection` (alias `table`) or a `view` — as a structured
document containing the on-disk YAML plus an enriched `_meta` block. It works
against a local directory (`--path`) or a remote Git repository (`--remote`) and
supports an output-format flag (`--format=yaml|json|native|sql|SQL`) whose grammar
is intentionally reusable by the future cross-engine `datatug describe` command.

## Problem

There is no CLI affordance today for inspecting a collection's or view's schema.
Users must open `.collection/definition.yaml` (or `<col>/$views/<name>.yaml`)
manually. This blocks: (a) terminal-speed "what columns does X have?" checks,
(b) scripted introspection in CI and code generators, and (c) operators on a
remote (`--remote=…`) repository who cannot just `cat` the file.

The library layer already exposes `DescribeCollection`
(see [`dalgo2ingitdb-dbschema-ddl-coverage`](../../dalgo2ingitdb-dbschema-ddl-coverage/README.md)); this
feature is the CLI surface on top of that machinery.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb describe <kind> <name>`. The verb `desc`
MUST be accepted as an alias of `describe` at the top level.

#### REQ: collection-kind

The kind `collection` MUST be supported: `ingitdb describe collection <name>`.

#### REQ: table-alias

The kind `table` MUST be accepted as an alias of `collection`:
`ingitdb describe table <name>` MUST produce identical output to
`ingitdb describe collection <name>`.

#### REQ: view-kind

The kind `view` MUST be supported: `ingitdb describe view <name>`. It MUST
accept the optional flag `--in=<collection>` to disambiguate when the same view
name exists in multiple collections.

#### REQ: bare-name-shortcut

`ingitdb describe <name>` (no kind) MUST resolve `<name>` against root
collections first and views second. If exactly one match exists, the command
MUST describe it. If a collection and a view share the same name, the command
MUST exit non-zero with an error of the form:
`name "<name>" is ambiguous — exists as both collection and view; use 'describe collection <name>' or 'describe view <name>'`.

### Flags

#### REQ: source-selection

`--path=PATH` and `--remote=HOST/OWNER/REPO[@REF]` MUST be mutually exclusive.
When neither is given the current working directory is used. `--token=PAT` and
`--provider=github|gitlab|bitbucket` MUST be accepted on the remote path with
the same semantics as `drop` and `list collections`.

#### REQ: format-flag

The flag `--format=VALUE` MUST accept the following values and MUST default to
`yaml` when omitted:

| Value    | Behavior on ingitdb                                                       |
|----------|----------------------------------------------------------------------------|
| `yaml`   | Emit the document as YAML.                                                 |
| `json`   | Emit the document as JSON.                                                 |
| `native` | Resolve to the engine's canonical format. For ingitdb this is `yaml`.      |
| `sql`    | Alias for `native`; for ingitdb errors (see REQ:sql-format-on-ingitdb).    |
| `SQL`    | Case-insensitive alias of `sql`.                                           |

Any value not in this table MUST cause exit non-zero with an error listing the
valid values.

#### REQ: sql-format-on-ingitdb

`--format=sql` (or `SQL`) against an ingitdb source MUST exit non-zero with the
error: `engine "ingitdb" native format is "yaml"; use --format=yaml or --format=native`.
The exact message is intentionally specific so the same resolver can be reused
by `datatug describe` against a SQL engine where `--format=sql` succeeds.

### Output

#### REQ: output-shape

When the command succeeds, stdout MUST contain exactly one document with two
top-level keys: `definition:` (the on-disk file content, round-trip-safe) and
`_meta:` (enriched derived information). The document MUST be encoded in the
format selected by `--format`. The command MUST exit `0` on success.

#### REQ: definition-faithfulness

The `definition:` block MUST be a faithful serialization of the on-disk
collection or view definition. For a collection, that means the same fields
that `yaml.Marshal` of `ingitdb.CollectionDef` produces today: `titles`,
`record_file`, `data_dir`, `columns`, `columns_order`, `primary_key`,
`default_view`, `readme`. For a view it MUST match `ingitdb.ViewDef`'s yaml
fields. Fields that the runtime never persists (`ID`, `DirPath`,
`SubCollections`, `Views`) MUST NOT appear under `definition:`.

#### REQ: meta-block

The `_meta:` block MUST contain at minimum the keys `id` (the object's
identifier) and `kind` (`collection` or `view`). All path values inside
`_meta` MUST use forward slashes regardless of host OS.

For a **collection**, `_meta:` MUST additionally contain:

- `definition_path` — location of the on-disk schema directory (the directory
  that would be edited to change the schema), relative to the database root.
- `data_path` — location where records are stored, relative to the database
  root. When the collection's `definition.data_dir` is unset, `data_path` MUST
  equal `definition_path`.
- `views` — alphabetically sorted list of view names declared under
  `<col>/$views/`. Empty list when no views exist.
- `subcollections` — alphabetically sorted list of subcollection directory
  names. Empty list when no subcollections exist.

For a **view**, `_meta:` MUST additionally contain:

- `definition_path` — the path to the view's `.yaml` file, relative to the
  database root.
- `collection` — the owning root collection's id.

#### REQ: columns-order-deterministic

Output under `definition.columns` MUST be ordered deterministically: when the
collection's `definition.columns_order` is non-empty, columns MUST appear in
that order; any columns not listed in `columns_order` MUST follow, sorted
alphabetically. When `columns_order` is empty, columns MUST appear sorted
alphabetically. This applies to all output formats.

### Errors

#### REQ: collection-not-found

If the named collection does not exist, the command MUST exit non-zero with an
error of the form: `collection "<name>" not found in database at <path-or-remote>`.

#### REQ: view-not-found

If the named view does not exist (after applying `--in` if given), the
command MUST exit non-zero with an error of the form:
`view "<name>" not found in any collection` (when `--in` is absent) or
`view "<name>" not found in collection "<in>"` (when `--in` is present).

#### REQ: ambiguous-view

If `--in` is absent and the view name matches in multiple collections, the
command MUST exit non-zero with an error of the form:
`view "<name>" is ambiguous — exists in collections: [<a>, <b>, …]; use --in=<collection>`.
The list of collections in the message MUST be sorted ascending.

#### REQ: in-collection-not-found

If `--in=<collection>` names a collection that does not exist, the command
MUST exit non-zero with an error of the form:
`collection "<in>" (from --in) not found`. This case takes precedence over
view existence checks.

## Out of Scope

- **Subcollections.** `describe collection <parent>/<child>` (or
  `<parent>.<child>`) is deferred until the addressing convention is agreed.
  v1 handles only root collections, mirroring `drop` and `list collections`.
- **Real SQL DDL emission.** ingitdb has no native SQL dialect; cross-engine
  DDL belongs to `datatug`. This feature errors clearly when asked for SQL.
- **Human-readable text format (`--format=text`).** Not added until a concrete
  use case surfaces; YAML default is readable enough at the terminal.
- **`describe database` / `describe settings`.** Separate ideas.
- **Output-filtering flags** (`--columns`, `--no-meta`, …). Wait for demand.
- **`--if-exists`-style idempotent-missing.** `describe` is read-only; a
  missing target is the answer and a plain error is the right UX.

## Dependencies

- path-targeting (shared with `list collections`, `drop`)
- remote-repo-access (shared with `list collections`, `drop`)
- `pkg/dalgo2ingitdb` — `DescribeCollection` (library-level provider) from
  feature `dalgo2ingitdb-dbschema-ddl-coverage`

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/describe`):

- `cmd/ingitdb/commands/describe.go` — parent command, `collection`/`view`
  subcommands, `table` alias on the collection subcommand, `desc` alias on
  the parent.
- `cmd/ingitdb/commands/describe_format.go` — single source of truth for
  `--format` value resolution and the SQL-on-ingitdb error wording.
  Pure function `resolveFormat(raw string) (canonical string, err error)`;
  reusable by datatug.
- `cmd/ingitdb/commands/describe_output.go` — pure builder that takes a
  `*CollectionDef` or `*ViewDef` plus owning-collection context and returns
  the `{definition, _meta}` payload as `map[string]any`. Format-agnostic.
- `cmd/ingitdb/main.go` — register `commands.Describe(homeDir, getWd, readDefinition)`.

A partial scaffold of `describe.go` + `describe_test.go` predating this spec
exists in the working tree. It implements only `describe collection <name>` in
YAML with no format flag, no `table` alias, no bare-name resolution, and no
view describe. Treat it as a starting skeleton to reshape against this spec —
not a reference implementation.

## Acceptance Criteria

### AC: describes-collection-yaml

**Requirements:** cli/describe#req:subcommand-name, cli/describe#req:collection-kind, cli/describe#req:format-flag, cli/describe#req:output-shape, cli/describe#req:definition-faithfulness, cli/describe#req:meta-block

**Given** a database whose root collections include `users` with two columns (`id`, `email`), no views, no subcollections, and no custom `data_dir`
**When** the user runs `ingitdb describe collection users`
**Then** stdout is a YAML document whose top-level keys are exactly `definition` and `_meta`; `definition.columns` contains `id` and `email`; `_meta.id == "users"`, `_meta.kind == "collection"`, `_meta.definition_path == "users"`, `_meta.data_path == "users"`, `_meta.views == []`, `_meta.subcollections == []`; exit code is `0`.

### AC: table-alias-equivalent

**Requirements:** cli/describe#req:table-alias

**Given** the same database as in AC:describes-collection-yaml
**When** the user runs `ingitdb describe table users`
**Then** stdout is byte-identical to the output of `ingitdb describe collection users` and exit code is `0`.

### AC: desc-alias-equivalent

**Requirements:** cli/describe#req:subcommand-name

**Given** the same database as in AC:describes-collection-yaml
**When** the user runs `ingitdb desc collection users`
**Then** stdout is byte-identical to the output of `ingitdb describe collection users` and exit code is `0`.

### AC: describes-view-with-meta

**Requirements:** cli/describe#req:view-kind, cli/describe#req:output-shape, cli/describe#req:meta-block

**Given** a database with collection `users` containing a single view `top_buyers` at `users/$views/top_buyers.yaml` whose definition declares `order_by: total_spend DESC` and `top: 100`
**When** the user runs `ingitdb describe view top_buyers`
**Then** stdout's `definition.order_by` equals `total_spend DESC`, `definition.top` equals `100`, `_meta.id == "top_buyers"`, `_meta.kind == "view"`, `_meta.collection == "users"`, `_meta.definition_path == "users/$views/top_buyers.yaml"`; exit code is `0`.

### AC: ambiguous-view-requires-in

**Requirements:** cli/describe#req:ambiguous-view

**Given** a database where two collections `users` and `orders` each declare a view named `recent`
**When** the user runs `ingitdb describe view recent`
**Then** the command exits non-zero and stderr contains `view "recent" is ambiguous — exists in collections: [orders, users]; use --in=<collection>`.

### AC: in-flag-collection-missing

**Requirements:** cli/describe#req:in-collection-not-found

**Given** a database whose root collections do not include `ghosts`
**When** the user runs `ingitdb describe view anything --in=ghosts`
**Then** the command exits non-zero and stderr contains `collection "ghosts" (from --in) not found`.

### AC: view-resolved-by-in-flag

**Requirements:** cli/describe#req:view-kind

**Given** the same database as in AC:ambiguous-view-requires-in
**When** the user runs `ingitdb describe view recent --in=orders`
**Then** stdout's `_meta.collection == "orders"` and exit code is `0`.

### AC: bare-name-collection

**Requirements:** cli/describe#req:bare-name-shortcut

**Given** the same database as in AC:describes-collection-yaml
**When** the user runs `ingitdb describe users`
**Then** stdout is byte-identical to the output of `ingitdb describe collection users` and exit code is `0`.

### AC: bare-name-ambiguous

**Requirements:** cli/describe#req:bare-name-shortcut

**Given** a database that has both a collection named `archive` and a view named `archive` (in some collection)
**When** the user runs `ingitdb describe archive`
**Then** the command exits non-zero and stderr contains `name "archive" is ambiguous — exists as both collection and view; use 'describe collection archive' or 'describe view archive'`.

### AC: format-yaml-and-json-equivalent-shape

**Requirements:** cli/describe#req:format-flag, cli/describe#req:output-shape

**Given** the same database as in AC:describes-collection-yaml
**When** the user runs `ingitdb describe collection users --format=json`
**Then** stdout parses as valid JSON; the parsed value has exactly the same top-level keys (`definition`, `_meta`) and the same nested key set as the YAML form; exit code is `0`.

### AC: format-native-resolves-to-yaml-on-ingitdb

**Requirements:** cli/describe#req:format-flag

**Given** the same database as in AC:describes-collection-yaml
**When** the user runs `ingitdb describe collection users --format=native`
**Then** stdout is byte-identical to the output of `ingitdb describe collection users --format=yaml` and exit code is `0`.

### AC: format-sql-errors-on-ingitdb

**Requirements:** cli/describe#req:sql-format-on-ingitdb

**Given** the same database as in AC:describes-collection-yaml
**When** the user runs `ingitdb describe collection users --format=sql` (or `--format=SQL`)
**Then** the command exits non-zero, stderr contains `engine "ingitdb" native format is "yaml"; use --format=yaml or --format=native`, and stdout is empty.

### AC: unknown-format-rejected

**Requirements:** cli/describe#req:format-flag

**Given** the same database as in AC:describes-collection-yaml
**When** the user runs `ingitdb describe collection users --format=xml`
**Then** the command exits non-zero and stderr lists the valid `--format` values.

### AC: collection-not-found-error

**Requirements:** cli/describe#req:collection-not-found

**Given** a database whose root collections do not include `widgets`
**When** the user runs `ingitdb describe collection widgets --path=/tmp/db`
**Then** the command exits non-zero and stderr contains `collection "widgets" not found in database at /tmp/db`.

### AC: view-not-found-error

**Requirements:** cli/describe#req:view-not-found

**Given** a database with a collection `users` and no view anywhere named `ghost`
**When** the user runs `ingitdb describe view ghost`
**Then** the command exits non-zero and stderr contains `view "ghost" not found in any collection`.

### AC: data-dir-divergence-surfaces-both-paths

**Requirements:** cli/describe#req:meta-block

**Given** a collection `events` whose `definition.yaml` sets `data_dir: ../events-archive`
**When** the user runs `ingitdb describe collection events`
**Then** `_meta.definition_path == "events"` and `_meta.data_path == "events-archive"`; exit code is `0`.

### AC: columns-order-respected

**Requirements:** cli/describe#req:columns-order-deterministic

**Given** a collection `users` whose `definition.columns_order` is `[email, id, name]` and whose `columns` map contains exactly those three keys
**When** the user runs `ingitdb describe collection users --format=yaml`
**Then** `definition.columns` keys in the YAML output appear in the order `email`, `id`, `name`; running the same command twice produces byte-identical output.

### AC: columns-order-fallback-alphabetical

**Requirements:** cli/describe#req:columns-order-deterministic

**Given** a collection `users` whose `definition.columns_order` is empty and whose `columns` map contains `id`, `email`, `name`
**When** the user runs `ingitdb describe collection users --format=yaml`
**Then** `definition.columns` keys in the YAML output appear in alphabetical order (`email`, `id`, `name`); running the same command twice produces byte-identical output.

### AC: source-selection-mutual-exclusion

**Requirements:** cli/describe#req:source-selection

**Given** the user invokes the command with both `--path` and `--remote`
**When** any subcommand of `describe` runs
**Then** the command exits non-zero and stderr contains `--path and --remote are mutually exclusive`.

## Outstanding Questions

None at this time. The originating Idea's open questions on column ordering
and `data_dir` divergence are answered by REQ:columns-order-deterministic and
REQ:meta-block respectively.

---
*This document follows the https://specscore.md/feature-specification*
