# Idea: Project-Level Default Record Format + CSV Format Support

**Status:** Approved
**Date:** 2026-05-12
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

inGitDB today supports six record formats (`yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`) but the choice is set **per collection** via `.collection/definition.yaml`. There is no **project-level default** that newly-created collections inherit. The CLI also lacks a `--format` flag on project- and collection-creation commands, so a user creating a new project today has no way to express "all my collections should default to format X." And CSV — a universally useful tabular format — is not supported at all.

How might we add (a) a project-level default record format that newly-created collections inherit when they don't specify their own, (b) a `--format` flag on project/collection-creation CLI commands, and (c) CSV as a seventh first-class supported format — without breaking the existing per-collection mechanism?

## Context

**Important reality check (added after surveying the codebase):** the format-handling machinery is far more advanced than the original framing of this Idea suggested. Already implemented in ingitdb-cli today:

- **Six `RecordFormat` constants** in `pkg/ingitdb/constants.go`: `yaml`, `yml`, `json`, `markdown`, `toml`, `ingr`.
- **Read + write paths for all six** in `pkg/dalgo2ingitdb/parse.go` and `pkg/dalgo2ghingitdb/tx_readwrite.go`.
- **Markdown** has special handling: YAML frontmatter + body stored under a `content_field` column (default `$content`).
- **INGR** is restricted to `ListOfRecords` or `MapOfRecords` record types (not `SingleRecord`).
- **Per-collection format choice** lives in `.collection/definition.yaml`'s `record_file.format` field.
- **`.ingitdb/settings.yaml`** project-level config already exists (`config.Settings` struct in `pkg/ingitdb/config/root_config.go`, currently with `DefaultNamespace` and `Languages` fields). No `recordFormat` field lives there yet — adding one is an additive YAML-tagged extension to the existing struct.
- The Go INGR parser at [`github.com/ingr-io/ingr-go`](https://github.com/ingr-io/ingr-go) is already a dependency and wired up.

What's **NOT** implemented (the real surface this Idea covers):

1. **Project-level default `recordFormat`** field — a fallback for collections that don't specify their own format.
2. **`--format` CLI flag** on project- and collection-creation commands.
3. **CSV reader/writer** — a seventh format for tabular use cases (Excel, `awk`, Pandas integration). Today inGitDB cannot read or write CSV.
4. **Documentation/spec coverage** of the six existing formats — useful for future contributors but optional for this Idea's scope.

The trigger for committing the project-level default now: the DALgo schema-modification (DDL) work in `dal-go/dalgo` (now Implemented) will start writing schema files to inGitDB projects via `dalgo2ingitdb`, and `datatug-cli`'s cross-engine db-copy command will generate bulk records. Both need a clear answer to "what format do I write when the user hasn't specified one for the new collection?" Today the answer is "hard-coded fallback in code"; we want "project config decides."

## Recommended Direction

Ship three small, additive changes layered on top of the existing six-format machinery:

**1. Project-level default `recordFormat` config field.** Add a new field `RecordFormat` (YAML tag `record_format`) to the existing `config.Settings` struct in `pkg/ingitdb/config/root_config.go` — the struct already loads from `.ingitdb/settings.yaml`. When a new collection is created without an explicit format, the fallback chain is: explicit per-collection setting (`.collection/definition.yaml#record_file.format`) → project-level default (`.ingitdb/settings.yaml#record_format`) → hard fallback (YAML). Existing collections with their own `record_file.format` are unaffected. The hard fallback is **YAML**, matching the existing codebase convention and editor/CI ecosystem familiarity. The new field is `omitempty`-tagged so existing projects without the field keep working unchanged.

**2. `--format` CLI flag.** Add a `--format` flag (with `yaml` as the flag's default-default when no project setting exists) to whichever project- and collection-creation commands the CLI exposes. The flag accepts any of the seven supported formats (six existing + CSV). On project creation, the flag's value populates `.ingitdb/settings.yaml#record_format`. On collection creation, the flag's value populates `.collection/definition.yaml#record_file.format` directly.

**3. CSV reader/writer.** Add `RecordFormatCSV` (`csv`) as a seventh constant. Implement read and write paths in `pkg/dalgo2ingitdb/parse.go` (and the GitHub backend's `tx_readwrite.go`) using Go's standard `encoding/csv`. CSV's structural constraint — flat records only, no nested or array-valued fields — surfaces as a typed error at write time when a record fails to serialize cleanly. By analogy with INGR's `RecordType` restriction, CSV's natural `RecordType` is `ListOfRecords` (the file is a table; rows are records); writer errors when the record type is `SingleRecord` or `MapOfRecords`.

**What's intentionally NOT changing:** the six existing formats stay as-is. Markdown's frontmatter handling, INGR's RecordType restriction, TOML's parser choice, the YAML/YML aliasing — none of those mechanisms change. This Idea is strictly additive.

## Alternatives Considered

- **Promote INGR as the default for new projects.** Originally recommended in earlier revisions of this Idea; reconsidered. Rejected because YAML's existing ecosystem advantages (editors, CI, Git hosting, every Go developer's familiarity) outweigh INGR's marginal Git-merge-noise gain on record-shaped data. INGR remains a first-class opt-in via the project-level `recordFormat: ingr` setting.
- **Skip the project-level config; only add the CLI flag.** Rejected — without a stored project-level default, the `--format` choice would have to be repeated for every collection-creation command. The project-level field is the natural place for "what does this project prefer."
- **Skip CSV; defer to a future Idea.** Considered; rejected. CSV's value proposition (Excel/data-pipeline integration) is real, the implementation cost is tiny (Go stdlib), and bundling it with the project-default work means one MVP instead of two.
- **Add per-project AND per-collection-tree (subdirectory-level) defaults.** Considered; rejected for MVP. A subdirectory-level default would complicate the fallback chain (collection → subdir → project → hard default) for marginal benefit. Project-level default plus per-collection override covers the realistic use cases.
- **Make CSV the new default.** Rejected — CSV's flat-record constraint disqualifies it as a universal default. CSV is a per-project opt-in for users whose schemas are tabular.

## MVP Scope

Three deliverables, scoped tight:

1. **`RecordFormatCSV` constant** in `pkg/ingitdb/constants.go` + read/write paths in `pkg/dalgo2ingitdb/parse.go` and `pkg/dalgo2ghingitdb/tx_readwrite.go`. CSV writer fails cleanly with a typed error when asked to serialize a record with a nested or array-valued field. CSV's natural `RecordType` is `ListOfRecords`; validation rejects `SingleRecord` and `MapOfRecords` analogously to INGR's existing restriction.

2. **Project-level `record_format` field** added to `config.Settings` in `pkg/ingitdb/config/root_config.go`, loaded from `.ingitdb/settings.yaml`. Reader-side fallback chain centralized in one helper (e.g. `config.RootConfig.ResolveRecordFormat(collection)`): per-collection `record_file.format` → project-level `record_format` → hard default (`yaml`). When the field is unset, behavior matches today (per-collection or hard fallback). When the field is set to an unrecognized value, `config.Settings.Validate` (or the loader) fails with a clear error naming the offending value and the valid options.

3. **`--format` CLI flag** on project-creation and collection-creation commands (current state: identify which `ingitdb` CLI commands those are). Accepts the seven values (yaml, yml, json, markdown, toml, ingr, csv). On project creation, populates `.ingitdb/settings.yaml#record_format`. On collection creation, populates `.collection/definition.yaml#record_file.format`. If the user passes an unsupported value, the CLI exits non-zero with a clear error message listing the seven supported formats.

Verification: round-trip a 3-table demo project (with flat-only schemas) through CSV — `ingitdb` create + insert + read for each format. Confirm the fallback chain: a project with `recordFormat: ingr` but a collection that explicitly sets `record_file.format: yaml` reads/writes that collection's records as YAML. Confirm CSV writer errors typed-cleanly on a nested-field record.

## Not Doing (and Why)

- Promoting any non-YAML format as the hard default — rejected per the Recommended Direction. YAML is the hard fallback; project-level `recordFormat` lets users override.
- Migrating existing collections from one format to another automatically — out of scope; conversion is a separate one-shot command if users want it.
- Removing or deprecating any of the seven supported formats — all seven (yaml, yml, json, markdown, toml, ingr, csv) remain first-class.
- Defining the INGR or TOML formats themselves — both are external specifications. INGR via `github.com/ingr-io/ingr-go`; TOML via `github.com/pelletier/go-toml/v2`.
- Per-record format overrides (mixing formats within a single table) — already supported by the file-aware reader; no new CLI surface to author that way.
- Per-subdirectory format overrides — covered today by per-collection settings; subdirectory granularity is not added.
- Supporting nested or array-valued fields in CSV — the writer fails cleanly with a typed error in MVP; flattening to a wide schema is a consumer choice.
- Performance benchmarking of parsers across formats — defer; correctness first.
- Re-specifying the existing six formats in SpecScore Feature files — useful documentation hygiene but out of scope for this Idea. Could be a separate `record-formats-catalog` documentation Feature later.

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | Adding a `RecordFormat` field with `yaml:"record_format,omitempty"` to the existing `config.Settings` struct (in `pkg/ingitdb/config/root_config.go`) does not break existing projects whose `.ingitdb/settings.yaml` doesn't have the field. | Audit the existing YAML unmarshal path; confirm `omitempty` zero-value behavior. The struct already loads cleanly with missing optional fields (e.g. existing projects without `Languages`). |
| Must-be-true | The fallback chain (collection → project → hard default `yaml`) is implementable without a fragile order-of-operations bug. The configuration MUST resolve consistently regardless of where in the codebase it's queried. | Centralize fallback resolution in one function; all read/write call sites consult that function rather than reading raw config. |
| Must-be-true | Go's `encoding/csv` is sufficient for the inGitDB-relevant subset of CSV behavior (header row, comma-separated, RFC 4180 quoting). | Cross-check against the existing `record_file` shape: column names, primitive values, escaped strings. |
| Must-be-true | CSV's `RecordType: ListOfRecords` constraint can be validated at collection-definition load time AND at write time, matching INGR's existing pattern. | Audit `pkg/ingitdb/record_file_def.go`'s INGR validation (lines 68–70); add CSV with the same shape. |
| Must-be-true | The CSV writer cleanly refuses (with a typed error) to serialize records with nested or array-valued fields, AND clearly communicates the constraint to the caller. | Construct a record with a nested object field; attempt to write as CSV; assert the error type and message identify which field is unrepresentable. |
| Should-be-true | YAML's Git-merge behavior is acceptable for most real-world record edits; users who hit YAML diff noise on a hot path opt into INGR via `recordFormat: ingr`. | Construct a 50-record demo collection; run a scripted edit and compare `git diff` output across YAML and INGR. |
| Should-be-true | Users with CSV/analytics needs (Excel, Pandas, `awk`) can flip a project to `recordFormat: csv` for a flat-schema use case and have it work cleanly. | Write a flat-schema demo project as CSV; verify it loads in Excel and Pandas (`read_csv`). |
| Should-be-true | The `--format` CLI flag is more useful than confusing on project- and collection-creation commands. Users won't be surprised when a flag value differs from the project default. | UX walkthrough with a representative user (or rubber-duck): does the flag's behavior match their mental model? |
| Might-be-true | The seven-format menu is too large; users will get confused choosing. | Defer; instrument format-choice frequency after MVP ships. If 90% pick yaml/json, the rest are arguably "expert opt-ins" and that's fine. |

## SpecScore Integration

- **New Features this would create (likely as siblings or under a `record-format/` umbrella in `spec/features/`):**
  - **CSV support** in `pkg/dalgo2ingitdb` and `pkg/dalgo2ghingitdb` (Feature spec TBD). Includes the new `RecordFormatCSV` constant, validation rule restricting `RecordType: ListOfRecords`, and read/write implementations.
  - **Project-level `recordFormat` config + fallback resolution** (Feature spec TBD). New field, loader, fallback-resolution helper.
  - **`--format` CLI flag** on project- and collection-creation commands (Feature spec TBD).
- **Existing Features affected:**
  - Project-creation CLI command (whichever exists in the current ingitdb CLI) — gains a `--format` flag.
  - Collection-creation CLI commands — gains a `--format` flag.
  - Record read/write paths in `pkg/dalgo2ingitdb/parse.go` and `pkg/dalgo2ghingitdb/tx_readwrite.go` — gain CSV cases and consult the new fallback resolver.
- **Dependencies:**
  - **CSV** — Go stdlib `encoding/csv`. No external dependency.
  - **Existing format infrastructure** — yaml.v3, encoding/json, github.com/pelletier/go-toml/v2, github.com/ingr-io/ingr-go (all already imported).
- **Downstream consumers (informational; Synchestra manages `promotes_to`):**
  - [`dal-go/dalgo` Idea `dalgo-schema-modification`](https://github.com/dal-go/dalgo/blob/main/spec/ideas/dalgo-schema-modification.md) — DDL writes schema files via `dalgo2ingitdb`; will pick up the project-level format choice via the fallback resolver.
  - [`datatug/datatug-cli` Idea `cross-engine-db-copy`](https://github.com/datatug/datatug-cli/blob/main/spec/ideas/cross-engine-db-copy.md) — `db copy --to ingitdb://...` will materialize content in the configured format.

## Outstanding Questions

- ~~Project-config file name and location.~~ **Resolved:** `.ingitdb/settings.yaml`, governed by `pkg/ingitdb/config.Settings` struct (constants `IngitDBDirName` and `SettingsFileName`).
- ~~Existing project-config infrastructure.~~ **Resolved:** `config.RootConfig` + `config.Settings` already exist. Extension: add a `RecordFormat` field with `yaml:"record_format,omitempty"`. The loader already handles missing optional fields correctly.
- **CSV file extension.** `.csv` is the natural choice; confirm no conflicts with existing inGitDB file conventions inside `.collection/` folders.
- **CSV header row + schema co-location.** Is the first row of a CSV file column headers (matching the collection's column definitions)? How does the reader handle a CSV file whose header doesn't match the current schema? Decide at specify time.
- **CSV column ordering.** The collection's schema defines an ordered list of columns. Does the CSV writer emit columns in schema order, alphabetical order, or insertion order? Schema-order is the obvious choice; confirm.
- **CLI command surface for collection creation.** Identify which existing `ingitdb` CLI command(s) create collections at specify time.
- **`--format` precedence vs project-level default.** When a user runs the project-creation command with `--format json` AND the project-config file (yet to be created) doesn't yet exist, the flag's value populates the new file. But what if the user runs collection-creation with `--format json` against a project whose `recordFormat: yaml` is already set? The flag wins for that one collection. Document explicitly.
- **Naming: `recordFormat` vs `defaultRecordFormat`.** Project-level field name. Defer to specify time.

---
*This document follows the https://specscore.md/idea-specification*
