# 🔁 Continuous integration for inGitDB

## 📂 Release Workflow

The release workflow is defined in [`.github/workflows/release.yml`](../.github/workflows/release.yml) and triggered on version tags (`v*`).

### Release Jobs

#### 1. `macos-releaser` (runs on macOS)

Builds and publishes for macOS.

**Config:** [`.github/goreleaser-macos.yaml`](../.github/goreleaser-macos.yaml)

- **Builds:** Darwin (macOS) binaries for amd64 and arm64
- **Signing & Notarization:** Code signs and notarizes macOS binaries with Apple credentials
- **Distribution:** Publishes macOS Cask to [`ingitdb/homebrew-cli`](https://github.com/ingitdb/homebrew-cli) Homebrew tap

**Environment variables:**
- `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD` — Apple code signing certificate
- `NOTARIZE_ISSUER_ID`, `NOTARIZE_KEY_ID`, `NOTARIZE_KEY` — Apple notarization credentials

#### 2. `linux-releaser` (runs on Ubuntu)

Builds and publishes for Linux, Windows, and Snap.

**Config:** [`.github/goreleaser-linux.yaml`](../.github/goreleaser-linux.yaml)

- **Builds:**
  - Linux binaries (amd64, arm64) for GitHub releases
  - Windows binaries (amd64) for GitHub releases
  - Linux binaries (amd64, arm64) for Snapcraft
- **Distribution:**
  - Publishes Linux and Windows archives to GitHub releases
  - Publishes Homebrew Formula to [`ingitdb/homebrew-cli`](https://github.com/ingitdb/homebrew-cli) (Linuxbrew)
  - Publishes to AUR as `ingitdb-bin`
  - Publishes snap to [Snapcraft Store](https://snapcraft.io/ingitdb)

**Environment variables:**
- `AUR_SSH_PRIVATE_KEY` — SSH private key for AUR publishing
- `SNAPCRAFT_STORE_CREDENTIALS` — Snapcraft Store login token

### Deployment

After both `macos-releaser` and `linux-releaser` complete successfully, the following jobs run in parallel:

- `deploy-server` — Deploys to Google Cloud Run
- `deploy-website` — Deploys website to Firebase

### GoReleaser Configurations

ingitdb uses separate GoReleaser configs for clarity:

| Config | Purpose |
|--------|---------|
| [`goreleaser-macos.yaml`](../.github/goreleaser-macos.yaml) | macOS builds, signing, notarization, and Cask |
| [`goreleaser-linux.yaml`](../.github/goreleaser-linux.yaml) | Linux/Windows builds, Homebrew Formula, AUR, Snap |