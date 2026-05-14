# Idea: CLI describe command

**Status:** Draft
**Date:** 2026-05-14
**Owner:** alexandertrakhimenok
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** extends:cli-sql-verbs

## Problem Statement

How might we surface a collection's or view's full definition from the CLI in
a way that round-trips on disk, fits the existing SQL-verb shape, and reserves
room for cross-engine DDL output via datatug?

## Context

ingitdb's CLI already exposes the SQL-verb family (`select`, `insert`, `update`,
`delete`, `drop`) plus `list collections` / `list views`. There is no command
to view a collection's or view's schema today — users have to open
`.collection/definition.yaml` (or the corresponding `$views/<name>.yaml`)
manually. This blocks two recurring tasks: quick "what columns does X have"
checks at the terminal, and scripted introspection (CI, code generators).

A related cross-engine introspection command (against sqlite/postgres/etc.) is
intentionally scoped to a different CLI (`datatug`). The shape decided here
should not paint datatug into a corner: the format-flag grammar in particular
should be reusable so a future `datatug describe table dbo.users` reads the
same way.

The `pkg/dalgo2ingitdb` package already has a library-level `DescribeCollection`
implementation (see `dalgo2ingitdb-dbschema-ddl-coverage`); this idea is the CLI
surface on top of that machinery.

## Recommended Direction

Add a top-level `describe` command (alias `desc`) that mirrors the existing
`drop` / `list` shape: kind + name, with `--path` for local and `--remote=…`
for GitHub-backed databases. Two kinds in v1: `collection <name>` (with `table`
as an alias for SQL muscle memory) and `view <name>` (with `--in=<collection>`
to disambiguate, identical to `drop view`). A bare-name shortcut
`describe <name>` resolves when unambiguous and errors loud when a collection
and view share the name.

Output is structured by default, controlled by `--format=yaml|json|native|sql|SQL`.
Default is `yaml`. `native` resolves to the engine's canonical format — for
ingitdb that is YAML; for sqlite (future datatug) it would be SQL `CREATE TABLE`.
`sql`/`SQL` are case-insensitive aliases that route to `native` and error on
ingitdb with `engine "ingitdb" native format is "yaml"; use --format=yaml or
--format=native`. This keeps a single shared flag grammar for ingitdb-cli and
datatug without forcing ingitdb to grow a SQL emitter it has no use for.

The output payload is a two-key YAML/JSON document: `definition:` holds the
on-disk file contents (round-trip-safe), `_meta:` holds enriched, derivable
information (id, kind, path, list of views and subcollections by name,
owning collection for views). The two are namespaced so a copy-paste of the
`definition:` block remains a valid on-disk file.

## Alternatives Considered

**Bare-name only (`ingitdb describe users`).** Terser, mimics kubectl's
default. Rejected because view and collection names can collide and the
existing `drop view` already requires `--in=` to disambiguate views across
collections — an inferred-kind verb would either inherit that pain silently
or guess wrong. The bare-name shortcut is kept as a convenience that fails
loud on ambiguity rather than a default.

**Subject-first grouping (`ingitdb collections describe users`).** Cleaner
namespace, but doubles keystrokes and breaks SQL muscle memory — the same
reason `cli-sql-verbs` rejected it for the read/write verbs. Adopting it just
for `describe` would create a one-off inconsistency.

**`definition` as the verb (`ingitdb definition users`).** Reads as a noun
next to a family of verb-first commands. Loses SQL ergonomics (`DESCRIBE` /
`DESC` is what users already type into MySQL/Oracle).

**Text-only human-readable default (`kubectl describe` style).** Considered,
but ingitdb's source-of-truth for schemas already _is_ YAML. Defaulting to
YAML makes copy-paste-to-file trivial and keeps the implementation
proportional to the value. A future `--format=text` can be added if a
genuine "scan this in the terminal" use case emerges.

**Emit real SQL `CREATE TABLE` from ingitdb.** Would require choosing a
dialect, lossy-translating record-file format, markdown bodies, subcollection
nesting, etc. Cross-engine DDL belongs to datatug; this command stays in its
lane and errors with a clear pointer when asked for SQL.

## MVP Scope

A single PR that lands:

1. `ingitdb describe collection <name>` (and `table` alias) emitting the
   `{definition, _meta}` shape in YAML.
2. `ingitdb describe view <name>` with `--in=<collection>` disambiguation,
   same shape.
3. Bare-name `ingitdb describe <name>` resolution with loud-on-ambiguity
   error.
4. `--format=yaml|json|native|sql|SQL` resolver living in one file
   (`describe_format.go`) reusable by datatug later.
5. `--path` / `--remote` / `--token` / `--provider` support, identical to
   `drop` and `list collections`.
6. Tests covering each kind, each format value, not-found, ambiguous view,
   ambiguous bare name, and the `--format=sql` error wording.

No subcollections, no `--format=text`, no real SQL emission, no
`describe database` / `describe settings`. Those are separate ideas if
demand surfaces.

## Not Doing (and Why)

- **Subcollections (`describe collection users/orders` or `users.orders`).**
  Out of scope until the addressing convention is settled — `drop` and
  `list` don't handle them either. Filed as a follow-up idea.
- **Real `--format=sql` emission for ingitdb.** Cross-engine DDL belongs to
  datatug; ingitdb has no SQL dialect of its own. We error with a helpful
  message instead of half-implementing it.
- **`describe database` / `describe settings`.** Useful but distinct from
  per-object describe; file separately when there is a concrete need.
- **`--columns` / `--no-meta` / output-filtering flags.** Wait for a real
  use case before growing the flag surface.
- **`--if-exists`-style idempotent missing.** `describe` is read-only;
  missing-target _is_ the answer, and a plain error is the right UX.

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | YAML is what users want to see when asking "what is this collection?" — not a text summary. | First two weeks post-merge: if there are issues asking for `--format=text` or `kubectl`-style output, revisit. |
| Must-be-true | The `--format=native`/`sql` flag grammar will be reusable by datatug without rework. | Cross-check the resolver design with the datatug roadmap before landing v1. |
| Should-be-true | `table` as an alias for `collection` is enough SQL ergonomics; no demand for `ingitdb describe table users` with `table` as the canonical kind. | Watch CLI usage telemetry / docs questions for 1–2 releases. |
| Might-be-true | The enriched `_meta` block (views/subcollections name lists) is worth the extra walk over `$views/` and the subcollections directory. | If users only ever pipe `definition:` into jq and ignore `_meta`, consider a `--no-meta` flag or moving `_meta` behind a verbosity flag. |

## SpecScore Integration

- **New Features this would create:** `cli/describe` — feature spec to be
  authored on promotion. Will reference `pkg/dalgo2ingitdb`'s
  `DescribeCollection` (from `dalgo2ingitdb-dbschema-ddl-coverage`).
- **Existing Features affected:** none directly. Conceptually adjacent to
  `cli/list` and `cli/drop`; the spec should cross-link those for shape and
  flag-grammar consistency.
- **Dependencies:** none beyond what `list collections` / `drop collection`
  already depend on (root-collections.yaml reader, remote GitHub reader).
- **Existing skeleton:** a partial implementation predates this idea and
  lives in the working tree at `cmd/ingitdb/commands/describe.go` +
  `describe_test.go`, registered in `cmd/ingitdb/main.go`. It implements
  only the simplest shape: `describe collection <name>` with hard-coded
  YAML output, no `--format` flag, no `table` alias, no bare-name
  resolution, no view describe. Treat it as a scaffold to reshape against
  this design, not a reference implementation. Specifically the feature
  spec will need to: add the `desc` alias and `table` kind alias; add
  `describe view <name>` with `--in=<collection>`; add bare-name
  resolution with loud-on-ambiguity error; replace the literal
  `yaml.Marshal(colDef)` with the `{definition, _meta}` payload builder;
  add the `--format` resolver and SQL-on-ingitdb error wording; wire
  `--remote` support.

## Open Questions

1. **Default for `_meta.path`** — should it be the relative path inside the
   DB (`users`, `users/$views/top_buyers.yaml`) or absolute? Relative is
   round-trippable across machines and matches forward-slash invariant
   already used on disk; absolute is debuggable. Lean: relative, always
   forward-slash, even on Windows. Confirm at feature-spec time.
2. **YAML key ordering inside `definition:`** — `yaml.Marshal` of a Go
   struct emits in field-declaration order, which is deterministic but
   differs from the alphabetical order `yaml.Marshal` uses for maps inside
   `columns:` (Go map iteration). Stable diff-able output would need
   `MarshalYAML` impls or post-processing. Decide whether to do it now or
   note the limitation and revisit if users complain.

---
*This document follows the https://specscore.md/idea-specification*
