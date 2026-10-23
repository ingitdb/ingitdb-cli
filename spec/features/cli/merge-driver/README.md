---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Merge driver

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/merge-driver?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/merge-driver?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/merge-driver?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/merge-driver?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

`ingitdb merge-driver` is a Git merge driver: Git calls it, instead of its own line merge, for each
file that both sides changed. Git's own text merge always runs first, and its result is kept whenever
it is clean and valid. Only when that merge conflicts, or produces a file that does not parse, does
the driver merge by meaning:

- **Record files** (YAML and JSON) are merged field by field. Edits to different fields merge cleanly.
  A real conflict on one field stays a conflict, reported field by field, with markers around that
  field only.
- **Generated files** (DataTug's project manifest, and inGitDB's materialized views and indexes under
  `$ingitdb/`) are merged as sets of entries where that is exact. Then they are regenerated from the
  merged tree before the merge is committed.

The Go core is a library in `ingitdb-go`. The datatug-cli reuses it as `datatug merge-driver` for the
project manifest
([datatug/backstage `project-manifest`](https://github.com/datatug/backstage/blob/main/spec/features/project-manifest/README.md)).
The founder, 2026-10-23: very interested "to have that for datatug and for ingitdb specifically".

`ingitdb resolve` ([`cli/resolve`](../resolve/README.md)) already resolves conflicts that a merge
left behind. This Feature moves the same engine to the moment Git merges, so most of those conflicts
never appear.

## Problem

Git merges line by line, but inGitDB's files are records, not lines. Three cases produce conflicts
that are not real:

- Two branches that each add a record to the same list file, or a row to the same sorted file, change
  neighbouring lines. Git reports a conflict although the records are independent.
- Two branches that edit different fields of one record on adjacent lines conflict in the same way.
- A generated file conflicts whenever its sources change on both sides. The right answer is to
  regenerate it, never to hand-edit it.

Today the user runs `ingitdb resolve` after Git stops. That needs the CLI at hand, and works only after
the merge has already failed. A driver runs inside every local `git merge`, `git pull`, `git rebase`
and `git cherry-pick`, whatever tool started it.

## Behavior

### Delivery phases

- **Phase 1:** record files (REQ:never-worse-than-text, REQ:record-field-merge, REQ:conflict-markers),
  the edge cases, installation of the attributes and the driver configuration, the self-proving wrapper
  and the fallback. Phase 1 needs no hooks, and its installer writes none.
- **Phase 2:** generated files and regeneration before commit (REQ:generated-files,
  REQ:regenerate-before-commit, REQ:check-and-regenerate-modes).

### The driver contract

#### REQ: git-driver-contract

`ingitdb merge-driver %O %A %B %L %P %S %X %Y` MUST follow Git's merge-driver contract (gitattributes,
"Defining a custom merge driver"):

- It reads the ancestor (`%O`), current (`%A`) and other (`%B`) versions, and writes the result over
  `%A`.
- It exits `0` for a clean merge and `1` for a conflict, and with no other status.
- `%L` is the conflict-marker size.
- `%S`, `%X` and `%Y` are the marker labels. Git older than 2.44 passes them through as the literal
  strings `%S`, `%X` and `%Y`; the driver treats those as absent and uses `base`, `ours` and `theirs`.

Git runs the driver from the working-tree root. During the merge the working tree holds ours'
pre-merge files: merge-ort merges in memory and updates the working tree at the end. Any internal
error MUST NOT crash the merge. The driver falls back to the text result (REQ:safe-fallback) and exits
0 or 1 according to that result.

#### REQ: never-worse-than-text

The driver MUST first run Git's three-way text merge in memory. It uses `git merge-file -p` with the
same marker size, labels and conflict style, and with `--diff-algorithm=histogram` (Git 2.44 and later).
merge-ort uses histogram, and `git merge-file` defaults to Myers, so without the option the results can
differ. If that merge is clean, the driver writes it byte for byte and exits 0, provided the result:

- parses in the file's format, with no duplicate object key and no duplicate record ID. Go's
  `encoding/json` silently keeps the last of duplicate keys, so a clean text merge that adds `"x"` in
  two places would otherwise drop a value without notice.
- for a record file, validates against ours' collection schema.

For a generated file, the text result supplies only the content. Queuing and the exit rule of
REQ:generated-files apply whether the text merge was clean or not, because a text-clean generated file
can still be stale.

Comments, hand formatting and anything Git merges cleanly today therefore stay exactly as Git would
leave them. The storage-format no-reformatting rule (`storage-format#req:no-rewrite-without-change`)
holds. Only a text conflict, or a clean text result that fails one of these checks, goes on to the
merge by meaning.

#### REQ: file-classes

The driver classifies `%P` against the database definition in the working tree, and, for DataTug,
against `.datatug.yaml`:

| Class | Paths | Merge after a text conflict |
|---|---|---|
| **Record file** | YAML or JSON files of a collection, in any `record_file.type` layout | REQ:record-field-merge |
| **Generated file** | `$ingitdb/**`, and DataTug `**/.datatug/manifest/*.json` | REQ:generated-files |
| **Anything else** | every other path the attribute matched, including TOML, Markdown, INGR and CSV record files | the text result, with its markers |

TOML, Markdown, INGR and CSV record files stay with the text result in this Feature, because markers
inside their framing need a design of their own (Open Question 4). Collection `README.md` files are not
in the attribute patterns. `ingitdb resolve` regenerates them.

### Record files

#### REQ: record-field-merge

A record file MUST be merged with the record-merge engine of
[`cli/resolve/auto-resolve/record-merge`](../resolve/auto-resolve/record-merge/README.md). It uses the
same case catalogue (DM-1 to DM-17), the same strict typed equality and the same rule never to drop a
change. This Feature extends the engine, which amends that Stable spec:

- **Field granularity:** nested maps are merged key by key, along the path to each key, so
  `address.city` is a field. Today `recordmerge` merges top-level fields only. Lists are single values.
- **Same record, different fields (DM-9 to DM-11):** these merge cleanly when the effective
  `record_merge.same_record` is true. Open Question 1 recommends making true the default, because edits
  to disjoint fields are the case the founder asked to merge. When it is false, every field both sides
  changed is contested, and the whole record goes between markers.
- **Partial result:** a contested field does not stop the rest of the merge. Every uncontested change
  is applied, and only the contested parts carry markers (REQ:conflict-markers).
- **Schema:** the schema is ours' pre-merge collection definition from the working tree. When all three
  versions parse with it, values are typed by it. Otherwise the merge is schema-less: values are taken
  as parsed and their kinds are compared strictly. The validation in REQ:regenerate-before-commit, or
  CI, decides.
- **Record IDs:** a record that stores its own ID must still match its file name after the merge.
  Otherwise the result is a conflict. This catches Git's rename detection pairing two different small
  records.

#### REQ: conflict-markers

When contested parts remain, the driver writes the record with Git-style markers around them only,
and exits 1. It uses `%L` for the marker size, and adds a base section when `merge.conflictStyle` is
`diff3` or `zdiff3`. Placement:

| Case | Between the markers |
|---|---|
| a contested field (DM-13, DM-14, DM-16) | that field's lines |
| the same ID added with different content (DM-12) in a list or map file | each side's whole record |
| delete versus edit (DM-15) in a list or map file | the edited record, and an empty block for the deleting side |
| `same_record: false`, both sides changed one record | each side's whole record |

Each side's block holds complete lines, as the serialiser would write them if that side won. In JSON
that includes the comma the position needs. Deleting the markers and one side therefore leaves a valid
file.

Whenever the driver merges by meaning, after a text conflict, it writes the whole file in the
collection's canonical form, whether or not markers remain. Comments and hand formatting in that file
are then lost. That is the trade-off for resolving a conflict that Git's text merge could not. It
applies only to files whose text merge conflicted, or that failed the checks of
REQ:never-worse-than-text.

In the example below every line is indented by two spaces, so that this spec does not trip
conflict-marker checks. Real output is not indented.

```yaml
  name: Acme
  <<<<<<< ours
  country: gb
  ||||||| base
  country: uk
  =======
  country: ie
  >>>>>>> theirs
  employees: 120
```

The driver prints one line per contested part to standard error, with each value truncated to 80
characters:

```
CONFLICT (field) companies/acme.yaml: country: base "uk", ours "gb", theirs "ie"
```

It appends the same facts as JSON lines to `$(git rev-parse --git-path ingitdb/conflicts.jsonl)`.
Git keeps the three versions in the index (stages 1 to 3), so `ingitdb resolve`'s manual resolution
works unchanged.

### Generated files

#### REQ: generated-files

Each generated kind declares whether a set merge of its entries is exact:

- **Exact keyed sets.** A DataTug manifest section is keyed by `path`, and each entry depends only on
  its own item. An inGitDB foreign-key index (`$ingitdb/**/$fk/**`) is keyed by `$ID`. After a text
  conflict, these files are merged as a three-way set:
  - an entry added, changed or removed on one side only is taken from that side;
  - an entry changed the same way on both sides is taken once.
  For INGR, the header line (`# INGR.io | <name>: <columns>`) MUST be identical on all three sides.
  Otherwise the kind is not exact for this merge. A row is merged as its whole block of one line per
  column, never line by line. The `# N records` footer is recomputed.
- **Derived files** are everything else: the DataTug `index.json`, top-N views, aggregate views,
  joined root-level views, an exact kind with a contested key, and an INGR file whose headers differ.
  The driver takes ours for these.

Every generated file the driver writes, exact or derived, is queued for regeneration
(REQ:regenerate-before-commit). The exit status depends on whether Git will run the pre-merge-commit
hook. The driver trusts one positive signal: `GIT_REFLOG_ACTION` starts with `merge`, or starts with
`pull` while no rebase is in progress. It looks for `rebase-merge` and `rebase-apply` with
`git rev-parse --git-path`, so linked worktrees work. Git 2.54 sets this variable as follows:
`merge` and `merge --squash` set `merge …`, and a merging pull sets `pull …`. Rebases, picks, reverts
and `stash pop` leave it unset or set `rebase (pick)`. The signal has one known limit: a user or tool
that exports `GIT_REFLOG_ACTION=merge…` around another operation gets a false positive. The CI check
catches the stale file that results.

- **When the signal holds,** the driver exits 0, and the hook regenerates before the commit.
- **Otherwise:** a rebase, a single or ranged cherry-pick, a revert, `stash pop`, `checkout -m`, and
  every operation Git adds later. No hook will run, so only an exact merge with no contested key exits
  0. A derived file exits 1, keeping ours and printing "needs regeneration: run `ingitdb resolve` (or
  `datatug validate --fix`), then continue".

#### REQ: regenerate-before-commit

The driver sees only three versions of one file, not the merged tree. So a generated file is not final
until it has been regenerated from the merged tree.

Git calls the driver only for paths that both sides changed. A derived view that changed on one side
only, and that depends on sources the other side changed, is never routed to the driver. So the hooks
do not rely on the driver's list. They regenerate every generated file affected by the source paths
the merge changed. They diff the index against `HEAD` (`git diff --cached --name-only HEAD`). When
the pre-merge-commit hook runs, `MERGE_HEAD` does not exist yet (checked with Git 2.54), so the other
parent cannot be named. Diffing against `HEAD` is enough when both parents were consistent when
committed, which CI checks.

The driver still adds each generated path it handles to `$(git rev-parse --git-path ingitdb/regenerate)`,
a list kept per worktree. The list drives the "needs regeneration" message and the post-rewrite
warning, and is cleared by a successful regeneration. A list left behind by `git merge --abort` or
`git rebase --abort` only causes one extra regeneration later, which changes nothing. Virtual-base calls
(REQ:virtual-base) queue nothing.

How regeneration runs:

- **`git merge` without conflicts:** Git runs the `pre-merge-commit` hook. The hook runs
  `ingitdb resolve --regenerate-pending` (for DataTug, `datatug validate --fix --index`).
  - If nothing changed, it exits 0, and the merge commits.
  - If anything changed, it stages the files, prints "Regenerated N derived files. Run `git commit` to
    finish the merge.", and exits 1. Git keeps `MERGE_HEAD`, and `git commit` then commits the
    regenerated files.

  A hook cannot simply stage the files and exit 0. Checked with Git 2.54: files a pre-merge-commit
  hook stages are not included in the merge commit, and they stay staged afterwards. The stop-and-commit
  flow was checked on the same version: `git commit` recorded both parents and the regenerated content.
- **A merge with conflicts, finished with `git commit`, and `git cherry-pick --continue`:** the
  `pre-commit` hook regenerates and stages, and the commit includes the files.
- **`git rebase`, and a clean `git cherry-pick`:** neither hook runs, so derived files stop the
  operation (REQ:generated-files). `ingitdb rebase` and `ingitdb pull` regenerate after each step. A
  `post-rewrite` hook warns when a pending list remains after a rebase.

Regeneration also validates the collections the merge touched. A validation failure stops the commit
and names the record (DM-17, applied to the merged tree).

#### REQ: check-and-regenerate-modes

This Feature adds two modes, and amends `cli/validate` and `cli/resolve` to match:

- `ingitdb validate --check-generated` regenerates views and indexes in memory, compares them with the
  files, and writes nothing. It exits non-zero and names every stale file. CI uses this mode. The
  materialized-views rule `rebuild-on-validate` still applies to plain `ingitdb validate`.
- `ingitdb resolve --regenerate-pending` regenerates every generated file affected by the source
  paths staged against `HEAD`, plus the paths on the pending list. It stages them, validates the touched collections, and clears the
  list.

A one-line pointer to each mode is added to the `cli/validate` and `cli/resolve` specs in this pull
request. Their full amendment lands with phase 2.

Here, "indexes" are the foreign-key index files under `$ingitdb/**/$fk/`
(materialized-views#req:output-under-$ingitdb).

### Edge cases

#### REQ: deletes-renames

- **One record per file:** a delete on one side and an edit on the other is a tree-level
  modify/delete conflict. Git reports it without calling the driver, and `ingitdb resolve` handles it.
- **List and map files:** a record deleted on one side and edited on the other is DM-15, placed as
  REQ:conflict-markers says.
- **Renamed record file (one record per file):** Git's rename detection pairs the old and new paths
  and calls the driver once, with `%P` set to the new path. The driver resolves the collection from
  that path. A rename combined with large edits, below Git's similarity threshold, reaches Git as a
  delete plus an add, and is left to Git.
- **A changed key inside a list or map file:** this is a delete plus an add, so DM-15 applies if the
  other side edited the old key.

#### REQ: schema-changes

When a collection's `.definition.yaml` changes on theirs side, the driver sees only ours' pre-merge
definition. A version that does not parse under it is merged schema-less (REQ:record-field-merge). The
validation of the merged tree, in the hooks or in CI, is the authority. A conflict in the definition
file itself is a text conflict for a person to resolve.

#### REQ: virtual-base

In a criss-cross merge, Git calls the driver for each internal merge between common ancestors, and the
result becomes `%O` of the final merge. `merge.ingitdb.recursive` MUST name a second driver entry,
`ingitdb-base`, which runs `ingitdb merge-driver --virtual-base` through the same wrapper as
REQ:safe-fallback. That mode:

- skips the text-first step and merges by meaning;
- writes a base that parses: a contested field takes the older base value, and an add contested by
  both sides is omitted;
- honours `%L`, which Git raises for internal merges;
- prints nothing and appends nothing to `conflicts.jsonl`;
- queues nothing;
- always exits 0.

"Always exits 0" is a rule for the driver itself. When the binary is missing, the wrapper's
`git merge-file` fallback writes a base that may contain markers, and exits 1. Git treats that as a
conflicted internal merge, and the final merge then text-merges with both sides kept. That was checked
with Git 2.54, and it is safe.

#### REQ: binary-large-and-line-endings

- **Binary:** content with a NUL byte or invalid UTF-8 is treated as Git's built-in `binary` driver
  treats it. The driver keeps ours, exits 1 and prints that the file is not text.
- **Large:** a version over 32 MiB (`ingitdb.mergeDriver.maxBytes`), or a merge by meaning that takes
  longer than 10 s, keeps the text result.
- **Line endings:** LF, CRLF, mixed endings and a UTF-8 byte order mark are accepted on any side.
  Differences in line endings alone are representation noise (DM-7). A merge by meaning writes the
  dominant line ending of `%A` and keeps `%A`'s byte order mark.

### Installation and fallback

#### REQ: install

Git does not carry `merge.<name>.driver` configuration in the repository. So every clone needs it:

- **`ingitdb setup`** (a new database) and a new idempotent **`ingitdb merge-driver install`** (an
  existing database; `setup` refuses to run on an initialised directory) MUST:
  - write a marked block into `.gitattributes`. The block is generated from the definition: one
    `merge=ingitdb` pattern per collection directory and YAML or JSON record extension, plus
    `$ingitdb/** merge=ingitdb`.
  - set the local Git configuration: `merge.ingitdb.name`, `merge.ingitdb.driver` (the wrapper of
    REQ:safe-fallback), `merge.ingitdb.recursive = ingitdb-base`, and `merge.ingitdb-base.driver`.
  - install `pre-merge-commit`, `pre-commit` and `post-rewrite` hook blocks. They honour
    `core.hooksPath`, add a marked block to an existing hook, never replace a hook, and print the line
    to add for a hook that a hook manager owns (wb shims, husky, lefthook, pre-commit).

  `--global` writes the driver configuration to the user's global Git configuration instead, so every
  clone on the machine is covered.
- **`datatug init`** does the same with the driver names `datatug` and `datatug-base`, and the pattern
  `**/.datatug/manifest/*.json text eol=lf merge=datatug`. The `**/` is needed because a pattern that
  contains a slash is anchored to the `.gitattributes` directory. It differs in one way, which
  project-manifest `REQ:hooks` specifies: its pre-commit block regenerates the manifest on every
  commit, not only when the pending list is non-empty.
- **Hook blocks** (phase 2) skip with a one-line warning when the binary is not on `PATH`. The
  ingitdb blocks also skip when the staged change touches no source of a generated file. So the
  ingitdb pre-commit block regenerates views on an ordinary commit exactly when that commit changes
  their sources. Teammates without the CLI can therefore still commit. `--no-verify` skips them too, and CI
  catches anything left stale.
- **`ingitdb validate`** warns when the `.gitattributes` block no longer matches the collections. It
  also warns when the driver is configured in neither the local nor the global Git configuration, but
  not when the `CI` environment variable is set.

#### REQ: safe-fallback

The driver may be configured for a clone that has the attributes but not the configuration, or for a
machine without a suitable binary. Both cases MUST degrade to Git's normal text merge and never lose a
side.

- **Attribute without configuration:** Git uses its built-in text merge. Checked with Git 2.54: a path
  whose `merge=` names an undefined driver merged with the built-in text merge, and non-overlapping
  changes merged cleanly.
- **The wrapper:** the configured command is an inline `sh -c '…'` wrapper in `merge.ingitdb.driver`.
  It needs no file on disk that could go missing.
  - It first runs `ingitdb merge-driver --protocol`, and it trusts the driver only if that prints
    exactly `ingitdb-merge-driver/1`.
  - Otherwise it runs `git merge-file -L <ours> -L <base> -L <theirs> --marker-size=%L
    --diff-algorithm=histogram %A %O %B`, and maps any non-zero result to 1. `git merge-file` exits
    with the number of conflicts.
  - The installer probes once whether the local `git merge-file` accepts `--diff-algorithm`, which
    needs Git 2.44 or later. If not, the installed wrapper and the text-first step omit the option and
    use Git's default diff.
  - A missing binary, an older binary without the subcommand, or a broken one must not leave ours
    without markers, which is what a plain failed command does. Checked with Git 2.54: the file was
    plain ours while Git reported a conflict, which invites the user to stage it and silently drop
    theirs. Today's `ingitdb merge-driver a b c` exits 1 with an error banner, which is why exit 1 alone
    proves nothing.
- **Internal error:** the driver itself writes the text result.

#### REQ: hosted-merges-and-ci

These merge as plain text and run no custom driver:

- GitHub's merge button, its "mergeable" check and its web conflict editor;
- CI runners;
- Git libraries: go-git and the GitHub merges API.

The consequences:

- A pull request that GitHub reports as conflicting is merged locally, where the driver runs, and then
  pushed. The repository README says so.
- A text merge on GitHub can succeed but leave a generated file stale. `ingitdb validate
  --check-generated`, and `datatug validate` for manifests, run by CI on the pull request's merge ref and
  on the target branch, fail on a stale generated file and on an invalid record. CI never commits a fix.
- Repositories that use the driver SHOULD require branches to be up to date before merging, or use a
  merge queue, so CI checks the exact merge result.
- A server that merges with a library MUST regenerate generated files on the merge result before it
  reports success, or report that the merge needs a local resolution. For DataTug Cloud this is
  project-manifest `REQ:cloud-writes-keep-it-current`.

### Library and reuse

#### REQ: go-library

The core MUST be a package in `ingitdb-go`, `ingitdb/mergedriver`, built on the existing
`ingitdb/recordmerge`. It MUST have no dependency on the CLI. It exposes:

- one entry point that takes the three versions, the path, the marker size, the labels, the conflict
  style, a mode (final or virtual base) and a file classifier;
- a registry of file kinds. Each kind has a parser, a canonical serialiser, a key function, a flag for
  an exact or a derived set, and an optional schema lookup.

The result carries the bytes, the exit status, the conflicts by field, and the paths to regenerate.
`recordmerge.Outcome` escalates a whole file with one reason today. It gains conflicts by field and a
partial merge.

The datatug-cli registers its kinds (the manifest index as derived, and the sections as exact keyed by
`path`) and supplies its own regeneration command. New code has 100% statement coverage.

## Dependencies

- cli/resolve
- cli/setup
- cli/validate

## Acceptance Criteria

### AC: text-clean-kept-verbatim (verifies REQ:never-worse-than-text)

**Given** a commented YAML record, where A edits a non-adjacent line and B edits a different item of a `tags` list
**When** `git merge B` runs on A with the driver installed
**Then** the result equals Git's own text merge byte for byte, including the comments, and the driver exits 0.

### AC: disjoint-fields-merge-clean (verifies REQ:record-field-merge, REQ:git-driver-contract)

**Given** `same_record: true`, and a one-record-per-file YAML record where A changes `name` and B changes the adjacent line `country`
**When** the merge runs
**Then** the driver is called with the `%P`, `%S`, `%X` and `%Y` values, writes `%A` and exits 0; the record has both changes; and `ingitdb validate` passes.

### AC: same-field-conflict-readable (verifies REQ:conflict-markers)

**Given** both branches set `country` of one record to different values, with `merge.conflictStyle=diff3` and `conflict-marker-size=9`
**When** the merge runs
**Then** it stops; the file has 9-character markers around the `country` line only, with a base section; the other fields are merged; standard error prints the CONFLICT line; `conflicts.jsonl` gains one entry; and removing the markers and one side yields a file that parses.

### AC: json-conflict-keeps-commas (verifies REQ:conflict-markers)

**Given** a JSON record whose last field `b` is set to different values on each side, while another field was added on one side
**When** the merge runs
**Then** each side's block between the markers is valid as the last field, so choosing either side leaves valid JSON.

### AC: same-record-disabled (verifies REQ:record-field-merge, REQ:conflict-markers)

**Given** `same_record: false` and the inputs of AC disjoint-fields-merge-clean
**When** the merge runs
**Then** the whole record from each side is between the markers, and the driver exits 1.

### AC: list-file-disjoint-additions (verifies REQ:record-field-merge)

**Given** a `[]map[string]any` YAML list file where each branch appends a record with a different ID at the end
**When** the merge runs
**Then** it is clean, and the file holds both records.

### AC: other-formats-text (verifies REQ:file-classes)

**Given** a TOML record and a collection `README.md`, each with overlapping edits
**When** the merge runs
**Then** the TOML file carries Git's text markers; the README is not routed to the driver; and `ingitdb resolve` regenerates the README.

### AC: ingr-index-merge (verifies REQ:generated-files)

**Given** a `$fk` INGR index where A adds row `$ID=fr` and B adds row `$ID=fi`, adjacent in sort order, with identical headers; and separately the same with one side's header changed
**When** each merge runs as a `git merge`
**Then** the first holds both rows with all their column lines and a recomputed `# N records` footer, and is queued; the second keeps ours, is queued, and the driver exits 0.

### AC: regeneration-before-merge-commit (verifies REQ:regenerate-before-commit, REQ:check-and-regenerate-modes)

**Given** a merge whose only text conflicts are in a top-N view and a DataTug manifest section, and whose regenerated view differs from ours
**When** `git merge` runs with the hooks installed
**Then** the merge stops with "Regenerated N derived files. Run `git commit` to finish the merge."; after `git commit`, the merge commit has both parents and the regenerated files; `ingitdb validate --check-generated` passes; and the pending list is empty.

### AC: manifest-index-recomputed (verifies REQ:generated-files, REQ:regenerate-before-commit)

**Given** two branches that each add a different query to the same DataTug project
**When** they merge with `datatug init` applied
**Then** `queries.json` holds both queries, and the merge commit's `index.json` carries the object ID and count of the merged `queries.json`.

### AC: rebase-stops-on-derived (verifies REQ:generated-files, REQ:regenerate-before-commit)

**Given** a branch rebased onto a target where both changed the same section, so the DataTug `index.json` is contested
**When** `git rebase` runs
**Then** it stops at that commit with the "needs regeneration" message; and after `datatug validate --fix`, `git add` and `git rebase --continue`, every rewritten commit passes `datatug validate`.

### AC: check-generated-writes-nothing (verifies REQ:check-and-regenerate-modes, REQ:hosted-merges-and-ci)

**Given** a commit, merged on GitHub with its text merge, whose view is stale
**When** CI runs `ingitdb validate --check-generated` on the merge ref
**Then** it exits non-zero naming the view, the working tree is unchanged, and CI creates no commit.

### AC: missing-driver-config (verifies REQ:safe-fallback)

**Given** a fresh clone with the committed `.gitattributes` but no driver configuration
**When** a merge with overlapping record edits runs
**Then** Git performs its normal text merge with standard markers, and no side's content is lost.

### AC: missing-or-old-binary (verifies REQ:safe-fallback)

**Given** in turn a configured driver whose binary is not on `PATH`, and one whose binary predates this Feature
**When** a merge with overlapping edits runs
**Then** both times the file holds standard text markers containing both sides, the path is reported as conflicted, and the wrapper exits 1.

### AC: internal-error-and-limits (verifies REQ:safe-fallback, REQ:binary-large-and-line-endings)

**Given** in turn an injected panic in the field merge, a merge by meaning that exceeds 10 s, a matched path holding a NUL byte, a 40 MiB record file, and a record whose sides differ only in CRLF versus LF and a byte order mark
**When** each merge runs
**Then** the first, second and fourth keep the text result; the third keeps ours and exits 1 saying it is not text; and the fifth merges clean with `%A`'s line endings and byte order mark.

### AC: delete-versus-edit (verifies REQ:deletes-renames)

**Given** in turn a one-record-per-file record deleted on A and edited on B, and a list-file record deleted on A and edited on B
**When** each merge runs
**Then** the first is Git's modify/delete conflict and the driver is not called; the second holds the edited record between markers with an empty block for the deleting side, and the driver exits 1.

### AC: rename-with-edit (verifies REQ:deletes-renames)

**Given** A renames `companies/acme.yaml` to `companies/acme-ltd.yaml` and edits `name` on the adjacent line, while B edits `employees` in `acme.yaml`
**When** the merge runs
**Then** the driver is called once with `%P` = `companies/acme-ltd.yaml`; that file carries both edits; and no `acme.yaml` remains.

### AC: schema-change-in-merge (verifies REQ:schema-changes)

**Given** A changes column `employees` from int to string in `.definition.yaml`, and B edits `employees` values as ints in conflicting lines
**When** the merge runs
**Then** the records merge schema-less, and the validation in the hook stops the commit naming each record that violates the merged schema.

### AC: criss-cross (verifies REQ:virtual-base)

**Given** a criss-cross history whose two merge bases contest one field of a record, while the final sides edit different fields
**When** the final merge runs
**Then** the virtual base parses; the final merge is clean by field; and no CONFLICT line or `conflicts.jsonl` entry comes from the virtual-base call.

### AC: install-idempotent (verifies REQ:install)

**Given** a repository with `core.hooksPath=.git-hooks` and an existing `.git-hooks/pre-commit`
**When** `ingitdb merge-driver install` runs twice; then `ingitdb setup` runs in a new directory; then `ingitdb merge-driver install --global` runs
**Then** `.gitattributes` holds one marked block matching the collections; the local configuration holds both driver entries once; each hook keeps its content plus one block; the new directory gets the same; and `--global` writes to the global configuration only.

### AC: hooks-without-binary (verifies REQ:install)

**Given** installed hook blocks and a teammate without `ingitdb` on `PATH`
**When** the teammate commits, and CI runs `ingitdb validate` with `CI=true`
**Then** the commit succeeds with one warning line, and CI prints no driver-configuration warning.

### AC: datatug-reuses-library (verifies REQ:go-library, REQ:hosted-merges-and-ci)

**Given** datatug-cli built against `ingitdb-go`'s `mergedriver` package
**When** its tests run the manifest scenarios above through `datatug merge-driver`
**Then** they pass, with no merge logic of its own beyond registering kinds and the regeneration command.

### AC: duplicate-key-not-kept (verifies REQ:never-worse-than-text)

**Given** a JSON record where A adds `"x": 1` near the top and B adds `"x": 2` further down, so the text merge is clean; and a YAML list file where both sides add a record with ID `k` at non-adjacent positions
**When** each merge runs
**Then** neither clean text result is kept; the first becomes a field conflict on `x`; the second places each side's whole record between markers (DM-12); and the driver exits 1 for both.

### AC: single-cherry-pick-stops-on-derived (verifies REQ:generated-files)

**Given** a commit that changes a DataTug section, and a target whose `index.json` changed too
**When** `git cherry-pick` of that one commit runs, and separately `git stash pop` with the same shape
**Then** both stop with the "needs regeneration" message and the driver exits 1 for `index.json`, because `GIT_REFLOG_ACTION` does not start with `merge` or `pull`.

### AC: one-sided-derived-view-regenerated (verifies REQ:regenerate-before-commit)

**Given** a joined root-level view V that branch A's source change updates, while branch B changes another source of V, so Git takes A's V without calling the driver
**When** `git merge B` runs with the hooks installed
**Then** the pre-merge-commit hook regenerates V from the merged tree and stops for `git commit`, and `ingitdb validate --check-generated` passes on the merge commit.

### AC: post-rewrite-warning (verifies REQ:regenerate-before-commit)

**Given** a pending list left non-empty after a rebase that stopped on a derived file and was continued without regenerating
**When** the rebase finishes
**Then** the post-rewrite hook prints one warning naming the list and the regeneration command.

### AC: hook-manager-respected (verifies REQ:install)

**Given** a repository whose hooks are managed by lefthook
**When** `ingitdb merge-driver install` runs
**Then** no hook file is edited, and the installer prints the lefthook lines to add.

### AC: old-git-label-placeholders (verifies REQ:git-driver-contract)

**Given** the driver invoked with the literal arguments `%S`, `%X` and `%Y`, as Git older than 2.44 passes them
**When** a field conflict is written
**Then** the markers carry the labels `base`, `ours` and `theirs`.

### AC: record-id-mismatch (verifies REQ:record-field-merge)

**Given** two small records that Git's rename detection pairs as one rename, so the merged file stores ID `b` in file `a.yaml`
**When** the merge runs
**Then** the driver reports a conflict naming the ID mismatch and exits 1.

### AC: phase-one-installer (verifies REQ:install)

**Given** a phase 1 build
**When** `ingitdb merge-driver install` runs
**Then** it writes the `.gitattributes` block and the driver configuration, and no hook file.

## Open Questions

1. **Should disjoint fields of the same record merge by default?** `record_merge.same_record` is `false`
   by default today, so cases DM-9 to DM-11 stop. Recommended: **make `true` the default** for both the
   driver and `resolve`. The founder asked for disjoint field edits to merge cleanly. Git's own text
   merge already merges such edits when the lines are not adjacent. This amends the Stable record-merge
   spec, together with the nested-map granularity of REQ:record-field-merge.
2. **Should the driver configuration be local by default, or global?** Recommended: **local by
   default, with `--global` as an option**. Local configuration changes nothing outside the repository,
   and a user with many repositories can opt in once.
3. **Should DataTug's driver merge project item files too, or only the manifest?** Recommended: **the
   manifest first**, then item JSON files (queries, boards, …) as schema-less records once the manifest
   ships. Item files are hand-edited JSON with nested maps, and they gain the most from merging field by
   field.
4. **How should TOML, Markdown, INGR and CSV record files merge by meaning?** Recommended: **keep them
   on the text result in this Feature**, then add them one format at a time. INGR and CSV markers would
   wrap whole records, and Markdown would merge its frontmatter by field and its body as text.
5. **How should lists inside a record merge?** Recommended: **as single values in this Feature**, and
   lists of objects with an `id` key merged by `id` in a follow-up. That covers query parameters and
   board rows. The text-first rule already keeps list edits that Git merges cleanly.
6. **Where does this spec live?** It is in ingitdb-cli, next to `cli/resolve` and `cli/setup`, because
   the engine, its case catalogue and the install command are specified there. The library part
   (REQ:go-library) is built in `ingitdb-go`. Recommended: **keep it here**, and link it from an
   `ingitdb-go` Feature when the package is planned.

---
*This document follows the https://specscore.md/feature-specification*
