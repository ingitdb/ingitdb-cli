# Idea: Configurable Record Format with YAML Default

**Status:** Approved
**Date:** 2026-05-12
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we let users choose the on-disk file format for inGitDB records and schemas on a per-project basis — picking a default that maximizes editor / CI / tooling familiarity, while letting users opt into more compact, more merge-optimized, or more pipeline-friendly formats when their workflow benefits?

## Context

inGitDB today stores records as plain YAML or JSON files (see project README). Four formats are in scope for this Idea:

- **YAML** — line-oriented, human-readable, universal editor/CI support, the existing format for most current inGitDB content. "Good enough" Git-merge-friendly for record-shaped data.
- **JSON** — universal interchange format. First-class tooling support (`jq`, every CI system, every programming language). Brace-stacking produces slightly noisier Git diffs than YAML on single-field edits.
- **INGR** ([ingr.io](https://ingr.io/)) — record-oriented format designed specifically for Git's three-way merge. Line-oriented, stable key ordering, minimal punctuation. Niche ecosystem; high theoretical merge advantage on hot-path edits. A Go parser already exists at [`github.com/ingr-io/ingr-go`](https://github.com/ingr-io/ingr-go) (local checkout: `~/projects/ingr/ingr-go/`), which removes the parser-availability risk for opt-in support.
- **CSV** — tabular, row-per-record, universal in data pipelines (Excel, `awk`/`sed`/`cut`, every data tool). Excellent Git-friendly behavior for additions/deletions (one row added = one diff line). Has a structural constraint: CSV expresses flat records only; collections with nested or array-valued fields cannot round-trip cleanly. CSV is a first-class option for projects whose schema is flat.

The trigger for committing now: the DALgo schema-modification (DDL) work in `dal-go/dalgo` (now Implemented) will start writing schema files to inGitDB projects via `dalgo2ingitdb`, and the cross-engine db-copy command in `datatug-cli` is the first consumer that will generate large amounts of inGitDB content programmatically. We want to fix the default AND the configurability mechanism before those new schema files and bulk records start appearing in user repos.

## Recommended Direction

Make the file format configurable per project via a `recordFormat` field in the inGitDB project config, with **YAML as the default** for newly created projects. Support **four first-class formats** — `yaml`, `json`, `ingr`, `csv`. The reader handles all four on a per-file basis (mixed-format projects are tolerated); the writer honors the project's configured default. The CLI surface adds a `--format` flag on project-creation commands (current state: the `ingitdb` CLI) that accepts `yaml`, `json`, `ingr`, or `csv`; the flag's own default is `yaml`. `dalgo2ingitdb` consults project config on write to pick the format. Existing projects on JSON or YAML continue to work unchanged.

The default-format rationale: **YAML is the best default for the broadest set of users.** It's the existing format for most current inGitDB content, is universally supported by editors / CI / git tooling, and is "good enough" Git-merge-friendly for line-oriented record data. YAML's whitespace-sensitivity edge cases bite in rare scenarios for record-shaped data. INGR's theoretical Git-merge advantages buy a marginal improvement on diff noise — real but small — at a large cost in ecosystem familiarity. Users with specific needs flip the default per-project: INGR for hot-path Git-friendliness, JSON for tooling-pipeline integration, CSV for tabular-only analytics-friendly storage.

**All four readers/writers ship in MVP.** INGR opt-in uses `github.com/ingr-io/ingr-go` as the parser, so MVP doesn't carry the cost of writing one from scratch. CSV ships with a known structural constraint: writing a record with nested or array-valued fields fails with a clear error (driver-side). Consumers that pick CSV either have a flat schema or accept the constraint.

The difference from the prior direction (which proposed INGR as default) is that no format is *imposed*: every format is *available* and users pick. INGR remains a first-class option for users who want its specific Git-friendliness profile — the configurable mechanism captures INGR's value without making it everyone's default.

## Alternatives Considered

- **Promote INGR as default for new projects.** Originally recommended in this Idea; reconsidered. Rejected because (a) YAML's existing ecosystem advantages (editors, CI, Git hosting, every Go developer's existing familiarity) outweigh INGR's marginal diff-noise gain; (b) imposing an unfamiliar format on every new project produces friction without a forcing-function consumer demand. The configurable mechanism captures INGR's value without making it everyone's default.
- **Keep YAML/JSON only; defer INGR and CSV entirely.** Considered; rejected for MVP. Shipping the multi-format reader/writer machinery without INGR or CSV means the next consumer asking for either triggers a second round of work. Since the parser and writer surface is what dominates the MVP cost (and INGR's parser already exists upstream, and CSV is trivial in Go's stdlib), including both as first-class opt-ins now is cheaper than deferring.
- **Adopt INGR as the only format; remove JSON/YAML.** Rejected — both formats have existing consumers and editor support. Removing them adds migration cost for no portability gain. Multi-format reading is cheap.
- **Per-table format choice baked into schema.** Rejected for MVP — adds surface and a config knob most users don't need. A project-level default with rare overrides covers the actual use cases. Readers tolerate mixed-format projects so a future per-table extension is non-breaking.
- **CSV as default.** Considered; rejected. CSV's flat-record constraint disqualifies it as a universal default — collections with any nested or array-valued fields would fail. CSV is a first-class option for users whose schemas warrant it, not the right default for everyone.

## MVP Scope

Four readers and four writers ship together in `pkg/dalgo2ingitdb`:

- **YAML** — the default; uses existing inGitDB YAML codepath (extended to be format-aware).
- **JSON** — uses existing inGitDB JSON codepath (extended).
- **INGR** — opt-in; uses [`github.com/ingr-io/ingr-go`](https://github.com/ingr-io/ingr-go) as the parser.
- **CSV** — opt-in; uses Go's `encoding/csv` for serialization. Writer returns a clear error when asked to serialize a record with nested or array-valued fields.

Project-level config carries a `recordFormat` field that defaults to `yaml` for newly created projects. Readers handle all four formats on a per-file basis; mixed-format projects (some records YAML, some INGR, some JSON, some CSV) are tolerated by the reader as long as each file's format can be detected from its extension or content. A `--format` flag is exposed on whichever project-creation command exists when this lands (current state: the `ingitdb` CLI).

Verification: round-trip a 3-table demo project (with flat-only schemas) through all four formats; confirm reads still work after manually converting a subset of records to a different format than the project default; confirm INGR diffs on a representative single-field record edit are measurably smaller than YAML/JSON (validates the INGR opt-in's stated benefit); confirm CSV writer fails cleanly with a typed error when given a record with a nested field.

## Not Doing (and Why)

- Promoting INGR (or any non-YAML format) as the default for new projects — rejected per the Recommended Direction. YAML is the default; other formats are opt-in.
- Migrating existing JSON/YAML projects to any other format automatically — out of scope; conversion is a separate one-shot command if users want it.
- Removing or deprecating any of the four formats — all four remain first-class for the foreseeable future.
- Defining the INGR format itself — that work belongs upstream at ingr.io; this Idea consumes the format as-is via `github.com/ingr-io/ingr-go`.
- Per-record format overrides (mixing formats within a single table) — supported by readers, but no CLI surface to author that way in MVP.
- Per-table or per-collection format overrides via project config — supported in principle by the file-aware reader, but no project-config surface for it in MVP. Project-level default is enough granularity.
- Supporting nested or array-valued fields in CSV — the writer fails cleanly with a typed error in MVP; flattening to a wide schema is a consumer choice, not something this package does.
- Performance benchmarking of parsers across formats — defer; correctness first.

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | `github.com/ingr-io/ingr-go` is feature-complete enough to ship as the INGR parser/writer in MVP and has a stable API surface we can depend on. | Audit the package: confirm it exports a Marshal/Unmarshal pair (or equivalent), handles the inGitDB-relevant subset of INGR features (records with primitive + string fields at minimum), has a tagged release or stable main, and the maintainer signal is healthy. |
| Must-be-true | `dalgo2ingitdb` can transparently read mixed-format projects (records in YAML, JSON, INGR, AND CSV coexisting in one collection) without breaking existing consumers. Format detection is per-file (by extension or content sniff). | Build a fixture with one record in each format in a single collection; round-trip via `dalgo2ingitdb` read path. |
| Must-be-true | A project-level config field (`recordFormat`) and a `--format` CLI flag give users enough control to switch formats per project. | Manual UX walkthrough: create a project, switch its format, add records, verify the writer honors the switch. |
| Must-be-true | The CSV writer cleanly refuses (with a typed error) to serialize records with nested or array-valued fields, AND clearly communicates the constraint to the caller. | Construct a record with a nested object field; attempt to write as CSV; assert the error type and message identify which field is unrepresentable. |
| Should-be-true | YAML's Git-merge behavior is acceptable for most real-world record edits; users who hit YAML diff noise on a hot path can opt into INGR and get measurable improvement. | Construct a 50-record demo collection; run a scripted edit (change one field in one record) and compare `git diff` output across YAML and INGR. INGR should be measurably smaller — validates the opt-in's value proposition. |
| Should-be-true | Users editing records by hand in editors (VS Code, JetBrains, vim, nano) find YAML readable and find INGR/CSV readable enough that they don't reach for a converter when they opt in. | Open representative records in each format across the four editors above; informal usability check. |
| Should-be-true | Users with JSON-tooling-integration needs (piping records to `jq`, scripting against a CI pipeline that expects JSON) can flip to `recordFormat: json` and have a working project. | Verify the JSON writer round-trips through `jq` cleanly. |
| Should-be-true | Users with CSV/analytics-tooling needs (importing into Excel, piping to `awk`, loading into Pandas) can flip to `recordFormat: csv` for a flat-schema project and have a working dataset. | Write a flat-schema demo project as CSV; verify it loads cleanly in Excel and in a one-line Pandas `read_csv`. |
| Might-be-true | INGR will eventually be adopted by tools outside inGitDB. | Defer; don't bet the inGitDB strategy on broader INGR adoption — adopt for our own users' reasons. |
| Might-be-true | Schema files (from the DALgo DDL work) might benefit from a different default than data records. | Defer; the project-level `recordFormat` field applies to both. If a real consumer reports the conflation as a problem, split into `schemaFormat` + `recordFormat` later (non-breaking addition). |

## SpecScore Integration

- **New Features this would create:**
  - YAML / JSON / INGR / CSV readers and writers inside `pkg/dalgo2ingitdb` (Feature spec TBD when implementation starts). YAML and JSON likely already partially exist; INGR uses `github.com/ingr-io/ingr-go`; CSV is greenfield but trivial (Go stdlib).
  - Project-config schema gains a `recordFormat` field (Feature spec TBD).
  - CLI surface for `--format` on project-creation commands (Feature spec TBD).
- **Existing Features affected:**
  - Any existing CLI command that creates an inGitDB project — gains a `--format` flag with `yaml` as default.
  - Existing YAML and JSON read/write paths — must continue to work; readers become format-aware on a per-file basis.
- **Dependencies:**
  - **INGR format spec + Go parser** — [`github.com/ingr-io/ingr-go`](https://github.com/ingr-io/ingr-go) (already exists; vendored or imported normally). Local checkout: `~/projects/ingr/ingr-go/`.
  - **CSV support** — Go stdlib `encoding/csv`. No external dependency.
- **Downstream consumers (informational; Synchestra manages `promotes_to`):**
  - [`dal-go/dalgo` Idea `dalgo-schema-modification`](https://github.com/dal-go/dalgo/blob/main/spec/ideas/dalgo-schema-modification.md) — DDL writes schema files via `dalgo2ingitdb`; will pick up whichever format the project is configured for (default `yaml`).
  - [`datatug/datatug-cli` Idea `cross-engine-db-copy`](https://github.com/datatug/datatug-cli/blob/main/spec/ideas/cross-engine-db-copy.md) — `db copy --to ingitdb://...` will materialize content in the configured format.

## Outstanding Questions

- **`github.com/ingr-io/ingr-go` API surface and maturity.** Need to audit at specify time: which functions are exported, what they accept, whether there's a stable release, whether the maintainer is responsive. The parser's existence resolves the previously-blocking question; remaining work is integration scoping.
- **Per-project vs per-collection config.** `recordFormat` lives at project root in MVP. Per-collection extension is non-breaking (the reader is already file-aware); revisit if real users ask.
- **Mixed-format projects in CI.** If a project legitimately mixes formats (e.g. user is in the middle of converting), do CI/validation commands warn, fail, or accept silently? Pick a default behavior at specify time.
- **CSV file extension and detection.** CSV files conventionally use `.csv`; YAML uses `.yaml`/`.yml`; JSON uses `.json`; INGR uses (TBD per ingr.io). The reader's per-file format detection needs an explicit rule — extension-based is the cleanest, but content-sniffing as fallback may help for legacy files. Pick at specify time.
- **CSV column ordering and schema co-location.** CSV's first row is conventionally column headers. For inGitDB's record model, the column order matters for diff stability. Does CSV write include a header row? How does the reader handle a CSV file with a different column order than the collection's current schema? Pick at specify time.

---
*This document follows the https://specscore.md/idea-specification*
