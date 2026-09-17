---
format: https://specscore.md/feature-specification
status: Implementing
---

# Feature: Install

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/install?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/install?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/install?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/install?op=request-change) |
**Status:** Implementing
**Source Ideas:** —

## Summary

`ingitdb install` is built entirely on the fleet-wide
[CLI Install Command Library](https://github.com/strongo/cli-helpers/blob/main/spec/features/cli-install/README.md)
(`github.com/strongo/cli-helpers/cliinstall`, Implementing), configured from
ingitdb's own compiled-in catalog entry — the same entry
[self-update](../self-update/README.md) builds its `selfupdate.Config` from
(cli-install#req:host-identity-from-catalog,
cli-install#req:catalog-identity-single-source). Listing, details, the
destination and Homebrew-cask decision, the confirmation gate,
checksum-verified download and `--dry-run`/`--format json` are all specified
once in that library, not restated here; this Feature specifies only
ingitdb's own host id and exit-code mapping.

## Problem

ingitdb's catalog entry declares it relevant to `datatug`, `ovdb`,
`synchestra` and `specscore` (the [CLI Install Command Library](https://github.com/strongo/cli-helpers/blob/main/spec/features/cli-install/README.md)'s
relevance matrix), and each of those CLIs' own `install` command lists
`ingitdb` in turn. Without `ingitdb install` itself, a user of one of those
CLIs could discover and install `ingitdb`, but a user who started from
`ingitdb` had no equivalent way to discover DataTug, OpenVaultDB, Synchestra
or SpecScore, or to install any of them consistently with how they installed
`ingitdb` — the same asymmetry [self-update](../self-update/README.md)'s
Problem section describes for the download-verify-swap machinery, one layer
up.

## Behavior

### Command surface

#### REQ: command-name

The CLI MUST expose the command as `ingitdb install`, built from
`github.com/strongo/cli-helpers/cliinstall/cobracmd`. The command inherits
the library's full flag surface — `--all`, `--yes`/`-y`, `--dry-run`,
`--dir`, and `--format text|json` — none of which is re-specified here.
`ingitdb` MUST NOT gain an `update` alias for any command, on either
`self-update` or `install`, because `ingitdb update` is the SQL UPDATE verb
that patches records (cli-install#req:update-alias-policy; see
[update](../update/README.md)).

### Host identity

#### REQ: host-id

`ingitdb install` MUST identify the host to the library as catalog id
`"ingitdb"` only (`cobracmd.CommandOptions.HostID`), never a hand-written
duplicate of the catalog entry (cli-install#req:host-identity-from-catalog).
`cliinstall.ByID("ingitdb")` is the same entry `self-update` resolves
(`cliinstall/catalog_ingitdb.go` in `strongo/cli-helpers`), so `install`'s
relevant-targets listing, and every other fleet CLI's `install ingitdb`,
resolve ingitdb's own release identically. `cobracmd.New` panics if
`"ingitdb"` is absent from the compiled catalog — a programming error
`TestInstall_Registration` catches, never a runtime state a user sees.

### Exit codes

#### REQ: exit-codes

ingitdb has no usage or invalid-state exit code distinct from its generic
error exit `1` (unlike its two special cases, `ValidationFailedExitCode` (2)
and `SelfUpdateAvailableExitCode` (10), neither of which `install` can ever
produce). Every install failure — a `*cobracmd.UsageError` (an invalid
`--format`, or `--all` combined with names), `selfupdate.KindUnknownTarget`,
`selfupdate.KindNoInstallDir`, `selfupdate.KindDestinationExists`, or any
self-update-shared kind (download, checksum, permission, non-interactive
refusal, a managed-command failure, ...) — is mapped explicitly, never
through a default branch (cli-install#req:host-owned-exit-codes), onto that
same generic exit `1`, with an `"install: "` message prefix and no
`"self-update: "` prefix. `install nosuchcli` MUST exit `1` and name the
unknown target.

| Exit code | Meaning |
|---|---|
| `0` | Success: every named target installed, already installed, redirected, or dry run |
| `1` | Any failure: an unknown target, no usable install directory, a destination that already exists, or any self-update-shared failure kind, or an invalid `--format`/`--all` usage |

### Upgrading

#### REQ: upgrade-command

The CLI MUST also expose `ingitdb upgrade [name...] [--all] [--check]
[--yes] [--dry-run] [--format text|json]`, built from
`cliinstall/cobracmd.NewUpgrade` against the same `HostID: "ingitdb"` and the
exact same `HostConfig` `self-update` builds
(`ingitdbSelfUpdateConfig`, [`self_update.go`](../../../../cmd/ingitdb/commands/self_update.go));
ingitdb's self-update has no after-update hook, so `upgrade` passes none
either. `ingitdb self-update` MUST therefore be `ingitdb upgrade ingitdb` by
construction
([cli-install#req:self-update-equals-upgrade-self](https://github.com/strongo/cli-helpers/blob/main/spec/features/cli-install/README.md#req-self-update-equals-upgrade-self)).
`upgrade --all` means every *installed* catalog id plus ingitdb itself, not
the relevance matrix `install` lists. `ingitdb` MUST NOT gain an `update`
alias on `upgrade` either, for the same reason it has none on `self-update`
or `install` ([REQ: command-name](#req-command-name);
[cli-install#req:update-alias-policy](https://github.com/strongo/cli-helpers/blob/main/spec/features/cli-install/README.md#req-update-alias-policy):
"ingitdb MUST NOT gain one, because its `update` command edits records").

`ingitdb upgrade` MUST use the same exit-code contract `install` and
`self-update` already keep: the three cli-install-only kinds map through the
same explicit `"upgrade: "`-prefixed exit `1` `install` already uses for
them, and every kind `self-update` already handles reuses
`selfUpdateErrors.Failure` UNCHANGED — same error value, same exit code.
`--check`/the bare report finding an upgrade available or undetermined MUST
map through `SelfUpdateAvailableExitCode` (`10`), exactly like `self-update
--check` — quietly, with no extra "error:" line, via the same
`ErrSelfUpdateAvailable` sentinel `main.go`'s `exitCodeForError` and quiet
error handler already special-case.

| Exit code | Meaning |
|---|---|
| `0` | Success, or a report/check showing no available upgrade |
| `1` | Any failure: an unknown target, no usable install directory, a destination that already exists, any self-update-shared failure kind, or an invalid `--format`/`--all` usage |
| `10` | `--check`/the bare report found an upgrade available or an undetermined running version for at least one looked-up target — quiet, no error line |

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/install`):

- [`cmd/ingitdb/commands/install.go`](../../../../cmd/ingitdb/commands/install.go) —
  the `installErrors` exit-code mapper and the `cobracmd.New` wiring against
  `HostID: "ingitdb"`.
- [`cmd/ingitdb/commands/upgrade.go`](../../../../cmd/ingitdb/commands/upgrade.go) —
  the `upgradeErrors` mapper and the `cobracmd.NewUpgrade` wiring against the
  same `HostConfig` `self-update` builds.

The shared behavior lives upstream, not in this repository:
`github.com/strongo/cli-helpers` `cliinstall/`, `cliinstall/cliui/`,
`cliinstall/cobracmd/` (the library and its Cobra adapter), and
`cliinstall/catalog_ingitdb.go` (ingitdb's own catalog entry, shared with
`self-update`).

## Interaction with Other Features

| Feature | Interaction |
|---|---|
| [self-update](../self-update/README.md) | Both commands build from the same `cliinstall.ByID("ingitdb")` / `selfupdate.Config` catalog entry, so `install`'s view of ingitdb (shown by other CLIs) and `self-update`'s own release identity never disagree. |
| [update](../update/README.md) | Name collision constraint shared with `self-update`: `ingitdb update` is the SQL UPDATE verb, which is why neither `self-update`, `install`, nor `upgrade` has an `update` alias ([REQ: command-name](#req-command-name), [REQ: upgrade-command](#req-upgrade-command)). |
| [version](../version/README.md) | Targets `install` lists are probed through the fleet-wide `version --json` contract that [version](../version/README.md) implements; ingitdb's own entry in another CLI's `install` listing is probed the same way. |

## Acceptance Criteria

### AC: registration-and-host-id

**Requirements:** cli/install#req:command-name, cli/install#req:host-id

**Given** the compiled `cliinstall` catalog
**When** `Install()` builds the command
**Then** it registers `--all`, `--yes`/`-y`, `--dry-run`, `--dir` and
`--format`, resolves against catalog id `"ingitdb"`, and carries no `update`
alias.

### AC: unknown-target-exit-code

**Requirements:** cli/install#req:exit-codes

**Given** the real command built exactly as `main.go` wires it
**When** the user runs `ingitdb install nosuchcli`
**Then** the command fails before any confirmation, network request or
write, names `nosuchcli` in its error, and exits `1` — never
`ValidationFailedExitCode` or `SelfUpdateAvailableExitCode`.

### AC: upgrade-exit-code-contract

**Requirements:** cli/install#req:upgrade-command

**Given** an installed `ingitdb` binary
**When** the user runs `ingitdb upgrade nosuchcli`
**Then** the command exits `1` before any release lookup, the same way
`ingitdb install nosuchcli` does; and when the user runs
`ingitdb self-update --check` and `ingitdb upgrade ingitdb --check` against
the same release, both exit `10` quietly for an available or undetermined
upgrade and `0` for up to date, because both reach the exact same
`selfupdate.Config.Check` call.

The remaining behavior — the relevance matrix, listing and status probing,
destination policy, Homebrew-cask installs, checksum-verified direct
installs, the confirmation gate, `--dry-run`, `--format json`, and
upgrade's own target selection, release lookups and per-target policy — is
specified and tested once in the
[CLI Install Command Library](https://github.com/strongo/cli-helpers/blob/main/spec/features/cli-install/README.md)'s
own Acceptance Criteria, which this command inherits by construction rather
than re-proving.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
