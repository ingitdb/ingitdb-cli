# Continuous integration for inGitDB

## Release Workflow

The release workflow is defined in [`.github/workflows/release.yml`](../.github/workflows/release.yml) and triggered by **pushing a `vX.Y.Z` tag**:

```bash
git tag v1.32.0 && git push origin v1.32.0
```

The tag fixes the version (inGitDB is past v1, so we never want an automatic
major bump from a `feat!:` commit). The shared strongo/cicd release workflow
skips its auto-bump step on a tag ref and releases the exact tag pushed.

### Release Jobs

#### 1. `core` (runs on Ubuntu — the shared `strongo/cicd` reusable workflow)

Builds every OS and runs every publisher that works on a single ubuntu job.

**Config:** [`.goreleaser.yaml`](../.goreleaser.yaml) (the default root config)

- Builds Linux, macOS (darwin), and Windows binaries (CGO off, cross-compiled)
- Creates the GitHub release and uploads archives + checksums — **before** any
  publisher runs, so a publisher failure never blocks the (unsigned) macOS
  binaries from shipping
- Publishes the **AUR** package (`ingitdb-bin`)
- Publishes the **Homebrew cask** to [`ingitdb/homebrew-cli`](https://github.com/ingitdb/homebrew-cli)
- Publishes the **Scoop** manifest to `ingitdb/scoop-bucket`
- Generates the **WinGet** manifests and opens a cross-fork PR to `microsoft/winget-pkgs`

**Secrets used (forwarded by the shared workflow):** `INGITDB_GORELEASER_GITHUB_TOKEN` (→ `$GORELEASER_GITHUB_TOKEN`, Homebrew + Scoop), `WINGET_GITHUB_TOKEN`, `AUR_SSH_PRIVATE_KEY`

#### 2. `chocolatey` (runs on Windows, after `core`)

**Config:** [`.github/goreleaser-chocolatey.yaml`](../.github/goreleaser-chocolatey.yaml)

Kept separate because GoReleaser's chocolatey pipe shells out to the `choco`
CLI, which exists only on Windows runners. Packages the same Windows zip the
`core` job uploaded and pushes the `.nupkg` to the Chocolatey Community Repository.

**Secrets used:** `INGITDB_GORELEASER_GITHUB_TOKEN`, `CHOCOLATEY_API_KEY`

#### 3. `snap` (runs on Ubuntu, after `core`)

**Config:** [`.github/goreleaser-snap.yaml`](../.github/goreleaser-snap.yaml)

Kept separate because it needs `snapcraft` installed (not in the shared image)
and to keep a Snap Store outage from failing the GitHub release or the other
publishers. Builds its own Linux binaries and publishes to the
[Snapcraft Store](https://snapcraft.io/ingitdb).

**Secrets used:** `SNAPCRAFT_STORE_CREDENTIALS`

### GoReleaser Configurations

| Config                                                                        | Job          | Purpose                                                                                    |
| ----------------------------------------------------------------------------- | ------------ | ------------------------------------------------------------------------------------------ |
| [`.goreleaser.yaml`](../.goreleaser.yaml)                                     | `core`       | All-OS builds, GitHub release, AUR, Homebrew cask, Scoop, WinGet (one ubuntu job)          |
| [`goreleaser-chocolatey.yaml`](../.github/goreleaser-chocolatey.yaml)         | `chocolatey` | Chocolatey `.nupkg` (Windows runner — needs `choco`)                                       |
| [`goreleaser-snap.yaml`](../.github/goreleaser-snap.yaml)                     | `snap`       | Snapcraft build and publish (needs `snapcraft`; isolated)                                   |

> **macOS notarization** is not currently wired. The shipped darwin binaries are
> unsigned (as they were under the previous `build-macos` job). Enabling signed
> releases would need a `notarize` block plus forwarding the `MACOS_SIGN_*` /
> `NOTARIZE_*` secrets — a separate follow-up.

## Initial Setup

To enable all package manager distributions, follow the guides in [`docs/release/`](./release/README.md):

- **AUR** — Register package on Arch Linux User Repository
- **Snapcraft** — Reserve snap name and generate credentials
- **Homebrew** — Set up tap repository
- **Chocolatey** — Create account and generate API key
- **WinGet** — Fork microsoft/winget-pkgs to ingitdb org
- **Scoop** — Create ingitdb/scoop-bucket repository
