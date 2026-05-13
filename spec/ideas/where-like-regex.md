# Idea: LIKE/Regex Predicates in --where

**Status:** Draft
**Date:** 2026-05-12
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we add pattern-matching predicates (substring, glob, regex) to --where in a way that feels natural for SQL-fluent users without inventing a parser incompatible with select's comparison grammar?

## Context

The shared-cli-flags feature spec (req:comparison-operators) currently restricts --where to comparison-only operators (==, ===, !=, !==, >=, <=, >, <). The cli-sql-verbs Idea explicitly retains find as a distinct cross-collection text-search verb. This Idea asks the narrower question: should single-collection select also support pattern matching via --where, and if so, what operator syntax fits the existing grammar?

## Recommended Direction

Pending evaluation. Five candidate approaches are listed in Alternatives Considered; one will be promoted to Recommended Direction after Phase-2 stress-test.

## Alternatives Considered

Five candidate approaches, none yet chosen. Each is presented with its
operator surface, parser cost, and SQL-fluency score for the audience
locked in by `cli-sql-verbs`.

### A — SQL `LIKE` with `%` and `_` wildcards

`--where='name LIKE "Ire%"'`. Maximum SQL fluency; users know it from
every relational database. Parser cost is high: today's `--where` is
operator-driven with `strings.Index` (no whitespace tokenization);
`LIKE` introduces a keyword-style operator with quoted string operands.
Case-sensitivity is database-dependent in real SQL and would need an
explicit choice here (probably case-sensitive by default with `ILIKE`
for case-insensitive, mirroring Postgres).

### B — Glob operator `~`

`--where='name~"Ire*"'` with shell-style globs (`*`, `?`). Familiar
from shells; less SQL-pure. Single-character operator slots cleanly
into the existing multi-operator parser. Cheap to implement; cheap to
remember; loses some SQL signal.

### C — Regex operators `~=` (POSIX) and/or `=~` (Perl/Ruby)

`--where='name~=^Ire'` or `--where='name=~^Ire'`. Familiar from
sed/awk/Ruby. Multi-character operator that fits the existing parser
identically to `>=` and `<=`. Maximally expressive but easiest to
shoot yourself in the foot with. Single operator covers starts-with,
ends-with, contains, and arbitrary patterns.

### D — Distinct operators per predicate shape

`^=` starts-with, `$=` ends-with, `*=` contains, `~=` regex.
`--where='name^=Ire'` reads naturally for prefix searches without
needing regex. Parser-friendly; four operators to teach instead of
one. Most ergonomic for common cases; risk of operator-bloat
fatigue.

### E — Separate flags `--like` / `--match-regex`

`select --from=countries --like='name:Ire%' --match-regex='note:^x$'`.
Avoids touching the `--where` grammar entirely. Familiar to grep
users (`grep -E '…' file`). Breaks the "all predicates in one place"
mental model; the user must mentally compose ANDs across multiple
flag types instead of repeating `--where`.

## MVP Scope

Pending direction selection. The smallest sensible MVP is one new --where operator that covers the 80% case (substring or glob), with regex deferred if it lands later. Done when --where supports the chosen predicate(s) end-to-end, shared-cli-flags spec is updated, and one verb (select) exercises it.

## Not Doing (and Why)

- Full SQL LIKE escape grammar (ESCAPE clause) — defer; users rarely need it on small Git-backed datasets
- Replacing the find verb — find spans multiple collections; --where predicates are single-collection. They coexist.
- Backreferences and named capture groups in regex output — --where is a filter, not an extractor

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | A meaningful share of `select` users want pattern matching on a single collection (vs cross-collection via `find`) | Capture demand: count user requests / scripts using `find -c` post-cli-sql-verbs ship |
| Must-be-true | The chosen approach can be implemented without changing the existing `parseWhereExpr` strategy beyond adding an operator entry | Spike one operator (probably `~=`) into `cmd/ingitdb/commands/query_parser.go`; measure delta |
| Should-be-true | Go's standard `regexp` package (RE2) is enough — users don't need PCRE-only features (lookbehind, possessive quantifiers) | Survey: are there real ingitdb queries that need PCRE-specific features? Default no. |
| Should-be-true | Operator overload doesn't degrade discoverability — adding `~=` next to `>=` and `==` is intuitive | Think-aloud test: ask a user to write a "names starting with Ire" query given just `--help` |
| Might-be-true | Case-insensitive matching needs its own flag (`-i` or `--ignore-case`) rather than being baked into the operator (e.g. `~~=`) | Defer; revisit when a real script needs it |
| Might-be-true | Approach D (multiple per-shape operators) is worth the surface for the prefix/suffix common cases vs always-regex | Defer; ship the chosen MVP first |

## SpecScore Integration

- **New Features this would create:** none new; this Idea updates the existing `shared-cli-flags` feature with new requirements under "Operators in `--where`".
- **Existing Features affected:**
  - [shared-cli-flags](../features/shared-cli-flags/README.md) — `req:comparison-operators` would be extended to include the chosen pattern operator(s); a new REQ would define pattern semantics; an Outstanding Question in shared-cli-flags is resolved.
  - [cli/select](../features/cli/select/README.md) — gains AC examples exercising the new operator. (Spec does not yet exist; will be created from `cli-sql-verbs`.)
- **Dependencies:**
  - [cli-sql-verbs](cli-sql-verbs.md) — must be specified first; this Idea modifies its descendant `shared-cli-flags` feature.

## Outstanding Questions

- Which of the five approaches wins? Author leans toward C (single `~=` regex operator) for simplicity, optionally combined with D's `^=`/`$=`/`*=` for ergonomic shortcuts. User decides at promote-to-spec time.
- Case sensitivity default: case-sensitive (matches Go regex defaults, matches Postgres `LIKE`) or case-insensitive (matches MySQL default, matches casual user expectation)? Author leans case-sensitive with an explicit case-insensitive flag/modifier.
- Regex dialect: Go's RE2 (no lookaround, but linear-time guaranteed) is the obvious choice; document this so users don't expect PCRE features.
- Should pattern operators support `$id` (e.g. `--where='$id~=^us-'`)? Author leans yes for parity with comparison operators.

---
*This document follows the https://specscore.md/idea-specification*
