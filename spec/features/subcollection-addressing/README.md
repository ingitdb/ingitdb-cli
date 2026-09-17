---
format: https://specscore.md/feature-specification
status: Implementing
---

# Feature: Subcollection Addressing

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/subcollection-addressing?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/subcollection-addressing?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/subcollection-addressing?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/subcollection-addressing?op=request-change) |
**Status:** Implementing
**Source Ideas:** —

## Summary

`ingitdb select --from` and the terminal UI reach the records of a subcollection, not only
of a root collection. `--from=lists/to-buy/items` returns the `items` of the list record
`to-buy`, with every set-mode flag working as it does for a root collection, and the terminal
UI lets a person open a record's declared subcollection and go back with Esc.

## Problem

A collection definition can declare subcollections, and the storage drivers already keep a
subcollection's records under their parent record (`lists/to-buy/items/...`). The CLI cannot
reach them: `select --from` and the terminal UI accept only root collection IDs, so
`select --from=lists/to-buy/items` fails with `collection "lists/to-buy/items" not found in
definition` and the terminal UI shows a subcollection only as a name in the schema panel.
Nested data, such as the TODO demo's list items ([cli/demo](../cli/demo/README.md)), cannot
be queried or browsed.

## Behavior

### Select

#### REQ: from-subcollection-path

`select --from` MUST accept a subcollection path of the form
`<collection>/<record-key>/<subcollection>`, repeatable to deeper levels
(`<collection>/<record-key>/<subcollection>/<record-key>/<subcollection>`). The first segment
MUST be a root collection ID. Segments at even positions (second, fourth, ...) are parent
record keys. Each subcollection segment MUST be a subcollection declared by the definition of
the collection before it. The result MUST be the records of that subcollection under that
parent record only; records of the same subcollection under other parent records MUST NOT be
returned.

#### REQ: undeclared-segment-not-found

When a `--from` value containing `/` does not match
[REQ:from-subcollection-path](#req-from-subcollection-path) — the root segment is not a
declared root collection, a subcollection segment is not declared by its parent's definition,
the value ends with a record key, or a segment is empty, `.`, `..` or contains a path
separator (`/` or `\`) — `select` MUST exit non-zero with the
existing error `collection "<value>" not found in definition`, naming the whole value, and
MUST write nothing to stdout.

#### REQ: set-mode-parity

With a subcollection path, `--where`, `--order-by`, `--fields`, `--limit`, `--min-affected` and
`--format` MUST behave exactly as they do for a root collection per
[cli/select](../cli/select/README.md) and [shared-cli-flags](../shared-cli-flags/README.md),
including the default `csv` format and the empty-result output. A parent record key with no
records in the subcollection MUST produce the empty-result output and exit `0`.

#### REQ: root-from-unchanged

A `--from` value that is a declared root collection ID MUST behave exactly as before this
feature.

#### REQ: driver-owned-layout

Subcollection records MUST be located through the storage driver's parent-chain scoping (the
local driver's `resolveScopedCollection`). The CLI MUST NOT compute or define its own on-disk
path for subcollection records.

### Terminal UI

#### REQ: tui-open-subcollection

On the collection screen, `enter` on a record of a collection whose definition declares
subcollections MUST open a collection screen listing that record's records in the declared
subcollection. With one declared subcollection it MUST open directly. With several, `enter`
MUST first show the declared subcollection IDs in sorted order; `↑`/`↓` choose, `enter`
opens the chosen one and `esc` closes the list without opening. The screen header MUST show
the full path (for example `lists › to-buy › items`). A subcollection screen MUST support the
same drill-down into its own declared subcollections. `enter` on a record of a collection
without subcollections MUST do nothing.

#### REQ: tui-back-with-esc

`esc` (and `backspace`) on a subcollection screen MUST return to the parent collection
screen with the record that was opened still selected. On a root collection screen they MUST
return to the home screen, as before this feature. `esc` MUST still close an open dropdown
first.

## Dependencies

- [cli/select](../cli/select/README.md) — set mode, flags and output.
- [shared-cli-flags](../shared-cli-flags/README.md) — `--from` and set-mode flag grammar.
- [output-formats](../output-formats/README.md) — `--format`.
- [tui-lazy-computed-cells](../tui-lazy-computed-cells/README.md) — a subcollection screen is
  a collection screen and keeps its lazy computed-cell behaviour.
- `dalgo2ingitdb4local` — parent-chain scoping of subcollection records.

## Implementation

Plan: [2026-09-17-cli-demo](../../plans/2026-09-17-cli-demo.md), Task 3. Source files
annotated with `// specscore: feature/subcollection-addressing`:

- [`cmd/ingitdb/commands/subcollection_path.go`](../../../cmd/ingitdb/commands/subcollection_path.go) — resolves a `--from` value to a collection reference carrying the parent record key.
- [`cmd/ingitdb/commands/select.go`](../../../cmd/ingitdb/commands/select.go) — set mode reads through that reference.
- [`cmd/ingitdb/tui/collection_screen.go`](../../../cmd/ingitdb/tui/collection_screen.go) and [`cmd/ingitdb/tui/model.go`](../../../cmd/ingitdb/tui/model.go) — drill-down, chooser and back navigation.
- [`cmd/ingitdb/tui/collection_data_panel.go`](../../../cmd/ingitdb/tui/collection_data_panel.go) — the subcollection chooser.

Tests: `cmd/ingitdb/commands/select_subcollection_test.go` and
`cmd/ingitdb/tui/subcollection_test.go`, over a fixture shaped like the TODO demo
(`internal/testutil/nested_lists.go`) whose records are written through the local driver.

## Acceptance Criteria

### AC: select-subcollection-every-format

**Requirements:** subcollection-addressing#req:from-subcollection-path, subcollection-addressing#req:set-mode-parity, subcollection-addressing#req:driver-owned-layout

**Given** a database with root collection `lists` declaring subcollection `items`, lists
`to-buy` (items Milk, Bananas, Coffee) and `to-watch` (items The Matrix, Interstellar) at the
driver's paths
**When** `ingitdb select --from=lists/to-buy/items` runs with each of `--format=csv`, `json`,
`yaml`, `md` and `ingr`, and with no `--format`
**Then** each exits `0` and returns exactly Milk, Bananas and Coffee in that format (csv when
omitted), and no `to-watch` item

### AC: set-flags-on-subcollection

**Requirements:** subcollection-addressing#req:set-mode-parity

**Given** the same database
**When** `select --from=lists/to-buy/items` runs with `--where`, with `--order-by`, with
`--fields` and with `--limit`, and `select --from=lists/nothing/items` runs
**Then** each result is what the same flags give on a root collection holding those three
records, and the unknown parent returns the empty-result output with exit `0`

### AC: undeclared-subcollection-rejected

**Requirements:** subcollection-addressing#req:undeclared-segment-not-found

**Given** the same database
**When** `select --from=lists/to-buy/notes`, `--from=lists/to-buy`, `--from=nope/to-buy/items`
and `--from=lists//items` run
**Then** each exits non-zero with `collection "<value>" not found in definition` and writes
nothing to stdout

### AC: root-from-unchanged

**Requirements:** subcollection-addressing#req:root-from-unchanged

**Given** the same database
**When** `select --from=lists --format=json` runs
**Then** it returns `to-buy` and `to-watch` with their titles, as before this feature

### AC: tui-open-and-leave-subcollection

**Requirements:** subcollection-addressing#req:tui-open-subcollection, subcollection-addressing#req:tui-back-with-esc

**Given** the terminal UI model on the same database with `lists` open
**When** the person selects `to-buy` and presses `enter`, then presses `esc`, then `esc` again
**Then** the first screen lists Milk, Bananas and Coffee with header path
`lists › to-buy › items`, the second is the `lists` screen with `to-buy` selected, and the
third is the home screen

### AC: tui-choose-among-subcollections

**Requirements:** subcollection-addressing#req:tui-open-subcollection

**Given** a collection declaring two subcollections
**When** the person presses `enter` on a record, moves down and presses `enter`
**Then** the list shows both subcollection IDs sorted, and the second one opens for that
record

## Open Questions

- How should `--id` address a subcollection record (for example `lists/to-buy/items/milk`)?
  [id-flag-format](../id-flag-format/README.md) is unchanged by this feature.
- Should `describe` and `list collections` show subcollections by path?
- Should write verbs (`insert --into`, `update --from`, `delete --from`) accept subcollection
  paths?
- Should `--remote` accept subcollection paths in `select --from`?

---
*This document follows the https://specscore.md/feature-specification*
