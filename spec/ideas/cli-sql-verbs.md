# Idea: SQL-Verb CLI Redesign

**Status:** Approved
**Date:** 2026-05-12
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we restructure the ingitdb CLI verb surface around SQL keywords (select, insert, update, delete, drop) so that anyone fluent in SQL can use the CLI without learning ingitdb-specific subcommand trees?

## Context

Today two commands (read record, query) overlap with SQL's SELECT for single-collection schema-aware access; delete is overloaded across records (DELETE) and schema objects (DROP); create record carries unique flag semantics that don't echo INSERT. The find verb is genuinely distinct — it searches across multiple collections that may have different schemas — and survives the redesign on its own merit. The pre-1.0 status of the CLI permits a hard rename without an aliasing tax. SpecScore feature specs in spec/features/cli/ already use SQL-flavored language (--from, --where) for the draft query command, so this idea formalizes a convention rather than inventing one.

## Recommended Direction

Adopt five top-level SQL verbs as the canonical CLI surface: select (read), insert (create), update (modify), delete (remove records), drop (remove schema objects). Use SQL's own prepositions for the 'which collection' flag: --from for select and delete, --into for insert. The update verb takes no collection flag at all — either --id=collection/key targets a single record, or --from=collection + --where targets a set; the collection is always derivable from one of those. drop uses positional kind+name subcommands (drop collection <name>, drop view <name>) so cobra tab-completion can enumerate kinds and existing object names from .ingitdb.yaml, matching SQL's DROP TABLE <name> grammar and scaling cleanly to future kinds (drop index, drop materialized-view) without mutually-exclusive flags.

Keep --id=collection/key for single-record operations across select/update/delete; --from + --where covers set operations. --where accepts two equality operators: == for loose comparison (current semantics, with type coercion where defined) and === for strict comparison (no coercion; types must match). --set uses single = for assignments (mirrors SQL's SET field = value), so update --set='active=true' --where='count==0' reads as SQL would. Set-scope delete and update with --from but no --where are refused; an explicit --all flag is required to operate on every record in a collection. Define --from, --into, --where, --order-by, --fields, --set, --id, --all once in a shared CLI flag spec rather than per-command.

Hard-rename: old verbs (read record, create record, query, delete record, delete records, delete collection, delete view) are removed in the same release; no aliases. The find verb is retained as a distinct cross-collection text-search command — it operates on multiple collections of potentially different schemas, which select (single-collection, schema-aware) cannot. Cross-format insert (input format differs from target collection format) is documented as a future enhancement, not in MVP.

## Alternatives Considered

- **Single `-c` / `--collection` flag for every verb.** Internally tidy and reuses the existing flag from `query`, but `select -c=countries` reads less like SQL than `select --from=countries`. Lost because the chosen audience is SQL-fluent users new to inGitDB — the SQL-grammar win is the whole point.
- **One mega-command: `ingitdb sql 'SELECT * FROM countries WHERE pop>1M'`.** Maximally SQL-pure, but loses shell-flag completion, shell history of individual flags, and scripted composition with shell variables. Lost on ergonomics.
- **Subject-first grouping: `ingitdb records select --from=…`, `ingitdb collections drop --name=…`.** Cleanly separates DML from DDL, but doubles keystrokes and fights SQL muscle memory. Lost for the chosen audience.
- **Aliases that keep old verbs working with deprecation warnings.** Considered for migration safety. Lost because the user chose a hard rename — pre-1.0 status makes the breakage cheap, and aliases would entrench the noise the redesign exists to remove.

## MVP Scope

Replace the existing verb surface with select, insert, update, delete, drop in one release. Each verb has a working implementation that the existing TUI and tests can consume. spec/features/shared-cli-flags/ exists and is referenced from each verb's feature spec. Cross-format insert is explicitly out. Done when: specscore lint passes; go test ./... passes; CLI help screens reflect the new verbs; the README quickstart uses only SQL verbs.

## Not Doing (and Why)

- Cross-format insert (JSON input into YAML/markdown collection) — separate Idea; adds parser-routing complexity orthogonal to verb naming
- Aliasing old verbs — hard rename is the chosen migration strategy; aliases would entrench the noise we are removing
- A SQL-string parser (e.g. ingitdb sql 'SELECT ...') — loses shell completion, history, and scripted flag composition; out of scope
- Joins, subqueries, GROUP BY — single-collection queries only; cross-collection is a separate Idea
- Renaming the TUI verb vocabulary — TUI changes can lag the CLI redesign; not in this Idea
- Merging find into select — find spans multiple collections with potentially different schemas; select is single-collection and schema-aware; the two have different contracts and stay separate
- Interactive confirmation prompt for delete --all — the explicit --all flag is the confirmation; git commit history provides revert; scripts and TTYs behave the same
- Stderr confirmation/summary messages for set-scope DML — operations are silent on success; git log is the audit trail and the message
- drop --keep-data / schema-only drops — drop is drop, mirroring real SQL DROP TABLE semantics; partial drops add cognitive load without a demonstrated need

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | SQL-fluent users find `select`/`insert`/`update`/`delete`/`drop` more discoverable than today's `read record`/`create record`/`query`/`find`/`delete collection` tree | Five-user think-aloud test: give each user a goal ("list every country with pop > 1M") with only `--help` to read; measure time-to-first-correct-command vs the current CLI |
| Must-be-true | No pre-1.0 production deployment is blocked by the hard rename | Audit usage: grep public repos and known internal users for `ingitdb read record`, `ingitdb query`, `ingitdb create record` invocations before merge |
| Should-be-true | Two preposition flag names (`--from`, `--into`) are remembered correctly by SQL-fluent users because each matches its SQL keyword's grammar | Same think-aloud test: measure flag-name errors per verb; tolerable threshold ≤ 1 error per user per session |
| Should-be-true | The existing `--id=collection/key` convention coexists cleanly with `--from`/`--where` set operations across `select`, `update`, `delete` | Spec lint + integration tests covering `--id` alone, `--from + --where` alone, and the error case where both are supplied |
| Might-be-true | The TUI can adopt the SQL verb vocabulary in a follow-up without re-architecting its command palette | Spike a single TUI screen using the new verbs after MVP CLI lands |


## SpecScore Integration

- **New Features this would create:**
  - `cli/select` (replaces `cli/read-record` and `cli/query`)
  - `cli/insert` (replaces `cli/create-record`)
  - `cli/update` (renames `cli/update-record`)
  - `cli/delete` (replaces `cli/delete-record` and `cli/delete-records`)
  - `cli/drop` (replaces `cli/delete-collection` and `cli/delete-view`)
  - `shared-cli-flags` — single source of truth for `--from`, `--into`, `--where`, `--order-by`, `--fields`, `--set`, `--id`, `--all`
- **Existing Features affected:**
  - [cli/read-record](../features/cli/read-record/README.md) — superseded by `cli/select`
  - [cli/create-record](../features/cli/create-record/README.md) — superseded by `cli/insert`
  - [cli/update-record](../features/cli/update-record/README.md) — renamed to `cli/update`
  - [cli/delete-record](../features/cli/delete-record/README.md) — folded into `cli/delete`
  - [cli/delete-records](../features/cli/delete-records/README.md) — folded into `cli/delete`
  - [cli/delete-collection](../features/cli/delete-collection/README.md) — superseded by `cli/drop`
  - [cli/query](../features/cli/query/README.md) — superseded by `cli/select`
  - [cli/find](../features/cli/find/README.md) — NOT affected; remains a distinct cross-collection text-search verb
  - [id-flag-format](../features/id-flag-format/README.md) — referenced from `shared-cli-flags`
- **Dependencies:**
  - [markdown-insert-ux](markdown-insert-ux.md) — its stdin/`--edit` behavior is inherited by the new `insert` verb; the two ideas must not contradict each other on input modes

## Open Questions

None at this time. (The `===` strict-comparison scope question was resolved during the `shared-cli-flags` feature spec: schema-declared date/time/timestamp columns trigger YAML-type-aware strict comparison; otherwise dates are compared as strings. See `shared-cli-flags#req:strict-equality-yaml-types`.)

---
*This document follows the https://specscore.md/idea-specification*
