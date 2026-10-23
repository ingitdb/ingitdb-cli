---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Merge driver

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/merge-driver?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/merge-driver?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/merge-driver?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/merge-driver?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

`ingitdb merge-driver` is a Git merge driver: Git calls it while it merges, instead of merging the
file line by line. It merges inGitDB files by what they mean. Generated, sorted files (materialized
views and indexes under `$ingitdb/`, and DataTug's project manifest) are merged as sets of entries
and then regenerated from the merged tree before the merge is committed. Record files are merged
field by field. Edits to different fields merge cleanly, and a real conflict on one field stays a
conflict, reported field by field with readable conflict markers.

The Go core is a library in `ingitdb-go`. The datatug-cli reuses it as `datatug merge-driver` for
DataTug project files and the project manifest
([datatug/backstage `project-manifest`](https://github.com/datatug/backstage/blob/main/spec/features/project-manifest/README.md)).
The founder, 2026-10-23: very interested "to have that for datatug and for ingitdb specifically".

`ingitdb resolve` ([`cli/resolve`](../resolve/README.md)) already resolves conflicts that a merge
left behind. This Feature moves the same engine to the moment Git merges, so most of those conflicts
never appear.

## Problem

Git merges line by line. inGitDB's files are records, not lines:

- Two branches that each add a record to the same list file, or a row to the same sorted view, change
  neighbouring lines. Git then reports a conflict although the records are independent.
- Two branches that edit different fields of one record on adjacent lines conflict in the same way.
- A generated file (a view, an index, a manifest) conflicts whenever its sources change on both sides.
  The right answer is to regenerate it, never to hand-edit it.

Today the user runs `ingitdb resolve` after Git stops. That works only with the CLI at hand, after
the merge has already failed, and for one Git operation at a time. A driver runs inside every local
`git merge`, `git pull`, `git rebase` and `git cherry-pick`, whatever tool started it.

## Behavior

### The driver contract

#### REQ: git-driver-contract

`ingitdb merge-driver %O %A %B %L %P %S %X %Y` MUST follow Git's merge-driver contract (gitattributes,
"Defining a custom merge driver"). It reads the ancestor (`%O`), current (`%A`) and other (`%B`)
versions. It writes the result over `%A`. It exits `0` for a clean merge and `1` for a conflict, and
never with any other status. `%L` is the conflict-marker size, and `%S`, `%X` and `%Y` are the marker
labels. When Git is older than 2.44 and does not pass `%S`/`%X`/`%Y`, the labels default to `base`,
`ours` and `theirs`. Git runs the driver with the working-tree root as its current directory. Any
internal error MUST NOT crash the merge: the driver falls back to a plain text merge
(REQ:safe-fallback) and exits 0 or 1 by the result.

#### REQ: file-classes

The driver classifies `%P` against the database definition in the working tree, and DataTug against
`.datatug.yaml`:

| Class | Paths | Merge |
|---|---|---|
| **Generated set** | `$ingitdb/**` views and indexes (INGR, JSON, CSV), and DataTug `**/.datatug/manifest/*.json` | REQ:generated-set-merge |
| **Record file** | files of a collection, in any layout `record_file.type` allows (one record per file, list, map) and any format | REQ:record-field-merge |
| **Anything else** | every other path the attribute matched | the text merge Git itself would do (REQ:safe-fallback) |

Collection `README.md` files stay with `ingitdb resolve`, which regenerates them. They are not in the
attribute patterns.

### Generated files

#### REQ: generated-set-merge

A generated file MUST be merged as a three-way set of entries keyed by its identity. The keys are:

- `$ID` for an INGR row (one INGR record spans one line per column, so rows are merged as whole
  records, never as single lines);
- the primary key for a JSON or CSV view row;
- `path` for a DataTug manifest section item;
- `name` for a DataTug manifest `index.json` entry.

An entry added, changed or removed on one side only is taken from that side. An entry changed the same
way on both sides is taken once. When both sides change the same key differently, the driver takes the
ours value and queues the file for regeneration. The file is always written back in the generator's
canonical form and sort order, and the driver exits 0. A file it cannot parse goes to
REQ:safe-fallback.

#### REQ: regenerate-before-commit

A driver run sees only three versions of one file, not the merged tree. So a generated file is not
final until it has been regenerated from the merged tree. The driver MUST add every generated path it
merges to a pending list at `$(git rev-parse --git-path ingitdb/regenerate)`, one per worktree. Then:

- **`git merge` with no conflicts:** Git runs the `pre-merge-commit` hook. The hook regenerates the
  pending files (`ingitdb resolve --regenerate-pending`; for DataTug, `datatug validate --fix --index`).
  If nothing changed, it clears the list and exits 0, and the merge commits. If anything changed, it
  stages the files, prints "Regenerated N derived files. Run `git commit` to finish the merge.", and
  exits 1. Git keeps `MERGE_HEAD`, and `git commit` then commits the regenerated files.

  A hook cannot simply stage the files and exit 0. Git 2.54 does not include files that a
  pre-merge-commit hook stages in the merge commit: the commit kept the old content, and the hook's
  change stayed staged afterwards. The stop-and-commit flow was verified on the same version:
  `git commit` recorded both parents and the regenerated content.
- **A merge with conflicts, finished with `git commit`:** the `pre-commit` hook regenerates and stages,
  and the commit includes the files.
- **`git rebase`, `git cherry-pick` and `git stash pop`:** these run neither hook. The list stays, and
  `ingitdb rebase`, `ingitdb pull`, any later `ingitdb`/`datatug` write command, or the next commit's
  pre-commit hook regenerates. Until then, CI's stale check catches a stale file
  (REQ:hosted-merges-and-ci).

Regeneration also validates the merged tree (`ingitdb validate` for the collections the merge
touched). A validation failure stops the commit and names the record. This is case DM-17 of
[record-merge](../resolve/auto-resolve/record-merge/README.md), applied to the merged tree.

### Record files

#### REQ: record-field-merge

A record file MUST be merged with the record-merge engine of
[`cli/resolve/auto-resolve/record-merge`](../resolve/auto-resolve/record-merge/README.md), with the
same case catalogue DM-1 to DM-17, the same strict typed equality and the same rule never to drop a
change. In addition:

- **Field granularity:** nested maps are merged key by key, using the path to each key. Lists are
  single values. A field is a path such as `address.city`.
- **Same record, different fields (DM-9 to DM-11):** the driver merges these cleanly when the effective
  `record_merge.same_record` is true. Open Question 1 recommends making true the default. Edits to
  disjoint fields are the case the founder asked to merge.
- **Partial result:** a contested field does not stop the rest of the merge. Every uncontested change
  is applied, and only the contested fields carry conflict markers (REQ:conflict-markers).
- **Schema:** the driver reads the collection definition from the working tree. When all three
  versions parse with it, values are typed by the schema. Otherwise the merge is schema-less (values as
  parsed, kinds compared strictly), and the regeneration step's validation decides (REQ:regenerate-before-commit).

#### REQ: conflict-markers

When fields stay contested, the driver writes the record in canonical form with Git-style markers
around the contested fields only, and exits 1. It uses `%L` for the marker size and adds the base
section when `merge.conflictStyle` is `diff3` or `zdiff3`. In the example below, every line is indented by two spaces so that this spec does not trip conflict-marker checks; the real output is not indented:

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

Each side's block holds complete lines for that field, as the serialiser would write them if that
side won. In JSON, that includes the trailing comma the field's position needs. Deleting the markers
and one side therefore leaves a valid file. The driver also prints one line per contested field to
standard error:

```
CONFLICT (field) companies/acme.yaml: country: base "uk", ours "gb", theirs "ie"
```

Each value is truncated to 80 characters. The driver appends the same facts as JSON lines to
`$(git rev-parse --git-path ingitdb/conflicts.jsonl)`. Git keeps the three versions in the index
(stages 1 to 3), so `ingitdb resolve`'s manual resolution works unchanged.

### Edge cases

#### REQ: deletes-renames

- **One record per file:** a delete on one side and an edit on the other is a tree-level
  modify/delete conflict. Git reports it without calling the driver, and `ingitdb resolve` handles it.
- **List and map files:** a record deleted on one side and edited on the other is DM-15. The record is
  written in full between markers, with an empty block for the deleting side.
- **Renamed record file (one record per file):** Git's rename detection pairs the old and new paths and
  calls the driver with the merged path. The driver then resolves the collection from that path. A
  rename combined with large edits that fall below Git's similarity threshold reaches Git as a
  delete plus an add, and is left to Git.
- **A changed key inside a list or map file:** this is a delete plus an add (DM-15 if the other side
  edited the old key).

#### REQ: schema-changes

When a collection's `.definition.yaml` changes in the same merge, the driver cannot know in which
order Git merges the definition and the records. It MUST NOT trust either schema: it merges any version
that does not parse under the working-tree definition schema-less. The regeneration step's validation
of the merged tree is the authority. A conflict in the definition file itself is a text conflict for a
person to resolve.

#### REQ: binary-large-and-line-endings

- **Binary:** content with a NUL byte or invalid UTF-8 is treated as Git's built-in `binary` driver
  treats it. The driver keeps ours, exits 1 and prints that the file is not text.
- **Large:** a version over 32 MiB (`ingitdb.mergeDriver.maxBytes`), or a merge that takes longer than
  10 s, goes to the text fallback.
- **Line endings:** LF, CRLF, mixed endings and a UTF-8 byte order mark are accepted on any side.
  Differences in line endings alone are representation noise (DM-7). The result uses the dominant line
  ending of `%A` and keeps `%A`'s byte order mark.
- **Virtual merge bases:** in a criss-cross merge, Git calls the driver to build a virtual ancestor.
  The driver MUST still write a result, with markers if needed, and exit 0 or 1. `merge.<name>.recursive`
  is left unset.

### Installation and fallback

#### REQ: install

Git does not carry `merge.<name>.driver` configuration in the repository, so every clone needs it set:

- `ingitdb setup` (a new database) and a new idempotent `ingitdb merge-driver install` (an existing
  database; `setup` refuses to run on an initialised directory) MUST:
  - write a marked block into `.gitattributes`. The block holds one `merge=ingitdb` pattern per
    collection directory and record extension, plus `$ingitdb/** merge=ingitdb`, and is generated from
    the definition;
  - set the local Git configuration `merge.ingitdb.name` and `merge.ingitdb.driver`;
  - install `pre-merge-commit` and `pre-commit` hooks that honour `core.hooksPath`, add a marked block
    to an existing hook and never replace it.

  `--global` writes the driver configuration to the user's global Git configuration instead, so every
  clone on the machine is covered.
- `datatug init` does the same with the driver name `datatug-manifest`. Its single manifest pattern is
  `**/.datatug/manifest/*.json` (the `**/` is needed because a pattern containing a slash is anchored
  to the `.gitattributes` directory). It adds patterns for project item files when Open Question 3 is
  approved.
- `ingitdb validate` warns when the `.gitattributes` block no longer matches the collections, and when
  the driver is configured in neither the local nor the global Git configuration.

#### REQ: safe-fallback

The driver is never configured for a clone that has the attributes but not the configuration, or for a
machine without the binary. Both cases MUST degrade to Git's normal text merge and never lose a side:

- **Attribute without configuration:** Git uses its built-in text merge. This was verified with Git
  2.54: a path whose `merge=` names an undefined driver merged with the built-in text merge, and
  non-overlapping changes merged cleanly.
- **Configuration without the binary:** the configured command is a wrapper. If the binary is missing,
  or exits with a status other than 0 or 1, the wrapper runs
  `git merge-file -L <ours> -L <base> -L <theirs> --marker-size=%L %A %O %B`, which is the same text
  merge with markers. A missing binary must not leave ours without markers, which is what a plain failed
  command does. That would invite the user to stage a file that silently drops theirs.
- **Internal error:** the driver itself falls back to the same text merge.

#### REQ: hosted-merges-and-ci

GitHub's merge button, its "mergeable" check, its web conflict editor, CI runners and Git libraries
(go-git, the GitHub merges API) do not run custom drivers. Each of them merges as plain text:

- A pull request that GitHub reports as conflicting is merged locally, where the driver runs. Then it
  is pushed. The repository README says so.
- A text merge on GitHub can succeed but leave a generated file stale. `ingitdb validate` (and
  `datatug validate` for manifests), run by CI on the pull request's merge ref and on the target
  branch, fails on a stale generated file and on an invalid record. CI never commits a fix.
- Repositories that use the driver SHOULD require branches to be up to date before merging, or use a
  merge queue, so CI checks the exact merge result.
- A server that merges with a library (DataTug Cloud's merge to a target branch) MUST regenerate
  generated files on the merge result before it reports success, or report that the merge needs a
  local resolution.

### Library and reuse

#### REQ: go-library

The core MUST be a package in `ingitdb-go` (`ingitdb/mergedriver`) built on the existing
`ingitdb/recordmerge`. It MUST have no dependency on the CLI. It exposes:

- one entry point that takes the three versions, the path, the marker size and labels, and a file
  classifier;
- a registry of file kinds. Each kind has a key function, a canonical serialiser, a flag that says
  whether it is regenerated, and an optional schema lookup.

The result carries the bytes, clean or conflicted, the conflicts by field, and the paths to
regenerate. `recordmerge.Outcome`, which today escalates a whole file with one reason, gains conflicts
by field and a partial merge. The datatug-cli registers its kinds (manifest index and sections keyed as
in REQ:generated-set-merge, and project item JSON as schema-less records) and supplies its own
regeneration command. New code has 100% statement coverage.

## Dependencies

- cli/resolve
- cli/setup
- cli/validate

## Acceptance Criteria

### AC: disjoint-fields-merge-clean (verifies REQ:record-field-merge, REQ:git-driver-contract)

**Given** `same_record: true` and a one-record-per-file YAML record where branch A changes `name` and branch B changes the adjacent line `country`
**When** `git merge B` runs on A with the driver installed
**Then** the merge succeeds with no conflict, the record has both changes, and `ingitdb validate` passes.

### AC: same-field-conflict-readable (verifies REQ:conflict-markers)

**Given** both branches set `country` of the same record to different values, and `merge.conflictStyle=diff3`
**When** the merge runs
**Then** it stops, the file holds markers around the `country` line only with a base section, the other fields are merged, standard error prints the one-line CONFLICT report, and removing the markers and one side yields a file that parses.

### AC: json-conflict-keeps-commas (verifies REQ:conflict-markers)

**Given** a JSON record whose last field `b` is set to different values on each side, while another field was added on one side
**When** the merge runs
**Then** each side's block between the markers is valid as the last field, so choosing either side leaves valid JSON.

### AC: list-file-disjoint-additions (verifies REQ:record-field-merge)

**Given** a `[]map[string]any` YAML list file where each branch appends a record with a different ID at the end
**When** the merge runs
**Then** it is clean and the file holds both records.

### AC: ingr-rows-merge-as-records (verifies REQ:generated-set-merge)

**Given** an INGR view where branch A adds row `$ID=fr` and branch B adds row `$ID=fi`, adjacent in sort order
**When** the merge runs
**Then** the file holds both rows, each with all of its column lines, in canonical order, and the path is queued for regeneration.

### AC: regeneration-before-merge-commit (verifies REQ:regenerate-before-commit)

**Given** a merge whose only text conflicts would be in a view and a DataTug manifest section, both of which the driver merges cleanly, and whose regenerated view differs from the set merge
**When** `git merge` runs with the hooks installed
**Then** the merge stops with "Regenerated N derived files. Run `git commit` to finish the merge.", and after `git commit` the merge commit has both parents, regenerated files and a passing `ingitdb validate`.

### AC: manifest-index-recomputed (verifies REQ:generated-set-merge, REQ:regenerate-before-commit)

**Given** two branches that each add a different query to the same DataTug project
**When** they merge with `datatug init` applied
**Then** `queries.json` holds both queries, and `index.json` carries the object ID and count of the merged `queries.json` in the merge commit.

### AC: missing-driver-config (verifies REQ:safe-fallback)

**Given** a fresh clone with the committed `.gitattributes` but no driver configuration
**When** a merge with overlapping record edits runs
**Then** Git performs its normal text merge with standard markers, and no side's content is lost.

### AC: missing-binary (verifies REQ:safe-fallback)

**Given** a configured driver whose binary is not on `PATH`
**When** a merge with overlapping edits runs
**Then** the file holds standard text markers containing both sides, not ours alone, and the path is reported as conflicted.

### AC: github-text-merge-caught (verifies REQ:hosted-merges-and-ci)

**Given** a pull request that GitHub merges with its text merge and whose merged view is stale
**When** CI runs on the merge ref
**Then** `ingitdb validate` fails naming the stale view, and CI creates no commit.

### AC: delete-versus-edit (verifies REQ:deletes-renames)

**Given** in turn a one-record-per-file record deleted on A and edited on B, and a list-file record deleted on A and edited on B
**When** each merge runs
**Then** the first is Git's modify/delete conflict and the driver is not called; the second holds the edited record between markers with an empty block for the deleting side, and the driver exits 1.

### AC: rename-with-edit (verifies REQ:deletes-renames)

**Given** A renames `companies/acme.yaml` to `companies/acme-ltd.yaml`, and B edits `employees` in `acme.yaml`
**When** the merge runs
**Then** `companies/acme-ltd.yaml` carries B's edit, and no `acme.yaml` remains.

### AC: schema-change-in-merge (verifies REQ:schema-changes)

**Given** A changes column `employees` from int to string in `.definition.yaml`, and B edits `employees` values as ints
**When** the merge runs
**Then** the records merge schema-less, and the regeneration step's validation stops the commit naming each record that violates the merged schema.

### AC: binary-large-eol (verifies REQ:binary-large-and-line-endings)

**Given** in turn a matched path holding a NUL byte, a 40 MiB record file, and a record where only line endings differ between the sides (CRLF and LF)
**When** each merge runs
**Then** the first keeps ours and exits 1 saying it is not text; the second is merged as text by the fallback; the third merges clean with `%A`'s line endings.

### AC: install-idempotent (verifies REQ:install)

**Given** a repository with `core.hooksPath=.git-hooks` and an existing `.git-hooks/pre-commit`
**When** `ingitdb merge-driver install` runs twice
**Then** `.gitattributes` holds one marked block, the local Git configuration holds the driver once, and each hook keeps its own content plus one marked block.

### AC: datatug-reuses-library (verifies REQ:go-library)

**Given** datatug-cli built against `ingitdb-go`'s `mergedriver` package
**When** its tests run the manifest and item-file scenarios above through `datatug merge-driver`
**Then** they pass with no merge logic of its own beyond registering kinds and the regeneration command.

## Open Questions

1. **Should disjoint fields of the same record merge by default?** `record_merge.same_record` is
   `false` by default today, so cases DM-9 to DM-11 stop. Recommended: **make `true` the default** for
   both the driver and `resolve`. The founder asked for disjoint field edits to merge cleanly, and Git's
   own text merge already merges such edits when the lines are not adjacent. This amends the Stable
   record-merge spec.
2. **Should the driver configuration be local by default or global?** Recommended: **local by default,
   with `--global` as an option**. Local configuration changes nothing outside the repository. A user
   with many repositories can opt in once.
3. **Should DataTug's driver merge project item files too, or only the manifest?** Recommended: **the
   manifest first, then item JSON files** (queries, boards, …) as schema-less records, once the manifest
   ships. Item files are hand-edited JSON with nested maps, and they gain the most from merging field by
   field.
4. **How should lists inside a record merge?** Recommended: **as single values in this Feature**, and
   lists of objects with an `id` key merged by `id` in a follow-up. That covers query parameters and
   board rows.
5. **Where does this spec live?** It is placed in ingitdb-cli next to `cli/resolve` and `cli/setup`,
   because the engine, the case catalogue and the install command are specified there. The library
   part (REQ:go-library) is built in `ingitdb-go`. Recommended: **keep it here** and link it from an
   `ingitdb-go` Feature when the package is planned.

---
*This document follows the https://specscore.md/feature-specification*
