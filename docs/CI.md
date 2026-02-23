# Continuous integration for inGitDB

## Release Workflow

The release workflow is defined in [`.github/workflows/release.yml`](../.github/workflows/release.yml) and triggered manually via `workflow_dispatch`.

### Release Jobs

#### 1. `build-linux` (runs on Ubuntu)

Builds Linux and Windows binaries, creates the GitHub release, and uploads artifacts.

**Config:** [`.github/goreleaser-linux.yaml`](../.github/goreleaser-linux.yaml)

- Builds Linux binaries (amd64, arm64) and Windows binaries (amd64)
- Creates the GitHub release and uploads archives + checksums

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`

#### 2. `publish-homebrew` (runs on macOS, after `build-linux`)

Builds, signs, and notarizes macOS binaries, then publishes the Homebrew Cask covering both macOS and Linux.

**Config:** [`.github/goreleaser-homebrew.yaml`](../.github/goreleaser-homebrew.yaml)

- Builds Darwin binaries (amd64, arm64) and Linux binaries (amd64, arm64)
- Code-signs and notarizes macOS binaries with Apple credentials
- Uploads macOS archives to the existing GitHub release
- Publishes Homebrew Cask (macOS + Linux) to [`ingitdb/homebrew-cli`](https://github.com/ingitdb/homebrew-cli)

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`, `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`, `NOTARIZE_ISSUER_ID`, `NOTARIZE_KEY_ID`, `NOTARIZE_KEY`

#### 3. `publish-aur` (runs on Ubuntu, after `build-linux`)

Publishes the AUR package.

**Config:** [`.github/goreleaser-publish-aur.yaml`](../.github/goreleaser-publish-aur.yaml)

- Rebuilds Linux binaries (goreleaser produces reproducible archives, so checksums match the release)
- Generates PKGBUILD and .SRCINFO and pushes to AUR as `ingitdb-bin` via SSH

**Secrets used:** `AUR_SSH_PRIVATE_KEY` (raw ED25519 private key)

#### 4. `publish-snap` (runs on Ubuntu, after `build-linux`)

Builds and publishes the Snapcraft package.

**Config:** [`.github/goreleaser-publish-snap.yaml`](../.github/goreleaser-publish-snap.yaml)

- Builds Linux binaries (snap requires its own build inside the snap toolchain)
- Publishes to [Snapcraft Store](https://snapcraft.io/ingitdb)

**Secrets used:** `SNAPCRAFT_STORE_CREDENTIALS`

#### 5. `publish-chocolatey` (runs on Windows, after `build-linux`)

Publishes the Chocolatey package. Runs on `windows-latest` because `choco` CLI is required.

**Config:** [`.github/goreleaser-publish-chocolatey.yaml`](../.github/goreleaser-publish-chocolatey.yaml)

- Builds Windows binary (amd64)
- Packs and pushes `.nupkg` to [Chocolatey Community Repository](https://community.chocolatey.org/packages/ingitdb)

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`, `CHOCOLATEY_API_KEY`

#### 6. `publish-winget` (runs on Ubuntu, after `build-linux`)

Publishes to WinGet via fork + PR to `microsoft/winget-pkgs`.

**Config:** [`.github/goreleaser-publish-winget.yaml`](../.github/goreleaser-publish-winget.yaml)

- Builds Windows binary (amd64)
- Generates WinGet manifests and pushes to `ingitdb/winget-pkgs` fork
- Opens a PR to `microsoft/winget-pkgs`

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`

#### 7. `publish-scoop` (runs on Ubuntu, after `build-linux`)

Publishes the Scoop manifest to `ingitdb/scoop-bucket`.

**Config:** [`.github/goreleaser-publish-scoop.yaml`](../.github/goreleaser-publish-scoop.yaml)

- Builds Windows binary (amd64)
- Generates Scoop manifest JSON and pushes to [`ingitdb/scoop-bucket`](https://github.com/ingitdb/scoop-bucket)

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`

#### 8. `deploy-server` and `deploy-website` (after `build-linux`)

- `deploy-server` — Deploys to Google Cloud Run
- `deploy-website` — Deploys website to Firebase

### GoReleaser Configurations

| Config | Job | Purpose |
|--------|-----|---------|
| [`goreleaser-linux.yaml`](../.github/goreleaser-linux.yaml) | `build-linux` | Linux/Windows builds, GitHub release |
| [`goreleaser-homebrew.yaml`](../.github/goreleaser-homebrew.yaml) | `publish-homebrew` | macOS builds, signing, notarization, Homebrew Cask (macOS + Linux) |
| [`goreleaser-publish-aur.yaml`](../.github/goreleaser-publish-aur.yaml) | `publish-aur` | AUR PKGBUILD generation and push |
| [`goreleaser-publish-snap.yaml`](../.github/goreleaser-publish-snap.yaml) | `publish-snap` | Snapcraft build and publish |
| [`goreleaser-publish-chocolatey.yaml`](../.github/goreleaser-publish-chocolatey.yaml) | `publish-chocolatey` | Chocolatey package pack and push |
| [`goreleaser-publish-winget.yaml`](../.github/goreleaser-publish-winget.yaml) | `publish-winget` | WinGet manifest and PR to microsoft/winget-pkgs |
| [`goreleaser-publish-scoop.yaml`](../.github/goreleaser-publish-scoop.yaml) | `publish-scoop` | Scoop manifest push to ingitdb/scoop-bucket |

## Initial Setup

To enable all package manager distributions, follow the guides in [`docs/release/`](./release/README.md):

- **AUR** — Register package on Arch Linux User Repository
- **Snapcraft** — Reserve snap name and generate credentials
- **Homebrew** — Set up tap repository
- **Chocolatey** — Create account and generate API key
- **WinGet** — Fork microsoft/winget-pkgs to ingitdb org
- **Scoop** — Create ingitdb/scoop-bucket repository