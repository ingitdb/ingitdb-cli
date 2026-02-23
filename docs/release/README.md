# Release Distribution Setup

This guide walks through setting up release distribution to multiple package managers: **Linux** (AUR, Snapcraft, Homebrew) and **Windows** (Chocolatey, WinGet, Scoop).

Each package manager has its own setup guide:

## Package Managers

### Linux Package Managers

- **[AUR (Arch Linux User Repository)](./aur.md)** - For Arch Linux users
- **[Snapcraft](./snapcraft.md)** - For universal Linux users
- **[Homebrew](./homebrew.md)** - For macOS Linuxbrew and macOS users

### Windows Package Managers

- **[Chocolatey](./chocolatey.md)** - Widest adoption, enterprise users
- **[WinGet](./winget.md)** - Microsoft's official package manager
- **[Scoop](./scoop.md)** - Popular with developers

---

## Summary Checklist

### Linux Packages

#### AUR
- [ ] Created AUR account at [aur.archlinux.org](https://aur.archlinux.org)
- [ ] Generated ED25519 SSH key: `ssh-keygen -t ed25519 -C "goreleaser@ingitdb" -N "" -f ~/.ssh/aur_key`
- [ ] Added public key to AUR account (SSH Public Keys section)
- [ ] Registered package by cloning, creating PKGBUILD + .SRCINFO, and pushing to master branch
- [ ] Stored private key as `AUR_SSH_PRIVATE_KEY` GitHub secret

#### Snapcraft
- [ ] Created Snapcraft account at [snapcraft.io/account/register](https://snapcraft.io/account/register)
- [ ] Reserved snap name `ingitdb` at [snapcraft.io/snaps](https://snapcraft.io/snaps)
- [ ] Generated login credentials: `snapcraft export-login /tmp/snap.login`
- [ ] Stored credentials as `SNAPCRAFT_STORE_CREDENTIALS` GitHub secret
- [ ] (After first release) Requested classic confinement at [forum.snapcraft.io](https://forum.snapcraft.io/c/snap-requests/49)

#### Homebrew Formula
- [ ] Created `homebrew-cli` repository in ingitdb organization
- [ ] Added README to homebrew-cli repo
- [ ] Verified goreleaser can push to the repo (uses `GITHUB_TOKEN` from workflow)

### Windows Packages

#### Chocolatey
- [ ] Created Chocolatey account at [community.chocolatey.org](https://community.chocolatey.org/account/Register)
- [ ] Generated API key from account settings
- [ ] Stored API key as `CHOCOLATEY_API_KEY` GitHub secret
- [ ] Awaited first package approval (1-2 days manual review)
- [ ] Verified installation: `choco install ingitdb` && `ingitdb --version`

#### WinGet
- [ ] No account setup required (uses GITHUB_TOKEN)
- [ ] Verified PR auto-submission to microsoft/winget-pkgs on first release
- [ ] Awaited PR approval and merge (1-3 days)
- [ ] Verified installation: `winget install ingitdb` && `ingitdb --version`

#### Scoop
- [ ] Created `scoop-bucket` repository in ingitdb organization
- [ ] Generated ED25519 SSH key: `ssh-keygen -t ed25519 -C "goreleaser@ingitdb" -N "" -f ~/.ssh/scoop_key`
- [ ] Added public key as deploy key to scoop-bucket repo (with write access)
- [ ] Stored private key as `SCOOP_BUCKET_SSH_KEY` GitHub secret
- [ ] Verified installation: `scoop bucket add ingitdb ...` && `scoop install ingitdb` && `ingitdb --version`

---

## Testing Releases

### Dry Run (No Upload)

Before making a real release, test the configs without uploading:

```bash
# Test linux-releaser config
goreleaser release --clean --config .github/goreleaser-linux.yaml --skip=upload

# Test macos-releaser config
goreleaser release --clean --config .github/goreleaser-macos.yaml --skip=upload

# Test Windows publishers (Chocolatey, WinGet, Scoop)
goreleaser release --clean --config .github/goreleaser-publish-chocolatey.yaml --skip=upload
goreleaser release --clean --config .github/goreleaser-publish-winget.yaml --skip=upload
goreleaser release --clean --config .github/goreleaser-publish-scoop.yaml --skip=upload
```

### Real Release

Once everything is verified:

1. Push a version tag:
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

2. Watch the release workflow:
   - Go to **Actions** → **Release**
   - All jobs should run: `build-linux`, `macos-releaser`, `publish-aur`, `publish-snap`, `publish-homebrew`, `publish-chocolatey`, `publish-winget`, `publish-scoop`
   - Check logs for successful package publishes

3. Verify packages appear:
   - **AUR:** [https://aur.archlinux.org/packages/ingitdb-bin](https://aur.archlinux.org/packages/ingitdb-bin)
   - **Snapcraft:** [https://snapcraft.io/ingitdb](https://snapcraft.io/ingitdb)
   - **Homebrew:** Clone `ingitdb/homebrew-cli` and check `Formula/ingitdb.rb`
   - **Chocolatey:** [https://community.chocolatey.org/packages/ingitdb](https://community.chocolatey.org/packages/ingitdb) (after moderation approval)
   - **WinGet:** Check PR in [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs/pulls) (auto-submitted)
   - **Scoop:** Clone `ingitdb/scoop-bucket` and check `bucket/ingitdb.json`

---

## Quick Links

- [GitHub Secrets Setup](https://github.com/ingitdb/ingitdb-cli/settings/secrets/actions)
- [Release Workflow](https://github.com/ingitdb/ingitdb-cli/actions/workflows/release.yml)
- [GoReleaser Documentation](https://goreleaser.com/)
