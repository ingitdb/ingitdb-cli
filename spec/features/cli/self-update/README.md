---
format: https://specscore.md/feature-specification
status: Implementing
---

# Feature: Self-Update

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/self-update?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/self-update?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/self-update?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/cli/self-update?op=request-change) |
**Status:** Implementing
**Source Ideas:** —

## Summary

`ingitdb self-update` brings a running `ingitdb` binary to the latest released version. It first detects how the binary was installed: package-managed installs (Homebrew cask, Snap) are never overwritten — the command prints the exact manager upgrade command instead — while manual installs (release-archive downloads, `go install`) are updated in place by downloading the matching release asset from `github.com/ingitdb/ingitdb-cli` releases, verifying its sha256 against the release's per-OS checksum file, and atomically replacing the executable. A `--check` mode reports update availability for any install method without modifying anything, and a `--version <tag>` flag installs a specific pinned release instead of the latest.

## Synopsis

```
ingitdb self-update                         # detect, then self-replace (manual) or redirect (managed)
ingitdb self-update --check                 # report availability only; never modifies
ingitdb self-update --yes                   # skip the confirmation prompt (non-interactive)
ingitdb self-update --version v0.24.1       # install a specific release (manual installs)
ingitdb self-update --version 0.24.1 --allow-downgrade   # roll back to an older release
```

## Problem

The CLI is distributed through several channels — a Homebrew cask, a Snap package, and direct GitHub release archives (plus `go install`). Users have no first-class way to move to the latest version, and the naive fix — "just overwrite the binary" — is only correct for manual installs. Overwriting a package-managed binary corrupts the manager's bookkeeping: the next `brew upgrade` sees an unexpected file, and the user ends up with two conflicting notions of "installed version."

The hard part is therefore not the file swap; it is **deciding whether a swap is even allowed**. Install-method detection is part of the product contract, not an implementation detail. When detection is uncertain, the safe outcome (do not self-replace; guide the user) must be the default.

## Behavior

### Command surface

#### REQ: command-name

The CLI MUST expose the command as `ingitdb self-update`. Unlike comparable CLIs that alias a bare `update`, `ingitdb` MUST NOT alias `update` to this command: `ingitdb update` is the SQL UPDATE verb that patches records (see [update](../update/README.md)), and a self-update alias would collide with it.

#### REQ: check-flag

A `--check` boolean flag MUST be accepted. In check mode the command performs install-method detection and the version check, reports the result, and exits without downloading or modifying anything (see [REQ: check-no-mutation](#req-check-no-mutation)).

#### REQ: confirm-before-replace

For the self-replace path, the command MUST show the version transition (`<current> → <latest>`) and require interactive confirmation before replacing the executable. A `--yes` flag (short `-y`) MUST skip the prompt for non-interactive use. When the command is not attached to an interactive terminal and `--yes` was not given, it MUST refuse to replace and exit non-zero rather than block on input.

### Install-method detection

Detection chooses between two mutually exclusive outcomes — *managed* (redirect) or *manual* (self-replace eligible) — preferring explicit signals over guesswork.

#### REQ: detect-managed

The command MUST classify the running binary as package-managed when its executable path matches a known manager layout: a Homebrew Cellar/Caskroom/prefix path, or a Snap path (under `/snap/`). Because the Homebrew cask installs a symlink in the brew prefix `bin` directory pointing into the Caskroom, detection MUST resolve symlinks on the executable path and classify BOTH the link path and the resolved target; a managed match on either side wins. A managed classification MUST route to the redirect path and MUST NOT self-replace.

#### REQ: detect-manual

The command MUST classify the running binary as manual when it is not recognized as managed and the path is a plausible user/Go install location (e.g., a release archive extracted to `~/bin` or `/usr/local/bin`, or a `go install` target under `GOBIN`/`GOPATH/bin`). A manual classification is eligible for the self-replace path.

#### REQ: ambiguous-safe-default

When detection cannot confidently classify the install method, the command MUST default to the safe outcome: do not self-replace, surface the ambiguity, and print manual-update guidance. Ambiguity MUST NOT resolve to "manual."

### Package-managed redirect

Managed installs are guided, never modified.

#### REQ: managed-no-overwrite

For a managed classification the command MUST NOT download, write, or replace the executable under any flag combination except `--check` (which never writes regardless).

#### REQ: managed-redirect-command

For a managed classification the command MUST print the detected manager and the exact upgrade command for it (`brew upgrade --cask ingitdb` or `snap refresh ingitdb`) and exit `0`.

### Version check

Both the action and `--check` compare the running version against the latest release.

#### REQ: latest-release-source

The command MUST determine the latest version from the published GitHub releases of `ingitdb/ingitdb-cli`, considering only the latest stable release — excluding both prereleases and drafts. (This stable-only filter governs the *unpinned* "latest" path only; an explicit `--version` pin per [REQ: pinned-exact-tag](#req-pinned-exact-tag) bypasses it.)

#### REQ: dev-build-undetermined

When the running binary reports the `dev` version placeholder (a build without `-ldflags`, e.g. `go install` of an untagged tree), the command MUST treat the current version as undetermined: `--check` reports it as undetermined (not "up to date"), and the self-replace path MAY offer to install the latest stable release subject to the normal confirmation in [REQ: confirm-before-replace](#req-confirm-before-replace).

### Pinned-version install

By default the command targets the latest stable release; `--version` lets the user install an exact release instead. This is a manual-install capability that reuses the self-replace machinery with a different target.

#### REQ: version-flag

The command MUST accept a `--version <tag>` flag that selects an exact release to install instead of the latest stable. The leading `v` is optional: `--version v0.24.1` and `--version 0.24.1` MUST resolve to the same release (the value is normalized to match the project's `v`-prefixed git tags). A pinned install reuses the same confirmation (`--yes`), checksum-verification, and atomic-replace machinery as the unpinned self-replace path; only the target release differs. (`--version` here is a `self-update`-local flag, distinct from the `ingitdb version` command that prints build identity.)

#### REQ: pinned-exact-tag

A `--version` pin MUST resolve to exactly the named release regardless of its prerelease or draft status. The stable-only selection in [REQ: latest-release-source](#req-latest-release-source) governs only the unpinned "latest" path; an explicit pin opts the user into precisely the requested tag.

#### REQ: pinned-downgrade-guard

When the pinned target version is strictly lower than the running version, the command MUST refuse unless `--allow-downgrade` is passed, printing a clear message that names both versions and the required flag, exiting non-zero without modifying the binary. With `--allow-downgrade` the command proceeds (still subject to confirmation / `--yes`), and the transition output indicates a downgrade. When the running version is undetermined (the `dev` placeholder), the downgrade guard does not trigger because direction cannot be determined.

#### REQ: pinned-unknown-tag

When the pinned tag does not correspond to a published release (or the release has no asset matching the host OS/architecture), the command MUST print a clear error, exit non-zero, and MUST NOT modify the existing binary.

#### REQ: pinned-managed-still-redirects

On a package-managed install, `--version` MUST NOT cause a self-replace; the command still follows the managed redirect path ([REQ: managed-no-overwrite](#req-managed-no-overwrite)). Pinning a version through `self-update` is a manual-install capability only.

### Self-replace (manual installs)

The manual path downloads, verifies, and atomically swaps the binary.

#### REQ: download-matching-asset

For an eligible self-replace the command MUST download the release asset matching the host OS and architecture from the target release's versioned download directory. Asset naming follows the project's GoReleaser configuration: `ingitdb_{version}_{os}_{arch}.tar.gz` for linux/darwin and `ingitdb_{version}_windows_{arch}.zip` for windows, with the binary `ingitdb` (`ingitdb.exe` on windows) inside the archive. When the release has no asset for the host OS/arch (e.g. a release whose darwin build job failed to publish), the command MUST produce a clear non-zero error and leave the binary untouched per [REQ: network-failure-clear](#req-network-failure-clear).

#### REQ: checksum-verification

Before replacing the executable the command MUST verify the downloaded asset's sha256 against the release's checksum file for the host OS. ingitdb releases publish one checksum file per OS build job — `checksums.txt` (linux), `checksums-darwin.txt` (darwin), and `checksums-windows.txt` (windows) — and the command MUST fetch the file matching the host OS. On mismatch (or a missing/unfetchable checksum entry) the command MUST abort with a non-zero exit and MUST NOT modify the existing binary.

#### REQ: atomic-replace

The executable MUST be replaced atomically: the verified new binary is staged and swapped into place such that an interrupted or failed operation leaves the original binary intact and runnable (no partial or truncated executable). This holds across macOS, Linux, and Windows (including replacing a running executable on Windows).

#### REQ: no-op-when-current

When the running version already equals the latest stable release, the command MUST report that it is up to date and exit `0` without downloading or replacing anything.

### Check-only mode

`--check` is read-only and scriptable.

#### REQ: check-no-mutation

With `--check` the command MUST NOT download an asset or modify the executable for any install method; it performs detection and the version check only.

#### REQ: check-exit-codes

`--check` MUST use exit code `0` when the binary is up to date, `10` when an update is available (or the current version is undetermined per [REQ: dev-build-undetermined](#req-dev-build-undetermined)), and a code distinct from `0` and `10` for operational errors (e.g., release lookup failure — the CLI's generic error exit `1`). The "update available" code MUST NOT collide with the error codes.

### Failure modes

Failures are explicit and never leave a broken binary.

#### REQ: network-failure-clear

When the release lookup or asset download fails (network error, rate limit, missing asset for the host OS/arch), the command MUST print a clear error, exit non-zero, and MUST NOT modify the existing binary.

#### REQ: permission-failure-clear

When the command lacks permission to replace the executable in its install location, it MUST report the failure with the path and a suggested remedy (e.g., re-run with elevated permissions or use the package manager), exit non-zero, and leave the original binary intact.

## Exit codes

| Exit code | Meaning |
|---|---|
| `0` | Success: self-replace completed, redirect printed, or already up to date |
| `10` | `--check` only — an update is available, or the current version is undetermined |
| non-zero (other) | Operational error: detection-ambiguous refusal, network/download failure, missing OS/arch asset, checksum mismatch, permission denied, non-interactive without `--yes`, unknown `--version` tag, or a refused downgrade (target older than current without `--allow-downgrade`) |

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/cli/self-update`):

- [`cmd/ingitdb/commands/self_update.go`](../../../../cmd/ingitdb/commands/self_update.go)
- [`internal/selfupdate/detect.go`](../../../../internal/selfupdate/detect.go)
- [`internal/selfupdate/release.go`](../../../../internal/selfupdate/release.go)
- [`internal/selfupdate/download.go`](../../../../internal/selfupdate/download.go)
- [`internal/selfupdate/replace.go`](../../../../internal/selfupdate/replace.go)
- [`internal/selfupdate/redirect.go`](../../../../internal/selfupdate/redirect.go)

## Interaction with Other Features

| Feature | Interaction |
|---|---|
| [version](../version/README.md) | The running version read by `self-update` is the build-time value pinned by goreleaser ldflags, including its `dev` placeholder behavior consumed by [REQ: dev-build-undetermined](#req-dev-build-undetermined). The post-swap sanity check runs `ingitdb version`. |
| [update](../update/README.md) | Name collision constraint: `ingitdb update` is the SQL UPDATE verb, which is why `self-update` has no `update` alias ([REQ: command-name](#req-command-name)). |

## Acceptance Criteria

### AC: canonical-name

**Requirements:** cli/self-update#req:command-name

**Given** an installed `ingitdb` binary
**When** the user runs `ingitdb self-update --check` and, separately, `ingitdb update`
**Then** `self-update` executes this command, while `update` executes the SQL UPDATE verb command — `self-update` has no `update` alias.

### AC: check-is-readonly

**Requirements:** cli/self-update#req:check-flag, cli/self-update#req:check-no-mutation

**Given** any install method and an available newer release
**When** the user runs `ingitdb self-update --check`
**Then** the command prints availability and the appropriate next step, and the on-disk executable is byte-for-byte unchanged (no download, no replace).

### AC: confirm-prompt-and-yes

**Requirements:** cli/self-update#req:confirm-before-replace

**Given** a manual install with a newer release available, attached to an interactive terminal
**When** the user runs `ingitdb self-update` without `--yes`
**Then** the command prints `<current> → <latest>` and waits for confirmation; running it with `--yes` performs the replacement without prompting.

### AC: noninteractive-without-yes-refuses

**Requirements:** cli/self-update#req:confirm-before-replace

**Given** a manual install with a newer release, running without an interactive terminal
**When** the user runs `ingitdb self-update` without `--yes`
**Then** the command refuses to replace, prints that `--yes` is required for non-interactive use, and exits non-zero, leaving the binary unchanged.

### AC: managed-is-redirected

**Requirements:** cli/self-update#req:detect-managed, cli/self-update#req:managed-no-overwrite, cli/self-update#req:managed-redirect-command

**Given** an `ingitdb` whose executable path (or symlink-resolved target) is a Homebrew or Snap managed location
**When** the user runs `ingitdb self-update`
**Then** the command prints the detected manager and its exact upgrade command (`brew upgrade --cask ingitdb` / `snap refresh ingitdb`), exits `0`, and the executable is unchanged.

### AC: manual-is-eligible

**Requirements:** cli/self-update#req:detect-manual

**Given** an `ingitdb` extracted from a release archive into `/usr/local/bin` (or installed via `go install`)
**When** the user runs `ingitdb self-update --check`
**Then** the command classifies the install as manual and reports the self-update path as available.

### AC: ambiguous-falls-back-safe

**Requirements:** cli/self-update#req:ambiguous-safe-default

**Given** an `ingitdb` whose install method cannot be confidently classified
**When** the user runs `ingitdb self-update`
**Then** the command does not replace the binary, states that the install method is ambiguous, prints manual-update guidance, and exits non-zero.

### AC: latest-stable-only

**Requirements:** cli/self-update#req:latest-release-source

**Given** the project's GitHub releases where the newest tagged release is a prerelease or a draft and the newest stable release is older
**When** the user runs `ingitdb self-update --check`
**Then** the "latest" the command compares against is the newest stable release, ignoring both prereleases and drafts.

### AC: dev-build-is-undetermined

**Requirements:** cli/self-update#req:dev-build-undetermined

**Given** a binary built without `-ldflags` (version reports `dev`)
**When** the user runs `ingitdb self-update --check`
**Then** the command reports the current version as undetermined (not "up to date") and exits `10`.

### AC: per-os-checksum-file

**Requirements:** cli/self-update#req:checksum-verification

**Given** a manual install on a given OS and a release publishing `checksums.txt`, `checksums-darwin.txt`, and `checksums-windows.txt`
**When** `ingitdb self-update` verifies the downloaded asset
**Then** the digest is read from the checksum file matching the host OS, not from a different OS's file.

### AC: checksum-mismatch-aborts

**Requirements:** cli/self-update#req:download-matching-asset, cli/self-update#req:checksum-verification

**Given** a manual install where the downloaded asset's sha256 does not match the release's per-OS checksum file
**When** `ingitdb self-update` runs the replacement
**Then** the command aborts before touching the executable, reports the verification failure, exits non-zero, and the original binary remains in place and runnable.

### AC: missing-os-asset-is-safe

**Requirements:** cli/self-update#req:download-matching-asset, cli/self-update#req:network-failure-clear

**Given** a manual install on an OS for which the target release published no asset (e.g. a release whose darwin job failed)
**When** the user runs `ingitdb self-update --yes`
**Then** the command prints a clear error, exits non-zero, and does not modify the existing binary.

### AC: replace-is-atomic

**Requirements:** cli/self-update#req:atomic-replace

**Given** a manual install on macOS, Linux, or Windows
**When** a verified asset is swapped in and the operation is interrupted or fails mid-way
**Then** the install location still contains a complete, runnable `ingitdb` binary (either the original or the new version) — never a partial or truncated file.

### AC: already-current-noop

**Requirements:** cli/self-update#req:no-op-when-current

**Given** a manual install already on the latest stable release
**When** the user runs `ingitdb self-update`
**Then** the command reports it is up to date and exits `0` without downloading or replacing anything.

### AC: check-exit-code-contract

**Requirements:** cli/self-update#req:check-exit-codes

**Given** three scenarios — up to date, update available, and a release-lookup error
**When** the user runs `ingitdb self-update --check` in each
**Then** the exit codes are `0`, `10`, and a third code distinct from both, respectively.

### AC: network-failure-is-safe

**Requirements:** cli/self-update#req:network-failure-clear

**Given** a manual install and an unreachable release source
**When** the user runs `ingitdb self-update`
**Then** the command prints a clear error, exits non-zero, and does not modify the existing binary.

### AC: permission-denied-is-safe

**Requirements:** cli/self-update#req:permission-failure-clear

**Given** a manual install where the executable's directory is not writable by the current user
**When** the user runs `ingitdb self-update --yes`
**Then** the command reports the permission failure with the path and a suggested remedy, exits non-zero, and leaves the original binary intact.

### AC: version-flag-selects-tag

**Requirements:** cli/self-update#req:version-flag

**Given** a manual install and a published release tagged `v0.24.1`
**When** the user runs `ingitdb self-update --version 0.24.1 --yes`
**Then** the command installs exactly the `v0.24.1` release (accepting the tag with or without the leading `v`), using the same checksum-verify and atomic-replace path as an unpinned update.

### AC: pinned-tag-allows-prerelease

**Requirements:** cli/self-update#req:pinned-exact-tag

**Given** a manual install and a published prerelease tagged `v0.41.0-rc.1`
**When** the user runs `ingitdb self-update --version v0.41.0-rc.1 --yes`
**Then** the command installs that prerelease exactly, even though the unpinned "latest" path would have skipped it.

### AC: downgrade-requires-flag

**Requirements:** cli/self-update#req:pinned-downgrade-guard

**Given** a manual install currently on `v0.40.1` and a pinned target of `v0.24.1`
**When** the user runs `ingitdb self-update --version v0.24.1` without `--allow-downgrade`
**Then** the command refuses, names both versions and the `--allow-downgrade` flag, exits non-zero, and leaves the binary unchanged; re-running with `--allow-downgrade --yes` performs the downgrade.

### AC: pinned-unknown-tag-errors

**Requirements:** cli/self-update#req:pinned-unknown-tag

**Given** a manual install and a `--version` tag that has no matching published release or asset
**When** the user runs `ingitdb self-update --version v9.9.9 --yes`
**Then** the command prints a clear error, exits non-zero, and does not modify the existing binary.

### AC: pinned-managed-still-redirects

**Requirements:** cli/self-update#req:pinned-managed-still-redirects

**Given** an `ingitdb` whose executable path is a Homebrew or Snap managed location
**When** the user runs `ingitdb self-update --version v0.24.1`
**Then** the command does not self-replace; it prints the detected manager and its upgrade command and exits `0` (the same redirect as an unpinned managed run).

## Open Questions

- Should a later iteration add cryptographic signature verification (cosign/sigstore) on top of the sha256 check?
- Should the release pipeline be fixed so every release publishes darwin assets (recent releases lack them), making self-update reliable on macOS manual installs?

---
*This document follows the https://specscore.md/feature-specification*
