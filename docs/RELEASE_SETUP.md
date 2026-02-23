# Release Distribution Setup

This guide walks through setting up the three Linux package managers for automated releases: **AUR** (Arch Linux), **Snapcraft** (universal Linux), and **Homebrew Formula** (Linuxbrew).

---

## 1. AUR (Arch Linux User Repository) Setup

AUR allows Arch Linux users to install ingitdb via `yay -S ingitdb-bin` or `paru -S ingitdb-bin`.

### 1.1 Create an AUR Account

1. Go to [https://aur.archlinux.org](https://aur.archlinux.org)
2. Click **Register** in the top-right corner
3. Choose a username (e.g., `ingitdb`) and set a strong password
4. Enter your email address
5. Accept the terms and click **Register**
6. Verify your email

### 1.2 Generate SSH Key for AUR

SSH is used to push updates to AUR without entering credentials each time.

```bash
# Generate ED25519 key (no passphrase, for CI use)
ssh-keygen -t ed25519 -C "goreleaser@ingitdb" -N "" -f /tmp/aur_key

# Output:
# Your identification has been saved in /tmp/aur_key
# Your public key has been saved in /tmp/aur_key.pub
```

### 1.3 Add SSH Public Key to AUR Account

1. Go to [https://aur.archlinux.org/account](https://aur.archlinux.org/account) (log in if needed)
2. Scroll to **SSH Public Keys** section
3. Paste the contents of `~/.ssh/aur_key.pub`:
   ```bash
   cat ~/.ssh/aur_key.pub
   ```
4. Click **Add**

Your SSH key is now registered with AUR.

**Note:** AUR may ask for "pacman verification" (running a `pacman` command to prove you're on Arch Linux). This is optional for automated releases via goreleaser — you only need it if you plan to manually maintain the AUR package. For CI/CD automation, SSH key authentication is sufficient.

### 1.4 Register the Package Name on AUR

The first time you publish to AUR, you must register the package name by cloning and pushing to the AUR repo.

```bash
# Clone the empty AUR repo (this registers the package name)
# Use SSH with explicit key if needed:
export GIT_SSH_COMMAND="ssh -i ~/.ssh/aur_key"
git clone ssh://aur@aur.archlinux.org/ingitdb-bin.git
cd ingitdb-bin

# Create a minimal PKGBUILD
echo 'pkgname=ingitdb-bin
pkgver=0.0.0
pkgrel=1
pkgdesc="Placeholder"
url="https://ingitdb.com"
license=("MIT")
' > PKGBUILD

# Create .SRCINFO file (required by AUR)
# Note: makepkg may not be available on non-Arch systems
# If makepkg isn't available, manually create .SRCINFO:
cat > .SRCINFO << 'SRCINFO'
pkgbase = ingitdb-bin
	pkgdesc = Placeholder
	pkgver = 0.0.0
	pkgrel = 1
	url = https://ingitdb.com
	arch = x86_64
	arch = aarch64
	license = MIT

pkgname = ingitdb-bin
	pkgdesc = Placeholder
	url = https://ingitdb.com
SRCINFO

# Commit and push to master branch (AUR requires 'master', not 'main')
git add PKGBUILD .SRCINFO
git commit -m "initial commit"
git push origin HEAD:master

cd ..
rm -rf ingitdb-bin
```

After this, goreleaser will automatically update the PKGBUILD and .SRCINFO on each release.

### 1.5 Store SSH Private Key as GitHub Secret

1. Print the raw private key:
   ```bash
   cat ~/.ssh/aur_key
   ```

2. Go to your GitHub repo → **Settings** → **Secrets and variables** → **Actions**

3. Click **New repository secret**
   - **Name:** `AUR_SSH_PRIVATE_KEY`
   - **Secret:** Paste the **entire** key content including the `-----BEGIN ...-----` and `-----END ...-----` lines

4. Click **Add secret**

5. Securely delete the local key files:
   ```bash
   rm ~/.ssh/aur_key ~/.ssh/aur_key.pub
   ```

**Note on CI/CD Implementation:**

The `publish-aur` GitHub Actions job automatically:
1. Downloads the pre-built Linux artifacts from the `build-linux` job (no rebuild)
2. Writes the SSH key from the secret to `~/.ssh/aur_key` with proper permissions (600)
3. Passes the file path to goreleaser, which expects `private_key` to be a path, not key content
4. Runs goreleaser with `--skip=build,archive` so checksums in the PKGBUILD match the actual GitHub release tarballs

### 1.6 Verify AUR Setup

Check that goreleaser can access the repo:

```bash
# Test SSH connection (may fail with "access denied" message, which is normal)
ssh -i /path/to/aur_key aur@aur.archlinux.org
# Expected: "git-upload-pack 'ingitdb-bin.git'" (then connection closes)

# Or test with git (if you saved the key)
GIT_SSH_COMMAND="ssh -i /path/to/aur_key" git ls-remote ssh://aur@aur.archlinux.org/ingitdb-bin.git
# Expected: shows refs
```

---

## 2. Snapcraft Setup

Snapcraft allows universal Linux users to install ingitdb via `snap install ingitdb`.

### 2.1 Create a Snapcraft Account

1. Go to [https://snapcraft.io/account/register](https://snapcraft.io/account/register)
2. Sign up with:
   - Email address
   - Password
   - Username
3. Verify your email
4. Log in to [https://snapcraft.io](https://snapcraft.io)

### 2.2 Reserve the Snap Name

1. Go to [https://snapcraft.io/snaps](https://snapcraft.io/snaps)
2. Click **Create a new snap** or **Submit a snap**
3. Register the name: `ingitdb`
4. Fill in basic info:
   - **Summary:** "A CLI for a schema-validated, AI-native database backed by a Git repository"
   - **Description:** Same as in goreleaser config
   - **License:** MIT
5. Click **Save** (you can publish the snap itself later)

### 2.3 Generate Snapcraft Login Credentials

This creates a login token for CI to use.

```bash
# Install snapcraft (if not already installed)
sudo snap install snapcraft --classic

# Log in and export credentials
snapcraft export-login /tmp/snap.login
# (When prompted, enter your Snapcraft username/password)

# This creates /tmp/snap.login with encrypted credentials
```

### 2.4 Store Credentials as GitHub Secret

1. Read the credentials file:
   ```bash
   cat /tmp/snap.login
   # Output: base64-encoded credentials
   ```

2. Go to your GitHub repo → **Settings** → **Secrets and variables** → **Actions**

3. Click **New repository secret**
   - **Name:** `SNAPCRAFT_STORE_CREDENTIALS`
   - **Secret:** Paste the entire contents of `/tmp/snap.login`

4. Click **Add secret**

5. Clean up:
   ```bash
   rm /tmp/snap.login
   ```

### 2.5 Request Classic Confinement (One-Time)

By default, snaps run in confined mode (restricted permissions). Ingitdb needs classic confinement to access the user's filesystem.

> **Note:** This is a one-time manual approval. Until approved, use `confinement: devmode` and `grade: devel` in goreleaser-linux.yaml for testing.

Once you've published a snap with `confinement: classic`, the Snap Store team reviews it:

1. Go to [https://forum.snapcraft.io/c/snap-requests/49](https://forum.snapcraft.io/c/snap-requests/49)

2. Click **New Topic** and include:
   ```
   Title: Classic confinement request for ingitdb

   Snap name: ingitdb

   Reason: ingitdb is a CLI database tool that needs full filesystem access to:
   - Read/write Git repositories
   - Access user home directory
   - Run git commands (requires /bin/sh, /usr/bin/git)

   Publishing link: https://snapcraft.io/ingitdb
   ```

3. The Snap Store team will review and approve within 1-2 days

### 2.6 Verify Snapcraft Setup

After publishing your first release:

```bash
# Search for the snap
snap search ingitdb

# Install the snap (from devmode release while waiting for approval)
sudo snap install ingitdb --devmode  # until confinement is approved

# Test it works
ingitdb --version
```

---

## 3. Homebrew Formula Setup

Homebrew Formula allows Linux users via Linuxbrew to install ingitdb via `brew install ingitdb`.

### 3.1 Prepare the homebrew-cli Tap Repository

The `ingitdb/homebrew-cli` tap repository must exist and be writable by the GitHub token.

#### If you don't have a homebrew tap yet:

1. Create a public GitHub repo named `homebrew-cli` in the `ingitdb` organization
   - Go to [https://github.com/new](https://github.com/new)
   - **Repository name:** `homebrew-cli`
   - **Owner:** ingitdb (organization)
   - **Visibility:** Public
   - **Initialize with:** (leave empty, goreleaser will create the formula)
   - Click **Create repository**

2. Clone it locally:
   ```bash
   git clone https://github.com/ingitdb/homebrew-cli.git
   cd homebrew-cli
   ```

3. Add a README:
   ```bash
   cat > README.md << 'EOF'
   # ingitdb Homebrew Tap

   Homebrew tap for ingitdb (macOS Cask and Linuxbrew Formula).

   ## Installation

   ### macOS (Cask)
   ```bash
   brew tap ingitdb/cli
   brew install ingitdb
   ```

   ### Linux (Linuxbrew / Formula)
   ```bash
   brew tap ingitdb/cli
   brew install ingitdb
   ```

   See [ingitdb.com](https://ingitdb.com) for documentation.
   EOF

   git add README.md
   git commit -m "chore: add tap documentation"
   git push origin main
   ```

#### If you already have a homebrew tap:

Ensure it has write permissions for the GitHub token being used (usually `GITHUB_TOKEN` from the release workflow).

### 3.2 Verify Homebrew Formula Setup

After the first release, goreleaser will create `Formula/ingitdb.rb` in the homebrew-cli repo:

```bash
# Check the formula was created
git clone https://github.com/ingitdb/homebrew-cli.git
ls Formula/

# You should see: ingitdb.rb
```

### 3.3 Test Homebrew Installation (Optional)

After a release, test the formula on Linux (Linuxbrew):

```bash
# Install Linuxbrew (if not already installed)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Add Homebrew to PATH
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"

# Install ingitdb
brew tap ingitdb/cli
brew install ingitdb

# Verify
ingitdb --version
```

---

## Summary Checklist

### AUR
- [ ] Created AUR account at [aur.archlinux.org](https://aur.archlinux.org)
- [ ] Generated ED25519 SSH key: `ssh-keygen -t ed25519 -C "goreleaser@ingitdb" -N "" -f ~/.ssh/aur_key`
- [ ] Added public key to AUR account (SSH Public Keys section)
- [ ] Registered package by cloning, creating PKGBUILD + .SRCINFO, and pushing to master branch:
  - `git clone ssh://aur@aur.archlinux.org/ingitdb-bin.git`
  - Create PKGBUILD and .SRCINFO files
  - `git commit -m "initial commit"`
  - `git push origin HEAD:master` (note: must be master, not main)
- [ ] Stored private key as `AUR_SSH_PRIVATE_KEY` GitHub secret:
  - `cat ~/.ssh/aur_key` (copy the full output including BEGIN/END lines)
  - Paste in GitHub → Settings → Secrets → AUR_SSH_PRIVATE_KEY

### Snapcraft
- [ ] Created Snapcraft account at [snapcraft.io/account/register](https://snapcraft.io/account/register)
- [ ] Reserved snap name `ingitdb` at [snapcraft.io/snaps](https://snapcraft.io/snaps)
- [ ] Generated login credentials: `snapcraft export-login /tmp/snap.login`
- [ ] Stored credentials as `SNAPCRAFT_STORE_CREDENTIALS` GitHub secret (entire file contents)
- [ ] (After first release) Requested classic confinement at [forum.snapcraft.io](https://forum.snapcraft.io/c/snap-requests/49)

### Homebrew Formula
- [ ] Created `homebrew-cli` repository in ingitdb organization
- [ ] Added README to homebrew-cli repo
- [ ] Verified goreleaser can push to the repo (uses `GITHUB_TOKEN` from workflow)

---

## Testing Releases

### Dry Run (No Upload)

Before making a real release, test the configs without uploading:

```bash
# Test linux-releaser config
goreleaser release --clean --config .github/goreleaser-linux.yaml --skip=upload

# Test macos-releaser config
goreleaser release --clean --config .github/goreleaser-macos.yaml --skip=upload
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
   - Both `macos-releaser` and `linux-releaser` should run
   - Check logs for successful AUR, Snapcraft, and Homebrew publishes

3. Verify packages appear:
   - **AUR:** [https://aur.archlinux.org/packages/ingitdb-bin](https://aur.archlinux.org/packages/ingitdb-bin)
   - **Snapcraft:** [https://snapcraft.io/ingitdb](https://snapcraft.io/ingitdb)
   - **Homebrew:** Clone `ingitdb/homebrew-cli` and check `Formula/ingitdb.rb`

---

## Troubleshooting

### AUR: "Permission denied (publickey)"

- Verify SSH key is added to AUR account: [aur.archlinux.org/account](https://aur.archlinux.org/account)
- Verify the `AUR_SSH_PRIVATE_KEY` secret contains the full raw key including `-----BEGIN ...-----` and `-----END ...-----` lines
- Re-generate the key and update the GitHub secret if needed

### AUR: "git: could not stat private_key: stat ***: file name too long"

This error occurs when goreleaser receives the SSH key content instead of a file path.

**Solution:** Ensure the `publish-aur` job in `release.yml` includes the "Setup SSH key for AUR" step before the goreleaser action. This step writes the secret to `~/.ssh/aur_key` and sets `AUR_SSH_PRIVATE_KEY` to that file path.

### Snapcraft: "Invalid credentials"

- Regenerate credentials: `snapcraft export-login /tmp/snap.login`
- Update the GitHub secret with new contents
- Ensure you're logged in to [snapcraft.io](https://snapcraft.io) before export

### Homebrew: "Repository not found"

- Verify `homebrew-cli` repository exists in `ingitdb` organization
- Check `GITHUB_TOKEN` in the workflow has `contents: write` permission
- Ensure the repo is public

---

## References

- [AUR Submission Guidelines](https://wiki.archlinux.org/title/AUR_submission_guidelines)
- [Snapcraft: Publishing to the Snap Store](https://snapcraft.io/docs/release-to-the-snap-store)
- [Homebrew Tap Documentation](https://docs.brew.sh/Taps)
- [GoReleaser: AUR Publishing](https://goreleaser.com/customization/aur/)
- [GoReleaser: Snapcraft Publishing](https://goreleaser.com/customization/snapcraft/)
- [GoReleaser: Homebrew Publishing](https://goreleaser.com/customization/homebrew/)
