# Idea: Unified materialize command for collections and views

**Status:** Specified
**Date:** 2026-06-04
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** cli/materialize
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we let users regenerate any subset of inGitDB's derived artifacts — collection READMEs and materialized views — with one consistent, memorable command?

## Context

inGitDB derives two kinds of artifacts from source records: per-collection `README.md` files and materialized view files under each view's configured `$views/` directory. Today these are produced by two unrelated commands — `ingitdb materialize` (views only; its `--views` flag is a registered no-op) and `ingitdb docs update --collection=GLOB` (collection READMEs, via the `docsbuilder` package). There is no single command to regenerate "everything derived," and no way to regenerate one collection's README and one view in the same run.

This Idea supersedes the sidekick seed `materialize-subcommands-and-collection-targeting`, whose original framing ("implement `materialize collection` / `materialize views` subcommands, or reconcile the spec to the flat command") was overtaken during design discussion: subcommands were rejected outright. The feature spec at `spec/features/cli/materialize/README.md` has already been rewritten around the flat-flag design this Idea records.

## Recommended Direction

Make `ingitdb materialize` a single flat command that regenerates both artifact types, selected by two tri-state flags: `--collections` and `--views`. Each flag has three states — absent (don't touch that type), present without a value (every artifact of that type), or present with a value (only the named ones). Bare `ingitdb materialize` with no flags regenerates everything. The two flags compose freely, so `materialize --views=active_cities --collections=cities,teams` rebuilds exactly one view and two READMEs in one pass. Files are written only when content differs, keeping runs idempotent and diffs minimal.

Both flag values are a list of glob patterns, separated by comma (canonical) or semicolon — e.g. `--collections='agile.teams/**,cities'`. This carries over the glob power that `docs update --collection` already has, while matching the list shape we want for `--views`. Because each flag is tri-state with a bare-flag "all" meaning, list values must be attached with `=` (`--views=v1,v2`); the space-separated form is not supported. This is the standard pflag `NoOptDefVal` mechanism and will be documented in help text.

Collection-README generation moves into `materialize --collections`, and `docs update` is deprecated in favor of it — one command owns all derived-artifact regeneration. The redundant idea of a separate `materialize all` is dropped, since bare `materialize` already means all. Two named flags beat both the rejected positional-subcommand shape (which produced the absurd `materialize views --views=v1`) and a generic single `--target` flag (which would bury type and instance in one opaque value).

## Alternatives Considered

- **Positional subcommands** (`materialize collection` / `materialize views`, each with its own selector flag). Lost because the singular/plural pairing was inconsistent with the codebase's own convention (`list collections`, `drop view <name>`), and the subset form was redundant: `materialize views --views=v1` says "views" twice meaning two different things. With only two artifact types, the subcommand ceremony buys nothing.
- **Keep `materialize` and `docs update` as separate commands.** Lost because two commands regenerating overlapping derived files is exactly the seam that drifts — users would not know which to run, and CI would have to call both. Folding into one command is the simplification.
- **A generic single target flag** (`--target=views:v1,collections:c1`). Lost because it pushes a type-and-instance sub-grammar into a flag value, which is harder to read, remember, and tab-complete than two plainly named flags.

## MVP Scope

A single flat `ingitdb materialize` that: (1) regenerates all collection READMEs and all views when run bare; (2) honors `--collections` and `--views` in all three states, each accepting a comma/semicolon glob list with `=`-attached values; (3) writes only changed files; (4) routes README generation through the existing `docsbuilder` and views through the existing `materializer`, with `docs update` marked deprecated. Local working tree only. Done when the rewritten `spec/features/cli/materialize` acceptance criteria pass.

## Not Doing (and Why)

- Positional subcommands (materialize collection / materialize views) — rejected: inconsistent plurality and redundant invocations like 'materialize views --views=v1'
- A separate 'materialize all' subcommand — redundant: bare 'materialize' already means all
- --remote write-back in a single commit — deferred to a follow-up; MVP targets the local working tree only
- A generic single target flag (e.g. --target=views:v1) — rejected: a sub-grammar in the value is harder to read than two named flags

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | pflag's `NoOptDefVal` cleanly supports the absent / bare / `=value` tri-state per flag, and the `=`-only value syntax is acceptable to users | spike the flag wiring; confirm `--views`, `--views=a,b`, and omission are distinguishable, and `--views a,b` fails predictably |
| Should-be-true | `docsbuilder.UpdateDocs` (README generation) and `materializer.BuildViews` can be driven from one command without refactoring their cores | trace both call paths; confirm both accept a path + selection and report a `MaterializeResult` |
| Should-be-true | a glob list is the right targeting grammar for both flags (vs plain IDs) | confirm `docs update`'s existing glob semantics map onto `--collections` and read naturally for `--views` |
| Might-be-true | nobody depends on `docs update` as a stable scripted entry point that a deprecation would break | grep CI configs and docs for `docs update` usage |

## SpecScore Integration

- **New Features this would create:** none — folds into the existing `cli/materialize` feature
- **Existing Features affected:** `cli/materialize` (rewritten around this design); `docs update` (deprecated, folded in)
- **Dependencies:** `path-targeting` (the `--path` flag)

## Open Questions

- Should `docs update` be removed outright or kept as a thin deprecated alias that forwards to `materialize --collections` for one release? Settle at spec/plan time.
- Should `--remote` write-back (single commit to a remote repo, as `drop` supports) be a fast follow after the local MVP lands?
- When an invocation regenerates no views (e.g. `--collections` only), should `--records-delimiter` be silently ignored or warn? Lean toward the existing applicability-check pattern.

---
*This document follows the https://specscore.md/idea-specification*
