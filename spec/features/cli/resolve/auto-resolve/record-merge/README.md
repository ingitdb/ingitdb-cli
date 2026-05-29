# Feature: Auto-Merge Logically Non-Conflicting Record Data

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/auto-resolve/record-merge?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/auto-resolve/record-merge?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/auto-resolve/record-merge?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/resolve/auto-resolve/record-merge?op=request-change) |
**Status:** Implementing
**Parent Feature:** [`cli/resolve/auto-resolve`](../README.md)

## Summary

Non-interactive resolution of merge conflicts in **source-data** record files
when the two sides do not logically conflict. Git produces a textual conflict
whenever both branches change nearby lines — but for record-oriented data, many
of those textual conflicts carry **no semantic conflict**: two users adding
records with distinct IDs, or editing different fields of the same record.

Where the auto-resolve parent's existing mechanism *regenerates* reproducible
artifacts from source, this mechanism performs a **record-aware three-way merge**
of the source data itself: it parses BASE / OURS / THEIRS into records, reasons
at the record and field level, and writes back a single merged file when — and
only when — the result is unambiguous. Anything ambiguous is handed to
[`manual-resolve`](../../manual-resolve/README.md).

## Problem

inGitDB stores many records in shared files (`type: []map[string]any` and
`type: map[$record_id]map[$field_name]any`). When two users append or edit
records, git frequently marks the *same lines* as conflicting even though the
edits are independent:

- Two users append new records at the end of the same file → textual conflict on
  the final line, but the records have different IDs and don't actually collide.
- Two users edit the same record but touch **different fields** → textual
  conflict on adjacent lines, but no field is contested.

Hand-resolving these is busywork and error-prone (it's easy to accidentally drop
one side's record). A line-based merge can't tell a real ID collision from two
harmless appends; a record-aware merge can.

## Concept

The merge is a **three-way** operation over the conflict stages git already
exposes in the working tree:

- **BASE** — common ancestor (`git show :1:<file>`)
- **OURS** — current side (`:2:<file>`)
- **THEIRS** — incoming side (`:3:<file>`)

Each stage is parsed into a record set keyed by primary key. The engine diffs
BASE→OURS and BASE→THEIRS at record and field granularity, then decides per the
case catalogue below. Three sides are required: without BASE the engine cannot
distinguish an *addition* on one side from a *deletion* on the other.

A merge is committed only if the result (a) is the complete union of both sides'
non-conflicting changes — never silently dropping a change — (b) preserves
primary-key uniqueness, and (c) re-parses and re-validates against the
collection schema. If any of these fail, the file escalates to `manual-resolve`.

## Case catalogue

Disposition legend:

- **Auto (default)** — resolved automatically out of the box. Sides are disjoint
  or non-divergent; there is no decision to make.
- **Auto (opt-in)** — resolved automatically only when same-record merging is
  enabled in config (see [Configuration](#configuration)). Sides touch the same
  record but no single field/value is contested.
- **Manual** — a real conflict; handed to `manual-resolve`. Never auto-merged.

Layout keys: **single** = `map[string]any` (one record per file), **list** =
`[]map[string]any`, **map** = `map[$record_id]…`.

### Auto-resolvable by default (disjoint / non-divergent)

| ID    | Scenario                                                                          | Layouts        | Resolution                                              |
|-------|-----------------------------------------------------------------------------------|----------------|---------------------------------------------------------|
| DM-1  | Disjoint additions: each side adds a record with a **distinct** ID                | list, map      | Keep both records; for `list`, append ours-then-theirs  |
| DM-2  | Identical addition: both sides add the **same** ID with semantically equal content | list, map     | Keep one copy                                           |
| DM-3  | Disjoint deletions: each side deletes a **different** record                      | list, map      | Apply both deletions                                    |
| DM-4  | Same deletion: both sides delete the same record                                  | all            | Record stays deleted                                    |
| DM-5  | Add vs. unrelated change: one side adds a record, the other edits/deletes a **different** record | list, map | Apply both                                  |
| DM-6  | Converging field edit: both sides set the **same field** to an **identical** value | all           | Take the agreed value                                   |
| DM-7  | Representation-only difference: whitespace, indentation, key order, quoting style, or trailing-newline / EOF differs but parsed records are equal | all | Canonicalize; sides are semantically equal → no conflict |
| DM-8  | Reordering only: record set identical, only ordering differs (order not significant) | map         | Write canonical order                                   |

### Auto-resolvable only when same-record merge is enabled (opt-in)

| ID    | Scenario                                                                          | Layouts        | Resolution                                              |
|-------|-----------------------------------------------------------------------------------|----------------|---------------------------------------------------------|
| DM-9  | Same record, **different fields** modified by each side                           | all            | Merge both field changes                                |
| DM-10 | Same record, each side **adds a different new field**                             | all            | Union the added fields                                  |
| DM-11 | Same record, one side edits field A, the other adds unrelated field B             | all            | Apply both                                              |

### Not auto-resolvable — escalate to manual-resolve

| ID    | Scenario                                                                          | Layouts        | Why it's a real conflict                                |
|-------|-----------------------------------------------------------------------------------|----------------|---------------------------------------------------------|
| DM-12 | Same ID added on both sides with **different** content (primary-key collision)    | list, map      | Two distinct records claim one ID                       |
| DM-13 | Same field set to **different** values                                            | all            | Contested value                                         |
| DM-14 | Same new field added by both sides with **different** values                      | all            | Contested value                                         |
| DM-15 | Delete/modify: one side deletes a record the other modifies                       | all            | Intent ambiguous (keep edited vs. honor delete)         |
| DM-16 | Same field's shape/type diverges (e.g. scalar ↔ list ↔ map)                       | all            | No safe structural union                                |
| DM-17 | Merged result fails to parse or fails schema validation                           | all            | Cannot guarantee a valid file                           |

## Configuration

Record-merge is configured **per database with per-collection override**. The
database default lives in `.ingitdb.yaml`; a collection overrides it in its own
`.definition.yaml`, under the same `conflict_resolution.record_merge` block. The
collection value wins for that collection.

```yaml
# .ingitdb.yaml — database default
conflict_resolution:
  record_merge:
    enabled: true        # DM-1..DM-8 (default-on, disjoint/non-divergent)
    same_record: false   # DM-9..DM-11 (opt-in same-record field merge)
```

```yaml
# <collection>/.definition.yaml — per-collection override (optional)
conflict_resolution:
  record_merge:
    same_record: true    # enable same-record field merge for this collection only
```

With `record_merge.enabled: false`, every data-row conflict in that scope goes
straight to `manual-resolve`.

## Behavior

### REQ: three-way-record-merge

For conflicted **source-data** record files, `resolve` MUST attempt a
record-aware three-way merge using the BASE / OURS / THEIRS conflict stages,
reasoning at record and field granularity rather than by text line.

### REQ: semantic-equality-typed-strict

Wherever a case depends on two values or records being "equal" (DM-2, DM-6,
DM-7), equality MUST be determined on **typed/parsed values** — each side parsed
into records via the collection's column types — so that representation noise
(key order, whitespace, quoting, flow vs. block style) does not register as a
difference. Equality MUST be **strict and kind-aware**: values of different
kinds are never equal (e.g. `1` integer ≠ `"1"` string), and no cross-kind
coercion is performed. If either side fails to parse or validate, or the two
sides interpret the same column under divergent schemas, the engine MUST
escalate to `manual-resolve` rather than assume equality.

### REQ: disjoint-auto-by-default

Cases DM-1 through DM-8 (disjoint or non-divergent — sides never contest the
same record's field value) MUST be auto-resolved by default and the merged file
staged with `git add`, without prompting.

### REQ: same-record-merge-opt-in

Cases DM-9 through DM-11 (same record, no contested field value) MUST be
auto-resolved **only** when same-record merging is enabled for that scope
(`record_merge.same_record: true`). When disabled, they MUST escalate to
`manual-resolve`.

### REQ: never-drop-a-change

An auto-merge MUST be the complete union of both sides' non-conflicting changes.
The engine MUST NOT silently discard any record or field present on either side;
if it cannot guarantee completeness it MUST escalate to `manual-resolve`.

### REQ: real-conflicts-escalate

Cases DM-12 through DM-17 — including primary-key collisions, contested field
values, delete/modify, divergent field types, ambiguous list ordering, and any
merge whose result fails schema validation — MUST NOT be auto-resolved and MUST
be handed to `manual-resolve`.

## Acceptance Criteria

### AC: disjoint-additions-unioned

**Given** a conflicted `[]map[string]any` collection file where OURS appended a
record with ID `a` and THEIRS appended a record with ID `b` (`a ≠ b`)
**When** `ingitdb resolve` runs
**Then** the merged file contains both records, parses and validates, is staged,
and is no longer reported by `git diff --name-only --diff-filter=U`.

### AC: identical-addition-deduplicated

**Given** both sides added a record with the same ID and semantically equal
content
**When** `ingitdb resolve` runs
**Then** the merged file contains exactly one copy of that record and is staged.

### AC: representation-noise-resolved

**Given** a conflict where the two sides differ only in key ordering / whitespace
but parse to identical records
**When** `ingitdb resolve` runs
**Then** the file is written in canonical form and staged, with no records added
or lost.

### AC: different-fields-merged-when-enabled

**Given** `same_record: true` and a conflict where OURS changed field `name` and
THEIRS changed field `email` of the **same** record, with no other divergence
**When** `ingitdb resolve` runs
**Then** the merged record contains both the new `name` and the new `email`, is
staged, and validates against the schema.

### AC: same-record-escalates-when-disabled

**Given** the same conflict as in `different-fields-merged-when-enabled` but with `same_record: false`
**When** `ingitdb resolve` runs
**Then** the file is left for `manual-resolve` and is still reported as
unresolved.

### AC: contested-field-escalates

**Given** a conflict where both sides set the **same** field of the **same**
record to **different** values
**When** `ingitdb resolve` runs
**Then** no auto-merge is written and the file is handed to `manual-resolve`.

### AC: invalid-merge-escalates

**Given** a conflict whose otherwise-disjoint merge would produce a file that
fails schema validation
**When** `ingitdb resolve` runs
**Then** the engine does not stage the invalid result and escalates the file to
`manual-resolve`.

## Dependencies

- path-targeting
- [`cli/resolve/manual-resolve`](../../manual-resolve/README.md) — destination
  for every case the engine declines to auto-merge.

## Implementation

Source files (annotated with `// specscore: feature/cli/resolve/auto-resolve/record-merge`):

- [`pkg/ingitdb/recordmerge/merge.go`](../../../../../../pkg/ingitdb/recordmerge/merge.go) —
  pure three-way merge engine (strict typed equality, record-set diff,
  field-level merge).
- [`pkg/ingitdb/recordmerge/bridge.go`](../../../../../../pkg/ingitdb/recordmerge/bridge.go) —
  layout-aware parsing of conflict stages into records (`MapOfRecords`,
  `SingleRecord`).
- [`pkg/ingitdb/conflict_resolution.go`](../../../../../../pkg/ingitdb/conflict_resolution.go) —
  `conflict_resolution.record_merge` config and the per-collection override
  cascade (`ResolveRecordMerge`).
- [`cmd/ingitdb/commands/record_merge_resolver.go`](../../../../../../cmd/ingitdb/commands/record_merge_resolver.go) —
  reads BASE/OURS/THEIRS git stages, runs the merge, serializes and stages the
  result, escalating the rest; wired into `resolve` ahead of `manual-resolve`.

Layouts supported today: `MapOfRecords`, `SingleRecord` (including markdown,
merged field-by-field on the frontmatter and re-serialized), and
`ListOfRecords` (CSV keyed by primary key / `$id` / `id`, and INGR keyed by
`$ID`). New code is covered by tests at 100%.

## Open Questions

None outstanding.

---
*This document follows the https://specscore.md/feature-specification*
