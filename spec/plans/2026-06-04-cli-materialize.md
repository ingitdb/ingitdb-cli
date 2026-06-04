# cli/materialize — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Each task ends with a build + `go test ./...` + `golangci-lint run` gate and a commit whose message cites the satisfied AC IDs.

**Goal:** Reshape `ingitdb materialize` into a single flat command that regenerates both derived-artifact types — collection `README.md` files and materialized views — selected by two tri-state flags `--collections` and `--views`. Bare `materialize` regenerates everything. Each flag is absent / bare (`=all`) / `=<glob-list>` (comma- or semicolon-separated). Collection-README generation folds in from `docs update`, which becomes deprecated. Local working tree only.

**Architecture:** `cmd/ingitdb/commands/materialize.go` gets its own flag set and a new run function that (a) resolves each flag's tri-state into a selection, (b) routes collection selections through `pkg/ingitdb/docsbuilder` (the same engine `docs update` uses) and view selections through `pkg/ingitdb/materializer`, and (c) aggregates one `ingitdb.MaterializeResult` summary to stderr. Tri-state is implemented with pflag `NoOptDefVal` (a sentinel meaning "all"), which forces `=`-attached list values. Glob lists are split on `,`/`;`; collection patterns reuse `docsbuilder.ResolveCollections` (`**`, `path/*`, `path/**`, exact); view patterns match against view names.

**Decoupling note:** `addMaterializeFlags` and `materializeRunE` are currently **shared with the `ci` command** (`ci.go`). This plan introduces a materialize-specific flag set and run function and leaves `ci` on the existing views-only path untouched, so CI behavior does not change.

**Tech Stack:** Go stdlib (`os`, `path/filepath`, `strings`), `github.com/spf13/cobra` / `pflag`, the project's `pkg/ingitdb`, `pkg/ingitdb/materializer`, and `pkg/ingitdb/docsbuilder` packages.

**Spec:** `spec/features/cli/materialize/README.md` (Source Idea: `spec/ideas/materialize-collections-and-views.md`)

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Modify | `cmd/ingitdb/commands/materialize.go` | Own flag set; new run func with tri-state selection + dual dispatch (docsbuilder + materializer) |
| Create | `cmd/ingitdb/commands/materialize_selection.go` | Pure helpers: flag-state resolution, `,`/`;` list split, view-name glob match |
| Create | `cmd/ingitdb/commands/materialize_selection_test.go` | Unit tests for the pure helpers |
| Modify | `cmd/ingitdb/commands/materialize_test.go` | Integration tests for each AC |
| Modify | `cmd/ingitdb/commands/flags.go` | Add `addMaterializeCommandFlags` (materialize-only); leave `addMaterializeFlags` for `ci` |
| Modify | `cmd/ingitdb/commands/docs_update.go` | Mark `docs update` deprecated, point to `materialize --collections` |
| Modify | `cmd/ingitdb/main.go` | Pass the docs/records reader into `Materialize` so it can drive `docsbuilder` |

**Reused (no edits):** `pkg/ingitdb/docsbuilder/update.go` (`UpdateDocs`, `ResolveCollections`), `pkg/ingitdb/materializer/view_builder.go` (`BuildViews`), `cobra_helpers.go` (`resolveDBPath`).

**Untouched:** `ci.go` (stays on the existing views-only `materializeRunE`), all other commands, the dalgo packages.

---

## Task 1 — Materialize-specific tri-state flags

**Context:** Give `materialize` its own flags without disturbing `ci`. Register `--collections` and `--views` as `String` flags, then set each flag's `NoOptDefVal` to a sentinel (e.g. `"\x00all"`) so the bare flag means "all" and list values require `=`. Keep `--path` and `--records-delimiter`. Help text documents the `=` requirement.

**Files:** `flags.go`, `materialize.go`, `materialize_test.go`

- [ ] **Step 1.1 — Failing test:** assert `materialize` registers `collections`, `views`, `path`, `records-delimiter`; assert each selector flag's `NoOptDefVal` is the all-sentinel; assert `--views=a,b` yields the value `a,b` while bare `--views` yields the sentinel and `--views a,b` leaves `a,b` as a positional arg.
- [ ] **Step 1.2 — Confirm failure** (`go test -run TestMaterialize_Flags ./cmd/ingitdb/commands/`).
- [ ] **Step 1.3 — Implement** `addMaterializeCommandFlags(cmd)` in `flags.go`; switch `Materialize(...)` in `materialize.go` to use it and set `NoOptDefVal` on both selector flags. Leave `addMaterializeFlags` (used by `ci`) unchanged.
- [ ] **Step 1.4 — Build + test + lint + commit.**

**Verifies:** cli/materialize#ac:equals-syntax-required

---

## Task 2 — Selection helpers (pure functions)

**Context:** Isolate the decision logic so it is unit-testable without cobra. Implement: `flagSelection(changed bool, raw string) selection` returning one of `{none, all, list([]string)}` based on absent / sentinel / value; `splitPatterns(raw string) []string` splitting on `,` and `;` and trimming; `matchViewNames(names []string, patterns []string) []string` for view-name glob matching. Collection patterns are delegated to `docsbuilder.ResolveCollections` and need no new matcher.

**Files:** `materialize_selection.go`, `materialize_selection_test.go`

- [ ] **Step 2.1 — Failing tests:** table-driven cases for each selection state; `splitPatterns("a, b;c")` → `[a b c]`; view-name matching for exact, `*`, and multi-pattern inputs.
- [ ] **Step 2.2 — Confirm failure.**
- [ ] **Step 2.3 — Implement** the three pure helpers.
- [ ] **Step 2.4 — Build + test + lint + commit.**

**Verifies:** cli/materialize#ac:views-subset, cli/materialize#ac:collections-glob-targeting

---

## Task 3 — Collections README path (fold in docsbuilder)

**Context:** When the collections selection is `all`, regenerate every collection README; when it is a glob list, call `docsbuilder.UpdateDocs` once per pattern (`all` maps to the `**` pattern) and merge the `MaterializeResult`s. `main.go` must hand `Materialize` a `RecordsReader` (use `materializer.NewFileRecordsReader()`), matching what `docs update` passes today.

**Files:** `materialize.go`, `main.go`, `materialize_test.go`

- [ ] **Step 3.1 — Failing test:** `materialize --collections` regenerates all collection READMEs and writes no view file; `--collections='agile.teams/**'` regenerates nested collections; `--collections='cities;teams'` targets exactly those two.
- [ ] **Step 3.2 — Confirm failure.**
- [ ] **Step 3.3 — Implement** the collections branch calling `docsbuilder.UpdateDocs` per resolved pattern; wire the reader through `main.go`.
- [ ] **Step 3.4 — Build + test + lint + commit.**

**Verifies:** cli/materialize#ac:collections-only-all, cli/materialize#ac:collections-glob-targeting

---

## Task 4 — Views path with subset filtering

**Context:** When the views selection is `all`, keep today's behavior (`BuildViews` per collection). When it is a glob list, build only views whose names match (filter via `matchViewNames` from Task 2, building per matched view). `--records-delimiter` applies only on this path (set `def.RuntimeOverrides.RecordsDelimiter` as today); when no view is regenerated it has no effect.

**Files:** `materialize.go`, `materialize_test.go`

- [ ] **Step 4.1 — Failing test:** `materialize --views` rebuilds all views and no README; `--views=active_cities` rebuilds only that view; `--views --records-delimiter=-1` disables the `#-` delimiter in INGR output; `--collections --records-delimiter=-1` behaves identically to `--collections` alone.
- [ ] **Step 4.2 — Confirm failure.**
- [ ] **Step 4.3 — Implement** the views branch with name-glob filtering and records-delimiter applicability.
- [ ] **Step 4.4 — Build + test + lint + commit.**

**Verifies:** cli/materialize#ac:views-only-all, cli/materialize#ac:views-subset, cli/materialize#ac:records-delimiter-applies-to-views

---

## Task 5 — Unified dispatch, defaults, and summary

**Context:** Tie the branches together. Bare `materialize` (both selections `none`) defaults to `all` for both types. Combined flags run both branches. Aggregate a single `ingitdb.MaterializeResult`, print a `created/updated/deleted/unchanged` summary to **stderr**, write nothing to stdout, exit `0`. Write-only-on-change is already provided by both engines; a second run reports all-unchanged.

**Files:** `materialize.go`, `materialize_test.go`

- [ ] **Step 5.1 — Failing test:** bare `materialize` regenerates all READMEs + all views, summary on stderr, empty stdout, exit 0; `--views=v1 --collections=c1,c2` touches only those; running twice writes zero files the second time.
- [ ] **Step 5.2 — Confirm failure.**
- [ ] **Step 5.3 — Implement** the default-to-all logic, dual-branch invocation, result aggregation, and stderr summary.
- [ ] **Step 5.4 — Build + test + lint + commit.**

**Verifies:** cli/materialize#ac:bare-materializes-everything, cli/materialize#ac:combined-subset, cli/materialize#ac:idempotent-second-run

---

## Task 6 — Deprecate `docs update`

**Context:** Set the `docs update` cobra command's `Deprecated` field (and/or a help note) directing users to `ingitdb materialize --collections`. Confirm output parity: `materialize --collections=GLOB` produces the same README bytes that `docs update --collection=GLOB` produced (both call `docsbuilder` with the same glob). Add a short hint guarding the `--collections` (plural) vs the old `--collection` (singular) name confusion (reviewer advisory).

**Files:** `docs_update.go`, `materialize_test.go`

- [ ] **Step 6.1 — Failing test:** `docs update --help` shows a deprecation notice naming `materialize --collections`; a parity test asserts identical README output between the two commands for one glob.
- [ ] **Step 6.2 — Confirm failure.**
- [ ] **Step 6.3 — Implement** the deprecation notice.
- [ ] **Step 6.4 — Build + test + lint + commit.**

**Verifies:** cli/materialize#ac:docs-update-deprecated

---

## Self-Review

**AC coverage.** Every AC in `spec/features/cli/materialize/README.md` is covered by a task:

| AC | Task |
|----|------|
| `equals-syntax-required` | 1 |
| `views-subset` | 2, 4 |
| `collections-glob-targeting` | 2, 3 |
| `collections-only-all` | 3 |
| `views-only-all` | 4 |
| `records-delimiter-applies-to-views` | 4 |
| `bare-materializes-everything` | 5 |
| `combined-subset` | 5 |
| `idempotent-second-run` | 5 |
| `docs-update-deprecated` | 6 |

No ACs deferred.

**Order/dependencies.** 1 (flags) → 2 (pure helpers) → 3 (collections) and 4 (views, uses Task 2's matcher) → 5 (dispatch, needs 3 + 4) → 6 (deprecation). Linear, no gaps; each task depends only on earlier ones.

**Known risks.** (1) View-name subset filtering has no existing materializer entry point that takes a name filter — Task 4 must build per matched view or add a thin filter; if `BuildViews` cannot be cleanly narrowed, fall back to building the collection's full view set and document the over-build. (2) `NoOptDefVal` removes the space-separated value form — intended, asserted in Task 1.

## Open Questions

- The `success-output` summary reports a `deleted` count, but no AC exercises deletion (stale view output removed when a view's output set shrinks). If deletion is in scope, add a deletion test in Task 5; otherwise drop `deleted` from the summary wording. (Reviewer advisory carried from specify.)
- `docs update` removal vs deprecated-alias: this plan only deprecates (keeps it working). Decide hard-removal in a follow-up after grepping CI/docs for `docs update` usage.

---
*This document follows the https://specscore.md/plan-specification*
