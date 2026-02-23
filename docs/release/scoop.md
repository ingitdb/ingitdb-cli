# Scoop Setup

Scoop allows Windows developers to install ingitdb via `scoop install ingitdb`.

## 1. Create Scoop Bucket Repository

First, create a GitHub repository to hold the Scoop manifest:

1. Go to [https://github.com/new](https://github.com/new)
2. Create a new repository:
   - **Repository name:** `scoop-bucket`
   - **Owner:** ingitdb (organization)
   - **Visibility:** Public
   - **Initialize with:** Add a README
   - Click **Create repository**

2. Clone it locally:
   ```bash
   git clone https://github.com/ingitdb/scoop-bucket.git
   cd scoop-bucket
   ```

3. Create an initial bucket structure:
   ```bash
   mkdir -p bucket

   # Create a README for the bucket
   cat > README.md << 'EOF'
   # ingitdb Scoop Bucket

   Scoop bucket for ingitdb (Windows package manager).

   ## Installation

   ```powershell
   scoop bucket add ingitdb https://github.com/ingitdb/scoop-bucket
   scoop install ingitdb
   ```

   See [ingitdb.com](https://ingitdb.com) for documentation.
   EOF

   git add README.md
   git commit -m "chore: initialize scoop bucket"
   git push origin main
   ```

## 2. Generate SSH Key for Scoop Bucket

SSH is used to push manifest updates from CI without entering credentials each time.

```bash
# Generate ED25519 key (no passphrase, for CI use)
ssh-keygen -t ed25519 -C "goreleaser@ingitdb" -N "" -f ~/.ssh/scoop_key

# Output:
# Your identification has been saved in ~/.ssh/scoop_key
# Your public key has been saved in ~/.ssh/scoop_key.pub
```

## 3. Add SSH Deploy Key to Scoop Bucket

1. Print the public key:
   ```bash
   cat ~/.ssh/scoop_key.pub
   ```

2. Go to **scoop-bucket** repository → **Settings** → **Deploy keys**

3. Click **Add deploy key**
   - **Title:** goreleaser-scoop
   - **Key:** Paste the contents of `~/.ssh/scoop_key.pub`
   - **Allow write access:** ✓ (checked)
   - Click **Add key**

## 4. Store SSH Private Key as GitHub Secret

1. Print the raw private key:
   ```bash
   cat ~/.ssh/scoop_key
   ```

2. Go to your GitHub repo (ingitdb-cli) → **Settings** → **Secrets and variables** → **Actions**

3. Click **New repository secret**
   - **Name:** `SCOOP_BUCKET_SSH_KEY`
   - **Secret:** Paste the **entire** key content including the `-----BEGIN ...-----` and `-----END ...-----` lines

4. Click **Add secret**

5. Securely delete the local key files:
   ```bash
   rm ~/.ssh/scoop_key ~/.ssh/scoop_key.pub
   ```

**Note on CI/CD Implementation:**

The `publish-scoop` GitHub Actions job automatically:
1. Writes the SSH key from the secret to `~/.ssh/scoop_key` with proper permissions
2. Passes the file path to goreleaser, which expects `private_key` to be a path, not key content
3. Pushes the manifest to the scoop-bucket repository on each release

## 5. Verify Scoop Setup

After publishing your first release:

```powershell
# Add the bucket
scoop bucket add ingitdb https://github.com/ingitdb/scoop-bucket

# Search for ingitdb
scoop search ingitdb

# Install ingitdb
scoop install ingitdb

# Verify installation
ingitdb --version

# Uninstall
scoop uninstall ingitdb

# Remove the bucket (optional)
scoop bucket rm ingitdb
```

---

## Troubleshooting

### "Permission denied (publickey)"

- Verify SSH key is added as a deploy key to scoop-bucket repo with **write access**
- Verify the `SCOOP_BUCKET_SSH_KEY` secret contains the full raw key including `-----BEGIN ...-----` and `-----END ...-----` lines
- Ensure the scoop-bucket repo exists and is accessible
- Re-generate the SSH key pair if needed and update the deploy key and GitHub secret

---

## References

- [Scoop Package Manager](https://scoop.sh/)
- [GoReleaser: Scoop Publishing](https://goreleaser.com/customization/scoop/)
