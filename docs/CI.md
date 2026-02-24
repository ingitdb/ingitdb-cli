# Continuous integration for inGitDB

## Release Workflow

The release workflow is defined in [`.github/workflows/release.yml`](../.github/workflows/release.yml) and triggered manually via `workflow_dispatch`.

### Release Jobs

#### 1. `build-linux` (runs on Ubuntu)

Builds Linux binaries, creates the GitHub release, and uploads artifacts.

**Config:** [`.github/goreleaser-linux.yaml`](../.github/goreleaser-linux.yaml)

- Builds Linux binaries (amd64, arm64)
- Creates the GitHub release and uploads archives + checksums

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`

#### 2. `build-windows` (runs on Windows, after `build-linux`)

Builds Windows binaries, zip archives, and all Windows distributions.

**Config:** [`.github/goreleaser-windows.yaml`](../.github/goreleaser-windows.yaml)

- Builds Windows binary (amd64) and uploads Windows `zip` archive to GitHub release
- Generates WinGet manifests and pushes to WinGet repo
- Builds Scoop manifest and pushes it
- Packs and pushes Chocolatey package `.nupkg` to Community Repository

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`, `WINGET_GITHUB_TOKEN`, `CHOCOLATEY_API_KEY`

#### 3. `publish-homebrew` (runs on macOS, after `build-linux`)

Builds, signs, and notarizes macOS binaries, then publishes the Homebrew Cask covering both macOS and Linux.

**Config:** [`.github/goreleaser-homebrew.yaml`](../.github/goreleaser-homebrew.yaml)

- Builds Darwin binaries (amd64, arm64) and Linux binaries (amd64, arm64)
- Code-signs and notarizes macOS binaries with Apple credentials
- Uploads macOS archives to the existing GitHub release
- Publishes Homebrew Cask (macOS + Linux) to [`ingitdb/homebrew-cli`](https://github.com/ingitdb/homebrew-cli)

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`, `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`, `NOTARIZE_ISSUER_ID`, `NOTARIZE_KEY_ID`, `NOTARIZE_KEY`

#### 4. `publish-aur` (runs on Ubuntu, after `build-linux`)

Publishes the AUR package.

**Config:** [`.github/goreleaser-publish-aur.yaml`](../.github/goreleaser-publish-aur.yaml)

- Rebuilds Linux binaries (goreleaser produces reproducible archives, so checksums match the release)
- Generates PKGBUILD and .SRCINFO and pushes to AUR as `ingitdb-bin` via SSH

**Secrets used:** `AUR_SSH_PRIVATE_KEY` (raw ED25519 private key)

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

| Config                                                                    | Job                | Purpose                                                            |
| ------------------------------------------------------------------------- | ------------------ | ------------------------------------------------------------------ |
| [`goreleaser-linux.yaml`](../.github/goreleaser-linux.yaml)               | `build-linux`      | Linux builds, GitHub release                                       |
| [`goreleaser-windows.yaml`](../.github/goreleaser-windows.yaml)           | `build-windows`    | Windows builds, WinGet, Scoop, and Chocolatey packages             |
| [`goreleaser-homebrew.yaml`](../.github/goreleaser-homebrew.yaml)         | `publish-homebrew` | macOS builds, signing, notarization, Homebrew Cask (macOS + Linux) |
| [`goreleaser-publish-aur.yaml`](../.github/goreleaser-publish-aur.yaml)   | `publish-aur`      | AUR PKGBUILD generation and push                                   |
| [`goreleaser-publish-snap.yaml`](../.github/goreleaser-publish-snap.yaml) | `publish-snap`     | Snapcraft build and publish                                        |

## Initial Setup

To enable all package manager distributions, follow the guides in [`docs/release/`](./release/README.md):

- **AUR** — Register package on Arch Linux User Repository
- **Snapcraft** — Reserve snap name and generate credentials
- **Homebrew** — Set up tap repository
- **Chocolatey** — Create account and generate API key
- **WinGet** — Fork microsoft/winget-pkgs to ingitdb org
- **Scoop** — Create ingitdb/scoop-bucket repository
