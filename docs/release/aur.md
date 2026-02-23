# AUR (Arch Linux User Repository) Setup

AUR allows Arch Linux users to install ingitdb via `yay -S ingitdb-bin` or `paru -S ingitdb-bin`.

## 1. Create an AUR Account

1. Go to [https://aur.archlinux.org](https://aur.archlinux.org)
2. Click **Register** in the top-right corner
3. Choose a username (e.g., `ingitdb`) and set a strong password
4. Enter your email address
5. Accept the terms and click **Register**
6. Verify your email

## 2. Generate SSH Key for AUR

SSH is used to push updates to AUR without entering credentials each time.

```bash
# Generate ED25519 key (no passphrase, for CI use)
ssh-keygen -t ed25519 -C "goreleaser@ingitdb" -N "" -f /tmp/aur_key

# Output:
# Your identification has been saved in /tmp/aur_key
# Your public key has been saved in /tmp/aur_key.pub
```

## 3. Add SSH Public Key to AUR Account

1. Go to [https://aur.archlinux.org/account](https://aur.archlinux.org/account) (log in if needed)
2. Scroll to **SSH Public Keys** section
3. Paste the contents of `~/.ssh/aur_key.pub`:
   ```bash
   cat ~/.ssh/aur_key.pub
   ```
4. Click **Add**

Your SSH key is now registered with AUR.

**Note:** AUR may ask for "pacman verification" (running a `pacman` command to prove you're on Arch Linux). This is optional for automated releases via goreleaser — you only need it if you plan to manually maintain the AUR package. For CI/CD automation, SSH key authentication is sufficient.

## 4. Register the Package Name on AUR

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

## 5. Store SSH Private Key as GitHub Secret

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
1. Rebuilds Linux binaries from the same source and flags — goreleaser strips timestamps from archives, making them reproducible, so checksums match the actual GitHub release tarballs
2. Writes the SSH key from the secret to `~/.ssh/aur_key` with proper permissions (600)
3. Passes the file path to goreleaser, which expects `private_key` to be a path, not key content

## 6. Verify AUR Setup

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

## Troubleshooting

### "Permission denied (publickey)"

- Verify SSH key is added to AUR account: [aur.archlinux.org/account](https://aur.archlinux.org/account)
- Verify the `AUR_SSH_PRIVATE_KEY` secret contains the full raw key including `-----BEGIN ...-----` and `-----END ...-----` lines
- Re-generate the key and update the GitHub secret if needed

### "git: could not stat private_key: stat ***: file name too long"

This error occurs when goreleaser receives the SSH key content instead of a file path.

**Solution:** Ensure the `publish-aur` job in `release.yml` includes the "Setup SSH key for AUR" step before the goreleaser action. This step writes the secret to `~/.ssh/aur_key` and sets `AUR_SSH_PRIVATE_KEY` to that file path.

---

## References

- [AUR Submission Guidelines](https://wiki.archlinux.org/title/AUR_submission_guidelines)
- [GoReleaser: AUR Publishing](https://goreleaser.com/customization/aur/)
