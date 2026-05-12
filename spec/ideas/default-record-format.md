# Idea: Default Record Format: INGR with Per-Project Configuration

**Status:** Approved
**Date:** 2026-05-12
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we choose a default on-disk file format for inGitDB records and schemas that is compact and Git-merge-friendly, while letting users opt into JSON or YAML when their tooling, editors, or downstream consumers require it?

## Context

inGitDB today stores records as plain YAML or JSON files (see project README). Both formats serve human review well but neither is purpose-built for Git's three-way merge: YAML's whitespace sensitivity and JSON's brace stacking both produce noisy diffs on small record changes, and both can hit merge conflicts on co-evolving edits that semantically don't conflict. INGR (https://ingr.io/) is a record-oriented text format designed specifically to be compact AND merge-friendly — line-oriented, stable key ordering, minimal punctuation. The trigger for choosing now: the DALgo schema-modification (DDL) work in dal-go/dalgo will start writing schema files to inGitDB projects via dalgo2ingitdb, and we want to commit to a default format before those schema files start appearing in user repos. The cross-engine db-copy command in datatug-cli is the first consumer that will generate large amounts of inGitDB content programmatically.

## Recommended Direction

Adopt INGR as the default file format for newly created inGitDB projects (records AND schema files). Make the format choice configurable per-project via project config (and overridable per-table for power users). Keep JSON and YAML as first-class supported FORMATS — readers must handle all three, writers honor the project's configured default. Existing projects on JSON or YAML continue to work unchanged. The CLI surface adds a --format flag on project-creation commands (e.g. on a future 'ingitdb init') that accepts 'ingr', 'json', or 'yaml'; default is 'ingr'. dalgo2ingitdb consults project config on write to pick the format. This is also a deliberate promotion of the INGR format — making inGitDB the reference implementation of an open record format and giving INGR a real-world adopter.

## Alternatives Considered

- **Keep YAML as default; promote INGR later.** Lost because the DALgo DDL work in `dal-go/dalgo` and the cross-engine db-copy work in `datatug/datatug-cli` will start materializing new inGitDB content imminently. Choosing now sets the default for those new projects; choosing later means migrating them or accepting two distinct conventions in the wild.
- **Adopt INGR as the only format; remove JSON/YAML.** Rejected — both formats have existing consumers and editor support. Removing them adds migration cost for no portability gain. Multi-format reading is cheap.
- **Per-table format choice baked into schema.** Rejected for MVP — adds surface and a config knob most users don't need. A project-level default with rare overrides covers the actual use cases.

## MVP Scope

INGR is implemented as a writer AND reader in dalgo2ingitdb. Project-level config carries a 'recordFormat' field that defaults to 'ingr' for newly created projects. JSON and YAML readers continue to work; mixed-format projects (some records JSON, some INGR) are tolerated by the reader. A --format flag is exposed on whichever project-creation command exists when this lands (current state: 'ingitdb' CLI). Verification: round-trip a 3-table demo project through all three formats; confirm Git diffs on a representative record edit are smaller for INGR than for JSON and YAML.

## Not Doing (and Why)

- Migrating existing JSON/YAML projects to INGR automatically — out of scope; conversion is a separate one-shot command if users want it
- Removing or deprecating JSON/YAML support — both remain first-class for the foreseeable future
- Defining the INGR format itself — that work belongs upstream at ingr.io; this Idea consumes the format as-is
- Per-record format overrides (mixing formats within a single table) — supported by readers, but no CLI surface to author that way in MVP
- Performance benchmarking of parsers across formats — defer; correctness first

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | INGR has a stable enough specification and a working Go parser (or one we can write quickly) to ship as the default for new projects. | Survey ingr.io for spec version and reference implementations; estimate cost of a Go reader/writer if none exists yet. |
| Must-be-true | INGR diffs on representative single-record edits are measurably smaller and merge-cleaner than equivalent JSON or YAML diffs. | Construct a 50-record demo collection; run a scripted edit (change one field in one record) and compare `git diff` output across all three formats. |
| Must-be-true | `dalgo2ingitdb` can transparently read mixed-format projects (records in JSON, YAML, AND INGR coexisting in one collection) without breaking existing consumers. | Build a fixture with one record in each format; round-trip via `dalgo2ingitdb` read path. |
| Should-be-true | Users editing records by hand in editors (VS Code, JetBrains, vim, nano) find INGR readable enough that they don't reach for a converter. | Open representative INGR records in the four editors above; informal usability check. |
| Should-be-true | A project-level config field (e.g. `recordFormat: ingr`) is enough granularity for MVP; per-table or per-record overrides are rare enough to defer. | Solicit feedback from early users after MVP ships; revisit if 2+ ask for per-table. |
| Might-be-true | INGR will eventually be adopted by tools outside inGitDB. | Defer; don't bet the inGitDB strategy on broader INGR adoption — adopt for our own reasons. |


## SpecScore Integration

- **New Features this would create:**
  - INGR writer/reader inside `pkg/dalgo2ingitdb` (Feature spec TBD when implementation starts).
  - Project-config schema gains a `recordFormat` field (Feature spec TBD).
  - CLI surface for `--format` on project-creation commands (Feature spec TBD).
- **Existing Features affected:**
  - Any existing CLI command that creates an inGitDB project — gains a `--format` flag with `ingr` as default.
  - Existing YAML and JSON read/write paths — must continue to work; readers become format-aware on a per-file basis.
- **Dependencies:**
  - **INGR format specification** at [ingr.io](https://ingr.io/) — external dependency; this Idea does not define the format.
  - **A Go INGR parser** (reference impl or our own).
- **Downstream consumers (informational; Synchestra manages `promotes_to`):**
  - [`dal-go/dalgo` Idea `dalgo-schema-modification`](https://github.com/dal-go/dalgo/blob/main/spec/ideas/dalgo-schema-modification.md) — DDL writes schema files via `dalgo2ingitdb`; will pick up INGR by default.
  - [`datatug/datatug-cli` Idea `cross-engine-db-copy`](https://github.com/datatug/datatug-cli/blob/main/spec/ideas/cross-engine-db-copy.md) — `db copy --to ingitdb://...` will materialize content in the configured format.

## Open Questions

- **Is INGR's specification stable enough to commit to as a default today?** Spec version, breaking-change history, and active maintenance at ingr.io need a quick audit.
- **Go INGR parser availability.** Does a reference Go implementation exist? If not, scope writing one (likely a tractable amount of work given the format's design, but uncertain until surveyed).
- **Per-project vs per-collection config.** Does `recordFormat` live at the project root or per collection? Project-level is simpler; per-collection allows per-table experimentation. MVP could be project-level with no API barrier to extending later.
- **Mixed-format projects in CI.** If a project legitimately mixes formats (e.g. user is in the middle of converting), do CI/validation commands warn, fail, or accept silently? Pick a default behavior.
- **Schema files specifically.** Schema files (from the DALgo DDL work) might benefit from a different default than data records (e.g. YAML for schema readability, INGR for record density). Should the format choice be one field or two (`schemaFormat` + `recordFormat`)?

---
*This document follows the https://specscore.md/idea-specification*
