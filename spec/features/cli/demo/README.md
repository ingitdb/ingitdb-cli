---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Demo Command

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/demo?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/demo?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/demo?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/demo?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

`ingitdb demo install` creates a small ready-made inGitDB database, the TODO demo: two
lists, **To buy** and **To watch**, with a few items each, stored as readable files in a
new Git repository. It is the same demo OpenVaultDB's `ovdb demo install` creates; the demo
records are defined once in the `ingitdb-go` library and both CLIs use them. After installing,
the command tells the person how to browse the lists in the inGitDB terminal UI, how to
query them with `ingitdb select`, and points to OpenVaultDB's web TODO app, which keeps its
own copy of the same lists.

## Synopsis

```
ingitdb demo install                      # create ./todo-demo
ingitdb demo install --path=~/lists       # create the demo somewhere else
ingitdb demo install --format=json        # structured result for scripts and agents
```

## Problem

A newcomer to inGitDB has to write `.ingitdb/` configuration, a collection definition and
records before any other command has something to show. `ingitdb setup` creates an empty
database, and the example databases in `ingitdb/demo-ingitdb` need a clone. There is no
one-command way to see the terminal UI, `select` and Git history working on real data.

OpenVaultDB already ships a TODO demo with the same purpose. Defining a second, different
demo for inGitDB would split the story ("your apps and agents share your data as files")
and let the two drift.

## Behavior

### Invocation

#### REQ: command-shape

The command MUST be invoked as `ingitdb demo install [--path=PATH] [--format=yaml|json]`.
It installs the TODO demo; the command shape MUST leave room for a later `--app <name>`
option without breaking these invocations. `ingitdb demo` without a subcommand MUST print
help listing `install` and exit `0`.

#### REQ: path-flag

`--path=PATH` MUST name the folder to create the demo in. Relative values MUST be resolved
against the current working directory, per
[path-targeting](../../path-targeting/README.md#req-resolves-relative-paths). When `--path`
is omitted the folder MUST be `todo-demo` in the current working directory. This is a
deliberate difference from [path-targeting](../../path-targeting/README.md#req-cwd-default):
the command creates a database rather than operating on one, and the current directory is
rarely empty.

### Data

#### REQ: shared-demo-data

The demo's records (list and item ids, list and item titles, `done` values and the
`added_at` rule) and the list paths MUST come from the shared Go package
`github.com/ingitdb/ingitdb-go/ingitdb/demos/todo`, which OpenVaultDB's built-in TODO demo
also uses. That package imports only the Go standard library. `ingitdb-cli` MUST NOT restate
any of these values, including the list paths it prints (`lists/to-buy`, `lists/to-watch`).
The titles are English demo data owned by the package, not user-interface copy, and are not
localised.

The collection definitions and the demo marker are inGitDB-specific and have one consumer,
so they live in `ingitdb-cli`, not in the shared package.

The installed database contains:

| Record | Data |
|---|---|
| `lists/to-buy` | `title: To buy` |
| `lists/to-buy/items/milk`, `…/bananas`, `…/coffee` | `title` (Milk, Bananas, Coffee), `done: false`, `added_at` |
| `lists/to-watch` | `title: To watch` |
| `lists/to-watch/items/the-matrix`, `…/interstellar` | `title` (The Matrix, Interstellar), `done: false`, `added_at` |

`added_at` values are RFC 3339 UTC timestamps one second apart in the order above, the last
one being the install time truncated to the second, so no item is in the future and every
client lists them in the same order.

#### REQ: valid-ingitdb-database

The installed folder MUST be a complete inGitDB database: `.ingitdb/settings.yaml`,
`.ingitdb/root-collections.yaml` registering `lists`, a definition for the root collection
`lists` and for its subcollection `items`, and one record file per record at the paths the
`dalgo2ingitdb` driver uses (for example `lists/$records/to-buy.yaml` and
`lists/to-buy/items/$records/milk.yaml`). The `items` definition declares `title` (string,
required), `done` (bool) and `added_at` (`datetime`, which accepts RFC 3339 strings). Every
record, root and item, MUST be valid against its definition. `ingitdb validate
--path=<folder>` MUST pass on a freshly installed demo; because `validate` does not yet check
subcollection records, item validity is proved by the definition tests, not by `validate`.

#### REQ: demo-marker

The install MUST record what it installed in `.ingitdb/demo.yaml` (`app: todo` and a format
version). The marker is how a later install recognises the
folder as the demo; the command MUST NOT recognise a demo by folder name or by record
content.

### Installing safely

#### REQ: refuses-conflicting-target

When `--path` names an existing file, or an existing folder that is not empty and has no
TODO demo marker, the command MUST exit non-zero, write nothing, name the folder and suggest
`ingitdb demo install --path=<another folder>`. A folder whose marker names a different demo
MUST be refused the same way. An existing empty folder MUST be used.

#### REQ: idempotent-reinstall

When `--path` names a folder whose marker says it holds the TODO demo, the command MUST
exit `0`, write nothing (not even to restore deleted or edited records), report
`The TODO demo is already installed in <folder>`, and print the same next steps as a fresh
install.

#### REQ: no-partial-install

If any step fails after writing has started, the command MUST remove everything it wrote
(the folder itself when the command created it, or the folder's contents when it was an
empty folder before), exit non-zero and say what failed.

#### REQ: no-confirmation-prompt

The command MUST NOT prompt, on a terminal or otherwise, and MUST NOT require a `--yes`
flag: it only ever creates a new folder or fills an empty one and never overwrites or
deletes existing data. This matches `ingitdb setup` and `ingitdb insert`; the CLI asks for
confirmation only before replacing existing things (see
[self-update](../self-update/README.md#req-confirm-before-replace)). Output MUST be the same
whether or not stdout is a terminal.

### Git

#### REQ: own-git-repository

Before writing any file, the command MUST initialise a Git repository in the demo folder,
so that no write can land in a repository that encloses it. After writing, it MUST create
exactly one commit, with the message `Install the TODO demo`, containing every file it
wrote, leaving a clean working tree. It MUST NOT change the history, index or working tree
of an enclosing repository.

#### REQ: git-identity

When Git has no `user.name` or no `user.email` configured for the demo folder, the install
commit MUST fill each missing part separately from `inGitDB <ingitdb@localhost>` (name
`inGitDB`, email `ingitdb@localhost`) for that commit only, through the Git author and
committer environment, without writing Git configuration. Configured parts MUST be used as
they are.

#### REQ: git-unavailable

When the `git` executable is not available, the command MUST still install the files,
exit `0`, and say that the demo is not a Git repository and that `git init` adds history.

### Output

#### REQ: human-output

Without `--format`, the command MUST print a short human-readable result to stdout: a
title (`The TODO demo is ready` or `The TODO demo is already installed in <folder>`), where
the lists are stored as an absolute path, whether it is a Git repository, and a **What
next?** list of labelled commands. The default is text rather than YAML under
[output-formats `command-specific-formats`](../../output-formats/README.md#req-command-specific-formats),
as `select` already defaults to CSV in set mode: the first reader of an install result is a
person.

#### REQ: structured-output

`--format=json` and `--format=yaml` MUST print a single structured document to stdout, per
[output-formats](../../output-formats/README.md#req-format-flag-name) and
[json-supported](../../output-formats/README.md#req-json-supported), with at least: `app`
(`todo`), `installed` (`true`), `already_installed`, `path` (absolute), `git`
(`repository` boolean and `commit` id or empty), `lists` (the shared package's list paths)
and `next` (a list of `label` and `command`). Nothing else MUST be written to stdout. Errors
MUST go to stderr with a non-zero exit, as in every other command.

#### REQ: next-steps

The next steps MUST be, in this order, each with a command that works when pasted into the
person's shell on their operating system (paths quoted when they contain spaces or
shell-special characters, with double quotes on Windows so the command works in both
`cmd.exe` and PowerShell, one command per line, no `&&`):

1. **Browse the lists in the terminal UI**: `ingitdb --path=<folder>`
2. **Query the lists**: `ingitdb select --from=lists --path=<folder>`
3. **Query what to buy**: `ingitdb select --from=<first list>/items --path=<folder>`, the
   first list path taken from the shared package (today `lists/to-buy/items`)
4. **Use the lists in a web TODO app (OpenVaultDB)**: `ovdb demo install --yes`, then
   `ovdb demo open`, with a note that OpenVaultDB keeps its own copy of the same lists and a
   link to `https://github.com/openvaultdb/ovdb` for installing it.

The third step, REQ:browsable-in-tui and REQ:queryable-with-select depend on subcollection
addressing (see [Dependencies](#dependencies)), which MUST ship in the same or an earlier
release than this command, so that no printed step fails.

### Browsing the demo

#### REQ: browsable-in-tui

`ingitdb --path=<folder>` on a terminal MUST open the terminal UI with the `lists` collection,
show both lists, and let the person open a list's `items` and see its items with their
`title`, `done` and `added_at`.

#### REQ: queryable-with-select

`ingitdb select --from=lists --path=<folder>` MUST return both lists, and
`ingitdb select --from=lists/to-buy/items --path=<folder>` MUST return Milk, Bananas and
Coffee.

### OpenVaultDB hand-off

#### REQ: no-ovdb-coupling

`ingitdb-cli` MUST NOT read or write OpenVaultDB's home or data folders (including
`demos.json`), MUST NOT require `ovdb` to be installed, and MUST NOT depend on any
OpenVaultDB module. The hand-off is the printed next step only.

#### REQ: ovdb-can-connect

A demo folder installed by this command MUST be a plain inGitDB database with no
OpenVaultDB-specific files, so that OpenVaultDB can connect it as an existing folder
(`ovdb databases connect <id> --engine ingitdb --path <absolute folder>`). What OpenVaultDB
does with such a folder (an ordinary connected database, never adopted as its TODO demo) is
OpenVaultDB's behaviour, specified and verified by REQ `ingitdb-cli-demo-folders` and AC
`ingitdb-demo-folder-not-adopted` in its
[TODO demo](https://github.com/openvaultdb/openvaultdb/blob/main/spec/features/todo-demo/README.md)
feature; this repository does not run `ovdb` in its own verification.

### Platforms

#### REQ: cross-platform

The command MUST behave as specified on Linux, macOS and Windows: paths printed in the
platform's native form, record files written with the same content on every platform, and
the install, reinstall, refusal and `select` acceptance criteria below passing on all three.

## Dependencies

- [path-targeting](../../path-targeting/README.md) — `--path` resolution (default differs, see REQ:path-flag).
- [output-formats](../../output-formats/README.md) — `--format=yaml|json`.
- [select](../select/README.md) — querying the demo.
- [validate](../validate/README.md) — the demo passes validation.
- [setup](../setup/README.md) — `.ingitdb/settings.yaml` shape.
- Subcollection addressing — `select --from=<collection>/<record>/<subcollection>` and
  terminal UI navigation into subcollections. Not yet specified: today `select`, `describe`
  and `list collections` handle root collections only (see
  [describe, Out of Scope](../describe/README.md#out-of-scope)). Specified and built first by
  the [implementation plan](../../../plans/2026-09-17-cli-demo.md).
- `github.com/ingitdb/ingitdb-go/ingitdb/demos/todo` — the shared demo records and list
  paths (new, standard library imports only).

## Implementation

Not implemented yet. Plan: [2026-09-17-cli-demo](../../../plans/2026-09-17-cli-demo.md).

## Acceptance Criteria

### AC: fresh-install

**Requirements:** cli/demo#req:command-shape, cli/demo#req:path-flag, cli/demo#req:shared-demo-data, cli/demo#req:valid-ingitdb-database, cli/demo#req:demo-marker, cli/demo#req:human-output

**Given** an empty working directory
**When** `ingitdb demo install` runs
**Then** it exits `0`, `./todo-demo` exists with `.ingitdb/demo.yaml` naming `todo`, stdout
starts with `The TODO demo is ready` and names the absolute folder,
`ingitdb validate --path=todo-demo` exits `0`, reading every record the shared package
lists through `dalgo2ingitdb` returns exactly the shared package's data, and a test validates
every root and item record with `datavalidator` against the `lists` and `items` definitions
the command writes (proving item validity, which `validate` does not check today)

### AC: install-at-path

**Requirements:** cli/demo#req:path-flag

**Given** a working directory without `lists-demo`
**When** `ingitdb demo install --path=lists-demo` runs, and separately
`ingitdb demo install --path=<absolute empty folder>` runs
**Then** both exit `0` and install into those folders, and no `todo-demo` folder is created

### AC: reinstall-keeps-edits

**Requirements:** cli/demo#req:idempotent-reinstall

**Given** the demo installed in `todo-demo`, an item `tea` inserted into `lists/to-buy/items`
and the item `coffee` deleted
**When** `ingitdb demo install` runs again
**Then** it exits `0`, prints `The TODO demo is already installed in <folder>` and the next
steps, `tea` is still there, `coffee` is still absent, and `git -C todo-demo status
--porcelain` and the commit count are unchanged

### AC: non-empty-folder-refused

**Requirements:** cli/demo#req:refuses-conflicting-target

**Given** a folder `todo-demo` containing `notes.txt` and no demo marker
**When** `ingitdb demo install` runs
**Then** it exits non-zero, stderr names the folder and suggests
`ingitdb demo install --path=<another folder>`, and the folder still contains only
`notes.txt`

### AC: file-target-refused

**Requirements:** cli/demo#req:refuses-conflicting-target

**Given** a regular file named `todo-demo`
**When** `ingitdb demo install` runs
**Then** it exits non-zero and the file is unchanged

### AC: empty-folder-used

**Requirements:** cli/demo#req:refuses-conflicting-target

**Given** an existing empty folder `todo-demo`
**When** `ingitdb demo install` runs
**Then** it exits `0` and installs the demo into that folder

### AC: failure-leaves-nothing

**Requirements:** cli/demo#req:no-partial-install

**Given** a fault injected so that writing the fourth record fails, once for a folder the
command creates and once for an existing empty folder
**When** `ingitdb demo install` runs
**Then** it exits non-zero naming the failure, the created folder no longer exists, and the
existing folder is empty again

### AC: no-prompt-without-terminal

**Requirements:** cli/demo#req:no-confirmation-prompt

**Given** stdin and stdout that are not terminals
**When** `ingitdb demo install` runs without any other flag
**Then** it installs without waiting for input and its stdout is identical, apart from the
timestamps and commit id, to a run attached to a terminal

### AC: one-commit-own-repository

**Requirements:** cli/demo#req:own-git-repository, cli/demo#req:git-identity

**Given** a working directory that is itself inside a Git repository with one commit, and Git
isolated from the machine's configuration (`GIT_CONFIG_GLOBAL` pointing at an empty file,
`GIT_CONFIG_NOSYSTEM=1`, a temporary `HOME`)
**When** `ingitdb demo install` runs with no identity configured, and separately with only
`user.name` configured and with only `user.email` configured
**Then** `todo-demo` is its own repository with exactly one commit `Install the TODO demo`,
authored by `inGitDB <ingitdb@localhost>` in the first run and by the configured part plus
the missing default part in the other two; `git -C todo-demo status --porcelain` is empty,
`todo-demo/.git/config` has no `user` section, and the enclosing repository's `HEAD` and
index are unchanged

### AC: without-git

**Requirements:** cli/demo#req:git-unavailable

**Given** a `PATH` without a `git` executable
**When** `ingitdb demo install` runs
**Then** it exits `0`, the demo files exist without a `.git` folder, and stdout says the demo
is not a Git repository and mentions `git init`

### AC: json-output

**Requirements:** cli/demo#req:structured-output

**Given** an empty working directory
**When** `ingitdb demo install --format=json` runs, and then runs again
**Then** each stdout parses as one JSON object with `app` `todo`, `installed` `true`, an
absolute `path`, `git.repository` `true`, a 40-character `git.commit`, `lists` equal to the
shared package's list paths and a non-empty `next`; `already_installed` is `false`
the first time and `true` the second; `--format=yaml` produces the same document as YAML

### AC: next-steps-in-order

**Requirements:** cli/demo#req:next-steps

**Given** a successful install into a folder whose absolute path contains a space
**When** the human output is read
**Then** it lists terminal UI, query the lists, query what to buy, and the OpenVaultDB web
TODO app, in that order; each `ingitdb` command quotes the path for the current platform's
shell (double quotes on Windows) and, when run through that shell, exits `0`

### AC: tui-shows-items

**Requirements:** cli/demo#req:browsable-in-tui

**Given** the demo installed
**When** the terminal UI model is started on the folder and the person selects `lists`,
then `to-buy`, then its `items`
**Then** the screens show both lists, then Milk, Bananas and Coffee with `done` false

### AC: select-lists-and-items

**Requirements:** cli/demo#req:queryable-with-select

**Given** the demo installed
**When** `ingitdb select --from=lists --path=todo-demo --format=json` and
`ingitdb select --from=lists/to-buy/items --path=todo-demo --order-by=added_at --format=json`
run
**Then** the first returns `to-buy` and `to-watch` with their titles, and the second returns
Milk, Bananas and Coffee in that order

### AC: plain-ingitdb-folder

**Requirements:** cli/demo#req:no-ovdb-coupling, cli/demo#req:ovdb-can-connect

**Given** the demo installed with `HOME` and the OpenVaultDB variables (`OVDB_HOME`,
`OVDB_DATA_HOME`) pointing at empty temporary folders
**When** the folder, those temporary folders and `go list -deps ./cmd/ingitdb` are inspected
**Then** the demo folder holds only `.git`, `.ingitdb/` (`settings.yaml`,
`root-collections.yaml`, `demo.yaml`) and `lists/`, the OpenVaultDB folders are still empty,
and no `github.com/openvaultdb/` package is in the dependency list

### AC: works-on-every-os

**Requirements:** cli/demo#req:cross-platform

**Given** CI runners for Linux, macOS and Windows
**When** the automated tests for fresh-install, reinstall-keeps-edits,
non-empty-folder-refused, one-commit-own-repository, json-output and select-lists-and-items
run, comparing printed absolute paths after resolving symbolic links (macOS temporary
folders are under `/var`, a link to `/private/var`)
**Then** they pass on all three

## Open Questions

- Should an `ingitdb demo reset` restore the seed data, as asked for `ovdb demo`?

---
*This document follows the https://specscore.md/feature-specification*
