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

#### 2. `macos-releaser` (runs on macOS, after `build-linux`)

Builds, signs, and notarizes macOS binaries, then publishes the Homebrew Cask.

**Config:** [`.github/goreleaser-macos.yaml`](../.github/goreleaser-macos.yaml)

- Builds Darwin binaries (amd64, arm64)
- Code-signs and notarizes with Apple credentials
- Uploads macOS archives to the existing GitHub release
- Publishes macOS Cask to [`ingitdb/homebrew-cli`](https://github.com/ingitdb/homebrew-cli)

**Secrets used:** `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`, `NOTARIZE_ISSUER_ID`, `NOTARIZE_KEY_ID`, `NOTARIZE_KEY`

#### 3. `publish-aur` (runs on Ubuntu, after `build-linux`)

Publishes the AUR package using the pre-built Linux artifacts.

**Config:** [`.github/goreleaser-publish-aur.yaml`](../.github/goreleaser-publish-aur.yaml)

- Rebuilds Linux binaries (goreleaser produces reproducible archives, so checksums match the release)
- Generates PKGBUILD and .SRCINFO and pushes to AUR as `ingitdb-bin` via SSH

**Secrets used:** `AUR_SSH_PRIVATE_KEY` (raw ED25519 private key)

#### 4. `publish-homebrew` (runs on Ubuntu, after `build-linux`)

Publishes the Homebrew Formula (Linuxbrew) using the pre-built Linux artifacts.

**Config:** [`.github/goreleaser-publish-homebrew.yaml`](../.github/goreleaser-publish-homebrew.yaml)

- Rebuilds Linux binaries (goreleaser produces reproducible archives, so checksums match the release)
- Pushes Formula to [`ingitdb/homebrew-cli`](https://github.com/ingitdb/homebrew-cli)

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`

#### 5. `publish-snap` (runs on Ubuntu, after `build-linux`)

Builds and publishes the Snapcraft package.

**Config:** [`.github/goreleaser-publish-snap.yaml`](../.github/goreleaser-publish-snap.yaml)

- Builds Linux binaries (snap requires its own build inside the snap toolchain)
- Publishes to [Snapcraft Store](https://snapcraft.io/ingitdb)

**Secrets used:** `SNAPCRAFT_STORE_CREDENTIALS`

#### 6. `deploy-server` and `deploy-website` (after `build-linux`)

- `deploy-server` — Deploys to Google Cloud Run
- `deploy-website` — Deploys website to Firebase

### GoReleaser Configurations

| Config | Job | Purpose |
|--------|-----|---------|
| [`goreleaser-linux.yaml`](../.github/goreleaser-linux.yaml) | `build-linux` | Linux/Windows builds, GitHub release |
| [`goreleaser-macos.yaml`](../.github/goreleaser-macos.yaml) | `macos-releaser` | macOS builds, signing, notarization, Homebrew Cask |
| [`goreleaser-publish-aur.yaml`](../.github/goreleaser-publish-aur.yaml) | `publish-aur` | AUR PKGBUILD generation and push |
| [`goreleaser-publish-homebrew.yaml`](../.github/goreleaser-publish-homebrew.yaml) | `publish-homebrew` | Homebrew Formula push (Linuxbrew) |
| [`goreleaser-publish-snap.yaml`](../.github/goreleaser-publish-snap.yaml) | `publish-snap` | Snapcraft build and publish |

## Initial Setup

To enable all package manager distributions, follow [RELEASE_SETUP.md](./RELEASE_SETUP.md) for step-by-step instructions on:

- **AUR** — Register package on Arch Linux User Repository
- **Snapcraft** — Reserve snap name and generate credentials
- **Homebrew** — Set up tap repository