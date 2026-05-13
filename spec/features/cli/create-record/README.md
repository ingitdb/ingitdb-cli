# Feature: Create Record Command

**Status:** Implementing

## Summary

The `ingitdb create record` command creates a new record in a collection. The record's
collection and key are taken from `--id`; its content can be supplied via `--data` (YAML/JSON
inline), via **stdin** (piped markdown or YAML file), or interactively via **`--edit`**
(opens `$EDITOR` with a schema-generated template). The command works against a local path or
a remote Git repository; remote writes require an authentication token.

## Problem

Users adding data to an inGitDB database should not have to hand-write the on-disk YAML/JSON
or Markdown in the exact location and shape the validator expects. A dedicated `create record`
command encapsulates the placement, encoding, and (for `--remote`) the commit creation in a single
invocation.

For **markdown-format collections**, the current `--data` flag only accepts structured YAML,
making it impossible to supply the Markdown body inline. Users need a natural way to pipe a
full `.md` file (frontmatter + body) into the command, matching the established heredoc and
pipe conventions of Unix CLIs.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb create record`. `--id` is always required.

### Flags

#### REQ: id-required

`--id=<collection-id>/<record-key>` MUST always be supplied.

#### REQ: data-flag

`--data=YAML` accepts inline YAML or JSON describing the record's fields
(e.g. `'{name: Ireland}'`). When `--data` is present it takes precedence over stdin and
`--edit`. `--data` is an **optional** flag — `MarkFlagRequired` on `--data` MUST be removed.
This is a deliberate, backward-compatible change: any invocation that previously worked
(always supplying `--data`) continues to work unchanged.

#### REQ: stdin-input

When `--data` is absent and stdin is not a terminal (i.e. stdin is a pipe or file redirect),
the command MUST read the full content of stdin and parse it via
`ParseRecordContentForCollection(content, colDef)` (from `pkg/dalgo2ingitdb/parse.go`).
That function handles all supported `SingleRecord` formats:

- `format: markdown` — `markdown.Parse()` for frontmatter; body bytes merged under
  `colDef.RecordFile.ResolvedContentField()` (default `$content`).
- `format: yaml` / `format: yml` / `format: json` — YAML/JSON unmarshal.
- `format: toml` — TOML unmarshal.

`format: ingr` does not support `SingleRecord` collections (enforced by schema validation),
so it is implicitly out of scope for this requirement.

For `format: markdown`, if the parsed frontmatter contains a key whose name equals the
resolved content-field name (default `$content` or the value of `content_field`), the
command MUST exit non-zero with a validation error. The content field is reserved for the
markdown body; allowing it in frontmatter would silently overwrite or shadow the body and
is rejected to prevent data corruption.

#### REQ: edit-flag

`--edit` opens `$EDITOR` (falling back to `vi` when the env var is unset) with a temporary
file pre-populated with a schema-derived template. `$EDITOR` is tokenized on whitespace so
common values like `code --wait` or `emacs -nw` work — the first token is the executable,
remaining tokens are flag arguments prepended before `tmpPath`. The editor MUST be invoked
via `exec.Command(prog, flags..., tmpPath)` — never via shell string interpolation — to
avoid shell injection.

Template content:

- For `format: markdown` collections: an opening `---` line, one `key: ` line (with an empty
  value) per column in `columns_order` order, columns present in `Columns` but absent from
  `columns_order` appended in alphabetical order, a closing `---` line, and a blank line for
  the body. When `columns_order` is empty, all columns appear in alphabetical order.
- For `format: yaml` / `format: json` / `format: yml` collections: a YAML skeleton with one
  `key: ` line per column in the same ordering as above.

On editor exit the command compares the saved file's bytes to the bytes written before
launching the editor. If they are identical, the command exits `0` and prints
`"no changes — record not created"`. If they differ, the file is parsed using the same rules
as `REQ: stdin-input` and the record is inserted.

#### REQ: tty-error

When `--data` is absent, `--edit` is absent, and stdin is a terminal, the command MUST exit
with code `1` and print a human-readable error to stderr, such as:

```
error: no record content provided — use --data, --edit, or pipe content via stdin
```

#### REQ: source-selection

`--path=PATH` and `--remote=HOST/OWNER/REPO[@REF]` MUST be mutually exclusive. When neither is
given the current working directory is used.

### Semantics

#### REQ: fails-if-exists

The command MUST fail when a record with the same key already exists in the target collection.
It MUST NOT silently overwrite existing data.

#### REQ: remote-write-requires-token

For `--remote` writes, an authentication token MUST be supplied via `--token` or a
host-derived environment variable (e.g. `GITHUB_TOKEN` for `github.com`). Each successful
create MUST result in exactly one commit in the remote repository (see
[remote-repo-access](../../remote-repo-access/README.md)).

## Data Flow

```
invoke create record
       │
       ├─ --data present ──────────────────────────────→ parse as YAML
       │
       ├─ --edit present ──→ open $EDITOR with template
       │                         │
       │                    file changed? ──no──→ exit 0, "no changes"
       │                         │ yes
       │                         ↓ parse saved file (same as stdin rules)
       │
       ├─ stdin is pipe/redirect ──→ read stdin
       │       └─ ParseRecordContentForCollection(stdin, colDef)
       │              ├─ markdown → markdown.Parse() + body → $content
       │              ├─ yaml/yml/json → yaml/json.Unmarshal()
       │              └─ toml → toml.Unmarshal()
       │
       └─ stdin is TTY, no --data, no --edit ──→ error + exit 1
                               │
                    (all paths converge here)
                               ↓
              validate + insert record → rebuild local views
```

## Dependencies

- [id-flag-format](../../id-flag-format/README.md)
- [path-targeting](../../path-targeting/README.md)
- [remote-repo-access](../../remote-repo-access/README.md)

## Acceptance Criteria

### AC: creates-local-record-via-data-flag

**Requirements:** cli/create-record#req:subcommand-name, cli/create-record#req:id-required,
cli/create-record#req:data-flag, cli/create-record#req:fails-if-exists

`ingitdb create record --id=countries/ie --data='{name: Ireland}'` writes a new record file
in the `countries` collection and exits `0`. Re-running the same command (without first
deleting the record) exits non-zero.

### AC: creates-markdown-record-via-stdin

**Requirements:** cli/create-record#req:id-required, cli/create-record#req:stdin-input

Given a collection with `format: markdown`, running:

```
printf -- '---\ntitle: Product 1\ncategory: software\n---\nBody here.\n' \
  | ingitdb create record --id=products/p1
```

writes a `.md` record file whose frontmatter contains `title: Product 1` and
`category: software`, and whose body is exactly `Body here.\n` verbatim. The command exits `0`.

### AC: creates-yaml-record-via-stdin

**Requirements:** cli/create-record#req:id-required, cli/create-record#req:stdin-input

Given a collection with `format: yaml`, running:

```
printf -- 'name: Ireland\n' | ingitdb create record --id=countries/ie
```

writes a record with field `name: Ireland` and exits `0`.

### AC: creates-toml-record-via-stdin

**Requirements:** cli/create-record#req:id-required, cli/create-record#req:stdin-input

Given a collection with `format: toml`, running:

```
printf -- 'name = "Ireland"\n' | ingitdb create record --id=countries/ie
```

writes a record with field `name: Ireland` and exits `0`.

### AC: markdown-rejects-content-field-in-frontmatter

**Requirements:** cli/create-record#req:stdin-input

Given a `format: markdown` collection (default `content_field` `$content`), piping
frontmatter that contains `$content: ...` MUST exit non-zero with an error message that
mentions the colliding key name. The same rule applies when `content_field` is overridden
(e.g. `content_field: body` → frontmatter `body: ...` is rejected).

### AC: tty-without-data-or-edit-errors

**Requirements:** cli/create-record#req:tty-error

When stdin is a terminal and neither `--data` nor `--edit` is supplied, the command exits
with code `1` and stderr contains the word `stdin` or `--edit` in the error message.

### AC: edit-flag-no-changes

**Requirements:** cli/create-record#req:edit-flag

Given a no-op editor script that exits without modifying the temp file
(e.g. `EDITOR='true'`), running `ingitdb create record --id=products/p1 --edit` exits `0`
and stdout or stderr contains `"no changes"`. No record file is written.

### AC: edit-flag-inserts-on-save

**Requirements:** cli/create-record#req:edit-flag

Given an editor script that appends `title: Product 1` to the temp file
(e.g. `EDITOR='sh -c "echo title: Product 1 >> $1"'`), running
`ingitdb create record --id=products/p1 --edit` against a `format: yaml` collection inserts a
record with `title: Product 1` and exits `0`.

### AC: creates-remote-record-with-token

**Requirements:** cli/create-record#req:source-selection,
cli/create-record#req:remote-write-requires-token

With `GITHUB_TOKEN` set, `ingitdb create record --remote=github.com/owner/repo
--id=countries/ie --data='{name: Ireland}'` creates one commit in `owner/repo` containing
the new record file. Without a token the command exits non-zero before any network request
that would require authentication.

## Rehearse Integration

New ACs for stdin, `--edit`, tty-error, and unsupported-format paths are all testable via
integration tests (CLI invocation with temp directories). The `--edit` ACs require a
controllable `EDITOR` environment variable. No Rehearse stubs are scaffolded because the
project uses Go integration tests (`*_test.go`) rather than Rehearse test files; coverage is
tracked in `go test`.

## Outstanding Questions

- This spec is scoped to `SingleRecord` (i.e. `map[string]any`) collections only. Support for
  `[]map[string]any` and `map[$record_id]map[$field_name]any` types is out of scope.
- `--edit` requires `--id` upfront. Prompting for ID after editing is deferred.

---
*This document follows the https://specscore.md/feature-specification*
