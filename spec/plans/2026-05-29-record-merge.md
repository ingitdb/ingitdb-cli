# Plan: Record-Merge Engine for Data-Row Auto-Resolution

**Status:** Approved
**Source Feature:** cli/resolve/auto-resolve/record-merge
**Date:** 2026-05-29
**Owner:** Alexander Trakhimenok
**Supersedes:** —

## Summary

Decomposes the approved `cli/resolve/auto-resolve/record-merge` Feature into five
dependency-ordered tasks that build a record-aware three-way merge engine: a
typed-parse + strict-equality foundation, disjoint union, a validation/escalation
gate, field-level contested detection, and config-gated same-record merging.

## Approach

The tasks are vertical slices ordered so each builds on the last. Task 1 lands the
three-way stage reader, typed parsing, the strict kind-aware equality primitive, and
the wiring into the existing conflict pass — the thinnest end-to-end slice (AC-3) that
makes everything else testable. Task 2 adds record-set diffing and the disjoint union
(the common default cases). Task 3 adds the re-validation gate and the escalation
channel to `manual-resolve` for anything non-mergeable. Task 4 adds field-level diffing
to detect contested fields. Task 5 adds the per-db / per-collection config and the
opt-in same-record field merge. No ACs are deferred.

## Tasks

### Task 1: Three-way stage foundation and strict typed equality

**Verifies:** cli/resolve/auto-resolve/record-merge#ac:representation-noise-resolved

Extract the BASE/OURS/THEIRS conflict stages (`git show :1/:2/:3:<file>`) for a
conflicted record file, parse each into keyed typed record sets via the collection
schema (detecting `single`/`list`/`map` layout), and implement the strict,
kind-aware equality primitive (REQ `semantic-equality-typed-strict`: no cross-kind
coercion, escalate on parse/validation failure). Wire this record-merge pass into
`cmd/ingitdb/commands/conflict_resolver.go` ahead of `manual-resolve` and write the
canonicalized result back via `git add` when both sides parse equal.

### Task 2: Record-set diff and disjoint union

**Verifies:** cli/resolve/auto-resolve/record-merge#ac:disjoint-additions-unioned, cli/resolve/auto-resolve/record-merge#ac:identical-addition-deduplicated

Compute BASE→OURS and BASE→THEIRS record-level diffs and classify the disjoint /
non-divergent cases (DM-1..DM-5, DM-8). Produce the union — keeping both records for
distinct IDs (appending ours-then-theirs for `list` layout), deduplicating fully
identical additions, and applying disjoint deletions — under the never-drop-a-change
invariant (REQ `never-drop-a-change`).

### Task 3: Re-validation gate and escalation channel

**Verifies:** cli/resolve/auto-resolve/record-merge#ac:invalid-merge-escalates

Add the post-merge step that re-parses and re-validates the candidate merged file
against the collection schema, and the escalation channel that leaves a file
unresolved for `manual-resolve` whenever the merge cannot be guaranteed safe —
covering invalid results (DM-17) and the primary-key-collision / type-divergence
real-conflict cases (DM-12, DM-16) from REQ `real-conflicts-escalate`.

### Task 4: Field-level diff and contested-field detection

**Verifies:** cli/resolve/auto-resolve/record-merge#ac:contested-field-escalates

For records changed on both sides, diff at field granularity and classify each field
as converging (same value), disjoint (different fields touched), or contested (same
field, different values). Route contested-field and delete/modify cases (DM-13, DM-14,
DM-15) to the escalation channel from Task 3.

### Task 5: Config plumbing and opt-in same-record merge

**Verifies:** cli/resolve/auto-resolve/record-merge#ac:different-fields-merged-when-enabled, cli/resolve/auto-resolve/record-merge#ac:same-record-escalates-when-disabled

Read `conflict_resolution.record_merge` (`enabled`, `same_record`) from `.ingitdb.yaml`
with per-collection override in `.definition.yaml`. When `same_record` is enabled, merge
non-contested same-record changes (DM-9..DM-11) using the Task 4 field diff; when
disabled, route them to escalation. Honor `enabled: false` by sending all data-row
conflicts straight to `manual-resolve`.

## Deferred AC Coverage

<!-- No ACs deferred. -->

## Open Questions

- `manual-resolve` is currently a placeholder screen, so escalated conflicts (Tasks 3–5)
  have no interactive destination yet; this plan stages them as unresolved and exits
  non-zero until `manual-resolve` is implemented.

---
*This document follows the https://specscore.md/plan-specification*
