---
format: https://specscore.md/feature-specification
status: Implementing
---

# Feature: Self-Update

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/self-update?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/self-update?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/self-update?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/self-update?op=request-change) |
**Status:** Implementing
**Source Ideas:** —

## Summary

`ingitdb self-update` is built entirely on the fleet-wide
[Self-Update Library](https://github.com/strongo/cli-helpers/blob/main/spec/features/self-update/README.md)
(`github.com/strongo/cli-helpers/selfupdate`, Stable), configured from
ingitdb's own compiled-in catalog entry in
[`cliinstall`](https://github.com/strongo/cli-helpers/blob/main/spec/features/cli-install/README.md)
(cli-install#req:host-identity-from-catalog). Detection (managed vs. manual
vs. ambiguous), the confirmation gate, checksum verification, atomic replace,
version pinning, `--dry-run` and `--format json` are all specified once in
that library, not restated here; this Feature specifies only ingitdb's own
configuration and where its behavior deviates from the shared contract.

## Problem

Every CLI that ships binaries needs the same install-method detection,
download-verify-swap machinery, and confirmation/refusal rules. Before this
Feature, ingitdb carried its own copy (`internal/selfupdate`), which had
drifted from the CLI's actual releases: it fetched `checksums-darwin.txt` and
`checksums-windows.txt`, but ingitdb's release publishes a single flat
`checksums.txt` for every platform, so darwin and windows self-update failed
with a download error. Its detection also recognized only Homebrew and Snap,
even though ingitdb publishes Scoop and WinGet packages too — a Scoop- or
WinGet-managed install classified as manual and was eligible for
self-replace, which would have desynchronized that manager's bookkeeping.

Both bugs are structural: they are exactly the class of drift the shared
Self-Update Library and its compiled-in catalog exist to close, by making a
CLI's release identity (repository, checksum naming, recognized managers)
one typed value shared with every other fleet CLI that needs to know it,
rather than a hand-maintained copy.

## Behavior

### Command surface

#### REQ: command-name

The CLI MUST expose the command as `ingitdb self-update`, built from
`github.com/strongo/cli-helpers/selfupdate/cobracmd`. Unlike comparable CLIs
that alias a bare `update`, `ingitdb` MUST NOT alias `update` to this
command: `ingitdb update` is the SQL UPDATE verb that patches records (see
[update](../update/README.md)), and a self-update alias would collide with
it. The command inherits the library's full flag surface — `--check`,
`--yes`/`-y`, `--version`, `--allow-downgrade`, `--dry-run`, and
`--format text|json` — none of which is re-specified here.

### Catalog configuration

#### REQ: catalog-configured-identity

ingitdb's `self-update` MUST build its `selfupdate.Config` from its own
`cliinstall.ByID("ingitdb")` catalog entry
(cli-install#req:host-identity-from-catalog,
cli-install#req:catalog-identity-single-source), never from hand-written
identity values, so its self-update and every other fleet CLI's
`install ingitdb` resolve releases identically. The entry declares:

- Repository `ingitdb/ingitdb-cli`, no tag prefix.
- A flat `checksums.txt` checksum name for every platform (overriding the
  library's per-version default), matching `.goreleaser.yaml`'s
  `checksum.name_template` — the fix for the darwin/windows 404 bug above.
- The default GoReleaser asset-naming convention (no override), matching
  `.goreleaser.yaml`'s `archives[].name_template`.
- `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, and
  `windows/amd64` as the supported platforms (matching
  `.goreleaser.yaml`'s build matrix minus `windows/arm64`, which was never
  published).
- Four managers, all **redirect-only** (the library prints the exact upgrade
  command; it never runs it): Homebrew (`brew upgrade --cask ingitdb`), Snap
  (`snap refresh ingitdb`, `/snap/` path marker), Scoop
  (`scoop update ingitdb`), and WinGet
  (`winget upgrade --id ingitdb.ingitdb`) — Scoop and WinGet are new
  recognized managers versus the pre-migration detection, closing the
  self-replace-of-a-managed-binary bug above.

A consumer test (`TestIngitdbCatalogEntry_MatchesGoReleaserConfig`) asserts
the catalog entry's archive/checksum naming, release repository, and
platform matrix match `.goreleaser.yaml`, so drift fails this repository's
own CI rather than surfacing as a broken download for a user.

### Exit codes

#### REQ: check-exit-codes

`ingitdb self-update --check` MUST use exit code `0` when the binary is up
to date, `10` when an update is available (or the current version is
undetermined), and the CLI's generic error exit `1` for every other outcome
— an operational failure (release-lookup, download, checksum, permission,
non-interactive refusal, a managed-command failure) or an invalid
`--format`. This is ingitdb's own choice among the exit-code contracts the
library supports (cli-install#req:host-owned-exit-codes); it is unchanged
from the command's pre-migration behavior. `ErrSelfUpdateAvailable`
(`cmd/ingitdb/commands/self_update.go`) is the sentinel `main.go` maps to
exit `10`; every other failure, including a `*selfupdate.Failure` of any
kind, passes through the mapper unchanged and falls into the generic exit
`1`.

| Exit code | Meaning |
|---|---|
| `0` | Success: self-replace completed, redirect printed, or already up to date |
| `10` | `--check` only — an update is available, or the current version is undetermined |
| `1` | Every other failure: ambiguous detection, network/download failure, missing OS/arch asset, checksum mismatch, permission denied, non-interactive without `--yes`, unknown `--version` tag, a refused downgrade, or an invalid `--format` |

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/self-update`):

- [`cmd/ingitdb/commands/self_update.go`](../../../../cmd/ingitdb/commands/self_update.go) —
  the catalog lookup, the `selfUpdateErrors` exit-code mapper, and the
  `cobracmd.New` wiring.

The shared behavior lives upstream, not in this repository:
`github.com/strongo/cli-helpers` `selfupdate/`, `selfupdate/cliui/`,
`selfupdate/cobracmd/` (the library and its Cobra adapter), and
`cliinstall/catalog_ingitdb.go` (ingitdb's own catalog entry).

## Interaction with Other Features

| Feature | Interaction |
|---|---|
| [version](../version/README.md) | `self-update`'s running version is ingitdb's own build identity (`buildinfo.Get("ingitdb").Version`), the same value `version --json` reports. |
| [update](../update/README.md) | Name collision constraint: `ingitdb update` is the SQL UPDATE verb, which is why `self-update` has no `update` alias ([REQ: command-name](#req-command-name)). |

## Acceptance Criteria

### AC: canonical-name

**Requirements:** cli/self-update#req:command-name

**Given** an installed `ingitdb` binary
**When** the user runs `ingitdb self-update --check` and, separately, `ingitdb update`
**Then** `self-update` executes this command, while `update` executes the SQL UPDATE verb command — `self-update` has no `update` alias.

### AC: catalog-identity-matches-releases

**Requirements:** cli/self-update#req:catalog-configured-identity

**Given** the compiled-in `cliinstall.ByID("ingitdb")` catalog entry and this repository's `.goreleaser.yaml`
**When** `TestIngitdbCatalogEntry_MatchesGoReleaserConfig` runs offline
**Then** the entry's checksum naming, archive naming, release repository, and supported platform matrix equal what `.goreleaser.yaml` actually publishes, and every declared manager is redirect-only.

### AC: check-exit-code-contract

**Requirements:** cli/self-update#req:check-exit-codes

**Given** three scenarios — up to date, update available, and a release-lookup error
**When** the user runs `ingitdb self-update --check` in each
**Then** the exit codes are `0`, `10`, and `1` respectively, and the `10` case is produced by `ErrSelfUpdateAvailable`, not by any other failure kind.

The remaining behavior — install-method detection and its safe-ambiguous
default, the confirmation gate and non-interactive refusal, checksum
verification and atomic replace, version pinning and the downgrade guard,
`--dry-run`, and `--format json` — is specified and tested once in the
[Self-Update Library](https://github.com/strongo/cli-helpers/blob/main/spec/features/self-update/README.md)'s
own Acceptance Criteria, which this command inherits by construction rather
than re-proving.

## Open Questions

- Should ingitdb also gain the shared `install` command
  (cli-install#req:fleet-cutover) so it can list and install other fleet
  CLIs relevant to it, and be listed by them in turn?

---
*This document follows the https://specscore.md/feature-specification*
